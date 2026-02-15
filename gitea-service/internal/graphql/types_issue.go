package graphql

import (
	"fmt"

	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/graphql-go/graphql"
)

// defineIssueUserType defines the IssueUser GraphQL type
func (s *Schema) defineIssueUserType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "IssueUser",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.Int},
			"login":     &graphql.Field{Type: graphql.String},
			"fullName":  &graphql.Field{Type: graphql.String},
			"email":     &graphql.Field{Type: graphql.String},
			"avatarUrl": &graphql.Field{Type: graphql.String},
		},
	})
}

// defineIssueLabelType defines the IssueLabel GraphQL type
func (s *Schema) defineIssueLabelType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "IssueLabel",
		Fields: graphql.Fields{
			"id":    &graphql.Field{Type: graphql.Int},
			"name":  &graphql.Field{Type: graphql.String},
			"color": &graphql.Field{Type: graphql.String},
		},
	})
}

// defineIssueMilestoneType defines the IssueMilestone GraphQL type
func (s *Schema) defineIssueMilestoneType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "IssueMilestone",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.Int},
			"title":       &graphql.Field{Type: graphql.String},
			"description": &graphql.Field{Type: graphql.String},
			"state":       &graphql.Field{Type: graphql.String},
		},
	})
}

// defineIssueType defines the Issue GraphQL type
func (s *Schema) defineIssueType(issueUserType, issueLabelType, issueMilestoneType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Issue",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.Int},
			"number":    &graphql.Field{Type: graphql.Int},
			"title":     &graphql.Field{Type: graphql.String},
			"body":      &graphql.Field{Type: graphql.String},
			"state":     &graphql.Field{Type: graphql.String},
			"user":      &graphql.Field{Type: issueUserType},
			"labels":    &graphql.Field{Type: graphql.NewList(issueLabelType)},
			"milestone": &graphql.Field{Type: issueMilestoneType},
			"createdAt": &graphql.Field{Type: graphql.String},
			"updatedAt": &graphql.Field{Type: graphql.String},
		},
	})
}

// defineIssueCommentType defines the IssueComment GraphQL type
func (s *Schema) defineIssueCommentType(issueUserType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "IssueComment",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.Int},
			"body":      &graphql.Field{Type: graphql.String},
			"user":      &graphql.Field{Type: issueUserType},
			"createdAt": &graphql.Field{Type: graphql.String},
			"updatedAt": &graphql.Field{Type: graphql.String},
		},
	})
}

// ============================================================================
// ISSUE QUERY RESOLVERS
// ============================================================================

func (s *Schema) resolveListIssues(p graphql.ResolveParams) (interface{}, error) {
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	repo := p.Args["repo"].(string)
	state := p.Args["state"].(string)
	page := p.Args["page"].(int)
	limit := p.Args["limit"].(int)

	var labels []string
	if labelsArg, ok := p.Args["labels"].([]interface{}); ok {
		for _, label := range labelsArg {
			if labelStr, ok := label.(string); ok {
				labels = append(labels, labelStr)
			}
		}
	}

	issues, err := s.giteaClient.ListIssues(owner, repo, state, labels, page, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}

	return issues, nil
}

func (s *Schema) resolveGetIssue(p graphql.ResolveParams) (interface{}, error) {
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))

	issue, err := s.giteaClient.GetIssue(owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	return issue, nil
}

func (s *Schema) resolveListIssueComments(p graphql.ResolveParams) (interface{}, error) {
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))

	comments, err := s.giteaClient.ListIssueComments(owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to list issue comments: %w", err)
	}

	return comments, nil
}

func (s *Schema) resolveListLabels(p graphql.ResolveParams) (interface{}, error) {
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	repo := p.Args["repo"].(string)
	page := p.Args["page"].(int)
	limit := p.Args["limit"].(int)

	labels, err := s.giteaClient.ListLabels(owner, repo, page, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}

	return labels, nil
}

func (s *Schema) resolveListMilestones(p graphql.ResolveParams) (interface{}, error) {
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	repo := p.Args["repo"].(string)
	state := p.Args["state"].(string)
	page := p.Args["page"].(int)
	limit := p.Args["limit"].(int)

	milestones, err := s.giteaClient.ListMilestones(owner, repo, state, page, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list milestones: %w", err)
	}

	return milestones, nil
}

// ============================================================================
// ISSUE MUTATION RESOLVERS
// ============================================================================

func (s *Schema) resolveCreateIssue(p graphql.ResolveParams) (interface{}, error) {
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	repo := p.Args["repo"].(string)
	title := p.Args["title"].(string)

	req := &gitea.CreateIssueRequest{
		Title: title,
	}

	if body, ok := p.Args["body"].(string); ok {
		req.Body = body
	}

	if assignees, ok := p.Args["assignees"].([]interface{}); ok {
		for _, a := range assignees {
			if assignee, ok := a.(string); ok {
				req.Assignees = append(req.Assignees, assignee)
			}
		}
	}

	if labels, ok := p.Args["labels"].([]interface{}); ok {
		for _, l := range labels {
			if label, ok := l.(int); ok {
				req.Labels = append(req.Labels, int64(label))
			}
		}
	}

	if milestone, ok := p.Args["milestone"].(int); ok {
		req.Milestone = int64(milestone)
	}

	issue, err := s.giteaClient.CreateIssue(owner, repo, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	s.logger.WithField("issue_number", issue.Number).Info("Created issue")

	return issue, nil
}

func (s *Schema) resolveUpdateIssue(p graphql.ResolveParams) (interface{}, error) {
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))

	req := &gitea.UpdateIssueRequest{}

	if title, ok := p.Args["title"].(string); ok {
		req.Title = &title
	}

	if body, ok := p.Args["body"].(string); ok {
		req.Body = &body
	}

	if state, ok := p.Args["state"].(string); ok {
		req.State = &state
	}

	if assignees, ok := p.Args["assignees"].([]interface{}); ok {
		for _, a := range assignees {
			if assignee, ok := a.(string); ok {
				req.Assignees = append(req.Assignees, assignee)
			}
		}
	}

	if labels, ok := p.Args["labels"].([]interface{}); ok {
		for _, l := range labels {
			if label, ok := l.(int); ok {
				req.Labels = append(req.Labels, int64(label))
			}
		}
	}

	if milestone, ok := p.Args["milestone"].(int); ok {
		m := int64(milestone)
		req.Milestone = &m
	}

	issue, err := s.giteaClient.UpdateIssue(owner, repo, number, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update issue: %w", err)
	}

	s.logger.WithField("issue_number", issue.Number).Info("Updated issue")

	return issue, nil
}

func (s *Schema) resolveCreateIssueComment(p graphql.ResolveParams) (interface{}, error) {
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	repo := p.Args["repo"].(string)
	number := int64(p.Args["number"].(int))
	body := p.Args["body"].(string)

	req := &gitea.CreateIssueCommentRequest{
		Body: body,
	}

	comment, err := s.giteaClient.CreateIssueComment(owner, repo, number, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue comment: %w", err)
	}

	s.logger.WithField("comment_id", comment.ID).Info("Created issue comment")

	return comment, nil
}

func (s *Schema) resolveCreateLabel(p graphql.ResolveParams) (interface{}, error) {
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	repo := p.Args["repo"].(string)
	name := p.Args["name"].(string)
	color := p.Args["color"].(string)

	req := &gitea.CreateLabelRequest{
		Name:  name,
		Color: color,
	}

	if desc, ok := p.Args["description"].(string); ok {
		req.Description = desc
	}

	label, err := s.giteaClient.CreateLabel(owner, repo, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create label: %w", err)
	}

	s.logger.WithField("label_name", label.Name).Info("Created label")

	return label, nil
}

func (s *Schema) resolveCreateMilestone(p graphql.ResolveParams) (interface{}, error) {
	owner := s.config.GetDefaultOwner()
	if ownerArg, ok := p.Args["owner"].(string); ok && ownerArg != "" {
		owner = ownerArg
	}

	repo := p.Args["repo"].(string)
	title := p.Args["title"].(string)
	state := p.Args["state"].(string)

	req := &gitea.CreateMilestoneRequest{
		Title: title,
		State: state,
	}

	if desc, ok := p.Args["description"].(string); ok {
		req.Description = desc
	}

	milestone, err := s.giteaClient.CreateMilestone(owner, repo, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create milestone: %w", err)
	}

	s.logger.WithField("milestone_title", milestone.Title).Info("Created milestone")

	return milestone, nil
}
