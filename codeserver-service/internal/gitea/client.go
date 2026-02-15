package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/devplatform/codeserver-service/internal/config"
	"github.com/sirupsen/logrus"
)

// Client calls gitea-service GraphQL API
type Client struct {
	giteaServiceURL string
	giteaURL        string
	giteaToken      string
	httpClient      *http.Client
	logger          *logrus.Logger
}

// Repository represents a Gitea repository
type Repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	Owner         string `json:"owner"`
	CloneURL      string `json:"cloneUrl"`
	SSHURL        string `json:"sshUrl"`
	HTMLURL       string `json:"htmlUrl"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"defaultBranch"`
}

type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// NewClient creates a new Gitea client
func NewClient(cfg *config.Config, logger *logrus.Logger) *Client {
	return &Client{
		giteaServiceURL: cfg.GiteaServiceURL,
		giteaURL:        cfg.GiteaURL,
		giteaToken:      cfg.GiteaToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// GetUserRepositories gets repositories accessible by the user via gitea-service
func (c *Client) GetUserRepositories(ctx context.Context, token string) ([]*Repository, error) {
	query := `query {
		myRepositories {
			id
			name
			fullName
			owner {
				login
			}
			cloneUrl
			sshUrl
			htmlUrl
			private
			defaultBranch
		}
	}`

	data, err := c.doGraphQL(ctx, token, query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		MyRepositories []struct {
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			FullName      string `json:"fullName"`
			Owner         struct {
				Login string `json:"login"`
			} `json:"owner"`
			CloneURL      string `json:"cloneUrl"`
			SSHURL        string `json:"sshUrl"`
			HTMLURL       string `json:"htmlUrl"`
			Private       bool   `json:"private"`
			DefaultBranch string `json:"defaultBranch"`
		} `json:"myRepositories"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	repos := make([]*Repository, len(result.MyRepositories))
	for i, r := range result.MyRepositories {
		repos[i] = &Repository{
			ID:            r.ID,
			Name:          r.Name,
			FullName:      r.FullName,
			Owner:         r.Owner.Login,
			CloneURL:      r.CloneURL,
			SSHURL:        r.SSHURL,
			HTMLURL:       r.HTMLURL,
			Private:       r.Private,
			DefaultBranch: r.DefaultBranch,
		}
	}

	return repos, nil
}

// GetRepository gets a specific repository via gitea-service
func (c *Client) GetRepository(ctx context.Context, token, owner, name string) (*Repository, error) {
	query := `query GetRepository($owner: String!, $name: String!) {
		getRepository(owner: $owner, name: $name) {
			id
			name
			fullName
			owner {
				login
			}
			cloneUrl
			sshUrl
			htmlUrl
			private
			defaultBranch
		}
	}`

	variables := map[string]interface{}{
		"owner": owner,
		"name":  name,
	}

	data, err := c.doGraphQL(ctx, token, query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		GetRepository *struct {
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			FullName      string `json:"fullName"`
			Owner         struct {
				Login string `json:"login"`
			} `json:"owner"`
			CloneURL      string `json:"cloneUrl"`
			SSHURL        string `json:"sshUrl"`
			HTMLURL       string `json:"htmlUrl"`
			Private       bool   `json:"private"`
			DefaultBranch string `json:"defaultBranch"`
		} `json:"getRepository"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	if result.GetRepository == nil {
		return nil, fmt.Errorf("repository not found: %s/%s", owner, name)
	}

	return &Repository{
		ID:            result.GetRepository.ID,
		Name:          result.GetRepository.Name,
		FullName:      result.GetRepository.FullName,
		Owner:         result.GetRepository.Owner.Login,
		CloneURL:      result.GetRepository.CloneURL,
		SSHURL:        result.GetRepository.SSHURL,
		HTMLURL:       result.GetRepository.HTMLURL,
		Private:       result.GetRepository.Private,
		DefaultBranch: result.GetRepository.DefaultBranch,
	}, nil
}

// ValidateRepoAccess checks if user can access the repository
func (c *Client) ValidateRepoAccess(ctx context.Context, token, owner, repoName string) (bool, error) {
	repo, err := c.GetRepository(ctx, token, owner, repoName)
	if err != nil {
		return false, err
	}
	return repo != nil, nil
}

// GetRepoCloneURL returns the clone URL with embedded token for private repos
func (c *Client) GetRepoCloneURL(ctx context.Context, token, owner, repoName string) (string, error) {
	repo, err := c.GetRepository(ctx, token, owner, repoName)
	if err != nil {
		return "", err
	}

	// For private repos, embed the token in the git URL
	// Format: https://token:TOKEN@gitea.example.com/owner/repo.git
	if repo.Private && c.giteaToken != "" {
		parsed, err := url.Parse(repo.CloneURL)
		if err != nil {
			c.logger.WithError(err).Warn("Failed to parse clone URL, using original")
			return repo.CloneURL, nil
		}
		parsed.User = url.UserPassword("token", c.giteaToken)
		return parsed.String(), nil
	}

	return repo.CloneURL, nil
}

// HealthCheck checks if gitea-service is accessible
func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.giteaServiceURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitea-service health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gitea-service unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) doGraphQL(ctx context.Context, token, query string, variables map[string]interface{}) (json.RawMessage, error) {
	reqBody := graphqlRequest{
		Query:     query,
		Variables: variables,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.giteaServiceURL+"/graphql", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	c.logger.WithFields(logrus.Fields{
		"url": c.giteaServiceURL + "/graphql",
	}).Debug("Calling gitea-service GraphQL")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitea-service request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitea-service returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return nil, err
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
	}

	return gqlResp.Data, nil
}
