package graphql

import (
	"fmt"
	"time"

	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/graphql-go/graphql"
)

// Define PR-related GraphQL types
func (s *Schema) definePRTypes() (*graphql.Object, *graphql.Object, *graphql.Object, *graphql.Object, *graphql.Object, *graphql.Object, *graphql.Object) {
	// PRUser type
	prUserType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PRUser",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.Int},
			"login":     &graphql.Field{Type: graphql.String},
			"fullName":  &graphql.Field{Type: graphql.String},
			"email":     &graphql.Field{Type: graphql.String},
			"avatarUrl": &graphql.Field{Type: graphql.String},
		},
	})

	// PRLabel type
	prLabelType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PRLabel",
		Fields: graphql.Fields{
			"id":    &graphql.Field{Type: graphql.Int},
			"name":  &graphql.Field{Type: graphql.String},
			"color": &graphql.Field{Type: graphql.String},
		},
	})

	// PRMilestone type
	prMilestoneType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PRMilestone",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.Int},
			"title":       &graphql.Field{Type: graphql.String},
			"description": &graphql.Field{Type: graphql.String},
			"state":       &graphql.Field{Type: graphql.String},
			"dueOn":       &graphql.Field{Type: graphql.String},
		},
	})

	// PRBranchInfo type
	prBranchInfoType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PRBranchInfo",
		Fields: graphql.Fields{
			"label": &graphql.Field{Type: graphql.String},
			"ref":   &graphql.Field{Type: graphql.String},
			"sha":   &graphql.Field{Type: graphql.String},
		},
	})

	// PullRequest type
	pullRequestType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PullRequest",
		Fields: graphql.Fields{
			"id":           &graphql.Field{Type: graphql.Int},
			"number":       &graphql.Field{Type: graphql.Int},
			"state":        &graphql.Field{Type: graphql.String},
			"title":        &graphql.Field{Type: graphql.String},
			"body":         &graphql.Field{Type: graphql.String},
			"user":         &graphql.Field{Type: prUserType},
			"head":         &graphql.Field{Type: prBranchInfoType},
			"base":         &graphql.Field{Type: prBranchInfoType},
			"mergeable":    &graphql.Field{Type: graphql.Boolean},
			"merged":       &graphql.Field{Type: graphql.Boolean},
			"mergedAt":     &graphql.Field{Type: graphql.String},
			"mergedBy":     &graphql.Field{Type: prUserType},
			"createdAt":    &graphql.Field{Type: graphql.String},
			"updatedAt":    &graphql.Field{Type: graphql.String},
			"closedAt":     &graphql.Field{Type: graphql.String},
			"dueDate":      &graphql.Field{Type: graphql.String},
			"assignees":    &graphql.Field{Type: graphql.NewList(prUserType)},
			"labels":       &graphql.Field{Type: graphql.NewList(prLabelType)},
			"milestone":    &graphql.Field{Type: prMilestoneType},
			"comments":     &graphql.Field{Type: graphql.Int},
			"additions":    &graphql.Field{Type: graphql.Int},
			"deletions":    &graphql.Field{Type: graphql.Int},
			"changedFiles": &graphql.Field{Type: graphql.Int},
			"htmlUrl":      &graphql.Field{Type: graphql.String},
			"diffUrl":      &graphql.Field{Type: graphql.String},
			"patchUrl":     &graphql.Field{Type: graphql.String},
		},
	})

	// PRComment type
	prCommentType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PRComment",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.Int},
			"user":      &graphql.Field{Type: prUserType},
			"body":      &graphql.Field{Type: graphql.String},
			"createdAt": &graphql.Field{Type: graphql.String},
			"updatedAt": &graphql.Field{Type: graphql.String},
			"htmlUrl":   &graphql.Field{Type: graphql.String},
		},
	})

	// PRReview type
	prReviewType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PRReview",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.Int},
			"user":        &graphql.Field{Type: prUserType},
			"body":        &graphql.Field{Type: graphql.String},
			"state":       &graphql.Field{Type: graphql.String},
			"commitId":    &graphql.Field{Type: graphql.String},
			"submittedAt": &graphql.Field{Type: graphql.String},
			"htmlUrl":     &graphql.Field{Type: graphql.String},
		},
	})

	// PRFile type
	prFileType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PRFile",
		Fields: graphql.Fields{
			"filename":    &graphql.Field{Type: graphql.String},
			"status":      &graphql.Field{Type: graphql.String},
			"additions":   &graphql.Field{Type: graphql.Int},
			"deletions":   &graphql.Field{Type: graphql.Int},
			"changes":     &graphql.Field{Type: graphql.Int},
			"patchUrl":    &graphql.Field{Type: graphql.String},
			"rawUrl":      &graphql.Field{Type: graphql.String},
			"contentsUrl": &graphql.Field{Type: graphql.String},
		},
	})

	return pullRequestType, prCommentType, prReviewType, prFileType, prUserType, prLabelType, prMilestoneType
}

// PR Query resolvers

func (s *Schema) resolveListPullRequests(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	state := "open"
	if st, ok := p.Args["state"].(string); ok {
		state = st
	}
	page := 1
	if pg, ok := p.Args["page"].(int); ok {
		page = pg
	}
	limit := 30
	if lm, ok := p.Args["limit"].(int); ok {
		limit = lm
	}

	prs, err := s.giteaService.ListPullRequests(p.Context, owner, repo, state, page, limit, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list pull requests")
		return nil, fmt.Errorf("failed to list pull requests: %w", err)
	}

	return s.convertPRsToMaps(prs), nil
}

func (s *Schema) resolveGetPullRequest(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))

	pr, err := s.giteaService.GetPullRequest(p.Context, owner, repo, number, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get pull request")
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}

	return s.convertPRToMap(pr), nil
}

func (s *Schema) resolveListPRComments(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))
	page := p.Args["page"].(int)
	limit := p.Args["limit"].(int)

	// Enforce limits
	if limit > 100 {
		limit = 100
	}

	comments, err := s.giteaService.ListPRComments(p.Context, owner, repo, number, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list PR comments")
		return nil, fmt.Errorf("failed to list PR comments: %w", err)
	}

	// Client-side pagination
	total := len(comments)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return []*gitea.PRComment{}, nil
	}
	if end > total {
		end = total
	}

	return comments[start:end], nil
}

func (s *Schema) resolveListPRReviews(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))
	page := p.Args["page"].(int)
	limit := p.Args["limit"].(int)

	// Enforce limits
	if limit > 100 {
		limit = 100
	}

	reviews, err := s.giteaService.ListPRReviews(p.Context, owner, repo, number, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list PR reviews")
		return nil, fmt.Errorf("failed to list PR reviews: %w", err)
	}

	// Client-side pagination
	total := len(reviews)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return []*gitea.PRReview{}, nil
	}
	if end > total {
		end = total
	}

	return reviews[start:end], nil
}

func (s *Schema) resolveListPRFiles(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))
	page := p.Args["page"].(int)
	limit := p.Args["limit"].(int)

	// Enforce limits
	if limit > 500 {
		limit = 500
	}

	files, err := s.giteaService.ListPRFiles(p.Context, owner, repo, number, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list PR files")
		return nil, fmt.Errorf("failed to list PR files: %w", err)
	}

	// Client-side pagination
	total := len(files)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return []*gitea.PRFile{}, nil
	}
	if end > total {
		end = total
	}

	return files[start:end], nil
}

func (s *Schema) resolveGetPRDiff(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))

	diff, err := s.giteaService.GetPRDiff(p.Context, owner, repo, number, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get PR diff")
		return nil, fmt.Errorf("failed to get PR diff: %w", err)
	}

	return diff, nil
}

func (s *Schema) resolveIsPRMerged(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))

	merged, err := s.giteaService.IsPullRequestMerged(p.Context, owner, repo, number, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to check if PR is merged")
		return nil, fmt.Errorf("failed to check if PR is merged: %w", err)
	}

	return merged, nil
}

// PR Mutation resolvers

func (s *Schema) resolveCreatePullRequest(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	title := p.Args["title"].(string)
	head := p.Args["head"].(string)
	base := p.Args["base"].(string)

	req := &gitea.CreatePullRequestRequest{
		Title: title,
		Head:  head,
		Base:  base,
	}

	if body, ok := p.Args["body"].(string); ok {
		req.Body = body
	}

	pr, err := s.giteaService.CreatePullRequest(p.Context, owner, repo, req, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create pull request")
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	return s.convertPRToMap(pr), nil
}

func (s *Schema) resolveUpdatePullRequest(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))

	req := &gitea.UpdatePullRequestRequest{}

	if title, ok := p.Args["title"].(string); ok {
		req.Title = &title
	}
	if body, ok := p.Args["body"].(string); ok {
		req.Body = &body
	}
	if state, ok := p.Args["state"].(string); ok {
		req.State = &state
	}

	pr, err := s.giteaService.UpdatePullRequest(p.Context, owner, repo, number, req, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to update pull request")
		return nil, fmt.Errorf("failed to update pull request: %w", err)
	}

	return s.convertPRToMap(pr), nil
}

func (s *Schema) resolveMergePullRequest(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))

	method := "merge"
	if m, ok := p.Args["method"].(string); ok {
		method = m
	}

	req := &gitea.MergePullRequestRequest{
		Do: method,
	}

	if deleteBranch, ok := p.Args["deleteBranchAfterMerge"].(bool); ok {
		req.DeleteBranchAfterMerge = deleteBranch
	}

	err = s.giteaService.MergePullRequest(p.Context, owner, repo, number, req, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to merge pull request")
		return false, fmt.Errorf("failed to merge pull request: %w", err)
	}

	return true, nil
}

func (s *Schema) resolveCreatePRComment(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))
	body := p.Args["body"].(string)

	req := &gitea.CreatePRCommentRequest{
		Body: body,
	}

	comment, err := s.giteaService.CreatePRComment(p.Context, owner, repo, number, req, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create PR comment")
		return nil, fmt.Errorf("failed to create PR comment: %w", err)
	}

	return comment, nil
}

func (s *Schema) resolveCreatePRReview(p graphql.ResolveParams) (interface{}, error) {
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))
	event := p.Args["event"].(string) // APPROVE, REQUEST_CHANGES, COMMENT

	req := &gitea.CreatePRReviewRequest{
		Event: event,
	}

	if body, ok := p.Args["body"].(string); ok {
		req.Body = body
	}

	review, err := s.giteaService.CreatePRReview(p.Context, owner, repo, number, req, user, token)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create PR review")
		return nil, fmt.Errorf("failed to create PR review: %w", err)
	}

	return review, nil
}

// Helper functions

func (s *Schema) convertPRToMap(pr *gitea.PullRequest) map[string]interface{} {
	result := map[string]interface{}{
		"id":           pr.ID,
		"number":       pr.Number,
		"state":        pr.State,
		"title":        pr.Title,
		"body":         pr.Body,
		"mergeable":    pr.Mergeable,
		"merged":       pr.Merged,
		"comments":     pr.Comments,
		"additions":    pr.Additions,
		"deletions":    pr.Deletions,
		"changedFiles": pr.ChangedFiles,
		"htmlUrl":      pr.HTMLURL,
		"diffUrl":      pr.DiffURL,
		"patchUrl":     pr.PatchURL,
		"createdAt":    pr.CreatedAt.Format(time.RFC3339),
		"updatedAt":    pr.UpdatedAt.Format(time.RFC3339),
		"user": map[string]interface{}{
			"id":        pr.User.ID,
			"login":     pr.User.Login,
			"fullName":  pr.User.FullName,
			"email":     pr.User.Email,
			"avatarUrl": pr.User.AvatarURL,
		},
		"head": map[string]interface{}{
			"label": pr.Head.Label,
			"ref":   pr.Head.Ref,
			"sha":   pr.Head.SHA,
		},
		"base": map[string]interface{}{
			"label": pr.Base.Label,
			"ref":   pr.Base.Ref,
			"sha":   pr.Base.SHA,
		},
	}

	if pr.MergedAt != nil {
		result["mergedAt"] = pr.MergedAt.Format(time.RFC3339)
	}
	if pr.ClosedAt != nil {
		result["closedAt"] = pr.ClosedAt.Format(time.RFC3339)
	}
	if pr.DueDate != nil {
		result["dueDate"] = pr.DueDate.Format(time.RFC3339)
	}
	if pr.MergedBy != nil {
		result["mergedBy"] = map[string]interface{}{
			"id":        pr.MergedBy.ID,
			"login":     pr.MergedBy.Login,
			"fullName":  pr.MergedBy.FullName,
			"email":     pr.MergedBy.Email,
			"avatarUrl": pr.MergedBy.AvatarURL,
		}
	}

	// Convert assignees
	assignees := []map[string]interface{}{}
	for _, a := range pr.Assignees {
		assignees = append(assignees, map[string]interface{}{
			"id":        a.ID,
			"login":     a.Login,
			"fullName":  a.FullName,
			"email":     a.Email,
			"avatarUrl": a.AvatarURL,
		})
	}
	result["assignees"] = assignees

	// Convert labels
	labels := []map[string]interface{}{}
	for _, l := range pr.Labels {
		labels = append(labels, map[string]interface{}{
			"id":    l.ID,
			"name":  l.Name,
			"color": l.Color,
		})
	}
	result["labels"] = labels

	if pr.Milestone != nil {
		milestone := map[string]interface{}{
			"id":          pr.Milestone.ID,
			"title":       pr.Milestone.Title,
			"description": pr.Milestone.Description,
			"state":       pr.Milestone.State,
		}
		if pr.Milestone.DueOn != nil {
			milestone["dueOn"] = pr.Milestone.DueOn.Format(time.RFC3339)
		}
		result["milestone"] = milestone
	}

	return result
}

func (s *Schema) convertPRsToMaps(prs []*gitea.PullRequest) []map[string]interface{} {
	result := make([]map[string]interface{}, len(prs))
	for i, pr := range prs {
		result[i] = s.convertPRToMap(pr)
	}
	return result
}
