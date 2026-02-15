package ldap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Client represents a client for the LDAP Manager service
type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *logrus.Logger
}

// User represents a user from LDAP Manager service
type User struct {
	UID          string   `json:"uid"`
	CN           string   `json:"cn"`
	SN           string   `json:"sn"`
	GivenName    string   `json:"givenName"`
	Mail         string   `json:"mail"`
	Department   string   `json:"department"`
	UIDNumber    int      `json:"uidNumber"`
	GIDNumber    int      `json:"gidNumber"`
	HomeDir      string   `json:"homeDirectory"`
	Repositories []string `json:"repositories"`
	DN           string   `json:"dn"`
}

// Department represents a department from LDAP Manager service
type Department struct {
	OU           string   `json:"ou"`
	Description  string   `json:"description"`
	Manager      string   `json:"manager,omitempty"`
	Members      []string `json:"members"`
	Repositories []string `json:"repositories"`
	DN           string   `json:"dn"`
}

// Group represents a group from LDAP Manager service
type Group struct {
	CN           string   `json:"cn"`
	GIDNumber    int      `json:"gidNumber"`
	Description  string   `json:"description,omitempty"`
	Members      []string `json:"members"`
	Repositories []string `json:"repositories"`
	DN           string   `json:"dn"`
}

// NewClient creates a new LDAP Manager client
func NewClient(baseURL string, timeout time.Duration, logger *logrus.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// doGraphQLRequest performs a GraphQL request to LDAP Manager
func (c *Client) doGraphQLRequest(ctx context.Context, query string, token string) (map[string]interface{}, error) {
	requestBody := map[string]interface{}{
		"query": query,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/graphql", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	c.logger.WithFields(logrus.Fields{
		"url": url,
	}).Debug("Making LDAP Manager request")

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
		}).Error("LDAP Manager API error")
		return nil, fmt.Errorf("LDAP Manager API error: status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for GraphQL errors
	if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %v", errors)
	}

	return result, nil
}

// GetUser gets a user by UID from LDAP Manager
func (c *Client) GetUser(ctx context.Context, uid string, token string) (*User, error) {
	query := fmt.Sprintf(`
		query {
			user(uid: "%s") {
				uid
				cn
				sn
				givenName
				mail
				department
				repositories
				dn
			}
		}
	`, uid)

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return nil, err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	userMap, ok := data["user"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("user not found")
	}

	userBytes, err := json.Marshal(userMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user: %w", err)
	}

	var user User
	if err := json.Unmarshal(userBytes, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return &user, nil
}

// GetDepartment gets a department by OU from LDAP Manager
func (c *Client) GetDepartment(ctx context.Context, ou string, token string) (*Department, error) {
	query := fmt.Sprintf(`
		query {
			department(ou: "%s") {
				ou
				description
				manager
				members
				repositories
				dn
			}
		}
	`, ou)

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return nil, err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	deptMap, ok := data["department"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("department not found")
	}

	deptBytes, err := json.Marshal(deptMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal department: %w", err)
	}

	var dept Department
	if err := json.Unmarshal(deptBytes, &dept); err != nil {
		return nil, fmt.Errorf("failed to unmarshal department: %w", err)
	}

	return &dept, nil
}

// ListAllUsers retrieves all users from LDAP
func (c *Client) ListAllUsers(token string) ([]*User, error) {
	ctx := context.Background()

	query := `
		query {
			usersAll {
				uid
				cn
				mail
				department
				repositories
			}
		}
	`

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return nil, err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	usersSlice, ok := data["usersAll"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("usersAll not found in response")
	}

	users := make([]*User, 0, len(usersSlice))
	for _, userInterface := range usersSlice {
		userMap, ok := userInterface.(map[string]interface{})
		if !ok {
			continue
		}

		userBytes, err := json.Marshal(userMap)
		if err != nil {
			continue
		}

		var user User
		if err := json.Unmarshal(userBytes, &user); err != nil {
			continue
		}

		users = append(users, &user)
	}

	return users, nil
}

// GetGroup retrieves a group from LDAP Manager by CN
func (c *Client) GetGroup(ctx context.Context, cn string) (*Group, error) {
	query := `
		query GetGroup($cn: String!) {
			group(cn: $cn) {
				cn
				gidNumber
				description
				members
				repositories
				dn
			}
		}
	`

	variables := map[string]interface{}{
		"cn": cn,
	}

	queryJSON, err := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	url := fmt.Sprintf("%s/graphql", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(queryJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

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

	var result struct {
		Data struct {
			Group *Group `json:"group"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	if result.Data.Group == nil {
		return nil, fmt.Errorf("group not found: %s", cn)
	}

	return result.Data.Group, nil
}

// AssignReposToUser calls the LDAP Manager's assignRepoToUser mutation
// to update a user's githubRepository attribute in LDAP
func (c *Client) AssignReposToUser(ctx context.Context, uid string, repos []string, token string) error {
	// Build JSON array of repo strings
	reposJSON := "["
	for i, repo := range repos {
		if i > 0 {
			reposJSON += ", "
		}
		reposJSON += fmt.Sprintf(`"%s"`, repo)
	}
	reposJSON += "]"

	query := fmt.Sprintf(`
		mutation {
			assignRepoToUser(uid: "%s", repositories: %s) {
				uid
				repositories
			}
		}
	`, uid, reposJSON)

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to assign repos to user %s: %w", uid, err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format from assignRepoToUser")
	}

	if data["assignRepoToUser"] == nil {
		return fmt.Errorf("assignRepoToUser returned nil for user %s", uid)
	}

	c.logger.WithFields(logrus.Fields{
		"uid":        uid,
		"reposCount": len(repos),
	}).Info("Assigned repositories to user in LDAP")

	return nil
}

// ListAllGroups retrieves all groups from LDAP Manager
func (c *Client) ListAllGroups(ctx context.Context, token string) ([]*Group, error) {
	query := `
		query {
			groupsAll {
				cn
				description
				gidNumber
				members
				repositories
				dn
			}
		}
	`

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return nil, err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	groupsSlice, ok := data["groupsAll"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("groupsAll not found in response")
	}

	groups := make([]*Group, 0, len(groupsSlice))
	for _, gi := range groupsSlice {
		gMap, ok := gi.(map[string]interface{})
		if !ok {
			continue
		}
		gBytes, err := json.Marshal(gMap)
		if err != nil {
			continue
		}
		var group Group
		if err := json.Unmarshal(gBytes, &group); err != nil {
			continue
		}
		groups = append(groups, &group)
	}

	return groups, nil
}

// ListAllDepartments retrieves all departments from LDAP Manager
func (c *Client) ListAllDepartments(ctx context.Context, token string) ([]*Department, error) {
	query := `
		query {
			departmentsAll {
				ou
				description
				manager
				members
				repositories
				dn
			}
		}
	`

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return nil, err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	deptsSlice, ok := data["departmentsAll"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("departmentsAll not found in response")
	}

	departments := make([]*Department, 0, len(deptsSlice))
	for _, di := range deptsSlice {
		dMap, ok := di.(map[string]interface{})
		if !ok {
			continue
		}
		dBytes, err := json.Marshal(dMap)
		if err != nil {
			continue
		}
		var dept Department
		if err := json.Unmarshal(dBytes, &dept); err != nil {
			continue
		}
		departments = append(departments, &dept)
	}

	return departments, nil
}

// CreateGroup creates a new LDAP group via the LDAP Manager
func (c *Client) CreateGroup(ctx context.Context, cn, description string, token string) (*Group, error) {
	query := fmt.Sprintf(`
		mutation {
			createGroup(cn: "%s", description: "%s") {
				cn
				description
				gidNumber
				members
				repositories
				dn
			}
		}
	`, cn, description)

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return nil, fmt.Errorf("failed to create group %s: %w", cn, err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format from createGroup")
	}

	groupMap, ok := data["createGroup"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("createGroup returned nil for group %s", cn)
	}

	groupBytes, err := json.Marshal(groupMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal group: %w", err)
	}

	var group Group
	if err := json.Unmarshal(groupBytes, &group); err != nil {
		return nil, fmt.Errorf("failed to unmarshal group: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"cn": cn,
	}).Info("Created group in LDAP")

	return &group, nil
}

// DeleteGroup deletes an LDAP group via the LDAP Manager
func (c *Client) DeleteGroup(ctx context.Context, cn string, token string) error {
	query := fmt.Sprintf(`
		mutation {
			deleteGroup(cn: "%s")
		}
	`, cn)

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete group %s: %w", cn, err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format from deleteGroup")
	}

	success, ok := data["deleteGroup"].(bool)
	if !ok || !success {
		return fmt.Errorf("deleteGroup returned false for group %s", cn)
	}

	c.logger.WithFields(logrus.Fields{
		"cn": cn,
	}).Info("Deleted group from LDAP")

	return nil
}

// AddUserToGroup adds a user to an LDAP group via the LDAP Manager
func (c *Client) AddUserToGroup(ctx context.Context, uid, groupCN string, token string) error {
	query := fmt.Sprintf(`
		mutation {
			addUserToGroup(uid: "%s", groupCn: "%s")
		}
	`, uid, groupCN)

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to add user %s to group %s: %w", uid, groupCN, err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format from addUserToGroup")
	}

	success, ok := data["addUserToGroup"].(bool)
	if !ok || !success {
		return fmt.Errorf("addUserToGroup returned false for user %s group %s", uid, groupCN)
	}

	c.logger.WithFields(logrus.Fields{
		"uid":     uid,
		"groupCN": groupCN,
	}).Info("Added user to group in LDAP")

	return nil
}

// RemoveUserFromGroup removes a user from an LDAP group via the LDAP Manager
func (c *Client) RemoveUserFromGroup(ctx context.Context, uid, groupCN string, token string) error {
	query := fmt.Sprintf(`
		mutation {
			removeUserFromGroup(uid: "%s", groupCn: "%s")
		}
	`, uid, groupCN)

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to remove user %s from group %s: %w", uid, groupCN, err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format from removeUserFromGroup")
	}

	success, ok := data["removeUserFromGroup"].(bool)
	if !ok || !success {
		return fmt.Errorf("removeUserFromGroup returned false for user %s group %s", uid, groupCN)
	}

	c.logger.WithFields(logrus.Fields{
		"uid":     uid,
		"groupCN": groupCN,
	}).Info("Removed user from group in LDAP")

	return nil
}

// AssignReposToGroup updates a group's repositories in LDAP via the LDAP Manager
func (c *Client) AssignReposToGroup(ctx context.Context, groupCN string, repos []string, token string) error {
	reposJSON := "["
	for i, repo := range repos {
		if i > 0 {
			reposJSON += ", "
		}
		reposJSON += fmt.Sprintf(`"%s"`, repo)
	}
	reposJSON += "]"

	query := fmt.Sprintf(`
		mutation {
			assignRepoToGroup(groupCn: "%s", repositories: %s) {
				cn
				repositories
			}
		}
	`, groupCN, reposJSON)

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to assign repos to group %s: %w", groupCN, err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format from assignRepoToGroup")
	}

	if data["assignRepoToGroup"] == nil {
		return fmt.Errorf("assignRepoToGroup returned nil for group %s", groupCN)
	}

	c.logger.WithFields(logrus.Fields{
		"groupCN":    groupCN,
		"reposCount": len(repos),
	}).Info("Assigned repositories to group in LDAP")

	return nil
}

// AssignReposToDepartment updates a department's repositories in LDAP via the LDAP Manager
func (c *Client) AssignReposToDepartment(ctx context.Context, ou string, repos []string, token string) error {
	reposJSON := "["
	for i, repo := range repos {
		if i > 0 {
			reposJSON += ", "
		}
		reposJSON += fmt.Sprintf(`"%s"`, repo)
	}
	reposJSON += "]"

	query := fmt.Sprintf(`
		mutation {
			assignRepoToDepartment(ou: "%s", repositories: %s) {
				ou
				repositories
			}
		}
	`, ou, reposJSON)

	result, err := c.doGraphQLRequest(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to assign repos to department %s: %w", ou, err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format from assignRepoToDepartment")
	}

	if data["assignRepoToDepartment"] == nil {
		return fmt.Errorf("assignRepoToDepartment returned nil for department %s", ou)
	}

	c.logger.WithFields(logrus.Fields{
		"ou":         ou,
		"reposCount": len(repos),
	}).Info("Assigned repositories to department in LDAP")

	return nil
}

// HealthCheck checks if LDAP Manager service is accessible
func (c *Client) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LDAP Manager unhealthy: status %d", resp.StatusCode)
	}

	return nil
}
