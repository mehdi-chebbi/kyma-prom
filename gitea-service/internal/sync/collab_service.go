package sync

import (
	"context"
	"fmt"

	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/devplatform/gitea-service/internal/ldap"
	"github.com/sirupsen/logrus"
)

// CollabService orchestrates the drag-and-drop collaboration workflow.
// It coordinates LDAP changes + immediate Gitea team sync for:
//   - Adding/removing repos to groups
//   - Adding/removing repos to departments (with manager admin)
//   - Creating/deleting collab groups (department + extra members)
type CollabService struct {
	giteaClient      *gitea.Client
	ldapClient       *ldap.Client
	groupSyncService *GroupSyncService
	controller       *Controller
	logger           *logrus.Logger
}

// NewCollabService creates a new collaboration service
func NewCollabService(
	giteaClient *gitea.Client,
	ldapClient *ldap.Client,
	groupSyncService *GroupSyncService,
	controller *Controller,
	logger *logrus.Logger,
) *CollabService {
	return &CollabService{
		giteaClient:      giteaClient,
		ldapClient:       ldapClient,
		groupSyncService: groupSyncService,
		controller:       controller,
		logger:           logger,
	}
}

// GroupAccess describes how a group/department has access to a repository
type GroupAccess struct {
	CN             string   `json:"cn"`
	GroupType      string   `json:"groupType"` // "group", "department", "collab"
	Members        []string `json:"members"`
	Permission     string   `json:"permission"`
	BaseDepartment string   `json:"baseDepartment,omitempty"`
	ExtraMembers   []string `json:"extraMembers,omitempty"`
}

// ============================================================================
// GROUP → REPO OPERATIONS
// ============================================================================

// AddRepoToGroup adds a repository to a group and triggers immediate Gitea sync
func (s *CollabService) AddRepoToGroup(ctx context.Context, groupCN, repo, token string) (*SyncResult, error) {
	// Get current group to read existing repos
	group, err := s.ldapClient.GetGroup(ctx, groupCN)
	if err != nil {
		return nil, fmt.Errorf("failed to get group %s: %w", groupCN, err)
	}

	// Append and deduplicate
	repos := appendUnique(group.Repositories, repo)

	// Update LDAP
	if err := s.ldapClient.AssignReposToGroup(ctx, groupCN, repos, token); err != nil {
		return nil, fmt.Errorf("failed to assign repo to group %s: %w", groupCN, err)
	}

	// Immediate sync to Gitea
	orgName := s.controller.cfg.GetDefaultOwner()
	result, err := s.groupSyncService.SyncGroupToTeam(ctx, groupCN, orgName, groupCN, "write", "")
	if err != nil {
		s.logger.WithError(err).Warn("Immediate group sync failed after adding repo")
		return nil, fmt.Errorf("sync failed after adding repo: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"groupCN": groupCN,
		"repo":    repo,
	}).Info("Added repo to group and synced")

	return result, nil
}

// RemoveRepoFromGroup removes a repo from a group and resyncs
func (s *CollabService) RemoveRepoFromGroup(ctx context.Context, groupCN, repo, token string) (*SyncResult, error) {
	group, err := s.ldapClient.GetGroup(ctx, groupCN)
	if err != nil {
		return nil, fmt.Errorf("failed to get group %s: %w", groupCN, err)
	}

	repos := removeFromSlice(group.Repositories, repo)

	if err := s.ldapClient.AssignReposToGroup(ctx, groupCN, repos, token); err != nil {
		return nil, fmt.Errorf("failed to update group repos: %w", err)
	}

	// Sync to Gitea (will update team repos)
	orgName := s.controller.cfg.GetDefaultOwner()
	result, err := s.groupSyncService.SyncGroupToTeam(ctx, groupCN, orgName, groupCN, "write", "")
	if err != nil {
		return nil, fmt.Errorf("sync failed after removing repo: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"groupCN": groupCN,
		"repo":    repo,
	}).Info("Removed repo from group and synced")

	return result, nil
}

// ============================================================================
// DEPARTMENT → REPO OPERATIONS
// ============================================================================

// AddRepoToDepartment adds a repo to a department and syncs (manager gets admin)
func (s *CollabService) AddRepoToDepartment(ctx context.Context, ou, repo, token string) (*SyncResult, error) {
	dept, err := s.ldapClient.GetDepartment(ctx, ou, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get department %s: %w", ou, err)
	}

	repos := appendUnique(dept.Repositories, repo)

	// Update LDAP (using assignRepoToDepartment mutation)
	if err := s.ldapClient.AssignReposToDepartment(ctx, ou, repos, token); err != nil {
		return nil, fmt.Errorf("failed to assign repo to department %s: %w", ou, err)
	}

	// Immediate sync — manager gets admin access
	orgName := s.controller.cfg.GetDefaultOwner()
	result, err := s.groupSyncService.SyncDepartmentToTeam(ctx, ou, orgName, ou, "write", token)
	if err != nil {
		return nil, fmt.Errorf("sync failed after adding repo to department: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"department": ou,
		"repo":       repo,
		"manager":    dept.Manager,
	}).Info("Added repo to department and synced")

	return result, nil
}

// RemoveRepoFromDepartment removes a repo from a department and resyncs
func (s *CollabService) RemoveRepoFromDepartment(ctx context.Context, ou, repo, token string) (*SyncResult, error) {
	dept, err := s.ldapClient.GetDepartment(ctx, ou, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get department %s: %w", ou, err)
	}

	repos := removeFromSlice(dept.Repositories, repo)

	if err := s.ldapClient.AssignReposToDepartment(ctx, ou, repos, token); err != nil {
		return nil, fmt.Errorf("failed to update department repos: %w", err)
	}

	orgName := s.controller.cfg.GetDefaultOwner()
	result, err := s.groupSyncService.SyncDepartmentToTeam(ctx, ou, orgName, ou, "write", token)
	if err != nil {
		return nil, fmt.Errorf("sync failed after removing repo from department: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"department": ou,
		"repo":       repo,
	}).Info("Removed repo from department and synced")

	return result, nil
}

// ============================================================================
// COLLAB GROUP OPERATIONS
// ============================================================================

// CreateCollabGroup creates a dynamic collab group (department + extra members)
// and immediately syncs it to a Gitea team
func (s *CollabService) CreateCollabGroup(
	ctx context.Context,
	name, baseDepartment string,
	extraMembers, repos []string,
	token string,
) (*SyncResult, error) {
	// Get base department for member list
	dept, err := s.ldapClient.GetDepartment(ctx, baseDepartment, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get base department %s: %w", baseDepartment, err)
	}

	// Create the LDAP group
	description := fmt.Sprintf("Collab group based on %s", baseDepartment)
	_, err = s.ldapClient.CreateGroup(ctx, name, description, token)
	if err != nil {
		return nil, fmt.Errorf("failed to create LDAP group %s: %w", name, err)
	}

	// Merge members: department + extras
	allMembers := make(map[string]bool)
	for _, m := range dept.Members {
		allMembers[m] = true
	}
	for _, m := range extraMembers {
		allMembers[m] = true
	}

	// Add all members to the LDAP group
	for uid := range allMembers {
		if err := s.ldapClient.AddUserToGroup(ctx, uid, name, token); err != nil {
			s.logger.WithError(err).Warnf("Failed to add member %s to collab group %s", uid, name)
		}
	}

	// Assign repositories
	if len(repos) > 0 {
		if err := s.ldapClient.AssignReposToGroup(ctx, name, repos, token); err != nil {
			s.logger.WithError(err).Warn("Failed to assign repos to collab group")
		}
	}

	// Register collab group metadata in controller (persisted to state.json)
	meta := &CollabGroupMeta{
		BaseDepartment: baseDepartment,
		ExtraMembers:   extraMembers,
	}
	s.controller.RegisterCollabGroup(name, meta)

	// Immediate sync to Gitea
	orgName := s.controller.cfg.GetDefaultOwner()
	result, err := s.groupSyncService.SyncCollabGroup(ctx, name, meta, orgName, "write", token)
	if err != nil {
		s.logger.WithError(err).Warn("Immediate collab group sync failed")
		return nil, fmt.Errorf("sync failed after creating collab group: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"collabGroup":    name,
		"baseDepartment": baseDepartment,
		"extraMembers":   len(extraMembers),
		"repos":          len(repos),
	}).Info("Created collab group and synced")

	return result, nil
}

// DeleteCollabGroup removes a collab group: deletes LDAP group + Gitea team
func (s *CollabService) DeleteCollabGroup(ctx context.Context, groupCN, token string) error {
	// Delete the Gitea team
	orgName := s.controller.cfg.GetDefaultOwner()
	if err := s.groupSyncService.DeleteTeamByName(ctx, orgName, groupCN); err != nil {
		s.logger.WithError(err).Warn("Failed to delete Gitea team for collab group")
	}

	// Delete the LDAP group
	if err := s.ldapClient.DeleteGroup(ctx, groupCN, token); err != nil {
		return fmt.Errorf("failed to delete LDAP group %s: %w", groupCN, err)
	}

	// Unregister from controller
	s.controller.UnregisterCollabGroup(groupCN)

	s.logger.WithField("groupCN", groupCN).Info("Deleted collab group")
	return nil
}

// AddCollabGroupMembers adds extra members to a collab group
func (s *CollabService) AddCollabGroupMembers(ctx context.Context, groupCN string, members []string, token string) error {
	meta := s.controller.GetCollabGroupMeta(groupCN)
	if meta == nil {
		return fmt.Errorf("collab group %s not found", groupCN)
	}

	// Add to LDAP group
	for _, uid := range members {
		if err := s.ldapClient.AddUserToGroup(ctx, uid, groupCN, token); err != nil {
			s.logger.WithError(err).Warnf("Failed to add member %s to collab group %s", uid, groupCN)
		}
	}

	// Update metadata with new extra members
	existingExtra := make(map[string]bool)
	for _, m := range meta.ExtraMembers {
		existingExtra[m] = true
	}
	for _, m := range members {
		if !existingExtra[m] {
			meta.ExtraMembers = append(meta.ExtraMembers, m)
		}
	}
	s.controller.RegisterCollabGroup(groupCN, meta)

	s.logger.WithFields(logrus.Fields{
		"groupCN": groupCN,
		"added":   len(members),
	}).Info("Added members to collab group")

	return nil
}

// RemoveCollabGroupMembers removes extra members from a collab group
func (s *CollabService) RemoveCollabGroupMembers(ctx context.Context, groupCN string, members []string, token string) error {
	meta := s.controller.GetCollabGroupMeta(groupCN)
	if meta == nil {
		return fmt.Errorf("collab group %s not found", groupCN)
	}

	// Remove from LDAP group
	for _, uid := range members {
		if err := s.ldapClient.RemoveUserFromGroup(ctx, uid, groupCN, token); err != nil {
			s.logger.WithError(err).Warnf("Failed to remove member %s from collab group %s", uid, groupCN)
		}
	}

	// Update metadata: remove from extra members
	removeSet := make(map[string]bool)
	for _, m := range members {
		removeSet[m] = true
	}
	newExtra := make([]string, 0)
	for _, m := range meta.ExtraMembers {
		if !removeSet[m] {
			newExtra = append(newExtra, m)
		}
	}
	meta.ExtraMembers = newExtra
	s.controller.RegisterCollabGroup(groupCN, meta)

	s.logger.WithFields(logrus.Fields{
		"groupCN": groupCN,
		"removed": len(members),
	}).Info("Removed members from collab group")

	return nil
}

// ListRepoAccess lists all groups/departments that have access to a repository
func (s *CollabService) ListRepoAccess(ctx context.Context, repo, token string) ([]*GroupAccess, error) {
	var access []*GroupAccess

	// Check departments
	departments, err := s.ldapClient.ListAllDepartments(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to list departments: %w", err)
	}

	for _, dept := range departments {
		if containsString(dept.Repositories, repo) {
			access = append(access, &GroupAccess{
				CN:         dept.OU,
				GroupType:  "department",
				Members:    dept.Members,
				Permission: "write",
			})
		}
	}

	// Check groups
	groups, err := s.ldapClient.ListAllGroups(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	for _, group := range groups {
		if containsString(group.Repositories, repo) {
			ga := &GroupAccess{
				CN:         group.CN,
				GroupType:  "group",
				Members:    group.Members,
				Permission: "write",
			}

			// Check if this is a collab group
			if meta := s.controller.GetCollabGroupMeta(group.CN); meta != nil {
				ga.GroupType = "collab"
				ga.BaseDepartment = meta.BaseDepartment
				ga.ExtraMembers = meta.ExtraMembers
			}

			access = append(access, ga)
		}
	}

	return access, nil
}

// ============================================================================
// HELPERS
// ============================================================================

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
