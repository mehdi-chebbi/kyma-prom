package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// InitConfig holds configuration for Gitea initialization
type InitConfig struct {
	GiteaURL      string
	AdminUser     string
	AdminPassword string
	AdminEmail    string
	TokenName     string
}

// InitClient handles Gitea admin setup and token generation
type InitClient struct {
	config     *InitConfig
	httpClient *http.Client
	logger     *logrus.Logger
}

// NewInitClient creates a new initialization client
func NewInitClient(config *InitConfig, logger *logrus.Logger) *InitClient {
	return &InitClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// EnsureAdminToken creates admin user if needed and returns an API token.
// Retries up to 5 times with 10s backoff to survive the postStart lifecycle
// hook delay (~15s) where the admin user doesn't exist yet.
func (c *InitClient) EnsureAdminToken() (string, error) {
	c.logger.Info("Initializing Gitea admin and token...")

	// Wait for Gitea to be ready
	if err := c.waitForGitea(); err != nil {
		return "", fmt.Errorf("gitea not ready: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		if attempt > 1 {
			c.logger.WithField("attempt", attempt).Info("Retrying token generation...")
			time.Sleep(10 * time.Second)
		}

		// Try to create admin user (will fail if exists, that's OK)
		c.createAdminUser()

		// Generate or get existing token
		token, err := c.generateToken()
		if err == nil {
			c.logger.WithField("token_prefix", token[:8]+"...").Info("Gitea token ready")
			return token, nil
		}
		lastErr = err
		c.logger.WithError(err).WithField("attempt", attempt).Warn("Token generation failed, will retry")
	}

	return "", fmt.Errorf("failed after 5 attempts: %w", lastErr)
}

func (c *InitClient) waitForGitea() error {
	c.logger.Info("Waiting for Gitea to be ready...")

	for i := 0; i < 30; i++ {
		resp, err := c.httpClient.Get(c.config.GiteaURL + "/api/healthz")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			c.logger.Info("Gitea is ready")
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("gitea did not become ready in time")
}

func (c *InitClient) createAdminUser() {
	c.logger.Info("Ensuring admin user exists...")

	body := map[string]interface{}{
		"username":             c.config.AdminUser,
		"password":             c.config.AdminPassword,
		"email":                c.config.AdminEmail,
		"must_change_password": false,
		"visibility":           "public",
	}

	jsonData, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", c.config.GiteaURL+"/api/v1/admin/users", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.WithError(err).Debug("Could not create admin user (may already exist)")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		c.logger.Info("Admin user created")
	} else {
		c.logger.Debug("Admin user already exists or creation skipped")
	}
}

func (c *InitClient) generateToken() (string, error) {
	c.logger.Info("Generating API token...")

	// First, try to delete existing token by finding its ID
	c.deleteExistingToken()

	// Create new token
	body := map[string]interface{}{
		"name":   c.config.TokenName,
		"scopes": []string{"all"},
	}

	jsonData, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST",
		fmt.Sprintf("%s/api/v1/users/%s/tokens", c.config.GiteaURL, c.config.AdminUser),
		bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.AdminUser, c.config.AdminPassword)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create token: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		SHA1 string `json:"sha1"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if result.SHA1 == "" {
		return "", fmt.Errorf("empty token returned")
	}

	return result.SHA1, nil
}

func (c *InitClient) deleteExistingToken() {
	// List all tokens to find the one with our name
	req, _ := http.NewRequest("GET",
		fmt.Sprintf("%s/api/v1/users/%s/tokens", c.config.GiteaURL, c.config.AdminUser),
		nil)
	req.SetBasicAuth(c.config.AdminUser, c.config.AdminPassword)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.WithError(err).Debug("Failed to list tokens")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.WithField("status", resp.StatusCode).Debug("Failed to list tokens")
		return
	}

	body, _ := io.ReadAll(resp.Body)

	var tokens []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}

	if err := json.Unmarshal(body, &tokens); err != nil {
		c.logger.WithError(err).Debug("Failed to parse tokens")
		return
	}

	// Find and delete token with matching name
	for _, token := range tokens {
		if token.Name == c.config.TokenName {
			c.logger.WithField("token_id", token.ID).Debug("Deleting existing token")

			delReq, _ := http.NewRequest("DELETE",
				fmt.Sprintf("%s/api/v1/users/%s/tokens/%d", c.config.GiteaURL, c.config.AdminUser, token.ID),
				nil)
			delReq.SetBasicAuth(c.config.AdminUser, c.config.AdminPassword)

			delResp, err := c.httpClient.Do(delReq)
			if err != nil {
				c.logger.WithError(err).Debug("Failed to delete token")
				return
			}
			delResp.Body.Close()

			if delResp.StatusCode == http.StatusNoContent || delResp.StatusCode == http.StatusOK {
				c.logger.Info("Deleted existing token")
			}
			return
		}
	}
}
