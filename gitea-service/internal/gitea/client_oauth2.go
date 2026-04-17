package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/devplatform/gitea-service/internal/auth"
	"github.com/sirupsen/logrus"
)

// OAuth2Client is a Gitea client that uses OAuth2 JWT token passthrough
// Instead of generating its own tokens, it passes the Keycloak JWT to Gitea
type OAuth2Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *logrus.Logger
}

// NewOAuth2Client creates a new OAuth2-enabled Gitea client
func NewOAuth2Client(baseURL string, logger *logrus.Logger) *OAuth2Client {
	return &OAuth2Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// makeRequest creates an HTTP request with the JWT token from context
func (c *OAuth2Client) makeRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	// Extract user info from context (set by auth middleware)
	// Gitea is configured for reverse proxy authentication
	userID := auth.GetUserFromContext(ctx)
	email := auth.GetEmailFromContext(ctx)

	if userID != "" {
		// Pass user headers to Gitea for reverse proxy authentication
		req.Header.Set("X-Forwarded-User", userID)
		if email != "" {
			req.Header.Set("X-Forwarded-Email", email)
		}
		// Optionally set full name if available
		req.Header.Set("X-Forwarded-Full-Name", userID)
		c.logger.WithFields(logrus.Fields{
			"path": path,
			"user": userID,
		}).Debug("Passing user headers to Gitea for reverse proxy auth")
	} else {
		c.logger.Warn("No user in context - request may fail if Gitea requires auth")
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// doRequest executes an HTTP request and handles the response
func (c *OAuth2Client) doRequest(req *http.Request, result interface{}) error {
	c.logger.WithFields(logrus.Fields{
		"method": req.Method,
		"url":    req.URL.String(),
	}).Debug("Executing Gitea API request")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   string(bodyBytes),
		}).Error("Gitea API error response")
		return fmt.Errorf("gitea API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w, body: %s", err, string(bodyBytes))
		}
	}

	return nil
}

// ListRepositories lists repositories with OAuth2 token
func (c *OAuth2Client) ListRepositories(ctx context.Context, page, limit int) ([]*Repository, error) {
	path := fmt.Sprintf("/api/v1/user/repos?page=%d&limit=%d", page, limit)
	req, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var repos []*Repository
	if err := c.doRequest(req, &repos); err != nil {
		return nil, err
	}

	return repos, nil
}

// GetRepository gets a specific repository
func (c *OAuth2Client) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s", owner, repo)
	req, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var repository Repository
	if err := c.doRequest(req, &repository); err != nil {
		return nil, err
	}

	return &repository, nil
}

// CreateRepository creates a new repository
func (c *OAuth2Client) CreateRepository(ctx context.Context, input *CreateRepositoryRequest) (*Repository, error) {
	path := "/api/v1/user/repos"
	req, err := c.makeRequest(ctx, "POST", path, input)
	if err != nil {
		return nil, err
	}

	var repo Repository
	if err := c.doRequest(req, &repo); err != nil {
		return nil, err
	}

	return &repo, nil
}

// ListIssues lists issues for a repository
func (c *OAuth2Client) ListIssues(ctx context.Context, owner, repo, state string, labels []string, page, limit int) ([]*Issue, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/issues?state=%s&page=%d&limit=%d", owner, repo, state, page, limit)

	// Add labels to query if provided
	for _, label := range labels {
		path += fmt.Sprintf("&labels=%s", label)
	}

	req, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var issues []*Issue
	if err := c.doRequest(req, &issues); err != nil {
		return nil, err
	}

	return issues, nil
}

// CreateIssue creates a new issue
func (c *OAuth2Client) CreateIssue(ctx context.Context, owner, repo string, input *CreateIssueRequest) (*Issue, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/issues", owner, repo)
	req, err := c.makeRequest(ctx, "POST", path, input)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := c.doRequest(req, &issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

// ListPullRequests lists pull requests for a repository
func (c *OAuth2Client) ListPullRequests(ctx context.Context, owner, repo, state string, page, limit int) ([]*PullRequest, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/pulls?state=%s&page=%d&limit=%d", owner, repo, state, page, limit)
	req, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var prs []*PullRequest
	if err := c.doRequest(req, &prs); err != nil {
		return nil, err
	}

	return prs, nil
}

// CreatePullRequest creates a new pull request
func (c *OAuth2Client) CreatePullRequest(ctx context.Context, owner, repo string, input *CreatePullRequestRequest) (*PullRequest, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner, repo)
	req, err := c.makeRequest(ctx, "POST", path, input)
	if err != nil {
		return nil, err
	}

	var pr PullRequest
	if err := c.doRequest(req, &pr); err != nil {
		return nil, err
	}

	return &pr, nil
}

// HealthCheck checks if Gitea is accessible
func (c *OAuth2Client) HealthCheck(ctx context.Context) error {
	req, err := c.makeRequest(ctx, "GET", "/api/healthz", nil)
	if err != nil {
		return err
	}

	// Don't need to parse response for health check
	return c.doRequest(req, nil)
}
