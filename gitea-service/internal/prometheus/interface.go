package prometheus

import (
	"context"

	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/devplatform/gitea-service/internal/models"
)

// GiteaServiceInterface defines all methods from gitea.Service that are used by GraphQL
// This allows us to wrap the Service with metrics collection
type GiteaServiceInterface interface {
	// ═══════════════════════════════════════════════════════════════════════════
	// REPOSITORY OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// GetUserRepositories gets all repositories accessible by a user
	GetUserRepositories(ctx context.Context, user *models.User, token string) ([]*gitea.Repository, error)

	// GetRepository gets a specific repository if user has access
	GetRepository(ctx context.Context, user *models.User, owner, name string, token string) (*gitea.Repository, error)

	// SearchUserRepositories searches repositories accessible by user
	SearchUserRepositories(ctx context.Context, user *models.User, query string, limit int, token string) ([]*gitea.Repository, error)

	// CreateRepository creates a new repository
	CreateRepository(ctx context.Context, owner string, req *gitea.CreateRepositoryRequest, user *models.User) (*gitea.Repository, error)

	// MigrateRepository migrates a repository from an external source
	MigrateRepository(ctx context.Context, req *gitea.MigrateRepositoryRequest, user *models.User) (*gitea.Repository, error)

	// ForkRepository forks a repository
	ForkRepository(ctx context.Context, owner, repo, organization string, user *models.User, token string) (*gitea.Repository, error)

	// GetRepositoryStats gets statistics about user's repositories
	GetRepositoryStats(ctx context.Context, user *models.User, token string) (*gitea.RepositoryStats, error)

	// ═══════════════════════════════════════════════════════════════════════════
	// BRANCH OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// ListBranches lists all branches in a repository
	ListBranches(ctx context.Context, owner, repo string, user *models.User, token string) ([]*gitea.Branch, error)

	// GetBranch gets a specific branch
	GetBranch(ctx context.Context, owner, repo, branch string, user *models.User, token string) (*gitea.Branch, error)

	// CreateBranch creates a new branch
	CreateBranch(ctx context.Context, owner, repo, branchName, oldBranchName string, user *models.User, token string) (*gitea.Branch, error)

	// DeleteBranch deletes a branch
	DeleteBranch(ctx context.Context, owner, repo, branch string, user *models.User, token string) error

	// ═══════════════════════════════════════════════════════════════════════════
	// COMMIT OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// ListCommits lists commits in a repository
	ListCommits(ctx context.Context, owner, repo string, opts *gitea.CommitListOptions, user *models.User, token string) ([]*gitea.Commit, error)

	// GetCommit gets a specific commit
	GetCommit(ctx context.Context, owner, repo, sha string, user *models.User, token string) (*gitea.Commit, error)

	// ═══════════════════════════════════════════════════════════════════════════
	// TAG OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// ListTags lists all tags in a repository
	ListTags(ctx context.Context, owner, repo string, user *models.User, token string) ([]*gitea.Tag, error)

	// CreateTag creates a new tag
	CreateTag(ctx context.Context, owner, repo, tagName, target, message string, user *models.User, token string) (*gitea.Tag, error)

	// DeleteTag deletes a tag
	DeleteTag(ctx context.Context, owner, repo, tag string, user *models.User, token string) error

	// ═══════════════════════════════════════════════════════════════════════════
	// PULL REQUEST OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// ListPullRequests lists pull requests in a repository
	ListPullRequests(ctx context.Context, owner, repo, state string, page, limit int, user *models.User, token string) ([]*gitea.PullRequest, error)

	// GetPullRequest gets a specific pull request
	GetPullRequest(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (*gitea.PullRequest, error)

	// CreatePullRequest creates a new pull request
	CreatePullRequest(ctx context.Context, owner, repo string, req *gitea.CreatePullRequestRequest, user *models.User, token string) (*gitea.PullRequest, error)

	// UpdatePullRequest updates a pull request
	UpdatePullRequest(ctx context.Context, owner, repo string, number int64, req *gitea.UpdatePullRequestRequest, user *models.User, token string) (*gitea.PullRequest, error)

	// MergePullRequest merges a pull request
	MergePullRequest(ctx context.Context, owner, repo string, number int64, req *gitea.MergePullRequestRequest, user *models.User, token string) error

	// IsPullRequestMerged checks if a pull request is merged
	IsPullRequestMerged(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (bool, error)

	// ListPRComments lists comments on a pull request
	ListPRComments(ctx context.Context, owner, repo string, number int64, user *models.User, token string) ([]*gitea.PRComment, error)

	// CreatePRComment creates a comment on a pull request
	CreatePRComment(ctx context.Context, owner, repo string, number int64, req *gitea.CreatePRCommentRequest, user *models.User, token string) (*gitea.PRComment, error)

	// ListPRReviews lists reviews on a pull request
	ListPRReviews(ctx context.Context, owner, repo string, number int64, user *models.User, token string) ([]*gitea.PRReview, error)

	// CreatePRReview creates a review on a pull request
	CreatePRReview(ctx context.Context, owner, repo string, number int64, req *gitea.CreatePRReviewRequest, user *models.User, token string) (*gitea.PRReview, error)

	// ListPRFiles lists files changed in a pull request
	ListPRFiles(ctx context.Context, owner, repo string, number int64, user *models.User, token string) ([]*gitea.PRFile, error)

	// GetPRDiff gets the diff of a pull request
	GetPRDiff(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (string, error)

	// GetPRPatch gets the patch of a pull request
	GetPRPatch(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (string, error)

	// ═══════════════════════════════════════════════════════════════════════════
	// USER SYNC OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// SyncLDAPUserToGitea syncs an LDAP user to Gitea
	SyncLDAPUserToGitea(ctx context.Context, ldapUser *models.User, defaultPassword string) (*gitea.GiteaUser, error)

	// SyncAllLDAPUsersToGitea syncs all LDAP users to Gitea
	SyncAllLDAPUsersToGitea(ctx context.Context, token string, defaultPassword string) ([]*gitea.GiteaUser, error)

	// GetGiteaUser gets a Gitea user by username
	GetGiteaUser(ctx context.Context, username string) (*gitea.GiteaUser, error)

	// SearchGiteaUsers searches for Gitea users
	SearchGiteaUsers(ctx context.Context, query string, limit int) ([]*gitea.GiteaUser, error)

	// ═══════════════════════════════════════════════════════════════════════════
	// REPO SYNC OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// SyncGiteaReposToLDAP syncs a single user's Gitea repos to their LDAP githubRepository attribute
	SyncGiteaReposToLDAP(ctx context.Context, uid string, token string) (*gitea.RepoSyncResult, error)

	// SyncAllGiteaReposToLDAP syncs all users' Gitea repos to their LDAP githubRepository attributes
	SyncAllGiteaReposToLDAP(ctx context.Context, token string) ([]*gitea.RepoSyncResult, error)
}
