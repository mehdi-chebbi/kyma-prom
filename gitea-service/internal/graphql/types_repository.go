package graphql

import (
	"fmt"
	"time"

	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/devplatform/gitea-service/internal/models"
	"github.com/graphql-go/graphql"
	"github.com/sirupsen/logrus"
)

// defineGiteaRepositoryType defines the Repository GraphQL type
func (s *Schema) defineGiteaRepositoryType() *graphql.Object {
	ownerType := graphql.NewObject(graphql.ObjectConfig{
		Name: "RepositoryOwner",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.Int},
			"login":     &graphql.Field{Type: graphql.String},
			"fullName":  &graphql.Field{Type: graphql.String},
			"email":     &graphql.Field{Type: graphql.String},
			"avatarUrl": &graphql.Field{Type: graphql.String},
		},
	})

	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Repository",
		Fields: graphql.Fields{
			"id": &graphql.Field{Type: graphql.Int},
			"owner": &graphql.Field{
				Type: ownerType,
			},
			"name":            &graphql.Field{Type: graphql.String},
			"fullName":        &graphql.Field{Type: graphql.String},
			"description":     &graphql.Field{Type: graphql.String},
			"private":         &graphql.Field{Type: graphql.Boolean},
			"fork":            &graphql.Field{Type: graphql.Boolean},
			"htmlUrl":         &graphql.Field{Type: graphql.String},
			"sshUrl":          &graphql.Field{Type: graphql.String},
			"cloneUrl":        &graphql.Field{Type: graphql.String},
			"defaultBranch":   &graphql.Field{Type: graphql.String},
			"createdAt":       &graphql.Field{Type: graphql.String},
			"updatedAt":       &graphql.Field{Type: graphql.String},
			"language":         &graphql.Field{Type: graphql.String},
			"size":            &graphql.Field{Type: graphql.Int},
			"stars":           &graphql.Field{Type: graphql.Int},
			"forks":           &graphql.Field{Type: graphql.Int},
			"openIssuesCount": &graphql.Field{Type: graphql.Int},
			"archived":        &graphql.Field{Type: graphql.Boolean},
		},
	})
}

// defineRepositoryStatsType defines the RepositoryStats GraphQL type
func (s *Schema) defineRepositoryStatsType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "RepositoryStats",
		Fields: graphql.Fields{
			"total":   &graphql.Field{Type: graphql.Int},
			"public":  &graphql.Field{Type: graphql.Int},
			"private": &graphql.Field{Type: graphql.Int},
		},
	})
}

// definePaginatedRepositoriesType defines the PaginatedRepositories GraphQL type
func (s *Schema) definePaginatedRepositoriesType(repoType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "PaginatedRepositories",
		Fields: graphql.Fields{
			"items": &graphql.Field{
				Type: graphql.NewList(repoType),
			},
			"total":   &graphql.Field{Type: graphql.Int},
			"limit":   &graphql.Field{Type: graphql.Int},
			"offset":  &graphql.Field{Type: graphql.Int},
			"hasMore": &graphql.Field{Type: graphql.Boolean},
		},
	})
}

// defineBranchType defines the Branch GraphQL type
func (s *Schema) defineBranchType() *graphql.Object {
	commitType := graphql.NewObject(graphql.ObjectConfig{
		Name: "BranchCommit",
		Fields: graphql.Fields{
			"id":  &graphql.Field{Type: graphql.String},
			"url": &graphql.Field{Type: graphql.String},
		},
	})

	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Branch",
		Fields: graphql.Fields{
			"name":   &graphql.Field{Type: graphql.String},
			"commit": &graphql.Field{Type: commitType},
		},
	})
}

// defineCommitType defines the Commit GraphQL type
func (s *Schema) defineCommitType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Commit",
		Fields: graphql.Fields{
			"sha": &graphql.Field{Type: graphql.String},
			"commit": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "CommitDetails",
					Fields: graphql.Fields{
						"message": &graphql.Field{Type: graphql.String},
						"author": &graphql.Field{
							Type: graphql.NewObject(graphql.ObjectConfig{
								Name: "CommitAuthor",
								Fields: graphql.Fields{
									"name":  &graphql.Field{Type: graphql.String},
									"email": &graphql.Field{Type: graphql.String},
									"date":  &graphql.Field{Type: graphql.String},
								},
							}),
						},
					},
				}),
			},
			"htmlUrl": &graphql.Field{Type: graphql.String},
		},
	})
}

// defineTagType defines the Tag GraphQL type
func (s *Schema) defineTagType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Tag",
		Fields: graphql.Fields{
			"name":    &graphql.Field{Type: graphql.String},
			"message": &graphql.Field{Type: graphql.String},
			"commit": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "TagCommit",
					Fields: graphql.Fields{
						"sha": &graphql.Field{Type: graphql.String},
						"url": &graphql.Field{Type: graphql.String},
					},
				}),
			},
			"zipballUrl": &graphql.Field{Type: graphql.String},
			"tarballUrl": &graphql.Field{Type: graphql.String},
		},
	})
}

// defineHealthType defines the Health GraphQL type
func (s *Schema) defineHealthType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Health",
		Fields: graphql.Fields{
			"status":  &graphql.Field{Type: graphql.String},
			"service": &graphql.Field{Type: graphql.String},
			"gitea": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "GiteaHealth",
					Fields: graphql.Fields{
						"status":  &graphql.Field{Type: graphql.String},
						"message": &graphql.Field{Type: graphql.String},
					},
				}),
			},
			"timestamp": &graphql.Field{Type: graphql.String},
		},
	})
}

// ============================================================================
// REPOSITORY QUERY RESOLVERS
// ============================================================================

func (s *Schema) resolveListRepositories(p graphql.ResolveParams) (interface{}, error) {
	limit := p.Args["limit"].(int)
	offset := p.Args["offset"].(int)

	if limit > 100 {
		limit = 100
	}
	if limit <= 0 {
		limit = 10
	}

	allRepos, err := s.giteaClient.ListRepositories()
	if err != nil {
		s.logger.WithError(err).Error("Failed to list repositories")
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	total := len(allRepos)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedRepos := allRepos[start:end]

	return map[string]interface{}{
		"items":   s.convertGiteaReposToMap(paginatedRepos),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"hasMore": end < total,
	}, nil
}

func (s *Schema) resolveSearchRepositories(p graphql.ResolveParams) (interface{}, error) {
	query := ""
	if q, ok := p.Args["query"].(string); ok {
		query = q
	}

	limit := p.Args["limit"].(int)
	offset := p.Args["offset"].(int)

	if limit > 100 {
		limit = 100
	}
	if limit <= 0 {
		limit = 10
	}

	// Fetch more to handle offset client-side
	fetchLimit := limit + offset + 50
	allRepos, err := s.giteaClient.SearchRepositories(query, fetchLimit)
	if err != nil {
		s.logger.WithError(err).Error("Failed to search repositories")
		return nil, fmt.Errorf("failed to search repositories: %w", err)
	}

	total := len(allRepos)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedRepos := allRepos[start:end]

	return map[string]interface{}{
		"items":   s.convertGiteaReposToMap(paginatedRepos),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"hasMore": end < total,
	}, nil
}

func (s *Schema) resolveGetRepository(p graphql.ResolveParams) (interface{}, error) {
	owner := p.Args["owner"].(string)
	name := p.Args["name"].(string)

	repo, err := s.giteaClient.GetRepository(owner, name)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get repository")
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	return s.convertGiteaRepoToMap(repo), nil
}

func (s *Schema) resolveMyRepositories(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	limit := p.Args["limit"].(int)
	offset := p.Args["offset"].(int)

	if limit > 100 {
		limit = 100
	}
	if limit <= 0 {
		limit = 10
	}

	repos, err := s.giteaService.GetUserRepositories(p.Context, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get user repositories")
		return nil, fmt.Errorf("failed to get repositories: %w", err)
	}

	convertedRepos := s.convertReposToModels(repos)
	total := len(convertedRepos)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedRepos := convertedRepos[start:end]

	return map[string]interface{}{
		"items":   paginatedRepos,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"hasMore": end < total,
	}, nil
}

func (s *Schema) resolveRepositoryStats(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	stats, err := s.giteaService.GetRepositoryStats(p.Context, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get repository stats")
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	return s.convertStatsToModel(stats), nil
}

func (s *Schema) resolveHealth(p graphql.ResolveParams) (interface{}, error) {
	giteaHealthy := s.giteaClient.HealthCheck() == nil
	ldapHealthy := s.ldapClient.HealthCheck(p.Context) == nil

	status := "healthy"
	if !giteaHealthy || !ldapHealthy {
		status = "unhealthy"
	}

	return &models.HealthStatus{
		Status:      status,
		Timestamp:   time.Now().Unix(),
		Gitea:       giteaHealthy,
		LDAPManager: ldapHealthy,
	}, nil
}

// ============================================================================
// BRANCH, COMMIT, TAG QUERY RESOLVERS
// ============================================================================

func (s *Schema) resolveListBranches(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	page := p.Args["page"].(int)
	limit := p.Args["limit"].(int)

	// Enforce limits
	if limit > 500 {
		limit = 500
	}

	branches, err := s.giteaService.ListBranches(p.Context, owner, repo, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list branches")
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	// Client-side pagination
	total := len(branches)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return []*gitea.Branch{}, nil
	}
	if end > total {
		end = total
	}

	return branches[start:end], nil
}

func (s *Schema) resolveGetBranch(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	branch := p.Args["branch"].(string)

	branchInfo, err := s.giteaService.GetBranch(p.Context, owner, repo, branch, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get branch")
		return nil, fmt.Errorf("failed to get branch: %w", err)
	}

	return branchInfo, nil
}

func (s *Schema) resolveListCommits(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	page := p.Args["page"].(int)
	limit := p.Args["limit"].(int)

	// Enforce limits
	if limit > 100 {
		limit = 100
	}

	opts := &gitea.CommitListOptions{}
	if sha, ok := p.Args["sha"].(string); ok {
		opts.SHA = sha
	}
	if path, ok := p.Args["path"].(string); ok {
		opts.Path = path
	}

	commits, err := s.giteaService.ListCommits(p.Context, owner, repo, opts, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list commits")
		return nil, fmt.Errorf("failed to list commits: %w", err)
	}

	// Client-side pagination
	total := len(commits)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return []*gitea.Commit{}, nil
	}
	if end > total {
		end = total
	}

	return commits[start:end], nil
}

func (s *Schema) resolveGetCommit(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	sha := p.Args["sha"].(string)

	commit, err := s.giteaService.GetCommit(p.Context, owner, repo, sha, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get commit")
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	return commit, nil
}

func (s *Schema) resolveListTags(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	page := p.Args["page"].(int)
	limit := p.Args["limit"].(int)

	// Enforce limits
	if limit > 100 {
		limit = 100
	}

	tags, err := s.giteaService.ListTags(p.Context, owner, repo, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list tags")
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	// Client-side pagination
	total := len(tags)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return []*gitea.Tag{}, nil
	}
	if end > total {
		end = total
	}

	return tags[start:end], nil
}

// ============================================================================
// REPOSITORY MUTATION RESOLVERS
// ============================================================================

func (s *Schema) resolveDeleteRepository(p graphql.ResolveParams) (interface{}, error) {
	owner := p.Args["owner"].(string)
	name := p.Args["name"].(string)

	err := s.giteaClient.DeleteRepository(owner, name)
	if err != nil {
		s.logger.WithError(err).Error("Failed to delete repository")
		return false, fmt.Errorf("failed to delete repository: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner": owner,
		"name":  name,
	}).Info("Repository deleted successfully")

	return true, nil
}

func (s *Schema) resolveUpdateRepository(p graphql.ResolveParams) (interface{}, error) {
	owner := p.Args["owner"].(string)
	name := p.Args["name"].(string)

	updates := make(map[string]interface{})
	if desc, ok := p.Args["description"].(string); ok {
		updates["description"] = desc
	}
	if private, ok := p.Args["private"].(bool); ok {
		updates["private"] = private
	}
	if branch, ok := p.Args["defaultBranch"].(string); ok {
		updates["default_branch"] = branch
	}

	repo, err := s.giteaClient.UpdateRepository(owner, name, updates)
	if err != nil {
		s.logger.WithError(err).Error("Failed to update repository")
		return nil, fmt.Errorf("failed to update repository: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"owner": owner,
		"name":  name,
	}).Info("Repository updated successfully")

	return s.convertGiteaRepoToMap(repo), nil
}

func (s *Schema) resolveCreateRepository(p graphql.ResolveParams) (interface{}, error) {
	user, _, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	// Use default owner if not specified
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	name := p.Args["name"].(string)

	req := &gitea.CreateRepositoryRequest{
		Name:     name,
		Private:  false,
		AutoInit: true,
	}

	if desc, ok := p.Args["description"].(string); ok {
		req.Description = desc
	}
	if private, ok := p.Args["private"].(bool); ok {
		req.Private = private
	}
	if autoInit, ok := p.Args["autoInit"].(bool); ok {
		req.AutoInit = autoInit
	}
	if gitignores, ok := p.Args["gitignores"].(string); ok {
		req.Gitignores = gitignores
	}
	if license, ok := p.Args["license"].(string); ok {
		req.License = license
	}
	if readme, ok := p.Args["readme"].(string); ok {
		req.Readme = readme
	}
	if defaultBranch, ok := p.Args["defaultBranch"].(string); ok {
		req.DefaultBranch = defaultBranch
	}

	repo, err := s.giteaService.CreateRepository(p.Context, owner, req, user)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create repository")
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return s.convertGiteaRepoToMap(repo), nil
}

func (s *Schema) resolveMigrateRepository(p graphql.ResolveParams) (interface{}, error) {
	user, _, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	cloneAddr := p.Args["cloneAddr"].(string)
	repoName := p.Args["repoName"].(string)

	// Use default owner if not specified
	repoOwner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["repoOwner"].(string); ok && ownerArg != "" {
		repoOwner = ownerArg
	}

	req := &gitea.MigrateRepositoryRequest{
		CloneAddr:    cloneAddr,
		RepoName:     repoName,
		RepoOwner:    repoOwner,
		Mirror:       false,
		Private:      false,
		Wiki:         true,
		Milestones:   true,
		Labels:       true,
		Issues:       true,
		PullRequests: true,
		Releases:     true,
	}
	if mirror, ok := p.Args["mirror"].(bool); ok {
		req.Mirror = mirror
	}
	if private, ok := p.Args["private"].(bool); ok {
		req.Private = private
	}
	if description, ok := p.Args["description"].(string); ok {
		req.Description = description
	}
	if wiki, ok := p.Args["wiki"].(bool); ok {
		req.Wiki = wiki
	}
	if milestones, ok := p.Args["milestones"].(bool); ok {
		req.Milestones = milestones
	}
	if labels, ok := p.Args["labels"].(bool); ok {
		req.Labels = labels
	}
	if issues, ok := p.Args["issues"].(bool); ok {
		req.Issues = issues
	}
	if pullRequests, ok := p.Args["pullRequests"].(bool); ok {
		req.PullRequests = pullRequests
	}
	if releases, ok := p.Args["releases"].(bool); ok {
		req.Releases = releases
	}
	if authUsername, ok := p.Args["authUsername"].(string); ok {
		req.AuthUsername = authUsername
	}
	if authPassword, ok := p.Args["authPassword"].(string); ok {
		req.AuthPassword = authPassword
	}
	if authToken, ok := p.Args["authToken"].(string); ok {
		req.AuthToken = authToken
	}
	if service, ok := p.Args["service"].(string); ok {
		req.Service = service
	}

	repo, err := s.giteaService.MigrateRepository(p.Context, req, user)
	if err != nil {
		s.logger.WithError(err).Error("Failed to migrate repository")
		return nil, fmt.Errorf("failed to migrate repository: %w", err)
	}

	return s.convertGiteaRepoToMap(repo), nil
}

func (s *Schema) resolveForkRepository(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	organization := ""

	if org, ok := p.Args["organization"].(string); ok {
		organization = org
	}

	forkedRepo, err := s.giteaService.ForkRepository(p.Context, owner, repo, organization, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to fork repository")
		return nil, fmt.Errorf("failed to fork repository: %w", err)
	}

	return s.convertGiteaRepoToMap(forkedRepo), nil
}

// ============================================================================
// BRANCH MUTATION RESOLVERS
// ============================================================================

func (s *Schema) resolveCreateBranch(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	branchName := p.Args["branchName"].(string)
	oldBranchName := p.Args["oldBranchName"].(string)

	branch, err := s.giteaService.CreateBranch(p.Context, owner, repo, branchName, oldBranchName, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create branch")
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}

	return branch, nil
}

func (s *Schema) resolveDeleteBranch(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	branch := p.Args["branch"].(string)

	err = s.giteaService.DeleteBranch(p.Context, owner, repo, branch, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to delete branch")
		return false, fmt.Errorf("failed to delete branch: %w", err)
	}

	return true, nil
}

// ============================================================================
// TAG MUTATION RESOLVERS
// ============================================================================

func (s *Schema) resolveCreateTag(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	tagName := p.Args["tagName"].(string)
	target := p.Args["target"].(string)
	message := ""

	if msg, ok := p.Args["message"].(string); ok {
		message = msg
	}

	tag, err := s.giteaService.CreateTag(p.Context, owner, repo, tagName, target, message, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create tag")
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	return tag, nil
}

func (s *Schema) resolveDeleteTag(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	tag := p.Args["tag"].(string)

	err = s.giteaService.DeleteTag(p.Context, owner, repo, tag, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to delete tag")
		return false, fmt.Errorf("failed to delete tag: %w", err)
	}

	return true, nil
}

// ============================================================================
// HELPER/CONVERTER FUNCTIONS
// ============================================================================

func (s *Schema) convertRepoToModel(repo *gitea.Repository) *models.GiteaRepository {
	if repo == nil {
		return nil
	}

	return &models.GiteaRepository{
		ID:            repo.ID,
		Name:          repo.Name,
		FullName:      repo.FullName,
		Description:   repo.Description,
		Private:       repo.Private,
		Fork:          repo.Fork,
		HTMLURL:       repo.HTMLURL,
		SSHURL:        repo.SSHURL,
		CloneURL:      repo.CloneURL,
		DefaultBranch: repo.DefaultBranch,
		Language:      repo.Language,
		Stars:         repo.Stars,
		Forks:         repo.Forks,
		Size:          repo.Size,
		CreatedAt:     repo.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     repo.UpdatedAt.Format(time.RFC3339),
		Owner: models.RepositoryOwner{
			ID:        repo.Owner.ID,
			Login:     repo.Owner.Login,
			FullName:  repo.Owner.FullName,
			Email:     repo.Owner.Email,
			AvatarURL: repo.Owner.AvatarURL,
		},
	}
}

func (s *Schema) convertReposToModels(repos []*gitea.Repository) []*models.GiteaRepository {
	result := make([]*models.GiteaRepository, len(repos))
	for i, r := range repos {
		result[i] = s.convertRepoToModel(r)
	}
	return result
}

func (s *Schema) convertStatsToModel(stats *gitea.RepositoryStats) *models.RepositoryStats {
	if stats == nil {
		return nil
	}

	languages := make([]models.LanguageDistribution, 0, len(stats.Languages))
	for lang, count := range stats.Languages {
		languages = append(languages, models.LanguageDistribution{
			Language: lang,
			Count:    count,
		})
	}

	return &models.RepositoryStats{
		TotalCount:   stats.TotalCount,
		PrivateCount: stats.PrivateCount,
		PublicCount:  stats.PublicCount,
		Languages:    languages,
	}
}

func (s *Schema) convertGiteaReposToMap(repos []*gitea.Repository) []map[string]interface{} {
	result := make([]map[string]interface{}, len(repos))
	for i, repo := range repos {
		result[i] = s.convertGiteaRepoToMap(repo)
	}
	return result
}

func (s *Schema) convertGiteaRepoToMap(repo *gitea.Repository) map[string]interface{} {
	return map[string]interface{}{
		"id":            repo.ID,
		"name":          repo.Name,
		"fullName":      repo.FullName,
		"description":   repo.Description,
		"private":       repo.Private,
		"fork":          repo.Fork,
		"htmlUrl":       repo.HTMLURL,
		"sshUrl":        repo.SSHURL,
		"cloneUrl":      repo.CloneURL,
		"defaultBranch": repo.DefaultBranch,
		"language":      repo.Language,
		"stars":         repo.Stars,
		"forks":         repo.Forks,
		"size":          repo.Size,
		"createdAt":     repo.CreatedAt.Format(time.RFC3339),
		"updatedAt":     repo.UpdatedAt.Format(time.RFC3339),
		"owner": map[string]interface{}{
			"id":        repo.Owner.ID,
			"login":     repo.Owner.Login,
			"fullName":  repo.Owner.FullName,
			"email":     repo.Owner.Email,
			"avatarUrl": repo.Owner.AvatarURL,
		},
	}
}
