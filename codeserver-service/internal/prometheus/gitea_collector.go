package prometheus

import (
	"context"
	"time"

	"github.com/devplatform/codeserver-service/internal/gitea"
)

// GiteaCollector wraps a GiteaClientInterface and records metrics for all operations
type GiteaCollector struct {
	next GiteaClientInterface
}

// NewGiteaCollector creates a new instrumented wrapper around a GiteaClientInterface
func NewGiteaCollector(next GiteaClientInterface) *GiteaCollector {
	return &GiteaCollector{next: next}
}

// recordGiteaOperation records Gitea API metrics
func recordGiteaOperation(operation string, start time.Time, err error) {
	success := "true"
	if err != nil {
		success = "false"
	}

	GiteaAPICallsTotal.WithLabelValues(operation, success).Inc()
	GiteaAPIDuration.WithLabelValues(operation).Observe(time.Since(start).Seconds())
}

// GetUserRepositories gets repositories accessible by the user
func (c *GiteaCollector) GetUserRepositories(ctx context.Context, token string) ([]*gitea.Repository, error) {
	start := time.Now()
	repos, err := c.next.GetUserRepositories(ctx, token)
	recordGiteaOperation("get_user_repositories", start, err)
	return repos, err
}

// ValidateRepoAccess checks if user can access the repository
func (c *GiteaCollector) ValidateRepoAccess(ctx context.Context, token, owner, repoName string) (bool, error) {
	start := time.Now()
	hasAccess, err := c.next.ValidateRepoAccess(ctx, token, owner, repoName)
	recordGiteaOperation("validate_repo_access", start, err)
	return hasAccess, err
}

// GetRepoCloneURL returns the clone URL with embedded token for private repos
func (c *GiteaCollector) GetRepoCloneURL(ctx context.Context, token, owner, repoName string) (string, error) {
	start := time.Now()
	url, err := c.next.GetRepoCloneURL(ctx, token, owner, repoName)
	recordGiteaOperation("get_repo_clone_url", start, err)
	return url, err
}

// HealthCheck checks if gitea-service is accessible
func (c *GiteaCollector) HealthCheck(ctx context.Context) error {
	start := time.Now()
	err := c.next.HealthCheck(ctx)
	recordGiteaOperation("health_check", start, err)
	return err
}
