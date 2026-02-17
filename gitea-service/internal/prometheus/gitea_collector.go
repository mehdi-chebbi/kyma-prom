package prometheus

import (
	"context"
	"time"

	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/devplatform/gitea-service/internal/models"
)

// GiteaCollector wraps a GiteaServiceInterface and records metrics for all operations
type GiteaCollector struct {
	next GiteaServiceInterface
}

// NewGiteaCollector creates a new instrumented wrapper around a GiteaServiceInterface
func NewGiteaCollector(next GiteaServiceInterface) *GiteaCollector {
	return &GiteaCollector{next: next}
}

// recordOperation records duration and count for an operation
func recordOperation(operation string, start time.Time, err error) {
	success := "true"
	if err != nil {
		success = "false"
	}

	OperationDuration.WithLabelValues(operation, success).Observe(time.Since(start).Seconds())
	OperationsTotal.WithLabelValues(operation, success).Inc()
}

// ═══════════════════════════════════════════════════════════════════════════
// REPOSITORY OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *GiteaCollector) GetUserRepositories(ctx context.Context, user *models.User, token string) ([]*gitea.Repository, error) {
	start := time.Now()
	repos, err := c.next.GetUserRepositories(ctx, user, token)
	recordOperation("get_user_repositories", start, err)

	// Update repo count gauge
	if err == nil && repos != nil {
		ReposTotal.Set(float64(len(repos)))
	}

	return repos, err
}

func (c *GiteaCollector) GetRepository(ctx context.Context, user *models.User, owner, name string, token string) (*gitea.Repository, error) {
	start := time.Now()
	repo, err := c.next.GetRepository(ctx, user, owner, name, token)
	recordOperation("get_repository", start, err)
	return repo, err
}

func (c *GiteaCollector) SearchUserRepositories(ctx context.Context, user *models.User, query string, limit int, token string) ([]*gitea.Repository, error) {
	start := time.Now()
	repos, err := c.next.SearchUserRepositories(ctx, user, query, limit, token)
	recordOperation("search_user_repositories", start, err)
	return repos, err
}

func (c *GiteaCollector) CreateRepository(ctx context.Context, owner string, req *gitea.CreateRepositoryRequest, user *models.User) (*gitea.Repository, error) {
	start := time.Now()
	repo, err := c.next.CreateRepository(ctx, owner, req, user)
	recordOperation("create_repository", start, err)

	if err == nil && repo != nil {
		// Increment counter with visibility label
		visibility := "public"
		if repo.Private {
			visibility = "private"
		}
		ReposCreatedTotal.WithLabelValues(visibility).Inc()
	}

	return repo, err
}

func (c *GiteaCollector) MigrateRepository(ctx context.Context, req *gitea.MigrateRepositoryRequest, user *models.User) (*gitea.Repository, error) {
	start := time.Now()
	repo, err := c.next.MigrateRepository(ctx, req, user)
	recordOperation("migrate_repository", start, err)

	if err == nil {
		// Increment migration counter with service label
		service := req.Service
		if service == "" {
			service = "unknown"
		}
		ReposMigratedTotal.WithLabelValues(service).Inc()
	}

	return repo, err
}

func (c *GiteaCollector) ForkRepository(ctx context.Context, owner, repo, organization string, user *models.User, token string) (*gitea.Repository, error) {
	start := time.Now()
	forkedRepo, err := c.next.ForkRepository(ctx, owner, repo, organization, user, token)
	recordOperation("fork_repository", start, err)

	if err == nil {
		ReposForkedTotal.Inc()
	}

	return forkedRepo, err
}

func (c *GiteaCollector) GetRepositoryStats(ctx context.Context, user *models.User, token string) (*gitea.RepositoryStats, error) {
	start := time.Now()
	stats, err := c.next.GetRepositoryStats(ctx, user, token)
	recordOperation("get_repository_stats", start, err)
	return stats, err
}

// ═══════════════════════════════════════════════════════════════════════════
// BRANCH OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *GiteaCollector) ListBranches(ctx context.Context, owner, repo string, user *models.User, token string) ([]*gitea.Branch, error) {
	start := time.Now()
	branches, err := c.next.ListBranches(ctx, owner, repo, user, token)
	recordOperation("list_branches", start, err)
	return branches, err
}

func (c *GiteaCollector) GetBranch(ctx context.Context, owner, repo, branch string, user *models.User, token string) (*gitea.Branch, error) {
	start := time.Now()
	branchInfo, err := c.next.GetBranch(ctx, owner, repo, branch, user, token)
	recordOperation("get_branch", start, err)
	return branchInfo, err
}

func (c *GiteaCollector) CreateBranch(ctx context.Context, owner, repo, branchName, oldBranchName string, user *models.User, token string) (*gitea.Branch, error) {
	start := time.Now()
	branch, err := c.next.CreateBranch(ctx, owner, repo, branchName, oldBranchName, user, token)
	recordOperation("create_branch", start, err)

	if err == nil {
		BranchesCreatedTotal.Inc()
	}

	return branch, err
}

func (c *GiteaCollector) DeleteBranch(ctx context.Context, owner, repo, branch string, user *models.User, token string) error {
	start := time.Now()
	err := c.next.DeleteBranch(ctx, owner, repo, branch, user, token)
	recordOperation("delete_branch", start, err)

	if err == nil {
		BranchesDeletedTotal.Inc()
	}

	return err
}

// ═══════════════════════════════════════════════════════════════════════════
// COMMIT OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *GiteaCollector) ListCommits(ctx context.Context, owner, repo string, opts *gitea.CommitListOptions, user *models.User, token string) ([]*gitea.Commit, error) {
	start := time.Now()
	commits, err := c.next.ListCommits(ctx, owner, repo, opts, user, token)
	recordOperation("list_commits", start, err)
	return commits, err
}

func (c *GiteaCollector) GetCommit(ctx context.Context, owner, repo, sha string, user *models.User, token string) (*gitea.Commit, error) {
	start := time.Now()
	commit, err := c.next.GetCommit(ctx, owner, repo, sha, user, token)
	recordOperation("get_commit", start, err)
	return commit, err
}

// ═══════════════════════════════════════════════════════════════════════════
// TAG OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *GiteaCollector) ListTags(ctx context.Context, owner, repo string, user *models.User, token string) ([]*gitea.Tag, error) {
	start := time.Now()
	tags, err := c.next.ListTags(ctx, owner, repo, user, token)
	recordOperation("list_tags", start, err)
	return tags, err
}

func (c *GiteaCollector) CreateTag(ctx context.Context, owner, repo, tagName, target, message string, user *models.User, token string) (*gitea.Tag, error) {
	start := time.Now()
	tag, err := c.next.CreateTag(ctx, owner, repo, tagName, target, message, user, token)
	recordOperation("create_tag", start, err)

	if err == nil {
		TagsCreatedTotal.Inc()
	}

	return tag, err
}

func (c *GiteaCollector) DeleteTag(ctx context.Context, owner, repo, tag string, user *models.User, token string) error {
	start := time.Now()
	err := c.next.DeleteTag(ctx, owner, repo, tag, user, token)
	recordOperation("delete_tag", start, err)

	if err == nil {
		TagsDeletedTotal.Inc()
	}

	return err
}

// ═══════════════════════════════════════════════════════════════════════════
// PULL REQUEST OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *GiteaCollector) ListPullRequests(ctx context.Context, owner, repo, state string, page, limit int, user *models.User, token string) ([]*gitea.PullRequest, error) {
	start := time.Now()
	prs, err := c.next.ListPullRequests(ctx, owner, repo, state, page, limit, user, token)
	recordOperation("list_pull_requests", start, err)

	// Update open PR gauge if listing open PRs
	if err == nil && state == "open" && prs != nil {
		PRsOpenTotal.Set(float64(len(prs)))
	}

	return prs, err
}

func (c *GiteaCollector) GetPullRequest(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (*gitea.PullRequest, error) {
	start := time.Now()
	pr, err := c.next.GetPullRequest(ctx, owner, repo, number, user, token)
	recordOperation("get_pull_request", start, err)
	return pr, err
}

func (c *GiteaCollector) CreatePullRequest(ctx context.Context, owner, repo string, req *gitea.CreatePullRequestRequest, user *models.User, token string) (*gitea.PullRequest, error) {
	start := time.Now()
	pr, err := c.next.CreatePullRequest(ctx, owner, repo, req, user, token)
	recordOperation("create_pull_request", start, err)

	if err == nil {
		PRsCreatedTotal.Inc()
	}

	return pr, err
}

func (c *GiteaCollector) UpdatePullRequest(ctx context.Context, owner, repo string, number int64, req *gitea.UpdatePullRequestRequest, user *models.User, token string) (*gitea.PullRequest, error) {
	start := time.Now()
	pr, err := c.next.UpdatePullRequest(ctx, owner, repo, number, req, user, token)
	recordOperation("update_pull_request", start, err)

	if err == nil {
		PRsUpdatedTotal.Inc()
	}

	return pr, err
}

func (c *GiteaCollector) MergePullRequest(ctx context.Context, owner, repo string, number int64, req *gitea.MergePullRequestRequest, user *models.User, token string) error {
	start := time.Now()
	err := c.next.MergePullRequest(ctx, owner, repo, number, req, user, token)
	recordOperation("merge_pull_request", start, err)

	if err == nil {
		// Increment merge counter with method label
		mergeMethod := req.Do
		if mergeMethod == "" {
			mergeMethod = "merge"
		}
		PRsMergedTotal.WithLabelValues(mergeMethod).Inc()
	}

	return err
}

func (c *GiteaCollector) IsPullRequestMerged(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (bool, error) {
	start := time.Now()
	merged, err := c.next.IsPullRequestMerged(ctx, owner, repo, number, user, token)
	recordOperation("is_pull_request_merged", start, err)
	return merged, err
}

func (c *GiteaCollector) ListPRComments(ctx context.Context, owner, repo string, number int64, user *models.User, token string) ([]*gitea.PRComment, error) {
	start := time.Now()
	comments, err := c.next.ListPRComments(ctx, owner, repo, number, user, token)
	recordOperation("list_pr_comments", start, err)
	return comments, err
}

func (c *GiteaCollector) CreatePRComment(ctx context.Context, owner, repo string, number int64, req *gitea.CreatePRCommentRequest, user *models.User, token string) (*gitea.PRComment, error) {
	start := time.Now()
	comment, err := c.next.CreatePRComment(ctx, owner, repo, number, req, user, token)
	recordOperation("create_pr_comment", start, err)

	if err == nil {
		PRCommentsCreatedTotal.Inc()
	}

	return comment, err
}

func (c *GiteaCollector) ListPRReviews(ctx context.Context, owner, repo string, number int64, user *models.User, token string) ([]*gitea.PRReview, error) {
	start := time.Now()
	reviews, err := c.next.ListPRReviews(ctx, owner, repo, number, user, token)
	recordOperation("list_pr_reviews", start, err)
	return reviews, err
}

func (c *GiteaCollector) CreatePRReview(ctx context.Context, owner, repo string, number int64, req *gitea.CreatePRReviewRequest, user *models.User, token string) (*gitea.PRReview, error) {
	start := time.Now()
	review, err := c.next.CreatePRReview(ctx, owner, repo, number, req, user, token)
	recordOperation("create_pr_review", start, err)

	if err == nil {
		// Increment review counter with state label
		state := req.Event
		if state == "" {
			state = "commented"
		}
		PRReviewsCreatedTotal.WithLabelValues(state).Inc()
	}

	return review, err
}

func (c *GiteaCollector) ListPRFiles(ctx context.Context, owner, repo string, number int64, user *models.User, token string) ([]*gitea.PRFile, error) {
	start := time.Now()
	files, err := c.next.ListPRFiles(ctx, owner, repo, number, user, token)
	recordOperation("list_pr_files", start, err)
	return files, err
}

func (c *GiteaCollector) GetPRDiff(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (string, error) {
	start := time.Now()
	diff, err := c.next.GetPRDiff(ctx, owner, repo, number, user, token)
	recordOperation("get_pr_diff", start, err)
	return diff, err
}

func (c *GiteaCollector) GetPRPatch(ctx context.Context, owner, repo string, number int64, user *models.User, token string) (string, error) {
	start := time.Now()
	patch, err := c.next.GetPRPatch(ctx, owner, repo, number, user, token)
	recordOperation("get_pr_patch", start, err)
	return patch, err
}

// ═══════════════════════════════════════════════════════════════════════════
// USER SYNC OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *GiteaCollector) SyncLDAPUserToGitea(ctx context.Context, ldapUser *models.User, defaultPassword string) (*gitea.GiteaUser, error) {
	start := time.Now()
	giteaUser, err := c.next.SyncLDAPUserToGitea(ctx, ldapUser, defaultPassword)

	UserSyncDuration.Observe(time.Since(start).Seconds())

	if err != nil {
		UsersSyncedTotal.WithLabelValues("error").Inc()
		recordOperation("sync_ldap_user_to_gitea", start, err)
		return nil, err
	}

	// We can't tell if created or updated from the return value alone,
	// but we record as "synced" for now. The service logs the actual action.
	UsersSyncedTotal.WithLabelValues("synced").Inc()
	recordOperation("sync_ldap_user_to_gitea", start, nil)

	return giteaUser, err
}

func (c *GiteaCollector) SyncAllLDAPUsersToGitea(ctx context.Context, token string, defaultPassword string) ([]*gitea.GiteaUser, error) {
	start := time.Now()
	users, err := c.next.SyncAllLDAPUsersToGitea(ctx, token, defaultPassword)

	BatchUserSyncDuration.Observe(time.Since(start).Seconds())

	if err != nil {
		recordOperation("sync_all_ldap_users_to_gitea", start, err)
		return nil, err
	}

	// Update user count gauge
	if users != nil {
		GiteaUsersTotal.Set(float64(len(users)))
	}

	recordOperation("sync_all_ldap_users_to_gitea", start, nil)

	return users, err
}

func (c *GiteaCollector) GetGiteaUser(ctx context.Context, username string) (*gitea.GiteaUser, error) {
	start := time.Now()
	user, err := c.next.GetGiteaUser(ctx, username)
	recordOperation("get_gitea_user", start, err)
	return user, err
}

func (c *GiteaCollector) SearchGiteaUsers(ctx context.Context, query string, limit int) ([]*gitea.GiteaUser, error) {
	start := time.Now()
	users, err := c.next.SearchGiteaUsers(ctx, query, limit)
	recordOperation("search_gitea_users", start, err)
	return users, err
}

// ═══════════════════════════════════════════════════════════════════════════
// REPO SYNC OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *GiteaCollector) SyncGiteaReposToLDAP(ctx context.Context, uid string, token string) (*gitea.RepoSyncResult, error) {
	start := time.Now()
	result, err := c.next.SyncGiteaReposToLDAP(ctx, uid, token)

	RepoSyncDuration.Observe(time.Since(start).Seconds())

	if err != nil {
		RepoSyncsTotal.WithLabelValues("error").Inc()
		recordOperation("sync_gitea_repos_to_ldap", start, err)
		return nil, err
	}

	RepoSyncsTotal.WithLabelValues("success").Inc()
	if result != nil {
		ReposSyncedTotal.Add(float64(result.ReposCount))
	}
	recordOperation("sync_gitea_repos_to_ldap", start, nil)

	return result, err
}

func (c *GiteaCollector) SyncAllGiteaReposToLDAP(ctx context.Context, token string) ([]*gitea.RepoSyncResult, error) {
	start := time.Now()
	results, err := c.next.SyncAllGiteaReposToLDAP(ctx, token)

	if err != nil {
		RepoSyncsTotal.WithLabelValues("error").Inc()
		recordOperation("sync_all_gitea_repos_to_ldap", start, err)
		return nil, err
	}

	// Count total repos synced
	if results != nil {
		totalRepos := 0
		for _, r := range results {
			totalRepos += r.ReposCount
		}
		ReposSyncedTotal.Add(float64(totalRepos))
	}

	RepoSyncsTotal.WithLabelValues("success").Inc()
	recordOperation("sync_all_gitea_repos_to_ldap", start, nil)

	return results, err
}
