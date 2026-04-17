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

// Client represents a Gitea API client
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     *logrus.Logger
}

// Repository represents a Gitea repository
type Repository struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	Private       bool      `json:"private"`
	Fork          bool      `json:"fork"`
	HTMLURL       string    `json:"html_url"`
	SSHURL        string    `json:"ssh_url"`
	CloneURL      string    `json:"clone_url"`
	DefaultBranch string    `json:"default_branch"`
	Language      string    `json:"language"`
	Stars         int       `json:"stars_count"`
	Forks         int       `json:"forks_count"`
	Size          int       `json:"size"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Owner         Owner     `json:"owner"`
}

// Owner represents the repository owner
type Owner struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// NewClient creates a new Gitea API client
func NewClient(baseURL, token string, logger *logrus.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// doRequest performs an HTTP request to Gitea API
func (c *Client) doRequest(method, path string) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add Authorization header
	if c.token == "" {
		return nil, fmt.Errorf("GITEA_TOKEN is required")
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	c.logger.WithFields(logrus.Fields{
		"method":        method,
		"url":           url,
		"authenticated": c.token != "" && c.token != "fake-token-for-testing",
	}).Debug("Making Gitea API request")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   string(body),
		}).Error("Gitea API error")
		return nil, fmt.Errorf("gitea API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	return body, nil
}

// doRequestWithBody makes an HTTP request with a JSON body
func (c *Client) doRequestWithBody(method, path string, body interface{}, result interface{}) error {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
	}

	url := fmt.Sprintf("%s%s", c.baseURL, path)

	var httpReq *http.Request
	if reqBody != nil {
		httpReq, err = http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	} else {
		httpReq, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.token == "" {
		return fmt.Errorf("GITEA_TOKEN is required")
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.WithFields(logrus.Fields{
		"method": method,
		"url":    url,
	}).Debug("Making Gitea API request with body")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   string(respBody),
		}).Error("Gitea API error")
		return fmt.Errorf("gitea API error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// ListRepositories lists all repositories accessible by the admin token
func (c *Client) ListRepositories() ([]*Repository, error) {
	// Use /repos/search endpoint to get all repositories
	body, err := c.doRequest("GET", "/repos/search?limit=1000")
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []*Repository `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	c.logger.WithField("count", len(result.Data)).Info("Fetched repositories from Gitea")
	return result.Data, nil
}

// GetRepository gets a specific repository by owner and name
func (c *Client) GetRepository(owner, name string) (*Repository, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, name)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var repo Repository
	if err := json.Unmarshal(body, &repo); err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}

	return &repo, nil
}

// SearchRepositories searches repositories by query
func (c *Client) SearchRepositories(query string, limit int) ([]*Repository, error) {
	if limit <= 0 {
		limit = 50
	}

	path := fmt.Sprintf("/repos/search?q=%s&limit=%d", query, limit)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []*Repository `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data, nil
}

// DeleteRepository deletes a repository
func (c *Client) DeleteRepository(owner, name string) error {
	path := fmt.Sprintf("/repos/%s/%s", owner, name)
	_, err := c.doRequest("DELETE", path)
	if err != nil {
		return err
	}

	c.logger.WithFields(logrus.Fields{
		"owner": owner,
		"name":  name,
	}).Info("Repository deleted from Gitea")
	return nil
}

// UpdateRepository updates a repository
func (c *Client) UpdateRepository(owner, name string, updates map[string]interface{}) (*Repository, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, name)

	// Convert updates to JSON
	jsonData, err := json.Marshal(updates)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updates: %w", err)
	}

	// Make PATCH request
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	if c.token != "" && c.token != "fake-token-for-testing" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
	req.Header.Set("Content-Type", "application/json")

	c.logger.WithFields(logrus.Fields{
		"owner":   owner,
		"name":    name,
		"updates": updates,
	}).Debug("Updating repository in Gitea")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   string(body),
		}).Error("Gitea API error")
		return nil, fmt.Errorf("gitea API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	var repo Repository
	if err := json.Unmarshal(body, &repo); err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}

	c.logger.Info("Repository updated successfully")
	return &repo, nil
}

// HealthCheck checks if Gitea API is accessible (without authentication)
func (c *Client) HealthCheck() error {
	url := fmt.Sprintf("%s/api/v1/version", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Don't add Authorization header for public endpoint
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	c.logger.Debug("Gitea health check successful")
	return nil
}

// CreateRepositoryRequest represents a repository creation request
type CreateRepositoryRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Private       bool   `json:"private"`
	AutoInit      bool   `json:"auto_init"`
	Gitignores    string `json:"gitignores,omitempty"`
	License       string `json:"license,omitempty"`
	Readme        string `json:"readme,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	TrustModel    string `json:"trust_model,omitempty"`
}

// CreateRepository creates a new repository
func (c *Client) CreateRepository(owner string, req *CreateRepositoryRequest) (*Repository, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/orgs/%s/repos", owner)
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token == "" {
		return nil, fmt.Errorf("GITEA_TOKEN is required")
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.WithFields(logrus.Fields{
		"owner": owner,
		"name":  req.Name,
	}).Info("Creating repository in Gitea")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitea API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	var repo Repository
	if err := json.Unmarshal(body, &repo); err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}

	c.logger.Info("Repository created successfully")
	return &repo, nil
}

// MigrateRepositoryRequest represents a repository migration request
// privee yaatik key wekahaw (mehdi testi use case), usecase with key : taz
type MigrateRepositoryRequest struct {
	CloneAddr    string `json:"clone_addr"`
	RepoName     string `json:"repo_name"`
	RepoOwner    string `json:"repo_owner,omitempty"`
	Mirror       bool   `json:"mirror"`
	Private      bool   `json:"private"`
	Description  string `json:"description,omitempty"`
	Wiki         bool   `json:"wiki"`
	Milestones   bool   `json:"milestones"`
	Labels       bool   `json:"labels"`
	Issues       bool   `json:"issues"`
	PullRequests bool   `json:"pull_requests"`
	Releases     bool   `json:"releases"`
	AuthUsername string `json:"auth_username,omitempty"`
	AuthPassword string `json:"auth_password,omitempty"`
	AuthToken    string `json:"auth_token,omitempty"`
	Service      string `json:"service,omitempty"` // github, gitlab, gitea, gogs
}

// MigrateRepository migrates a repository from an external source
func (c *Client) MigrateRepository(req *MigrateRepositoryRequest) (*Repository, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/repos/migrate", c.baseURL)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token == "" {
		return nil, fmt.Errorf("GITEA_TOKEN is required")
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.WithFields(logrus.Fields{
		"clone_addr": req.CloneAddr,
		"repo_name":  req.RepoName,
		"mirror":     req.Mirror,
		"service":    req.Service,
	}).Info("Migrating repository to Gitea")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitea API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	var repo Repository
	if err := json.Unmarshal(body, &repo); err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}

	c.logger.Info("Repository migrated successfully")
	return &repo, nil
}

// ForkRepository forks a repository
func (c *Client) ForkRepository(owner, repo, organization string) (*Repository, error) {
	path := fmt.Sprintf("/repos/%s/%s/forks", owner, repo)
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	reqBody := map[string]interface{}{}
	if organization != "" {
		reqBody["organization"] = organization
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token == "" {
		return nil, fmt.Errorf("GITEA_TOKEN is required")
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.WithFields(logrus.Fields{
		"owner":        owner,
		"repo":         repo,
		"organization": organization,
	}).Info("Forking repository in Gitea")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitea API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	var forkedRepo Repository
	if err := json.Unmarshal(body, &forkedRepo); err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}

	c.logger.Info("Repository forked successfully")
	return &forkedRepo, nil
}

// Branch represents a git branch
type Branch struct {
	Name              string     `json:"name"`
	Commit            CommitMeta `json:"commit"`
	Protected         bool       `json:"protected"`
	RequiredApprovals int        `json:"required_approvals"`
}

// CommitMeta represents commit metadata
type CommitMeta struct {
	SHA     string    `json:"sha"`
	URL     string    `json:"url"`
	Created time.Time `json:"created"`
}

// Commit represents a git commit
type Commit struct {
	SHA       string       `json:"sha"`
	URL       string       `json:"url"`
	Commit    CommitDetail `json:"commit"`
	Author    GitUser      `json:"author"`
	Committer GitUser      `json:"committer"`
}

// CommitDetail represents commit details
type CommitDetail struct {
	Message   string  `json:"message"`
	Tree      TreeRef `json:"tree"`
	Author    GitUser `json:"author"`
	Committer GitUser `json:"committer"`
}

// TreeRef represents a git tree reference
type TreeRef struct {
	SHA string `json:"sha"`
	URL string `json:"url"`
}

// GitUser represents a git user
type GitUser struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Date  time.Time `json:"date"`
}

// Tag represents a git tag
type Tag struct {
	Name       string     `json:"name"`
	Message    string     `json:"message"`
	Commit     CommitMeta `json:"commit"`
	ZipballURL string     `json:"zipball_url"`
	TarballURL string     `json:"tarball_url"`
}

// ListBranches lists all branches in a repository
func (c *Client) ListBranches(owner, repo string) ([]*Branch, error) {
	path := fmt.Sprintf("/repos/%s/%s/branches", owner, repo)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var branches []*Branch
	if err := json.Unmarshal(body, &branches); err != nil {
		return nil, fmt.Errorf("failed to parse branches: %w", err)
	}

	return branches, nil
}

// GetBranch gets a specific branch
func (c *Client) GetBranch(owner, repo, branch string) (*Branch, error) {
	path := fmt.Sprintf("/repos/%s/%s/branches/%s", owner, repo, branch)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var branchInfo Branch
	if err := json.Unmarshal(body, &branchInfo); err != nil {
		return nil, fmt.Errorf("failed to parse branch: %w", err)
	}

	return &branchInfo, nil
}

// CreateBranch creates a new branch
func (c *Client) CreateBranch(owner, repo, branchName, oldBranchName string) (*Branch, error) {
	path := fmt.Sprintf("/repos/%s/%s/branches", owner, repo)
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	reqBody := map[string]string{
		"new_branch_name": branchName,
		"old_branch_name": oldBranchName,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token == "" {
		return nil, fmt.Errorf("GITEA_TOKEN is required")
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitea API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	var branch Branch
	if err := json.Unmarshal(body, &branch); err != nil {
		return nil, fmt.Errorf("failed to parse branch: %w", err)
	}

	return &branch, nil
}

// DeleteBranch deletes a branch
func (c *Client) DeleteBranch(owner, repo, branch string) error {
	path := fmt.Sprintf("/repos/%s/%s/branches/%s", owner, repo, branch)
	_, err := c.doRequest("DELETE", path)
	return err
}

// ListCommits lists commits in a repository
func (c *Client) ListCommits(owner, repo string, opts *CommitListOptions) ([]*Commit, error) {
	path := fmt.Sprintf("/repos/%s/%s/commits", owner, repo)

	if opts != nil {
		params := []string{}
		if opts.SHA != "" {
			params = append(params, fmt.Sprintf("sha=%s", opts.SHA))
		}
		if opts.Path != "" {
			params = append(params, fmt.Sprintf("path=%s", opts.Path))
		}
		if len(params) > 0 {
			path += "?" + string(bytes.Join([][]byte{[]byte(params[0])}, []byte("&"))[0])
		}
	}

	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var commits []*Commit
	if err := json.Unmarshal(body, &commits); err != nil {
		return nil, fmt.Errorf("failed to parse commits: %w", err)
	}

	return commits, nil
}

// CommitListOptions represents options for listing commits
type CommitListOptions struct {
	SHA  string
	Path string
}

// GetCommit gets a specific commit
func (c *Client) GetCommit(owner, repo, sha string) (*Commit, error) {
	path := fmt.Sprintf("/repos/%s/%s/git/commits/%s", owner, repo, sha)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var commit Commit
	if err := json.Unmarshal(body, &commit); err != nil {
		return nil, fmt.Errorf("failed to parse commit: %w", err)
	}

	return &commit, nil
}

// ListTags lists all tags in a repository
func (c *Client) ListTags(owner, repo string) ([]*Tag, error) {
	path := fmt.Sprintf("/repos/%s/%s/tags", owner, repo)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var tags []*Tag
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}

	return tags, nil
}

// CreateTag creates a new tag
func (c *Client) CreateTag(owner, repo, tagName, target, message string) (*Tag, error) {
	path := fmt.Sprintf("/repos/%s/%s/tags", owner, repo)
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	reqBody := map[string]string{
		"tag_name": tagName,
		"target":   target,
		"message":  message,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token == "" {
		return nil, fmt.Errorf("GITEA_TOKEN is required")
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitea API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	var tag Tag
	if err := json.Unmarshal(body, &tag); err != nil {
		return nil, fmt.Errorf("failed to parse tag: %w", err)
	}

	return &tag, nil
}

// DeleteTag deletes a tag
func (c *Client) DeleteTag(owner, repo, tag string) error {
	path := fmt.Sprintf("/repos/%s/%s/tags/%s", owner, repo, tag)
	_, err := c.doRequest("DELETE", path)
	return err
}

// ========================
// Admin Webhook Management
// ========================

// AdminWebhook represents a Gitea system-level webhook
type AdminWebhook struct {
	ID     int64             `json:"id"`
	Type   string            `json:"type"`
	Active bool              `json:"active"`
	Config map[string]string `json:"config"`
	Events []string          `json:"events"`
}

// ListAdminWebhooks lists all system-level webhooks (admin API)
func (c *Client) ListAdminWebhooks() ([]*AdminWebhook, error) {
	body, err := c.doRequest("GET", "/admin/hooks")
	if err != nil {
		return nil, fmt.Errorf("failed to list admin webhooks: %w", err)
	}

	var hooks []*AdminWebhook
	if err := json.Unmarshal(body, &hooks); err != nil {
		return nil, fmt.Errorf("failed to parse admin webhooks: %w", err)
	}

	return hooks, nil
}

// CreateAdminWebhook creates a system-level webhook via admin API
func (c *Client) CreateAdminWebhook(targetURL, secret string, events []string) (*AdminWebhook, error) {
	payload := map[string]interface{}{
		"type":   "gitea",
		"active": true,
		"events": events,
		"config": map[string]string{
			"url":          targetURL,
			"content_type": "json",
			"secret":       secret,
		},
	}

	url := fmt.Sprintf("%s/api/v1/admin/hooks", c.baseURL)
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token == "" {
		return nil, fmt.Errorf("GITEA_TOKEN is required")
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to create admin webhook: %s (status: %d)", string(body), resp.StatusCode)
	}

	var hook AdminWebhook
	if err := json.Unmarshal(body, &hook); err != nil {
		return nil, fmt.Errorf("failed to parse webhook response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"webhook_id": hook.ID,
		"target_url": targetURL,
		"events":     events,
	}).Info("Created admin webhook in Gitea")

	return &hook, nil
}

// EnsureWebhook idempotently ensures a system webhook exists pointing to targetURL
func (c *Client) EnsureWebhook(targetURL, secret string) error {
	hooks, err := c.ListAdminWebhooks()
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", err)
	}

	// Check if a webhook with this target URL already exists
	for _, hook := range hooks {
		if hook.Config["url"] == targetURL && hook.Active {
			c.logger.WithField("webhook_id", hook.ID).Info("Webhook already exists")
			return nil
		}
	}

	// Create the webhook
	_, err = c.CreateAdminWebhook(targetURL, secret, []string{"repository"})
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}

	return nil
}

// ============================
// Repository Collaborators
// ============================

// AddCollaborator adds a user as collaborator on a specific repository.
// Permission can be "read", "write", or "admin".
// PUT /api/v1/repos/{owner}/{repo}/collaborators/{collaborator}
func (c *Client) AddCollaborator(owner, repo, username, permission string) error {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/collaborators/%s", owner, repo, username)
	body := map[string]string{
		"permission": permission,
	}

	if err := c.doRequestWithBody("PUT", path, body, nil); err != nil {
		return fmt.Errorf("failed to add collaborator %s to %s/%s: %w", username, owner, repo, err)
	}

	c.logger.WithFields(logrus.Fields{
		"owner":      owner,
		"repo":       repo,
		"username":   username,
		"permission": permission,
	}).Info("Added collaborator to repository")

	return nil
}

// RemoveCollaborator removes a collaborator from a repository.
// DELETE /api/v1/repos/{owner}/{repo}/collaborators/{collaborator}
func (c *Client) RemoveCollaborator(owner, repo, username string) error {
	path := fmt.Sprintf("/repos/%s/%s/collaborators/%s", owner, repo, username)
	_, err := c.doRequest("DELETE", path)
	if err != nil {
		return fmt.Errorf("failed to remove collaborator %s from %s/%s: %w", username, owner, repo, err)
	}

	c.logger.WithFields(logrus.Fields{
		"owner":    owner,
		"repo":     repo,
		"username": username,
	}).Info("Removed collaborator from repository")

	return nil
}
