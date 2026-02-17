package prometheus

import (
        promclient "github.com/prometheus/client_golang/prometheus"
)

// Business-level metrics for Gitea Service
// These track actual business operations, not just HTTP requests

var (
        // ═══════════════════════════════════════════════════════════════════════════
        // REPOSITORY METRICS
        // ═══════════════════════════════════════════════════════════════════════════

        // ReposCreatedTotal - Counter of repositories created, labeled by visibility
        ReposCreatedTotal = promclient.NewCounterVec(
                promclient.CounterOpts{
                        Name: "gitea_repos_created_total",
                        Help: "Total number of repositories created in Gitea",
                },
                []string{"visibility"}, // "public" or "private"
        )

        // ReposDeletedTotal - Counter of repositories deleted
        ReposDeletedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_repos_deleted_total",
                        Help: "Total number of repositories deleted from Gitea",
                },
        )

        // ReposTotal - Gauge of current total repositories
        ReposTotal = promclient.NewGauge(
                promclient.GaugeOpts{
                        Name: "gitea_repos_total",
                        Help: "Current total number of repositories in Gitea",
                },
        )

        // ReposMigratedTotal - Counter of repositories migrated
        ReposMigratedTotal = promclient.NewCounterVec(
                promclient.CounterOpts{
                        Name: "gitea_repos_migrated_total",
                        Help: "Total number of repositories migrated to Gitea",
                },
                []string{"service"}, // "github", "gitlab", "gitea", "gogs", etc.
        )

        // ReposForkedTotal - Counter of repositories forked
        ReposForkedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_repos_forked_total",
                        Help: "Total number of repositories forked in Gitea",
                },
        )

        // ═══════════════════════════════════════════════════════════════════════════
        // PULL REQUEST METRICS
        // ═══════════════════════════════════════════════════════════════════════════

        // PRsCreatedTotal - Counter of pull requests created
        PRsCreatedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_prs_created_total",
                        Help: "Total number of pull requests created in Gitea",
                },
        )

        // PRsMergedTotal - Counter of pull requests merged
        PRsMergedTotal = promclient.NewCounterVec(
                promclient.CounterOpts{
                        Name: "gitea_prs_merged_total",
                        Help: "Total number of pull requests merged in Gitea",
                },
                []string{"merge_method"}, // "merge", "squash", "rebase"
        )

        // PRsOpenTotal - Gauge of currently open pull requests
        PRsOpenTotal = promclient.NewGauge(
                promclient.GaugeOpts{
                        Name: "gitea_prs_open_total",
                        Help: "Current number of open pull requests in Gitea",
                },
        )

        // PRsUpdatedTotal - Counter of pull requests updated
        PRsUpdatedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_prs_updated_total",
                        Help: "Total number of pull requests updated in Gitea",
                },
        )

        // PRCommentsCreatedTotal - Counter of PR comments created
        PRCommentsCreatedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_pr_comments_created_total",
                        Help: "Total number of pull request comments created",
                },
        )

        // PRReviewsCreatedTotal - Counter of PR reviews created
        PRReviewsCreatedTotal = promclient.NewCounterVec(
                promclient.CounterOpts{
                        Name: "gitea_pr_reviews_created_total",
                        Help: "Total number of pull request reviews created",
                },
                []string{"state"}, // "approved", "changes_requested", "commented"
        )

        // ═══════════════════════════════════════════════════════════════════════════
        // BRANCH & TAG METRICS
        // ═══════════════════════════════════════════════════════════════════════════

        // BranchesCreatedTotal - Counter of branches created
        BranchesCreatedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_branches_created_total",
                        Help: "Total number of branches created in Gitea",
                },
        )

        // BranchesDeletedTotal - Counter of branches deleted
        BranchesDeletedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_branches_deleted_total",
                        Help: "Total number of branches deleted from Gitea",
                },
        )

        // TagsCreatedTotal - Counter of tags created
        TagsCreatedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_tags_created_total",
                        Help: "Total number of tags created in Gitea",
                },
        )

        // TagsDeletedTotal - Counter of tags deleted
        TagsDeletedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_tags_deleted_total",
                        Help: "Total number of tags deleted from Gitea",
                },
        )

        // ═══════════════════════════════════════════════════════════════════════════
        // COLLABORATION METRICS
        // ═══════════════════════════════════════════════════════════════════════════

        // CollaboratorsAddedTotal - Counter of collaborators added
        CollaboratorsAddedTotal = promclient.NewCounterVec(
                promclient.CounterOpts{
                        Name: "gitea_collaborators_added_total",
                        Help: "Total number of collaborators added to repositories",
                },
                []string{"permission"}, // "read", "write", "admin"
        )

        // CollaboratorsRemovedTotal - Counter of collaborators removed
        CollaboratorsRemovedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_collaborators_removed_total",
                        Help: "Total number of collaborators removed from repositories",
                },
        )

        // ═══════════════════════════════════════════════════════════════════════════
        // USER SYNC METRICS
        // ═══════════════════════════════════════════════════════════════════════════

        // UsersSyncedTotal - Counter of users synced from LDAP to Gitea
        UsersSyncedTotal = promclient.NewCounterVec(
                promclient.CounterOpts{
                        Name: "gitea_users_synced_total",
                        Help: "Total number of users synced from LDAP to Gitea",
                },
                []string{"status"}, // "created", "updated", "error"
        )

        // UserSyncDuration - Histogram of user sync duration
        UserSyncDuration = promclient.NewHistogram(
                promclient.HistogramOpts{
                        Name:    "gitea_user_sync_duration_seconds",
                        Help:    "Duration of individual user sync operations in seconds",
                        Buckets: promclient.DefBuckets,
                },
        )

        // BatchUserSyncDuration - Histogram of batch user sync duration
        BatchUserSyncDuration = promclient.NewHistogram(
                promclient.HistogramOpts{
                        Name:    "gitea_batch_user_sync_duration_seconds",
                        Help:    "Duration of batch user sync operations in seconds",
                        Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
                },
        )

        // ═══════════════════════════════════════════════════════════════════════════
        // REPO SYNC METRICS
        // ═══════════════════════════════════════════════════════════════════════════

        // RepoSyncsTotal - Counter of repo sync operations
        RepoSyncsTotal = promclient.NewCounterVec(
                promclient.CounterOpts{
                        Name: "gitea_repo_syncs_total",
                        Help: "Total number of repository sync operations to LDAP",
                },
                []string{"status"}, // "success", "error"
        )

        // ReposSyncedTotal - Counter of repos synced to LDAP
        ReposSyncedTotal = promclient.NewCounter(
                promclient.CounterOpts{
                        Name: "gitea_repos_synced_total",
                        Help: "Total number of repositories synced to LDAP",
                },
        )

        // RepoSyncDuration - Histogram of repo sync duration
        RepoSyncDuration = promclient.NewHistogram(
                promclient.HistogramOpts{
                        Name:    "gitea_repo_sync_duration_seconds",
                        Help:    "Duration of repository sync operations in seconds",
                        Buckets: promclient.DefBuckets,
                },
        )

        // ═══════════════════════════════════════════════════════════════════════════
        // OPERATION DURATION METRICS (Business-level, distinct from HTTP metrics)
        // ═══════════════════════════════════════════════════════════════════════════

        // OperationDuration - Histogram of Gitea business operation durations
        OperationDuration = promclient.NewHistogramVec(
                promclient.HistogramOpts{
                        Name:    "gitea_business_operation_duration_seconds",
                        Help:    "Duration of Gitea business operations in seconds",
                        Buckets: promclient.DefBuckets, // .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
                },
                []string{"operation", "success"}, // operation name, "true" or "false"
        )

        // OperationsTotal - Counter of all Gitea business operations
        OperationsTotal = promclient.NewCounterVec(
                promclient.CounterOpts{
                        Name: "gitea_business_operations_total",
                        Help: "Total number of Gitea business operations",
                },
                []string{"operation", "success"}, // operation name, "true" or "false"
        )

        // ═══════════════════════════════════════════════════════════════════════════
        // GITEA USER METRICS
        // ═══════════════════════════════════════════════════════════════════════════

        // GiteaUsersTotal - Gauge of total Gitea users
        GiteaUsersTotal = promclient.NewGauge(
                promclient.GaugeOpts{
                        Name: "gitea_users_total",
                        Help: "Current total number of users in Gitea",
                },
        )
)

// Init registers all metrics with Prometheus
func Init() {
        promclient.MustRegister(
                ReposCreatedTotal,
                ReposDeletedTotal,
                ReposTotal,
                ReposMigratedTotal,
                ReposForkedTotal,
                PRsCreatedTotal,
                PRsMergedTotal,
                PRsOpenTotal,
                PRsUpdatedTotal,
                PRCommentsCreatedTotal,
                PRReviewsCreatedTotal,
                BranchesCreatedTotal,
                BranchesDeletedTotal,
                TagsCreatedTotal,
                TagsDeletedTotal,
                CollaboratorsAddedTotal,
                CollaboratorsRemovedTotal,
                UsersSyncedTotal,
                UserSyncDuration,
                BatchUserSyncDuration,
                RepoSyncsTotal,
                ReposSyncedTotal,
                RepoSyncDuration,
                OperationDuration,
                OperationsTotal,
                GiteaUsersTotal,
        )
}
