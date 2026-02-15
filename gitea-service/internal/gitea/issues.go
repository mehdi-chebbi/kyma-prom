package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// IssueUser represents user in issue context (alias to User from pullrequest.go)
type IssueUser = User

// IssueLabel represents label in issue context (alias to Label from pullrequest.go)
type IssueLabel = Label

// IssueMilestone represents milestone in issue context (alias to Milestone from pullrequest.go)
type IssueMilestone = Milestone

// Issue represents a Gitea issue
type Issue struct {
	ID          int64            `json:"id"`
	Number      int64            `json:"number"`
	Title       string           `json:"title"`
	Body        string           `json:"body"`
	State       string           `json:"state"` // open, closed
	User        *IssueUser       `json:"user"`
	Labels      []*IssueLabel    `json:"labels"`
	Milestone   *IssueMilestone  `json:"milestone"`
	Assignees   []*IssueUser     `json:"assignees"`
	Comments    int              `json:"comments"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	ClosedAt    *time.Time       `json:"closed_at"`
	DueDate     *time.Time       `json:"due_date"`
	PullRequest *struct {
		Merged bool `json:"merged"`
	} `json:"pull_request,omitempty"`
	Repository *struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
	} `json:"repository,omitempty"`
}

// CreateIssueRequest represents request to create an issue
type CreateIssueRequest struct {
	Title     string   `json:"title"`
	Body      string   `json:"body,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
	Labels    []int64  `json:"labels,omitempty"`
	Milestone int64    `json:"milestone,omitempty"`
	Closed    bool     `json:"closed,omitempty"`
	DueDate   *time.Time `json:"due_date,omitempty"`
}

// UpdateIssueRequest represents request to update an issue
type UpdateIssueRequest struct {
	Title     *string    `json:"title,omitempty"`
	Body      *string    `json:"body,omitempty"`
	Assignees []string   `json:"assignees,omitempty"`
	Labels    []int64    `json:"labels,omitempty"`
	Milestone *int64     `json:"milestone,omitempty"`
	State     *string    `json:"state,omitempty"` // open, closed
	DueDate   *time.Time `json:"due_date,omitempty"`
}

// IssueComment represents a comment on an issue
type IssueComment struct {
	ID        int64      `json:"id"`
	HTMLURL   string     `json:"html_url"`
	IssueURL  string     `json:"issue_url"`
	Body      string     `json:"body"`
	User      *IssueUser `json:"user"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// CreateIssueCommentRequest represents request to create an issue comment
type CreateIssueCommentRequest struct {
	Body string `json:"body"`
}

// CreateLabelRequest represents request to create a label
type CreateLabelRequest struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
}

// CreateMilestoneRequest represents request to create a milestone
type CreateMilestoneRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	DueDate     *time.Time `json:"due_on,omitempty"`
	State       string     `json:"state,omitempty"` // open, closed
}

// ========================
// Issue Operations
// ========================

// ListIssues lists issues in a repository
func (c *Client) ListIssues(owner, repo, state string, labels []string, page, limit int) ([]*Issue, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues?state=%s&page=%d&limit=%d",
		c.baseURL, owner, repo, state, page, limit)

	if len(labels) > 0 {
		for _, label := range labels {
			url += fmt.Sprintf("&labels=%s", label)
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list issues: %s - %s", resp.Status, string(body))
	}

	var issues []*Issue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("failed to decode issues: %w", err)
	}

	return issues, nil
}

// GetIssue gets a specific issue
func (c *Client) GetIssue(owner, repo string, number int64) (*Issue, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d", c.baseURL, owner, repo, number)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get issue: %s - %s", resp.Status, string(body))
	}

	var issue Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("failed to decode issue: %w", err)
	}

	return &issue, nil
}

// CreateIssue creates a new issue
func (c *Client) CreateIssue(owner, repo string, req *CreateIssueRequest) (*Issue, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues", c.baseURL, owner, repo)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "token "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create issue: %s - %s", resp.Status, string(body))
	}

	var issue Issue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("failed to decode issue: %w", err)
	}

	return &issue, nil
}

// UpdateIssue updates an existing issue
func (c *Client) UpdateIssue(owner, repo string, number int64, req *UpdateIssueRequest) (*Issue, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d", c.baseURL, owner, repo, number)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "token "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update issue: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to update issue: %s - %s", resp.Status, string(body))
	}

	var issue Issue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("failed to decode issue: %w", err)
	}

	return &issue, nil
}

// ========================
// Issue Comment Operations
// ========================

// ListIssueComments lists comments on an issue
func (c *Client) ListIssueComments(owner, repo string, number int64) ([]*IssueComment, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d/comments", c.baseURL, owner, repo, number)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list comments: %s - %s", resp.Status, string(body))
	}

	var comments []*IssueComment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, fmt.Errorf("failed to decode comments: %w", err)
	}

	return comments, nil
}

// CreateIssueComment creates a comment on an issue
func (c *Client) CreateIssueComment(owner, repo string, number int64, req *CreateIssueCommentRequest) (*IssueComment, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d/comments", c.baseURL, owner, repo, number)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "token "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create comment: %s - %s", resp.Status, string(body))
	}

	var comment IssueComment
	if err := json.Unmarshal(body, &comment); err != nil {
		return nil, fmt.Errorf("failed to decode comment: %w", err)
	}

	return &comment, nil
}

// DeleteIssueComment deletes a comment from an issue
func (c *Client) DeleteIssueComment(owner, repo string, commentID int64) error {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/comments/%d", c.baseURL, owner, repo, commentID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete comment: %s - %s", resp.Status, string(body))
	}

	return nil
}

// ========================
// Label Operations
// ========================

// ListLabels lists labels in a repository
func (c *Client) ListLabels(owner, repo string, page, limit int) ([]*Label, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/labels?page=%d&limit=%d",
		c.baseURL, owner, repo, page, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list labels: %s - %s", resp.Status, string(body))
	}

	var labels []*Label
	if err := json.NewDecoder(resp.Body).Decode(&labels); err != nil {
		return nil, fmt.Errorf("failed to decode labels: %w", err)
	}

	return labels, nil
}

// CreateLabel creates a new label
func (c *Client) CreateLabel(owner, repo string, req *CreateLabelRequest) (*Label, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/labels", c.baseURL, owner, repo)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "token "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create label: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create label: %s - %s", resp.Status, string(body))
	}

	var label Label
	if err := json.Unmarshal(body, &label); err != nil {
		return nil, fmt.Errorf("failed to decode label: %w", err)
	}

	return &label, nil
}

// DeleteLabel deletes a label
func (c *Client) DeleteLabel(owner, repo string, labelID int64) error {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/labels/%d", c.baseURL, owner, repo, labelID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete label: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete label: %s - %s", resp.Status, string(body))
	}

	return nil
}

// ========================
// Milestone Operations
// ========================

// ListMilestones lists milestones in a repository
func (c *Client) ListMilestones(owner, repo, state string, page, limit int) ([]*Milestone, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/milestones?state=%s&page=%d&limit=%d",
		c.baseURL, owner, repo, state, page, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list milestones: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list milestones: %s - %s", resp.Status, string(body))
	}

	var milestones []*Milestone
	if err := json.NewDecoder(resp.Body).Decode(&milestones); err != nil {
		return nil, fmt.Errorf("failed to decode milestones: %w", err)
	}

	return milestones, nil
}

// CreateMilestone creates a new milestone
func (c *Client) CreateMilestone(owner, repo string, req *CreateMilestoneRequest) (*Milestone, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/milestones", c.baseURL, owner, repo)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "token "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create milestone: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create milestone: %s - %s", resp.Status, string(body))
	}

	var milestone Milestone
	if err := json.Unmarshal(body, &milestone); err != nil {
		return nil, fmt.Errorf("failed to decode milestone: %w", err)
	}

	return &milestone, nil
}

// DeleteMilestone deletes a milestone
func (c *Client) DeleteMilestone(owner, repo string, milestoneID int64) error {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/milestones/%d", c.baseURL, owner, repo, milestoneID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete milestone: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete milestone: %s - %s", resp.Status, string(body))
	}

	return nil
}
