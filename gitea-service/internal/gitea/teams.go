package gitea

import (
	"context"
	"fmt"
)

// Team represents a Gitea team
type Team struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Permission  string `json:"permission"` // read, write, admin
	CanCreateOrgRepo bool `json:"can_create_org_repo,omitempty"`
}

// CreateTeamRequest contains fields for creating a team
type CreateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Permission  string `json:"permission"` // read, write, admin
}

// CreateTeam creates a new team in an organization
func (c *Client) CreateTeam(ctx context.Context, orgName string, input *CreateTeamRequest) (*Team, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/teams", orgName)

	c.logger.WithFields(map[string]interface{}{
		"org":  orgName,
		"team": input.Name,
		"perm": input.Permission,
	}).Info("Creating Gitea team")

	var team Team
	if err := c.doRequestWithBody("POST", path, input, &team); err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	c.logger.WithField("teamId", team.ID).Info("Team created successfully")
	return &team, nil
}

// GetTeam retrieves a team by ID
func (c *Client) GetTeam(ctx context.Context, teamID int64) (*Team, error) {
	path := fmt.Sprintf("/api/v1/teams/%d", teamID)

	var team Team
	if err := c.doRequestWithBody("GET", path, nil, &team); err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	return &team, nil
}

// ListTeams lists all teams in an organization
func (c *Client) ListTeams(ctx context.Context, orgName string, page, limit int) ([]*Team, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/teams?page=%d&limit=%d", orgName, page, limit)

	var teams []*Team
	if err := c.doRequestWithBody("GET", path, nil, &teams); err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	return teams, nil
}

// AddTeamMember adds a user to a team
func (c *Client) AddTeamMember(ctx context.Context, teamID int64, username string) error {
	path := fmt.Sprintf("/api/v1/teams/%d/members/%s", teamID, username)

	c.logger.WithFields(map[string]interface{}{
		"teamId":   teamID,
		"username": username,
	}).Info("Adding member to team")

	if err := c.doRequestWithBody("PUT", path, nil, nil); err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}

	return nil
}

// RemoveTeamMember removes a user from a team
func (c *Client) RemoveTeamMember(ctx context.Context, teamID int64, username string) error {
	path := fmt.Sprintf("/api/v1/teams/%d/members/%s", teamID, username)

	c.logger.WithFields(map[string]interface{}{
		"teamId":   teamID,
		"username": username,
	}).Info("Removing member from team")

	if err := c.doRequestWithBody("DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("failed to remove team member: %w", err)
	}

	return nil
}

// ListTeamMembers lists all members of a team
func (c *Client) ListTeamMembers(ctx context.Context, teamID int64) ([]*User, error) {
	path := fmt.Sprintf("/api/v1/teams/%d/members", teamID)

	var members []*User
	if err := c.doRequestWithBody("GET", path, nil, &members); err != nil {
		return nil, fmt.Errorf("failed to list team members: %w", err)
	}

	return members, nil
}

// AddTeamRepository adds a repository to a team
func (c *Client) AddTeamRepository(ctx context.Context, teamID int64, owner, repo string) error {
	path := fmt.Sprintf("/api/v1/teams/%d/repos/%s/%s", teamID, owner, repo)

	c.logger.WithFields(map[string]interface{}{
		"teamId": teamID,
		"repo":   fmt.Sprintf("%s/%s", owner, repo),
	}).Info("Adding repository to team")

	if err := c.doRequestWithBody("PUT", path, nil, nil); err != nil {
		return fmt.Errorf("failed to add team repository: %w", err)
	}

	return nil
}

// RemoveTeamRepository removes a repository from a team
func (c *Client) RemoveTeamRepository(ctx context.Context, teamID int64, owner, repo string) error {
	path := fmt.Sprintf("/api/v1/teams/%d/repos/%s/%s", teamID, owner, repo)

	c.logger.WithFields(map[string]interface{}{
		"teamId": teamID,
		"repo":   fmt.Sprintf("%s/%s", owner, repo),
	}).Info("Removing repository from team")

	if err := c.doRequestWithBody("DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("failed to remove team repository: %w", err)
	}

	return nil
}

// ListTeamRepositories lists all repositories for a team
func (c *Client) ListTeamRepositories(ctx context.Context, teamID int64) ([]*Repository, error) {
	path := fmt.Sprintf("/api/v1/teams/%d/repos", teamID)

	var repos []*Repository
	if err := c.doRequestWithBody("GET", path, nil, &repos); err != nil {
		return nil, fmt.Errorf("failed to list team repositories: %w", err)
	}

	return repos, nil
}

// DeleteTeam deletes a team by ID
func (c *Client) DeleteTeam(ctx context.Context, teamID int64) error {
	path := fmt.Sprintf("/api/v1/teams/%d", teamID)

	c.logger.WithField("teamId", teamID).Info("Deleting Gitea team")

	if err := c.doRequestWithBody("DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}

	return nil
}

// DoDeleteRequest exposes a DELETE request for a given path (used by sync services)
func (c *Client) DoDeleteRequest(path string) error {
	return c.doRequestWithBody("DELETE", path, nil, nil)
}

// SearchTeams searches for teams by name in an organization
func (c *Client) SearchTeams(ctx context.Context, orgName, query string) ([]*Team, error) {
	// First get all teams, then filter by name
	// Gitea API doesn't have a direct search endpoint for teams
	teams, err := c.ListTeams(ctx, orgName, 1, 100)
	if err != nil {
		return nil, err
	}

	var filtered []*Team
	for _, team := range teams {
		if team.Name == query {
			filtered = append(filtered, team)
		}
	}

	return filtered, nil
}
