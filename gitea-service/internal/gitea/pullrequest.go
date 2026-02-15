package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PullRequest represents a pull request in Gitea
type PullRequest struct {
	ID        int64         `json:"id"`
	Number    int64         `json:"number"`
	State     string        `json:"state"` // open, closed, merged
	Title     string        `json:"title"`
	Body      string        `json:"body"`
	User      User          `json:"user"`
	Head      PRBranchInfo  `json:"head"`
	Base      PRBranchInfo  `json:"base"`
	Mergeable bool          `json:"mergeable"`
	Merged    bool          `json:"merged"`
	MergedAt  *time.Time    `json:"merged_at"`
	MergedBy  *User         `json:"merged_by"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	ClosedAt  *time.Time    `json:"closed_at"`
	DueDate   *time.Time    `json:"due_date"`
	Assignees []User        `json:"assignees"`
	Labels    []Label       `json:"labels"`
	Milestone *Milestone    `json:"milestone"`
	Comments  int           `json:"comments"`
	Additions int           `json:"additions"`
	Deletions int           `json:"deletions"`
	ChangedFiles int        `json:"changed_files"`
	HTMLURL   string        `json:"html_url"`
	DiffURL   string        `json:"diff_url"`
	PatchURL  string        `json:"patch_url"`
}

// PRBranchInfo represents branch information in a pull request
type PRBranchInfo struct {
	Label string     `json:"label"`
	Ref   string     `json:"ref"`
	SHA   string     `json:"sha"`
	Repo  Repository `json:"repo"`
}

// User represents a Gitea user (for PR context)
type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// Label represents a label
type Label struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Milestone represents a milestone
type Milestone struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	State       string     `json:"state"`
	DueOn       *time.Time `json:"due_on"`
}

// PRComment represents a comment on a pull request
type PRComment struct {
	ID        int64     `json:"id"`
	User      User      `json:"user"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	HTMLURL   string    `json:"html_url"`
}

// PRReview represents a review on a pull request
type PRReview struct {
	ID          int64     `json:"id"`
	User        User      `json:"user"`
	Body        string    `json:"body"`
	State       string    `json:"state"` // APPROVED, REQUEST_CHANGES, COMMENT, PENDING
	CommitID    string    `json:"commit_id"`
	SubmittedAt time.Time `json:"submitted_at"`
	HTMLURL     string    `json:"html_url"`
}

// PRFile represents a file changed in a pull request
type PRFile struct {
	Filename    string `json:"filename"`
	Status      string `json:"status"` // added, modified, deleted, renamed
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
	Changes     int    `json:"changes"`
	PatchURL    string `json:"patch_url"`
	RawURL      string `json:"raw_url"`
	ContentsURL string `json:"contents_url"`
}

// CreatePullRequestRequest represents a request to create a PR
type CreatePullRequestRequest struct {
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Head      string    `json:"head"`      // branch name
	Base      string    `json:"base"`      // target branch
	Assignees []string  `json:"assignees,omitempty"`
	Labels    []int64   `json:"labels,omitempty"`
	Milestone int64     `json:"milestone,omitempty"`
	DueDate   *time.Time `json:"due_date,omitempty"`
}

// UpdatePullRequestRequest represents a request to update a PR
type UpdatePullRequestRequest struct {
	Title     *string    `json:"title,omitempty"`
	Body      *string    `json:"body,omitempty"`
	State     *string    `json:"state,omitempty"` // open, closed
	Base      *string    `json:"base,omitempty"`
	Assignees []string   `json:"assignees,omitempty"`
	Labels    []int64    `json:"labels,omitempty"`
	Milestone *int64     `json:"milestone,omitempty"`
	DueDate   *time.Time `json:"due_date,omitempty"`
}

// MergePullRequestRequest represents a request to merge a PR
type MergePullRequestRequest struct {
	Do                string `json:"Do"` // merge, rebase, rebase-merge, squash
	MergeCommitID     string `json:"MergeCommitID,omitempty"`
	MergeTitleField   string `json:"MergeTitleField,omitempty"`
	MergeMessageField string `json:"MergeMessageField,omitempty"`
	DeleteBranchAfterMerge bool `json:"delete_branch_after_merge,omitempty"`
	ForceMerge        bool   `json:"force_merge,omitempty"`
}

// CreatePRCommentRequest represents a request to create a PR comment
type CreatePRCommentRequest struct {
	Body string `json:"body"`
}

// CreatePRReviewRequest represents a request to create a PR review
type CreatePRReviewRequest struct {
	Body     string `json:"body,omitempty"`
	Event    string `json:"event"` // APPROVE, REQUEST_CHANGES, COMMENT
	CommitID string `json:"commit_id,omitempty"`
}

// ListPullRequests lists all pull requests in a repository
func (c *Client) ListPullRequests(owner, repo string, state string, page, limit int) ([]*PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", owner, repo)

	// Add query parameters
	params := fmt.Sprintf("?state=%s&page=%d&limit=%d", state, page, limit)
	path += params

	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var prs []*PullRequest
	if err := json.Unmarshal(body, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse pull requests: %w", err)
	}

	c.logger.WithField("count", len(prs)).Info("Fetched pull requests from Gitea")
	return prs, nil
}

// GetPullRequest gets a specific pull request by number
func (c *Client) GetPullRequest(owner, repo string, number int64) (*PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var pr PullRequest
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse pull request: %w", err)
	}

	return &pr, nil
}

// CreatePullRequest creates a new pull request
func (c *Client) CreatePullRequest(owner, repo string, req *CreatePullRequestRequest) (*PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", owner, repo)
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" && c.token != "fake-token-for-testing" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.WithField("title", req.Title).Info("Creating pull request in Gitea")

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

	var pr PullRequest
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse pull request: %w", err)
	}

	c.logger.Info("Pull request created successfully")
	return &pr, nil
}

// UpdatePullRequest updates an existing pull request
func (c *Client) UpdatePullRequest(owner, repo string, number int64, req *UpdatePullRequestRequest) (*PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" && c.token != "fake-token-for-testing" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.WithField("number", number).Info("Updating pull request in Gitea")

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

	var pr PullRequest
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse pull request: %w", err)
	}

	c.logger.Info("Pull request updated successfully")
	return &pr, nil
}

// MergePullRequest merges a pull request
func (c *Client) MergePullRequest(owner, repo string, number int64, req *MergePullRequestRequest) error {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, number)
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" && c.token != "fake-token-for-testing" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.WithField("number", number).Info("Merging pull request in Gitea")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gitea API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	c.logger.Info("Pull request merged successfully")
	return nil
}

// IsPullRequestMerged checks if a pull request is merged
func (c *Client) IsPullRequestMerged(owner, repo string, number int64) (bool, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, number)
	_, err := c.doRequest("GET", path)
	if err != nil {
		// 404 means not merged
		return false, nil
	}
	return true, nil
}

// ListPRComments lists all comments on a pull request
func (c *Client) ListPRComments(owner, repo string, number int64) ([]*PRComment, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, number)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var comments []*PRComment
	if err := json.Unmarshal(body, &comments); err != nil {
		return nil, fmt.Errorf("failed to parse comments: %w", err)
	}

	return comments, nil
}

// CreatePRComment creates a comment on a pull request
func (c *Client) CreatePRComment(owner, repo string, number int64, req *CreatePRCommentRequest) (*PRComment, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" && c.token != "fake-token-for-testing" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
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

	var comment PRComment
	if err := json.Unmarshal(body, &comment); err != nil {
		return nil, fmt.Errorf("failed to parse comment: %w", err)
	}

	return &comment, nil
}

// ListPRReviews lists all reviews on a pull request
func (c *Client) ListPRReviews(owner, repo string, number int64) ([]*PRReview, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo, number)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var reviews []*PRReview
	if err := json.Unmarshal(body, &reviews); err != nil {
		return nil, fmt.Errorf("failed to parse reviews: %w", err)
	}

	return reviews, nil
}

// CreatePRReview creates a review on a pull request
func (c *Client) CreatePRReview(owner, repo string, number int64, req *CreatePRReviewRequest) (*PRReview, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo, number)
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" && c.token != "fake-token-for-testing" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
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

	var review PRReview
	if err := json.Unmarshal(body, &review); err != nil {
		return nil, fmt.Errorf("failed to parse review: %w", err)
	}

	return &review, nil
}

// ListPRFiles lists files changed in a pull request
func (c *Client) ListPRFiles(owner, repo string, number int64) ([]*PRFile, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files", owner, repo, number)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var files []*PRFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("failed to parse files: %w", err)
	}

	return files, nil
}

// GetPRDiff gets the diff of a pull request
func (c *Client) GetPRDiff(owner, repo string, number int64) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d.diff", owner, repo, number)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// GetPRPatch gets the patch of a pull request
func (c *Client) GetPRPatch(owner, repo string, number int64) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d.patch", owner, repo, number)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
