package gitea

import (
	"context"
	"fmt"
	"strings"

	"github.com/devplatform/gitea-service/internal/ldap"
	"github.com/devplatform/gitea-service/internal/models"
	"github.com/sirupsen/logrus"
)

// Service provides repository access control based on LDAP attributes
type Service struct {
	client     *Client
	ldapClient *ldap.Client
	logger     *logrus.Logger
}

// NewService creates a new Gitea service
func NewService(client *Client, ldapClient *ldap.Client, logger *logrus.Logger) *Service {
	return &Service{
		client:     client,
		ldapClient: ldapClient,
		logger:     logger,
	}
}

// GetUserRepositories gets all repositories accessible by a user
// User can access a repository if:
// 1. It's in their personal LDAP githubRepository attribute, OR
// 2. It's in their department's githubRepository attribute
func (s *Service) GetUserRepositories(ctx context.Context, user *models.User, token string) ([]*Repository, error) {
	// Get all repositories from Gitea
	allRepos, err := s.client.ListRepositories()
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	// Get user's allowed repository names
	allowedRepoNames := s.getUserAllowedRepos(ctx, user, token)

	// Filter repositories
	userRepos := make([]*Repository, 0)
	for _, repo := range allRepos {
		if s.isRepoAllowed(repo, allowedRepoNames) {
			userRepos = append(userRepos, repo)
		}
	}

	s.logger.WithFields(logrus.Fields{
		"uid":         user.UID,
		"total_repos": len(allRepos),
		"user_repos":  len(userRepos),
	}).Info("Filtered user repositories")

	return userRepos, nil
}

// GetRepository gets a specific repository if user has access
func (s *Service) GetRepository(ctx context.Context, user *models.User, owner, name string, token string) (*Repository, error) {
	// Get repository from Gitea
	repo, err := s.client.GetRepository(owner, name)
	if err != nil {
		return nil, err
	}

	// Check if user has access
	allowedRepoNames := s.getUserAllowedRepos(ctx, user, token)
	if !s.isRepoAllowed(repo, allowedRepoNames) {
		return nil, fmt.Errorf("access denied to repository: %s/%s", owner, name)
	}

	return repo, nil
}

// SearchUserRepositories searches repositories accessible by user
func (s *Service) SearchUserRepositories(ctx context.Context, user *models.User, query string, limit int, token string) ([]*Repository, error) {
	// Get user's accessible repos first
	userRepos, err := s.GetUserRepositories(ctx, user, token)
	if err != nil {
		return nil, err
	}

	// Filter by search query
	if query == "" {
		// Return all user repos if no query
		if limit > 0 && len(userRepos) > limit {
			return userRepos[:limit], nil
		}
		return userRepos, nil
	}

	// Search within user's accessible repos
	queryLower := strings.ToLower(query)
	results := make([]*Repository, 0)
	for _, repo := range userRepos {
		if s.matchesQuery(repo, queryLower) {
			results = append(results, repo)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// getUserAllowedRepos gets the list of repository names/patterns the user can access
func (s *Service) getUserAllowedRepos(ctx context.Context, user *models.User, token string) map[string]bool {
	allowedRepos := make(map[string]bool)

	// Add user's personal repositories
	for _, repo := range user.Repositories {
		allowedRepos[s.normalizeRepoName(repo)] = true
	}

	// Add department repositories
	if user.Department != "" {
		dept, err := s.ldapClient.GetDepartment(ctx, user.Department, token)
		if err == nil {
			for _, repo := range dept.Repositories {
				allowedRepos[s.normalizeRepoName(repo)] = true
			}
		} else {
			s.logger.WithError(err).Warn("Failed to get department repositories")
		}
	}

	return allowedRepos
}

// isRepoAllowed checks if a repository is in the allowed list
func (s *Service) isRepoAllowed(repo *Repository, allowedRepos map[string]bool) bool {
	// Check full name (owner/repo)
	if allowedRepos[strings.ToLower(repo.FullName)] {
		return true
	}

	// Check by name only
	if allowedRepos[strings.ToLower(repo.Name)] {
		return true
	}

	return false
}

// normalizeRepoName normalizes a repository name for comparison
// Handles formats like:
// - "owner/repo"
// - "https://github.com/owner/repo"
// - "https://gitea.example.com/owner/repo"
// - "repo" (just the name)
func (s *Service) normalizeRepoName(repoName string) string {
	// Remove trailing slashes
	repoName = strings.TrimSuffix(repoName, "/")

	// If it's a URL, extract owner/repo part
	if strings.HasPrefix(repoName, "http://") || strings.HasPrefix(repoName, "https://") {
		parts := strings.Split(repoName, "/")
		if len(parts) >= 5 {
			// Extract owner/repo from URL (last two parts)
			repoName = fmt.Sprintf("%s/%s", parts[len(parts)-2], parts[len(parts)-1])
		}
	}

	return strings.ToLower(repoName)
}

// matchesQuery checks if a repository matches the search query
func (s *Service) matchesQuery(repo *Repository, queryLower string) bool {
	return strings.Contains(strings.ToLower(repo.Name), queryLower) ||
		strings.Contains(strings.ToLower(repo.FullName), queryLower) ||
		strings.Contains(strings.ToLower(repo.Description), queryLower)
}

// GetRepositoryStats gets statistics about user's repositories
func (s *Service) GetRepositoryStats(ctx context.Context, user *models.User, token string) (*RepositoryStats, error) {
	repos, err := s.GetUserRepositories(ctx, user, token)
	if err != nil {
		return nil, err
	}

	stats := &RepositoryStats{
		TotalCount: len(repos),
		Languages:  make(map[string]int),
	}

	for _, repo := range repos {
		if repo.Private {
			stats.PrivateCount++
		} else {
			stats.PublicCount++
		}

		if repo.Language != "" {
			stats.Languages[repo.Language]++
		}
	}

	return stats, nil
}

// RepositoryStats contains statistics about repositories
type RepositoryStats struct {
	TotalCount   int            `json:"totalCount"`
	PrivateCount int            `json:"privateCount"`
	PublicCount  int            `json:"publicCount"`
	Languages    map[string]int `json:"languages"`
}

// CreateRepository creates a new repository
func (s *Service) CreateRepository(ctx context.Context, owner string, req *CreateRepositoryRequest, user *models.User) (*Repository, error) {
	// Create the repository
	repo, err := s.client.CreateRepository(owner, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner":   owner,
		"repo":    req.Name,
		"user":    user.UID,
		"private": req.Private,
	}).Info("Repository created")

	return repo, nil
}

// MigrateRepository migrates a repository from an external source
func (s *Service) MigrateRepository(ctx context.Context, req *MigrateRepositoryRequest, user *models.User) (*Repository, error) {
	// Migrate the repository
	repo, err := s.client.MigrateRepository(req)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate repository: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"clone_addr": req.CloneAddr,
		"repo_name":  req.RepoName,
		"user":       user.UID,
		"mirror":     req.Mirror,
		"service":    req.Service,
	}).Info("Repository migrated")

	return repo, nil
}

// ForkRepository forks a repository
func (s *Service) ForkRepository(ctx context.Context, owner, repo, organization string, user *models.User, token string) (*Repository, error) {
	// Check if user has access to the original repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Fork the repository
	forkedRepo, err := s.client.ForkRepository(owner, repo, organization)
	if err != nil {
		return nil, fmt.Errorf("failed to fork repository: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner":        owner,
		"repo":         repo,
		"user":         user.UID,
		"organization": organization,
	}).Info("Repository forked")

	return forkedRepo, nil
}

// ListBranches lists all branches in a repository
func (s *Service) ListBranches(ctx context.Context, owner, repo string, user *models.User, token string) ([]*Branch, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// List branches
	branches, err := s.client.ListBranches(owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	return branches, nil
}

// GetBranch gets a specific branch
func (s *Service) GetBranch(ctx context.Context, owner, repo, branch string, user *models.User, token string) (*Branch, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Get branch
	branchInfo, err := s.client.GetBranch(owner, repo, branch)
	if err != nil {
		return nil, fmt.Errorf("failed to get branch: %w", err)
	}

	return branchInfo, nil
}

// CreateBranch creates a new branch
func (s *Service) CreateBranch(ctx context.Context, owner, repo, branchName, oldBranchName string, user *models.User, token string) (*Branch, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Create branch
	branch, err := s.client.CreateBranch(owner, repo, branchName, oldBranchName)
	if err != nil {
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner":     owner,
		"repo":      repo,
		"branch":    branchName,
		"from":      oldBranchName,
		"user":      user.UID,
	}).Info("Branch created")

	return branch, nil
}

// DeleteBranch deletes a branch
func (s *Service) DeleteBranch(ctx context.Context, owner, repo, branch string, user *models.User, token string) error {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Delete branch
	if err := s.client.DeleteBranch(owner, repo, branch); err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner":  owner,
		"repo":   repo,
		"branch": branch,
		"user":   user.UID,
	}).Info("Branch deleted")

	return nil
}

// ListCommits lists commits in a repository
func (s *Service) ListCommits(ctx context.Context, owner, repo string, opts *CommitListOptions, user *models.User, token string) ([]*Commit, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// List commits
	commits, err := s.client.ListCommits(owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list commits: %w", err)
	}

	return commits, nil
}

// GetCommit gets a specific commit
func (s *Service) GetCommit(ctx context.Context, owner, repo, sha string, user *models.User, token string) (*Commit, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Get commit
	commit, err := s.client.GetCommit(owner, repo, sha)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	return commit, nil
}

// ListTags lists all tags in a repository
func (s *Service) ListTags(ctx context.Context, owner, repo string, user *models.User, token string) ([]*Tag, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// List tags
	tags, err := s.client.ListTags(owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	return tags, nil
}

// CreateTag creates a new tag
func (s *Service) CreateTag(ctx context.Context, owner, repo, tagName, target, message string, user *models.User, token string) (*Tag, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Create tag
	tag, err := s.client.CreateTag(owner, repo, tagName, target, message)
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner":  owner,
		"repo":   repo,
		"tag":    tagName,
		"target": target,
		"user":   user.UID,
	}).Info("Tag created")

	return tag, nil
}

// DeleteTag deletes a tag
func (s *Service) DeleteTag(ctx context.Context, owner, repo, tag string, user *models.User, token string) error {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Delete tag
	if err := s.client.DeleteTag(owner, repo, tag); err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner": owner,
		"repo":  repo,
		"tag":   tag,
		"user":  user.UID,
	}).Info("Tag deleted")

	return nil
}

// Pull Request Operations

// ListPullRequests lists pull requests in a repository
func (s *Service) ListPullRequests(ctx context.Context, owner, repo, state string, page, limit int, user *models.User, token string) ([]*PullRequest, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// List pull requests
	prs, err := s.client.ListPullRequests(owner, repo, state, page, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests: %w", err)
	}

	return prs, nil
}

// GetPullRequest gets a specific pull request
func (s *Service) GetPullRequest(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (*PullRequest, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Get pull request
	pr, err := s.client.GetPullRequest(owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}

	return pr, nil
}

// CreatePullRequest creates a new pull request
func (s *Service) CreatePullRequest(ctx context.Context, owner, repo string, req *CreatePullRequestRequest, user *models.User, token string) (*PullRequest, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Create pull request
	pr, err := s.client.CreatePullRequest(owner, repo, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner":  owner,
		"repo":   repo,
		"title":  req.Title,
		"number": pr.Number,
		"user":   user.UID,
	}).Info("Pull request created")

	return pr, nil
}

// UpdatePullRequest updates a pull request
func (s *Service) UpdatePullRequest(ctx context.Context, owner, repo string, number int64, req *UpdatePullRequestRequest, user *models.User, token string) (*PullRequest, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Update pull request
	pr, err := s.client.UpdatePullRequest(owner, repo, number, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update pull request: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner":  owner,
		"repo":   repo,
		"number": number,
		"user":   user.UID,
	}).Info("Pull request updated")

	return pr, nil
}

// MergePullRequest merges a pull request
func (s *Service) MergePullRequest(ctx context.Context, owner, repo string, number int64, req *MergePullRequestRequest, user *models.User, token string) error {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Merge pull request
	if err := s.client.MergePullRequest(owner, repo, number, req); err != nil {
		return fmt.Errorf("failed to merge pull request: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner":  owner,
		"repo":   repo,
		"number": number,
		"method": req.Do,
		"user":   user.UID,
	}).Info("Pull request merged")

	return nil
}

// IsPullRequestMerged checks if a pull request is merged
func (s *Service) IsPullRequestMerged(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (bool, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return false, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Check if merged
	merged, err := s.client.IsPullRequestMerged(owner, repo, number)
	if err != nil {
		return false, fmt.Errorf("failed to check if pull request is merged: %w", err)
	}

	return merged, nil
}

// ListPRComments lists comments on a pull request
func (s *Service) ListPRComments(ctx context.Context, owner, repo string, number int64, user *models.User, token string) ([]*PRComment, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// List comments
	comments, err := s.client.ListPRComments(owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	return comments, nil
}

// CreatePRComment creates a comment on a pull request
func (s *Service) CreatePRComment(ctx context.Context, owner, repo string, number int64, req *CreatePRCommentRequest, user *models.User, token string) (*PRComment, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Create comment
	comment, err := s.client.CreatePRComment(owner, repo, number, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner":  owner,
		"repo":   repo,
		"number": number,
		"user":   user.UID,
	}).Info("PR comment created")

	return comment, nil
}

// ListPRReviews lists reviews on a pull request
func (s *Service) ListPRReviews(ctx context.Context, owner, repo string, number int64, user *models.User, token string) ([]*PRReview, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// List reviews
	reviews, err := s.client.ListPRReviews(owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to list reviews: %w", err)
	}

	return reviews, nil
}

// CreatePRReview creates a review on a pull request
func (s *Service) CreatePRReview(ctx context.Context, owner, repo string, number int64, req *CreatePRReviewRequest, user *models.User, token string) (*PRReview, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Create review
	review, err := s.client.CreatePRReview(owner, repo, number, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create review: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner":  owner,
		"repo":   repo,
		"number": number,
		"state":  req.Event,
		"user":   user.UID,
	}).Info("PR review created")

	return review, nil
}

// ListPRFiles lists files changed in a pull request
func (s *Service) ListPRFiles(ctx context.Context, owner, repo string, number int64, user *models.User, token string) ([]*PRFile, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return nil, fmt.Errorf("access denied or repository not found: %w", err)
	}

	// List files
	files, err := s.client.ListPRFiles(owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to list PR files: %w", err)
	}

	return files, nil
}

// GetPRDiff gets the diff of a pull request
func (s *Service) GetPRDiff(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (string, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return "", fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Get diff
	diff, err := s.client.GetPRDiff(owner, repo, number)
	if err != nil {
		return "", fmt.Errorf("failed to get PR diff: %w", err)
	}

	return diff, nil
}

// GetPRPatch gets the patch of a pull request
func (s *Service) GetPRPatch(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (string, error) {
	// Check if user has access to the repository
	_, err := s.GetRepository(ctx, user, owner, repo, token)
	if err != nil {
		return "", fmt.Errorf("access denied or repository not found: %w", err)
	}

	// Get patch
	patch, err := s.client.GetPRPatch(owner, repo, number)
	if err != nil {
		return "", fmt.Errorf("failed to get PR patch: %w", err)
	}

	return patch, nil
}

// ========================
// User Sync Operations
// ========================

// SyncLDAPUserToGitea syncs an LDAP user to Gitea
// Creates the user in Gitea if they don't exist, updates if they do
func (s *Service) SyncLDAPUserToGitea(ctx context.Context, ldapUser *models.User, defaultPassword string) (*GiteaUser, error) {
	s.logger.WithFields(logrus.Fields{
		"uid":   ldapUser.UID,
		"email": ldapUser.Mail,
	}).Info("Syncing LDAP user to Gitea")

	// Check if user exists in Gitea
	giteaUser, err := s.client.GetUser(ldapUser.UID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if user exists: %w", err)
	}

	if giteaUser == nil {
		// User doesn't exist, create them
		createReq := &CreateUserRequest{
			Username:           ldapUser.UID,
			Email:              ldapUser.Mail,
			FullName:           ldapUser.CN,
			Password:           defaultPassword,
			MustChangePassword: false,
			SendNotify:         false,
			Visibility:         "public",
		}

		giteaUser, err = s.client.CreateUser(createReq)
		if err != nil {
			return nil, fmt.Errorf("failed to create user in Gitea: %w", err)
		}

		s.logger.WithFields(logrus.Fields{
			"uid":      ldapUser.UID,
			"gitea_id": giteaUser.ID,
		}).Info("Created user in Gitea")
	} else {
		// User exists, update their info
		loginName := ldapUser.UID
		updateReq := &UpdateUserRequest{
			LoginName: &loginName,
			Email:     &ldapUser.Mail,
			FullName:  &ldapUser.CN,
		}

		giteaUser, err = s.client.UpdateUser(ldapUser.UID, updateReq)
		if err != nil {
			return nil, fmt.Errorf("failed to update user in Gitea: %w", err)
		}

		s.logger.WithFields(logrus.Fields{
			"uid":      ldapUser.UID,
			"gitea_id": giteaUser.ID,
		}).Info("Updated user in Gitea")
	}

	return giteaUser, nil
}

// convertLDAPUserToModelsUser converts ldap.User to models.User
func convertLDAPUserToModelsUser(ldapUser *ldap.User) *models.User {
	return &models.User{
		UID:          ldapUser.UID,
		CN:           ldapUser.CN,
		SN:           ldapUser.SN,
		GivenName:    ldapUser.GivenName,
		Mail:         ldapUser.Mail,
		Department:   ldapUser.Department,
		UIDNumber:    ldapUser.UIDNumber,
		GIDNumber:    ldapUser.GIDNumber,
		HomeDir:      ldapUser.HomeDir,
		Repositories: ldapUser.Repositories,
	}
}

// SyncAllLDAPUsersToGitea syncs all LDAP users to Gitea
func (s *Service) SyncAllLDAPUsersToGitea(ctx context.Context, token string, defaultPassword string) ([]*GiteaUser, error) {
	s.logger.Info("Starting sync of all LDAP users to Gitea")

	// Get all users from LDAP
	ldapUsers, err := s.ldapClient.ListAllUsers(token)
	if err != nil {
		return nil, fmt.Errorf("failed to list LDAP users: %w", err)
	}

	s.logger.WithField("count", len(ldapUsers)).Info("Found LDAP users to sync")

	if len(ldapUsers) == 0 {
		s.logger.Warn("No LDAP users found — check LDAP Manager and OpenLDAP connectivity")
		return nil, nil
	}

	giteaUsers := make([]*GiteaUser, 0, len(ldapUsers))
	var syncErrors []string

	for _, ldapUser := range ldapUsers {
		modelsUser := convertLDAPUserToModelsUser(ldapUser)

		giteaUser, err := s.SyncLDAPUserToGitea(ctx, modelsUser, defaultPassword)
		if err != nil {
			errMsg := fmt.Sprintf("failed to sync user %s: %v", ldapUser.UID, err)
			s.logger.Error(errMsg)
			syncErrors = append(syncErrors, errMsg)
			continue
		}
		giteaUsers = append(giteaUsers, giteaUser)
	}

	if len(syncErrors) > 0 {
		s.logger.WithField("error_count", len(syncErrors)).Warn("Some users failed to sync")
	}

	// If ALL users failed, return an error so the caller knows
	if len(giteaUsers) == 0 && len(syncErrors) > 0 {
		return nil, fmt.Errorf("all %d users failed to sync: %s", len(syncErrors), syncErrors[0])
	}

	s.logger.WithField("synced_count", len(giteaUsers)).Info("Completed LDAP user sync")

	return giteaUsers, nil
}

// ========================
// Gitea → LDAP Repo Sync
// ========================

// RepoSyncResult holds the result of syncing repos for a single user
type RepoSyncResult struct {
	UID          string   `json:"uid"`
	ReposCount   int      `json:"reposCount"`
	Repositories []string `json:"repositories"`
}

// SyncGiteaReposToLDAP syncs a single user's Gitea repos to their LDAP githubRepository attribute
func (s *Service) SyncGiteaReposToLDAP(ctx context.Context, uid string, token string) (*RepoSyncResult, error) {
	s.logger.WithField("uid", uid).Info("Syncing Gitea repos to LDAP for user")

	// Get all repos from Gitea
	allRepos, err := s.client.ListRepositories()
	if err != nil {
		return nil, fmt.Errorf("failed to list Gitea repositories: %w", err)
	}

	// Filter repos owned by this user
	userRepos := make([]string, 0)
	for _, repo := range allRepos {
		if strings.EqualFold(repo.Owner.Login, uid) {
			userRepos = append(userRepos, repo.FullName)
		}
	}

	// Update LDAP with the user's repos
	if err := s.ldapClient.AssignReposToUser(ctx, uid, userRepos, token); err != nil {
		return nil, fmt.Errorf("failed to assign repos to user %s in LDAP: %w", uid, err)
	}

	s.logger.WithFields(logrus.Fields{
		"uid":        uid,
		"reposCount": len(userRepos),
	}).Info("Synced Gitea repos to LDAP")

	return &RepoSyncResult{
		UID:          uid,
		ReposCount:   len(userRepos),
		Repositories: userRepos,
	}, nil
}

// SyncAllGiteaReposToLDAP syncs all users' Gitea repos to their LDAP githubRepository attributes
func (s *Service) SyncAllGiteaReposToLDAP(ctx context.Context, token string) ([]*RepoSyncResult, error) {
	s.logger.Info("Starting sync of all Gitea repos to LDAP")

	// Get all LDAP users
	ldapUsers, err := s.ldapClient.ListAllUsers(token)
	if err != nil {
		return nil, fmt.Errorf("failed to list LDAP users: %w", err)
	}

	// Get all repos from Gitea
	allRepos, err := s.client.ListRepositories()
	if err != nil {
		return nil, fmt.Errorf("failed to list Gitea repositories: %w", err)
	}

	// Group repos by owner login (case-insensitive)
	reposByOwner := make(map[string][]string)
	for _, repo := range allRepos {
		ownerKey := strings.ToLower(repo.Owner.Login)
		reposByOwner[ownerKey] = append(reposByOwner[ownerKey], repo.FullName)
	}

	results := make([]*RepoSyncResult, 0, len(ldapUsers))
	syncErrors := make([]string, 0)

	for _, ldapUser := range ldapUsers {
		userRepos := reposByOwner[strings.ToLower(ldapUser.UID)]
		if userRepos == nil {
			userRepos = []string{}
		}

		if err := s.ldapClient.AssignReposToUser(ctx, ldapUser.UID, userRepos, token); err != nil {
			errMsg := fmt.Sprintf("failed to sync repos for user %s: %v", ldapUser.UID, err)
			s.logger.Error(errMsg)
			syncErrors = append(syncErrors, errMsg)
			continue
		}

		results = append(results, &RepoSyncResult{
			UID:          ldapUser.UID,
			ReposCount:   len(userRepos),
			Repositories: userRepos,
		})
	}

	if len(syncErrors) > 0 {
		s.logger.WithField("error_count", len(syncErrors)).Warn("Some users failed repo sync")
	}

	s.logger.WithField("synced_count", len(results)).Info("Completed Gitea repos to LDAP sync")

	return results, nil
}

// GetGiteaUser gets a Gitea user by username
func (s *Service) GetGiteaUser(ctx context.Context, username string) (*GiteaUser, error) {
	return s.client.GetUser(username)
}

// SearchGiteaUsers searches for Gitea users
func (s *Service) SearchGiteaUsers(ctx context.Context, query string, limit int) ([]*GiteaUser, error) {
	return s.client.SearchUsers(query, limit)
}
