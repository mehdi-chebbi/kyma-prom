package graphql

import (
        "context"
        "fmt"

        "github.com/devplatform/gitea-service/internal/auth"
        "github.com/devplatform/gitea-service/internal/config"
        "github.com/devplatform/gitea-service/internal/gitea"
        "github.com/devplatform/gitea-service/internal/ldap"
        "github.com/devplatform/gitea-service/internal/models"
        "github.com/devplatform/gitea-service/internal/prometheus"
        gosync "github.com/devplatform/gitea-service/internal/sync"
        "github.com/golang-jwt/jwt/v5"
        "github.com/graphql-go/graphql"
        "github.com/sirupsen/logrus"
)

// Schema represents the GraphQL schema
type Schema struct {
        schema        graphql.Schema
        giteaService  prometheus.GiteaServiceInterface
        ldapClient    *ldap.Client
        giteaClient   *gitea.Client
        collabService *gosync.CollabService
        config        *config.Config
        logger        *logrus.Logger
}

// JWT Claims (must match LDAP Manager for token validation)
type Claims struct {
        UID        string `json:"uid"`
        Mail       string `json:"mail"`
        Department string `json:"department"`
        jwt.RegisteredClaims
}

// NewSchema creates a new GraphQL schema
func NewSchema(giteaService prometheus.GiteaServiceInterface, ldapClient *ldap.Client, giteaClient *gitea.Client, collabService *gosync.CollabService, cfg *config.Config, logger *logrus.Logger) *Schema {
        s := &Schema{
                giteaService:  giteaService,
                ldapClient:    ldapClient,
                giteaClient:   giteaClient,
                collabService: collabService,
                config:        cfg,
                logger:        logger,
        }

        // Define types
        giteaRepoType := s.defineGiteaRepositoryType()
        repoStatsType := s.defineRepositoryStatsType()
        healthType := s.defineHealthType()
        paginatedReposType := s.definePaginatedRepositoriesType(giteaRepoType)
        branchType := s.defineBranchType()
        commitType := s.defineCommitType()
        tagType := s.defineTagType()

        // Define PR types
        pullRequestType, prCommentType, prReviewType, prFileType, _, _, _ := s.definePRTypes()

        // Define User Sync types
        giteaUserType := s.defineGiteaUserType()

        // Define Issue types
        issueUserType := s.defineIssueUserType()
        issueLabelType := s.defineIssueLabelType()
        issueMilestoneType := s.defineIssueMilestoneType()
        issueType := s.defineIssueType(issueUserType, issueLabelType, issueMilestoneType)
        issueCommentType := s.defineIssueCommentType(issueUserType)

        // Define Repo Sync types
        repoSyncResultType := s.defineRepoSyncResultType()

        // Define Team types
        teamType := s.defineTeamType(giteaUserType, giteaRepoType)
        syncResultType := s.defineSyncResultType(teamType)

        // Define Collab types
        groupAccessType := s.defineGroupAccessType()
        collabGroupType := s.defineCollabGroupType()

        // Define root query
        queryType := graphql.NewObject(graphql.ObjectConfig{
                Name: "Query",
                Fields: graphql.Fields{
                        "listRepositories": &graphql.Field{
                                Type: paginatedReposType,
                                Args: graphql.FieldConfigArgument{
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 10,
                                                Description:  "Number of items per page (default: 10, max: 100)",
                                        },
                                        "offset": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 0,
                                                Description:  "Number of items to skip",
                                        },
                                },
                                Resolve: s.resolveListRepositories,
                        },
                        "searchRepositories": &graphql.Field{
                                Type: paginatedReposType,
                                Args: graphql.FieldConfigArgument{
                                        "query": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Search query string",
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 10,
                                                Description:  "Number of items per page (default: 10, max: 100)",
                                        },
                                        "offset": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 0,
                                                Description:  "Number of items to skip",
                                        },
                                },
                                Resolve: s.resolveSearchRepositories,
                        },
                        "getRepository": &graphql.Field{
                                Type: giteaRepoType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "name": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                },
                                Resolve: s.resolveGetRepository,
                        },
                        "myRepositories": &graphql.Field{
                                Type: paginatedReposType,
                                Args: graphql.FieldConfigArgument{
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 10,
                                                Description:  "Number of items per page",
                                        },
                                        "offset": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 0,
                                                Description:  "Number of items to skip",
                                        },
                                },
                                Resolve: s.resolveMyRepositories,
                        },
                        "repositoryStats": &graphql.Field{
                                Type:    repoStatsType,
                                Resolve: s.resolveRepositoryStats,
                        },
                        "health": &graphql.Field{
                                Type:    healthType,
                                Resolve: s.resolveHealth,
                        },
                        "listBranches": &graphql.Field{
                                Type: graphql.NewList(branchType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                                Description:  "Page number (default: 1)",
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 100,
                                                Description:  "Items per page (default: 100, max: 500)",
                                        },
                                },
                                Resolve: s.resolveListBranches,
                        },
                        "getBranch": &graphql.Field{
                                Type: branchType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "branch": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Branch name",
                                        },
                                },
                                Resolve: s.resolveGetBranch,
                        },
                        "listCommits": &graphql.Field{
                                Type: graphql.NewList(commitType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "sha": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "SHA or branch name (optional)",
                                        },
                                        "path": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "File path to filter commits (optional)",
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                                Description:  "Page number (default: 1)",
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 50,
                                                Description:  "Items per page (default: 50, max: 100)",
                                        },
                                },
                                Resolve: s.resolveListCommits,
                        },
                        "getCommit": &graphql.Field{
                                Type: commitType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "sha": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Commit SHA",
                                        },
                                },
                                Resolve: s.resolveGetCommit,
                        },
                        "listTags": &graphql.Field{
                                Type: graphql.NewList(tagType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                                Description:  "Page number (default: 1)",
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 50,
                                                Description:  "Items per page (default: 50, max: 100)",
                                        },
                                },
                                Resolve: s.resolveListTags,
                        },
                        "listPullRequests": &graphql.Field{
                                Type: graphql.NewList(pullRequestType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "state": &graphql.ArgumentConfig{
                                                Type:         graphql.String,
                                                DefaultValue: "open",
                                                Description:  "State: open, closed, all",
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                                Description:  "Page number",
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 30,
                                                Description:  "Items per page",
                                        },
                                },
                                Resolve: s.resolveListPullRequests,
                        },
                        "getPullRequest": &graphql.Field{
                                Type: pullRequestType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Pull request number",
                                        },
                                },
                                Resolve: s.resolveGetPullRequest,
                        },
                        "listPRComments": &graphql.Field{
                                Type: graphql.NewList(prCommentType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Pull request number",
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                                Description:  "Page number (default: 1)",
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 30,
                                                Description:  "Items per page (default: 30, max: 100)",
                                        },
                                },
                                Resolve: s.resolveListPRComments,
                        },
                        "listPRReviews": &graphql.Field{
                                Type: graphql.NewList(prReviewType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Pull request number",
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                                Description:  "Page number (default: 1)",
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 30,
                                                Description:  "Items per page (default: 30, max: 100)",
                                        },
                                },
                                Resolve: s.resolveListPRReviews,
                        },
                        "listPRFiles": &graphql.Field{
                                Type: graphql.NewList(prFileType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Pull request number",
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                                Description:  "Page number (default: 1)",
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 100,
                                                Description:  "Items per page (default: 100, max: 500)",
                                        },
                                },
                                Resolve: s.resolveListPRFiles,
                        },
                        "getPRDiff": &graphql.Field{
                                Type: graphql.String,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Pull request number",
                                        },
                                },
                                Resolve: s.resolveGetPRDiff,
                        },
                        "isPRMerged": &graphql.Field{
                                Type: graphql.Boolean,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Pull request number",
                                        },
                                },
                                Resolve: s.resolveIsPRMerged,
                        },
                        // User Sync Queries
                        "giteaUser": &graphql.Field{
                                Type: giteaUserType,
                                Args: graphql.FieldConfigArgument{
                                        "username": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Gitea username",
                                        },
                                },
                                Resolve: s.resolveGetGiteaUser,
                        },
                        "searchGiteaUsers": &graphql.Field{
                                Type: graphql.NewList(giteaUserType),
                                Args: graphql.FieldConfigArgument{
                                        "query": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Search query",
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 10,
                                                Description:  "Number of results",
                                        },
                                },
                                Resolve: s.resolveSearchGiteaUsers,
                        },
                        // Issue Queries
                        "listIssues": &graphql.Field{
                                Type: graphql.NewList(issueType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Repository owner",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "state": &graphql.ArgumentConfig{
                                                Type:         graphql.String,
                                                DefaultValue: "open",
                                                Description:  "Issue state: open, closed, all",
                                        },
                                        "labels": &graphql.ArgumentConfig{
                                                Type:        graphql.NewList(graphql.String),
                                                Description: "Filter by labels",
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 30,
                                        },
                                },
                                Resolve: s.resolveListIssues,
                        },
                        "getIssue": &graphql.Field{
                                Type: issueType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.Int),
                                        },
                                },
                                Resolve: s.resolveGetIssue,
                        },
                        "listIssueComments": &graphql.Field{
                                Type: graphql.NewList(issueCommentType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.Int),
                                        },
                                },
                                Resolve: s.resolveListIssueComments,
                        },
                        "listLabels": &graphql.Field{
                                Type: graphql.NewList(issueLabelType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 30,
                                        },
                                },
                                Resolve: s.resolveListLabels,
                        },
                        "listMilestones": &graphql.Field{
                                Type: graphql.NewList(issueMilestoneType),
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "state": &graphql.ArgumentConfig{
                                                Type:         graphql.String,
                                                DefaultValue: "open",
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 30,
                                        },
                                },
                                Resolve: s.resolveListMilestones,
                        },
                        "listTeams": &graphql.Field{
                                Type:        graphql.NewList(teamType),
                                Description: "List teams in an organization",
                                Args: graphql.FieldConfigArgument{
                                        "orgName": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Organization name",
                                        },
                                        "page": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 1,
                                                Description:  "Page number",
                                        },
                                        "limit": &graphql.ArgumentConfig{
                                                Type:         graphql.Int,
                                                DefaultValue: 50,
                                                Description:  "Items per page",
                                        },
                                },
                                Resolve: s.resolveListTeams,
                        },
                        "getTeam": &graphql.Field{
                                Type:        teamType,
                                Description: "Get a team by ID",
                                Args: graphql.FieldConfigArgument{
                                        "teamId": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Team ID",
                                        },
                                },
                                Resolve: s.resolveGetTeam,
                        },
                        // Collab queries
                        "listRepoAccess": &graphql.Field{
                                Type:        graphql.NewList(groupAccessType),
                                Description: "List all groups/departments that have access to a repository",
                                Args: graphql.FieldConfigArgument{
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository URL",
                                        },
                                },
                                Resolve: s.resolveListRepoAccess,
                        },
                        "listCollabGroups": &graphql.Field{
                                Type:        graphql.NewList(collabGroupType),
                                Description: "List all dynamic collab groups",
                                Resolve: func(p graphql.ResolveParams) (interface{}, error) {
                                        if s.collabService == nil {
                                                return nil, nil
                                        }
                                        // The controller has collab group metadata but we need to convert to a list
                                        return nil, fmt.Errorf("use controller API for collab group listing")
                                },
                        },
                },
        })

        // Define root mutation
        mutationType := graphql.NewObject(graphql.ObjectConfig{
                Name: "Mutation",
                Fields: graphql.Fields{
                        "deleteRepository": &graphql.Field{
                                Type: graphql.Boolean,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "name": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                },
                                Resolve: s.resolveDeleteRepository,
                        },
                        "updateRepository": &graphql.Field{
                                Type: giteaRepoType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "name": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "description": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Repository description",
                                        },
                                        "private": &graphql.ArgumentConfig{
                                                Type:        graphql.Boolean,
                                                Description: "Make repository private",
                                        },
                                        "defaultBranch": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Default branch name",
                                        },
                                },
                                Resolve: s.resolveUpdateRepository,
                        },
                        "createRepository": &graphql.Field{
                                Type: giteaRepoType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Repository owner (optional, defaults to configured admin user)",
                                        },
                                        "name": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "description": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Repository description",
                                        },
                                        "private": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: false,
                                                Description:  "Make repository private",
                                        },
                                        "autoInit": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: true,
                                                Description:  "Initialize with README",
                                        },
                                        "gitignores": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Gitignore template (e.g., Go, Python)",
                                        },
                                        "license": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "License template (e.g., MIT, Apache-2.0)",
                                        },
                                        "readme": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "README template",
                                        },
                                        "defaultBranch": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Default branch name",
                                        },
                                },
                                Resolve: s.resolveCreateRepository,
                        },
                        "migrateRepository": &graphql.Field{
                                Type: giteaRepoType,
                                Args: graphql.FieldConfigArgument{
                                        "cloneAddr": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "URL of repository to migrate",
                                        },
                                        "repoName": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Name for new repository",
                                        },
                                        "repoOwner": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Owner of the new repository",
                                        },
                                        "mirror": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: false,
                                                Description:  "Create as mirror repository",
                                        },
                                        "private": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: false,
                                                Description:  "Make repository private",
                                        },
                                        "description": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Repository description",
                                        },
                                        "wiki": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: true,
                                                Description:  "Migrate wiki",
                                        },
                                        "milestones": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: true,
                                                Description:  "Migrate milestones",
                                        },
                                        "labels": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: true,
                                                Description:  "Migrate labels",
                                        },
                                        "issues": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: true,
                                                Description:  "Migrate issues",
                                        },
                                        "pullRequests": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: true,
                                                Description:  "Migrate pull requests",
                                        },
                                        "releases": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: true,
                                                Description:  "Migrate releases",
                                        },
                                        "authUsername": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Authentication username (for private repos)",
                                        },
                                        "authPassword": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Authentication password (for private repos)",
                                        },
                                        "authToken": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Authentication token (for private repos)",
                                        },
                                        "service": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Source service: github, gitlab, gitea, gogs",
                                        },
                                },
                                Resolve: s.resolveMigrateRepository,
                        },
                        "forkRepository": &graphql.Field{
                                Type: giteaRepoType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "organization": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Organization to fork to (optional)",
                                        },
                                },
                                Resolve: s.resolveForkRepository,
                        },
                        "createBranch": &graphql.Field{
                                Type: branchType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "branchName": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "New branch name",
                                        },
                                        "oldBranchName": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Source branch name",
                                        },
                                },
                                Resolve: s.resolveCreateBranch,
                        },
                        "deleteBranch": &graphql.Field{
                                Type: graphql.Boolean,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "branch": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Branch name to delete",
                                        },
                                },
                                Resolve: s.resolveDeleteBranch,
                        },
                        "createTag": &graphql.Field{
                                Type: tagType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "tagName": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Tag name",
                                        },
                                        "target": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Target commit SHA or branch",
                                        },
                                        "message": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Tag message",
                                        },
                                },
                                Resolve: s.resolveCreateTag,
                        },
                        "deleteTag": &graphql.Field{
                                Type: graphql.Boolean,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "tag": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Tag name to delete",
                                        },
                                },
                                Resolve: s.resolveDeleteTag,
                        },
                        "createPullRequest": &graphql.Field{
                                Type: pullRequestType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "title": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Pull request title",
                                        },
                                        "body": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Pull request description",
                                        },
                                        "head": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Source branch name",
                                        },
                                        "base": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Target branch name",
                                        },
                                },
                                Resolve: s.resolveCreatePullRequest,
                        },
                        "updatePullRequest": &graphql.Field{
                                Type: pullRequestType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Pull request number",
                                        },
                                        "title": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Pull request title",
                                        },
                                        "body": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Pull request description",
                                        },
                                        "state": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "State: open, closed",
                                        },
                                },
                                Resolve: s.resolveUpdatePullRequest,
                        },
                        "mergePullRequest": &graphql.Field{
                                Type: graphql.Boolean,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Pull request number",
                                        },
                                        "method": &graphql.ArgumentConfig{
                                                Type:         graphql.String,
                                                DefaultValue: "merge",
                                                Description:  "Merge method: merge, rebase, rebase-merge, squash",
                                        },
                                        "deleteBranchAfterMerge": &graphql.ArgumentConfig{
                                                Type:         graphql.Boolean,
                                                DefaultValue: false,
                                                Description:  "Delete source branch after merge",
                                        },
                                },
                                Resolve: s.resolveMergePullRequest,
                        },
                        "createPRComment": &graphql.Field{
                                Type: prCommentType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Pull request number",
                                        },
                                        "body": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Comment text",
                                        },
                                },
                                Resolve: s.resolveCreatePRComment,
                        },
                        "createPRReview": &graphql.Field{
                                Type: prReviewType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner username",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Pull request number",
                                        },
                                        "event": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Review type: APPROVE, REQUEST_CHANGES, COMMENT",
                                        },
                                        "body": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Review comment",
                                        },
                                },
                                Resolve: s.resolveCreatePRReview,
                        },
                        // User Sync Mutations
                        "syncLDAPUser": &graphql.Field{
                                Type: giteaUserType,
                                Args: graphql.FieldConfigArgument{
                                        "uid": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "LDAP user UID",
                                        },
                                        "defaultPassword": &graphql.ArgumentConfig{
                                                Type:         graphql.String,
                                                DefaultValue: "changeme123",
                                                Description:  "Default password for new Gitea users",
                                        },
                                },
                                Resolve: s.resolveSyncLDAPUser,
                        },
                        "syncAllLDAPUsers": &graphql.Field{
                                Type: graphql.NewList(giteaUserType),
                                Args: graphql.FieldConfigArgument{
                                        "defaultPassword": &graphql.ArgumentConfig{
                                                Type:         graphql.String,
                                                DefaultValue: "changeme123",
                                                Description:  "Default password for new Gitea users",
                                        },
                                },
                                Resolve: s.resolveSyncAllLDAPUsers,
                        },
                        // Repo Sync Mutations (Gitea → LDAP)
                        "syncGiteaReposToLDAP": &graphql.Field{
                                Type:        repoSyncResultType,
                                Description: "Sync a user's Gitea repositories to their LDAP githubRepository attribute",
                                Args: graphql.FieldConfigArgument{
                                        "uid": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "LDAP user UID whose Gitea repos to sync",
                                        },
                                },
                                Resolve: s.resolveSyncGiteaReposToLDAP,
                        },
                        "syncAllGiteaReposToLDAP": &graphql.Field{
                                Type:        graphql.NewList(repoSyncResultType),
                                Description: "Sync all users' Gitea repositories to their LDAP githubRepository attributes",
                                Resolve:     s.resolveSyncAllGiteaReposToLDAP,
                        },
                        // Issue Mutations
                        "createIssue": &graphql.Field{
                                Type: issueType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "title": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "body": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "assignees": &graphql.ArgumentConfig{
                                                Type: graphql.NewList(graphql.String),
                                        },
                                        "labels": &graphql.ArgumentConfig{
                                                Type: graphql.NewList(graphql.Int),
                                        },
                                        "milestone": &graphql.ArgumentConfig{
                                                Type: graphql.Int,
                                        },
                                },
                                Resolve: s.resolveCreateIssue,
                        },
                        "updateIssue": &graphql.Field{
                                Type: issueType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.Int),
                                        },
                                        "title": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "body": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "state": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "assignees": &graphql.ArgumentConfig{
                                                Type: graphql.NewList(graphql.String),
                                        },
                                        "labels": &graphql.ArgumentConfig{
                                                Type: graphql.NewList(graphql.Int),
                                        },
                                        "milestone": &graphql.ArgumentConfig{
                                                Type: graphql.Int,
                                        },
                                },
                                Resolve: s.resolveUpdateIssue,
                        },
                        "createIssueComment": &graphql.Field{
                                Type: issueCommentType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "number": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.Int),
                                        },
                                        "body": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                },
                                Resolve: s.resolveCreateIssueComment,
                        },
                        "createLabel": &graphql.Field{
                                Type: issueLabelType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "name": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "color": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "description": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                },
                                Resolve: s.resolveCreateLabel,
                        },
                        "createMilestone": &graphql.Field{
                                Type: issueMilestoneType,
                                Args: graphql.FieldConfigArgument{
                                        "owner": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "title": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "description": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                        "state": &graphql.ArgumentConfig{
                                                Type:         graphql.String,
                                                DefaultValue: "open",
                                        },
                                },
                                Resolve: s.resolveCreateMilestone,
                        },
                        "createTeam": &graphql.Field{
                                Type:        teamType,
                                Description: "Create a new team in an organization",
                                Args: graphql.FieldConfigArgument{
                                        "orgName": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Organization name",
                                        },
                                        "name": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Team name",
                                        },
                                        "description": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Team description",
                                        },
                                        "permission": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Team permission (read, write, admin)",
                                        },
                                },
                                Resolve: s.resolveCreateTeam,
                        },
                        "addTeamMember": &graphql.Field{
                                Type:        teamType,
                                Description: "Add a member to a team",
                                Args: graphql.FieldConfigArgument{
                                        "teamId": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Team ID",
                                        },
                                        "username": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Username to add",
                                        },
                                },
                                Resolve: s.resolveAddTeamMember,
                        },
                        "removeTeamMember": &graphql.Field{
                                Type:        teamType,
                                Description: "Remove a member from a team",
                                Args: graphql.FieldConfigArgument{
                                        "teamId": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Team ID",
                                        },
                                        "username": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Username to remove",
                                        },
                                },
                                Resolve: s.resolveRemoveTeamMember,
                        },
                        "addTeamRepository": &graphql.Field{
                                Type:        teamType,
                                Description: "Add a repository to a team",
                                Args: graphql.FieldConfigArgument{
                                        "teamId": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Team ID",
                                        },
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                },
                                Resolve: s.resolveAddTeamRepository,
                        },
                        "removeTeamRepository": &graphql.Field{
                                Type:        teamType,
                                Description: "Remove a repository from a team",
                                Args: graphql.FieldConfigArgument{
                                        "teamId": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.Int),
                                                Description: "Team ID",
                                        },
                                        "owner": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository owner",
                                        },
                                        "repo": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Repository name",
                                        },
                                },
                                Resolve: s.resolveRemoveTeamRepository,
                        },
                        // Collab mutations
                        "addRepoToGroup": &graphql.Field{
                                Type:        syncResultType,
                                Description: "Add a repo to a group and sync to Gitea team",
                                Args: graphql.FieldConfigArgument{
                                        "groupCn": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                        "repo":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                },
                                Resolve: s.resolveAddRepoToGroup,
                        },
                        "removeRepoFromGroup": &graphql.Field{
                                Type:        syncResultType,
                                Description: "Remove a repo from a group and sync",
                                Args: graphql.FieldConfigArgument{
                                        "groupCn": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                        "repo":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                },
                                Resolve: s.resolveRemoveRepoFromGroup,
                        },
                        "addRepoToDepartment": &graphql.Field{
                                Type:        syncResultType,
                                Description: "Add a repo to a department and sync (manager gets admin)",
                                Args: graphql.FieldConfigArgument{
                                        "ou":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                        "repo": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                },
                                Resolve: s.resolveAddRepoToDepartment,
                        },
                        "removeRepoFromDepartment": &graphql.Field{
                                Type:        syncResultType,
                                Description: "Remove a repo from a department and sync",
                                Args: graphql.FieldConfigArgument{
                                        "ou":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                        "repo": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                },
                                Resolve: s.resolveRemoveRepoFromDepartment,
                        },
                        "createCollabGroup": &graphql.Field{
                                Type:        syncResultType,
                                Description: "Create a collab group (department + extra members) and sync",
                                Args: graphql.FieldConfigArgument{
                                        "name":           &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                        "baseDepartment": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                        "extraMembers":   &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
                                        "repos":          &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
                                },
                                Resolve: s.resolveCreateCollabGroup,
                        },
                        "deleteCollabGroup": &graphql.Field{
                                Type:        graphql.Boolean,
                                Description: "Delete a collab group (LDAP + Gitea team)",
                                Args: graphql.FieldConfigArgument{
                                        "groupCn": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
                                },
                                Resolve: s.resolveDeleteCollabGroup,
                        },
                        "syncGroupToTeam": &graphql.Field{
                                Type:        syncResultType,
                                Description: "Synchronize an LDAP group to a Gitea team",
                                Args: graphql.FieldConfigArgument{
                                        "groupCn": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "LDAP group CN",
                                        },
                                        "orgName": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Gitea organization name",
                                        },
                                        "teamName": &graphql.ArgumentConfig{
                                                Type:        graphql.String,
                                                Description: "Team name (optional, defaults to group CN)",
                                        },
                                        "permission": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Team permission (read, write, admin)",
                                        },
                                },
                                Resolve: s.resolveSyncGroupToTeam,
                        },
                },
        })

        // Create schema
        schemaConfig := graphql.SchemaConfig{
                Query:    queryType,
                Mutation: mutationType,
        }

        schema, err := graphql.NewSchema(schemaConfig)
        if err != nil {
                logger.WithError(err).Fatal("Failed to create schema")
        }

        s.schema = schema
        return s
}

// GetSchema returns the GraphQL schema
func (s *Schema) GetSchema() graphql.Schema {
        return s.schema
}

// Type Definitions

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// Helper functions for all resolvers (kept in schema.go for centralized access)

func (s *Schema) getUserFromContext(ctx context.Context) (*models.User, string, error) {
        // Get user ID from context (set by auth middleware)
        userID := auth.GetUserFromContext(ctx)
        if userID == "" {
                return nil, "", fmt.Errorf("unauthorized")
        }

        // Get token from context
        token := auth.GetTokenFromContext(ctx)

        // Fetch full user profile from LDAP Manager (includes repositories + department)
        ldapUser, err := s.ldapClient.GetUser(ctx, userID, token)
        if err != nil {
                s.logger.WithFields(logrus.Fields{
                        "user":  userID,
                        "error": err.Error(),
                }).Warn("Failed to fetch full user from LDAP, falling back to context data")

                // Fallback to minimal user from context
                email := auth.GetEmailFromContext(ctx)
                return &models.User{
                        UID:  userID,
                        Mail: email,
                }, token, nil
        }

        user := &models.User{
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
                DN:           ldapUser.DN,
        }

        return user, token, nil
}

// ExtractUserFromToken validates JWT and fetches user from LDAP Manager
func (s *Schema) ExtractUserFromToken(ctx context.Context, tokenString string) (*models.User, error) {
        claims := &Claims{}

        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
                if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                }
                return []byte(s.config.JWTSecret), nil
        })

        if err != nil {
                return nil, err
        }

        if !token.Valid {
                return nil, fmt.Errorf("invalid token")
        }

        // Fetch full user details from LDAP Manager
        ldapUser, err := s.ldapClient.GetUser(ctx, claims.UID, tokenString)
        if err != nil {
                return nil, fmt.Errorf("failed to get user from LDAP Manager: %w", err)
        }

        // Convert to our User model
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
                DN:           ldapUser.DN,
        }, nil
}

// NOTE: All resolver functions have been moved to their respective type files:
// - Repository/Branch/Commit/Tag resolvers → types_repository.go
// - Issue resolvers → types_issue.go
// - User Sync resolvers → types_user.go
// - PR resolvers → types_pr.go
// - Team resolvers → types_team.go
