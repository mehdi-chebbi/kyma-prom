package graphql

import (
	"fmt"

	"github.com/devplatform/gitea-service/internal/auth"
	"github.com/graphql-go/graphql"
)

// defineGroupAccessType defines the GraphQL type for group/department repo access
func (s *Schema) defineGroupAccessType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "GroupAccess",
		Fields: graphql.Fields{
			"cn":             &graphql.Field{Type: graphql.String},
			"groupType":      &graphql.Field{Type: graphql.String},
			"members":        &graphql.Field{Type: graphql.NewList(graphql.String)},
			"permission":     &graphql.Field{Type: graphql.String},
			"baseDepartment": &graphql.Field{Type: graphql.String},
			"extraMembers":   &graphql.Field{Type: graphql.NewList(graphql.String)},
		},
	})
}

// defineCollabGroupType defines the GraphQL type for collab group info
func (s *Schema) defineCollabGroupType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "CollabGroup",
		Fields: graphql.Fields{
			"cn":             &graphql.Field{Type: graphql.String},
			"baseDepartment": &graphql.Field{Type: graphql.String},
			"extraMembers":   &graphql.Field{Type: graphql.NewList(graphql.String)},
		},
	})
}

// ============================================================================
// COLLAB RESOLVERS
// ============================================================================

func (s *Schema) resolveAddRepoToGroup(p graphql.ResolveParams) (interface{}, error) {
	token := auth.GetTokenFromContext(p.Context)
	groupCN := p.Args["groupCn"].(string)
	repo := p.Args["repo"].(string)

	if s.collabService == nil {
		return nil, fmt.Errorf("collaboration service not available")
	}

	result, err := s.collabService.AddRepoToGroup(p.Context, groupCN, repo, token)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Schema) resolveRemoveRepoFromGroup(p graphql.ResolveParams) (interface{}, error) {
	token := auth.GetTokenFromContext(p.Context)
	groupCN := p.Args["groupCn"].(string)
	repo := p.Args["repo"].(string)

	if s.collabService == nil {
		return nil, fmt.Errorf("collaboration service not available")
	}

	result, err := s.collabService.RemoveRepoFromGroup(p.Context, groupCN, repo, token)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Schema) resolveAddRepoToDepartment(p graphql.ResolveParams) (interface{}, error) {
	token := auth.GetTokenFromContext(p.Context)
	ou := p.Args["ou"].(string)
	repo := p.Args["repo"].(string)

	if s.collabService == nil {
		return nil, fmt.Errorf("collaboration service not available")
	}

	result, err := s.collabService.AddRepoToDepartment(p.Context, ou, repo, token)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Schema) resolveRemoveRepoFromDepartment(p graphql.ResolveParams) (interface{}, error) {
	token := auth.GetTokenFromContext(p.Context)
	ou := p.Args["ou"].(string)
	repo := p.Args["repo"].(string)

	if s.collabService == nil {
		return nil, fmt.Errorf("collaboration service not available")
	}

	result, err := s.collabService.RemoveRepoFromDepartment(p.Context, ou, repo, token)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Schema) resolveCreateCollabGroup(p graphql.ResolveParams) (interface{}, error) {
	token := auth.GetTokenFromContext(p.Context)
	name := p.Args["name"].(string)
	baseDepartment := p.Args["baseDepartment"].(string)

	var extraMembers []string
	if em, ok := p.Args["extraMembers"].([]interface{}); ok {
		for _, m := range em {
			if ms, ok := m.(string); ok {
				extraMembers = append(extraMembers, ms)
			}
		}
	}

	var repos []string
	if r, ok := p.Args["repos"].([]interface{}); ok {
		for _, repo := range r {
			if rs, ok := repo.(string); ok {
				repos = append(repos, rs)
			}
		}
	}

	if s.collabService == nil {
		return nil, fmt.Errorf("collaboration service not available")
	}

	result, err := s.collabService.CreateCollabGroup(p.Context, name, baseDepartment, extraMembers, repos, token)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Schema) resolveDeleteCollabGroup(p graphql.ResolveParams) (interface{}, error) {
	token := auth.GetTokenFromContext(p.Context)
	groupCN := p.Args["groupCn"].(string)

	if s.collabService == nil {
		return nil, fmt.Errorf("collaboration service not available")
	}

	err := s.collabService.DeleteCollabGroup(p.Context, groupCN, token)
	return err == nil, err
}

func (s *Schema) resolveListRepoAccess(p graphql.ResolveParams) (interface{}, error) {
	token := auth.GetTokenFromContext(p.Context)
	repo := p.Args["repo"].(string)

	if s.collabService == nil {
		return nil, fmt.Errorf("collaboration service not available")
	}

	return s.collabService.ListRepoAccess(p.Context, repo, token)
}
