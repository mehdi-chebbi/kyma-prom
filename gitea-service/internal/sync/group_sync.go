package sync

import (
	"context"
	"fmt"
	"strings"

	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/devplatform/gitea-service/internal/ldap"
	"github.com/sirupsen/logrus"
)

// GroupSyncService handles synchronization between LDAP groups and Gitea teams
type GroupSyncService struct {
	giteaClient *gitea.Client
	ldapClient  *ldap.Client
	logger      *logrus.Logger
}

// NewGroupSyncService creates a new group sync service
func NewGroupSyncService(giteaClient *gitea.Client, ldapClient *ldap.Client, logger *logrus.Logger) *GroupSyncService {
	return &GroupSyncService{
		giteaClient: giteaClient,
		ldapClient:  ldapClient,
		logger:      logger,
	}
}

// SyncResult contains the result of a sync operation
type SyncResult struct {
	Team               *gitea.Team
	MembersAdded       int
	MembersFailed      int
	RepositoriesAdded  int
	RepositoriesFailed int
	ManagerGranted     bool
	Errors             []string
}

// CollabGroupMeta stores metadata about a dynamically-created collaboration group.
// BaseDepartment is the department whose members form the core membership.
// ExtraMembers are additional UIDs added beyond the department.
type CollabGroupMeta struct {
	BaseDepartment string   `json:"base_department"`
	ExtraMembers   []string `json:"extra_members"`
}

// SyncGroupToTeam synchronizes an LDAP group to a Gitea team.
// If manager is non-empty, that user gets admin collaborator access on every repo.
// The controller calls this periodically to keep Gitea teams in sync with LDAP.
func (s *GroupSyncService) SyncGroupToTeam(
	ctx context.Context,
	groupCN string,
	orgName string,
	teamName string,
	permission string,
	manager string,
) (*SyncResult, error) {
	s.logger.WithFields(logrus.Fields{
		"groupCN":    groupCN,
		"orgName":    orgName,
		"teamName":   teamName,
		"permission": permission,
		"manager":    manager,
	}).Info("Starting LDAP group to Gitea team sync")

	result := &SyncResult{
		Errors: []string{},
	}

	// STEP 1: Get LDAP group information
	group, err := s.ldapClient.GetGroup(ctx, groupCN)
	if err != nil {
		return nil, fmt.Errorf("failed to get LDAP group %s: %w", groupCN, err)
	}

	s.logger.WithFields(logrus.Fields{
		"members":      len(group.Members),
		"repositories": len(group.Repositories),
	}).Info("LDAP group fetched successfully")

	if teamName == "" {
		teamName = group.CN
	}

	// STEP 2: Create or find Gitea team
	team, err := s.createOrGetTeam(ctx, orgName, teamName, group.Description, permission)
	if err != nil {
		return nil, fmt.Errorf("failed to create/get team: %w", err)
	}
	result.Team = team

	// STEP 3: Sync members
	s.syncMembers(ctx, team, group.Members, result)

	// STEP 4: Sync repositories
	s.syncRepositories(ctx, team, group.Repositories, result)

	// STEP 5: Grant manager admin access on all repos
	if manager != "" {
		s.grantManagerAdmin(ctx, manager, group.Repositories, result)
	}

	// STEP 6: Remove stale members from Gitea that are no longer in LDAP
	if err := s.RemoveMembersNotInLDAP(ctx, team.ID, group.Members); err != nil {
		s.logger.WithError(err).Warn("Failed to remove stale members from team")
	}

	s.logger.WithFields(logrus.Fields{
		"teamId":         team.ID,
		"teamName":       team.Name,
		"members":        result.MembersAdded,
		"repositories":   result.RepositoriesAdded,
		"managerGranted": result.ManagerGranted,
		"errors":         len(result.Errors),
	}).Info("Group sync completed")

	return result, nil
}

// SyncDepartmentToTeam syncs a department to a Gitea team.
// Department members get the specified permission (typically "write").
// The department manager automatically gets admin collaborator access on all repos.
func (s *GroupSyncService) SyncDepartmentToTeam(
	ctx context.Context,
	ou string,
	orgName string,
	teamName string,
	permission string,
	token string,
) (*SyncResult, error) {
	s.logger.WithFields(logrus.Fields{
		"ou":         ou,
		"orgName":    orgName,
		"teamName":   teamName,
		"permission": permission,
	}).Info("Starting department to Gitea team sync")

	result := &SyncResult{
		Errors: []string{},
	}

	// STEP 1: Get department from LDAP
	dept, err := s.ldapClient.GetDepartment(ctx, ou, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get department %s: %w", ou, err)
	}

	if teamName == "" {
		teamName = dept.OU
	}

	// STEP 2: Create or find Gitea team
	team, err := s.createOrGetTeam(ctx, orgName, teamName, dept.Description, permission)
	if err != nil {
		return nil, fmt.Errorf("failed to create/get team: %w", err)
	}
	result.Team = team

	// STEP 3: Sync members
	s.syncMembers(ctx, team, dept.Members, result)

	// STEP 4: Sync repositories
	s.syncRepositories(ctx, team, dept.Repositories, result)

	// STEP 5: Grant manager admin access on all repos
	if dept.Manager != "" {
		s.grantManagerAdmin(ctx, dept.Manager, dept.Repositories, result)
	}

	// STEP 6: Remove stale members
	if err := s.RemoveMembersNotInLDAP(ctx, team.ID, dept.Members); err != nil {
		s.logger.WithError(err).Warn("Failed to remove stale members from department team")
	}

	s.logger.WithFields(logrus.Fields{
		"teamId":         team.ID,
		"teamName":       team.Name,
		"department":     ou,
		"manager":        dept.Manager,
		"members":        result.MembersAdded,
		"repositories":   result.RepositoriesAdded,
		"managerGranted": result.ManagerGranted,
		"errors":         len(result.Errors),
	}).Info("Department sync completed")

	return result, nil
}

// SyncCollabGroup syncs a dynamic collaboration group to a Gitea team.
// Membership is: department members + extra members.
// Department manager gets admin collaborator access on all repos.
func (s *GroupSyncService) SyncCollabGroup(
	ctx context.Context,
	groupCN string,
	meta *CollabGroupMeta,
	orgName string,
	permission string,
	token string,
) (*SyncResult, error) {
	s.logger.WithFields(logrus.Fields{
		"groupCN":        groupCN,
		"baseDepartment": meta.BaseDepartment,
		"extraMembers":   len(meta.ExtraMembers),
	}).Info("Starting collab group sync")

	result := &SyncResult{
		Errors: []string{},
	}

	// STEP 1: Get the base department to resolve members and manager
	dept, err := s.ldapClient.GetDepartment(ctx, meta.BaseDepartment, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get base department %s: %w", meta.BaseDepartment, err)
	}

	// STEP 2: Get the LDAP group for repos
	group, err := s.ldapClient.GetGroup(ctx, groupCN)
	if err != nil {
		return nil, fmt.Errorf("failed to get collab group %s: %w", groupCN, err)
	}

	// STEP 3: Merge members (department + extra, deduplicated)
	memberSet := make(map[string]bool)
	for _, m := range dept.Members {
		memberSet[m] = true
	}
	for _, m := range meta.ExtraMembers {
		memberSet[m] = true
	}
	finalMembers := make([]string, 0, len(memberSet))
	for m := range memberSet {
		finalMembers = append(finalMembers, m)
	}

	// STEP 4: Sync LDAP group members to match finalMembers
	// Add missing members to LDAP group
	currentMemberSet := make(map[string]bool)
	for _, m := range group.Members {
		currentMemberSet[m] = true
	}
	for _, m := range finalMembers {
		if !currentMemberSet[m] {
			if err := s.ldapClient.AddUserToGroup(ctx, m, groupCN, token); err != nil {
				s.logger.WithError(err).Warnf("Failed to add member %s to LDAP group %s", m, groupCN)
			}
		}
	}
	// Remove stale members from LDAP group
	finalMemberSet := make(map[string]bool)
	for _, m := range finalMembers {
		finalMemberSet[m] = true
	}
	for _, m := range group.Members {
		if !finalMemberSet[m] {
			if err := s.ldapClient.RemoveUserFromGroup(ctx, m, groupCN, token); err != nil {
				s.logger.WithError(err).Warnf("Failed to remove stale member %s from LDAP group %s", m, groupCN)
			}
		}
	}

	// STEP 5: Create or find Gitea team
	team, err := s.createOrGetTeam(ctx, orgName, groupCN, group.Description, permission)
	if err != nil {
		return nil, fmt.Errorf("failed to create/get team: %w", err)
	}
	result.Team = team

	// STEP 6: Sync members to Gitea team
	s.syncMembers(ctx, team, finalMembers, result)

	// STEP 7: Sync repositories
	s.syncRepositories(ctx, team, group.Repositories, result)

	// STEP 8: Grant department manager admin access
	if dept.Manager != "" {
		s.grantManagerAdmin(ctx, dept.Manager, group.Repositories, result)
	}

	// STEP 9: Remove stale Gitea team members
	if err := s.RemoveMembersNotInLDAP(ctx, team.ID, finalMembers); err != nil {
		s.logger.WithError(err).Warn("Failed to remove stale members from collab team")
	}

	s.logger.WithFields(logrus.Fields{
		"teamId":         team.ID,
		"groupCN":        groupCN,
		"department":     meta.BaseDepartment,
		"totalMembers":   len(finalMembers),
		"managerGranted": result.ManagerGranted,
		"errors":         len(result.Errors),
	}).Info("Collab group sync completed")

	return result, nil
}

// syncMembers adds a list of UIDs to a Gitea team, tracking results in SyncResult
func (s *GroupSyncService) syncMembers(ctx context.Context, team *gitea.Team, members []string, result *SyncResult) {
	for _, memberUID := range members {
		if err := s.giteaClient.AddTeamMember(ctx, team.ID, memberUID); err != nil {
			s.logger.WithError(err).Warnf("Failed to add member %s to team", memberUID)
			result.MembersFailed++
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to add member %s: %v", memberUID, err))
		} else {
			result.MembersAdded++
		}
	}
}

// syncRepositories adds a list of repo URLs to a Gitea team, tracking results in SyncResult
func (s *GroupSyncService) syncRepositories(ctx context.Context, team *gitea.Team, repoURLs []string, result *SyncResult) {
	for _, repoURL := range repoURLs {
		owner, repoName := parseGitHubURL(repoURL)
		if owner == "" || repoName == "" {
			s.logger.Warnf("Invalid repository URL: %s", repoURL)
			result.RepositoriesFailed++
			result.Errors = append(result.Errors, fmt.Sprintf("Invalid repo URL: %s", repoURL))
			continue
		}

		if err := s.giteaClient.AddTeamRepository(ctx, team.ID, owner, repoName); err != nil {
			s.logger.WithError(err).Warnf("Failed to add repository %s/%s to team", owner, repoName)
			result.RepositoriesFailed++
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to add repo %s/%s: %v", owner, repoName, err))
		} else {
			result.RepositoriesAdded++
		}
	}
}

// grantManagerAdmin adds the manager as admin collaborator on every repo in the list
func (s *GroupSyncService) grantManagerAdmin(ctx context.Context, manager string, repoURLs []string, result *SyncResult) {
	for _, repoURL := range repoURLs {
		owner, repoName := parseGitHubURL(repoURL)
		if owner == "" || repoName == "" {
			continue
		}

		if err := s.giteaClient.AddCollaborator(owner, repoName, manager, "admin"); err != nil {
			s.logger.WithError(err).Warnf("Failed to grant manager %s admin on %s/%s", manager, owner, repoName)
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to grant manager admin on %s/%s: %v", owner, repoName, err))
		} else {
			result.ManagerGranted = true
		}
	}

	if result.ManagerGranted {
		s.logger.WithField("manager", manager).Info("Manager granted admin access on repositories")
	}
}

// DeleteTeamByName deletes a Gitea team by searching for it by name in an organization
func (s *GroupSyncService) DeleteTeamByName(ctx context.Context, orgName, teamName string) error {
	teams, err := s.giteaClient.SearchTeams(ctx, orgName, teamName)
	if err != nil {
		return fmt.Errorf("failed to search for team %s: %w", teamName, err)
	}

	if len(teams) == 0 {
		s.logger.WithField("teamName", teamName).Warn("Team not found, nothing to delete")
		return nil
	}

	// Gitea API: DELETE /api/v1/teams/{id}
	path := fmt.Sprintf("/api/v1/teams/%d", teams[0].ID)
	if err := s.giteaClient.DoDeleteRequest(path); err != nil {
		return fmt.Errorf("failed to delete team %s: %w", teamName, err)
	}

	s.logger.WithFields(logrus.Fields{
		"teamId":   teams[0].ID,
		"teamName": teamName,
	}).Info("Deleted Gitea team")

	return nil
}

// createOrGetTeam creates a new team or returns existing one
func (s *GroupSyncService) createOrGetTeam(
	ctx context.Context,
	orgName string,
	teamName string,
	description string,
	permission string,
) (*gitea.Team, error) {
	// Try to find existing team first
	teams, err := s.giteaClient.SearchTeams(ctx, orgName, teamName)
	if err == nil && len(teams) > 0 {
		s.logger.WithField("teamId", teams[0].ID).Info("Found existing team")
		return teams[0], nil
	}

	// Team doesn't exist, create it
	s.logger.Info("Team not found, creating new team")
	team, err := s.giteaClient.CreateTeam(ctx, orgName, &gitea.CreateTeamRequest{
		Name:        teamName,
		Description: description,
		Permission:  permission,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	return team, nil
}

// parseGitHubURL parses a GitHub URL to extract owner and repo name
// Examples:
//   - "https://github.com/devplatform/mobile-ios" → "devplatform", "mobile-ios"
//   - "https://github.com/org/repo" → "org", "repo"
//   - "git@github.com:org/repo.git" → "org", "repo"
func parseGitHubURL(url string) (owner, repo string) {
	// Remove common prefixes
	url = strings.TrimPrefix(url, "https://github.com/")
	url = strings.TrimPrefix(url, "http://github.com/")
	url = strings.TrimPrefix(url, "git@github.com:")

	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Split by /
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}

	return "", ""
}

// SyncMultipleGroups syncs multiple LDAP groups to Gitea teams
func (s *GroupSyncService) SyncMultipleGroups(
	ctx context.Context,
	orgName string,
	groups []string,
	permission string,
) (map[string]*SyncResult, error) {
	results := make(map[string]*SyncResult)

	for _, groupCN := range groups {
		result, err := s.SyncGroupToTeam(ctx, groupCN, orgName, "", permission, "")
		if err != nil {
			s.logger.WithError(err).Errorf("Failed to sync group %s", groupCN)
			results[groupCN] = &SyncResult{
				Errors: []string{err.Error()},
			}
		} else {
			results[groupCN] = result
		}
	}

	return results, nil
}

// RemoveMembersNotInLDAP removes members from Gitea team that are not in LDAP group
// This ensures the team membership stays in sync with LDAP
func (s *GroupSyncService) RemoveMembersNotInLDAP(
	ctx context.Context,
	teamID int64,
	ldapMembers []string,
) error {
	// Get current team members from Gitea
	giteaMembers, err := s.giteaClient.ListTeamMembers(ctx, teamID)
	if err != nil {
		return fmt.Errorf("failed to list team members: %w", err)
	}

	// Convert LDAP members to map for quick lookup
	ldapMemberMap := make(map[string]bool)
	for _, member := range ldapMembers {
		ldapMemberMap[member] = true
	}

	// Remove members not in LDAP
	for _, giteaMember := range giteaMembers {
		if !ldapMemberMap[giteaMember.Login] {
			s.logger.WithFields(logrus.Fields{
				"teamId":   teamID,
				"username": giteaMember.Login,
			}).Info("Removing member not in LDAP group")

			if err := s.giteaClient.RemoveTeamMember(ctx, teamID, giteaMember.Login); err != nil {
				s.logger.WithError(err).Warnf("Failed to remove member %s", giteaMember.Login)
			}
		}
	}

	return nil
}
