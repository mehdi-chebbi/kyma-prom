package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// will add controller that will call this function every x time we have new/update member,repo in ldap and every day for repo sync
// GiteaUser represents a user in Gitea
type GiteaUser struct {
	ID                int64  `json:"id"`
	UserName          string `json:"username"`
	LoginName         string `json:"login_name"`
	FullName          string `json:"full_name"`
	Email             string `json:"email"`
	AvatarURL         string `json:"avatar_url"`
	Language          string `json:"language"`
	IsAdmin           bool   `json:"is_admin"`
	LastLogin         string `json:"last_login"`
	Created           string `json:"created"`
	Restricted        bool   `json:"restricted"`
	Active            bool   `json:"active"`
	ProhibitLogin     bool   `json:"prohibit_login"`
	Location          string `json:"location"`
	Website           string `json:"website"`
	Description       string `json:"description"`
	Visibility        string `json:"visibility"`
	FollowersCount    int    `json:"followers_count"`
	FollowingCount    int    `json:"following_count"`
	StarredReposCount int    `json:"starred_repos_count"`
}

// CreateUserRequest represents request to create a new user in Gitea
type CreateUserRequest struct {
	Email              string `json:"email"`
	FullName           string `json:"full_name"`
	LoginName          string `json:"login_name"`
	MustChangePassword bool   `json:"must_change_password"`
	Password           string `json:"password"`
	SendNotify         bool   `json:"send_notify"`
	SourceID           int64  `json:"source_id"`
	Username           string `json:"username"`
	Visibility         string `json:"visibility,omitempty"`
}

// UpdateUserRequest represents request to update a user in Gitea
type UpdateUserRequest struct {
	Active                  *bool   `json:"active,omitempty"`
	Admin                   *bool   `json:"admin,omitempty"`
	AllowCreateOrganization *bool   `json:"allow_create_organization,omitempty"`
	AllowGitHook            *bool   `json:"allow_git_hook,omitempty"`
	AllowImportLocal        *bool   `json:"allow_import_local,omitempty"`
	Description             *string `json:"description,omitempty"`
	Email                   *string `json:"email,omitempty"`
	FullName                *string `json:"full_name,omitempty"`
	Location                *string `json:"location,omitempty"`
	LoginName               *string `json:"login_name,omitempty"`
	MaxRepoCreation         *int    `json:"max_repo_creation,omitempty"`
	MustChangePassword      *bool   `json:"must_change_password,omitempty"`
	Password                *string `json:"password,omitempty"`
	ProhibitLogin           *bool   `json:"prohibit_login,omitempty"`
	Restricted              *bool   `json:"restricted,omitempty"`
	SourceID                *int64  `json:"source_id,omitempty"`
	Visibility              *string `json:"visibility,omitempty"`
	Website                 *string `json:"website,omitempty"`
}

// GetUser retrieves a user by username from Gitea
func (c *Client) GetUser(username string) (*GiteaUser, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s", c.baseURL, username)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil // User not found
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user: %s - %s", resp.Status, string(body))
	}

	var user GiteaUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &user, nil
}

// CreateUser creates a new user in Gitea
func (c *Client) CreateUser(req *CreateUserRequest) (*GiteaUser, error) {
	url := fmt.Sprintf("%s/api/v1/admin/users", c.baseURL)

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
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create user: %s - %s", resp.Status, string(body))
	}

	var user GiteaUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &user, nil
}

// UpdateUser updates an existing user in Gitea
func (c *Client) UpdateUser(username string, req *UpdateUserRequest) (*GiteaUser, error) {
	url := fmt.Sprintf("%s/api/v1/admin/users/%s", c.baseURL, username)

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
		return nil, fmt.Errorf("failed to update user: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to update user: %s - %s", resp.Status, string(body))
	}

	var user GiteaUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &user, nil
}

// DeleteUser deletes a user from Gitea
func (c *Client) DeleteUser(username string) error {
	url := fmt.Sprintf("%s/api/v1/admin/users/%s", c.baseURL, username)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete user: %s - %s", resp.Status, string(body))
	}

	return nil
}

// search users for gitea
func (c *Client) SearchUsers(query string, limit int) ([]*GiteaUser, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	url := fmt.Sprintf("%s/api/v1/users/search?q=%s&limit=%d", c.baseURL, query, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search users: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Data []*GiteaUser `json:"data"`
		OK   bool         `json:"ok"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode users: %w", err)
	}

	return result.Data, nil
}
