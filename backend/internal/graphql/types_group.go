package graphql

import (
	"fmt"
	"strings"

	"github.com/devplatform/ldap-manager/internal/models"
	"github.com/graphql-go/graphql"
)

// defineGroupFilterInput defines the GroupFilterInput GraphQL input type
func (s *Schema) defineGroupFilterInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "GroupFilterInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"cn": &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Filter by group name (partial match)"},
		},
	})
}

// defineGroupType defines the Group GraphQL type
func (s *Schema) defineGroupType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Group",
		Fields: graphql.Fields{
			"cn":           &graphql.Field{Type: graphql.String},
			"description":  &graphql.Field{Type: graphql.String},
			"gidNumber":    &graphql.Field{Type: graphql.Int},
			"members":      &graphql.Field{Type: graphql.NewList(graphql.String)},
			"repositories": &graphql.Field{Type: graphql.NewList(graphql.String)},
			"dn":           &graphql.Field{Type: graphql.String},
		},
	})
}

// definePaginatedGroupsType defines the PaginatedGroups GraphQL type
func (s *Schema) definePaginatedGroupsType(groupType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "PaginatedGroups",
		Fields: graphql.Fields{
			"groups": &graphql.Field{
				Type:        graphql.NewList(groupType),
				Description: "List of groups",
			},
			"total": &graphql.Field{
				Type:        graphql.Int,
				Description: "Total number of groups",
			},
			"page": &graphql.Field{
				Type:        graphql.Int,
				Description: "Current page number",
			},
			"limit": &graphql.Field{
				Type:        graphql.Int,
				Description: "Items per page",
			},
		},
	})
}

// ============================================================================
// GROUP QUERY RESOLVERS
// ============================================================================

func (s *Schema) resolveGroup(p graphql.ResolveParams) (interface{}, error) {
	cn := p.Args["cn"].(string)
	return s.ldapMgr.GetGroup(p.Context, cn)
}

func (s *Schema) resolveGroups(p graphql.ResolveParams) (interface{}, error) {
	// Get pagination parameters
	limit := p.Args["limit"].(int)
	offset := p.Args["offset"].(int)

	// Enforce limit constraints
	if limit > 100 {
		limit = 100
	}
	if limit <= 0 {
		limit = 10
	}

	// Parse filter
	var filterCN string
	if filterInput, ok := p.Args["filter"].(map[string]interface{}); ok {
		if cn, ok := filterInput["cn"].(string); ok {
			filterCN = cn
		}
	}

	// Get all groups
	allGroups, err := s.ldapMgr.ListGroups(p.Context)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list groups")
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	// Apply filters
	var filteredGroups []*models.Group
	for _, group := range allGroups {
		if filterCN != "" {
			if !strings.Contains(strings.ToLower(group.CN), strings.ToLower(filterCN)) {
				continue
			}
		}
		filteredGroups = append(filteredGroups, group)
	}

	// Apply pagination
	total := len(filteredGroups)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedGroups := filteredGroups[start:end]

	return map[string]interface{}{
		"items":   paginatedGroups,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"hasMore": end < total,
	}, nil
}

// ============================================================================
// GROUP MUTATION RESOLVERS
// ============================================================================

func (s *Schema) resolveCreateGroup(p graphql.ResolveParams) (interface{}, error) {
	cn := p.Args["cn"].(string)
	description := ""
	if desc, ok := p.Args["description"].(string); ok {
		description = desc
	}

	return s.ldapMgr.CreateGroup(p.Context, cn, description)
}

func (s *Schema) resolveAddUserToGroup(p graphql.ResolveParams) (interface{}, error) {
	uid := p.Args["uid"].(string)
	groupCn := p.Args["groupCn"].(string)

	err := s.ldapMgr.AddUserToGroup(p.Context, uid, groupCn)
	return err == nil, err
}

func (s *Schema) resolveAssignRepoToGroup(p graphql.ResolveParams) (interface{}, error) {
	groupCn := p.Args["groupCn"].(string)
	repoInterfaces := p.Args["repositories"].([]interface{})

	repos := make([]string, len(repoInterfaces))
	for i, r := range repoInterfaces {
		repos[i] = r.(string)
	}

	return s.ldapMgr.AssignRepositoriesToGroup(p.Context, groupCn, repos)
}

func (s *Schema) resolveGroupsAll(p graphql.ResolveParams) (interface{}, error) {
	allGroups, err := s.ldapMgr.ListGroups(p.Context)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list all groups")
		return nil, fmt.Errorf("failed to list all groups: %w", err)
	}
	return allGroups, nil
}

func (s *Schema) resolveDeleteGroup(p graphql.ResolveParams) (interface{}, error) {
	cn := p.Args["cn"].(string)
	err := s.ldapMgr.DeleteGroup(p.Context, cn)
	return err == nil, err
}

func (s *Schema) resolveRemoveUserFromGroup(p graphql.ResolveParams) (interface{}, error) {
	uid := p.Args["uid"].(string)
	groupCn := p.Args["groupCn"].(string)
	err := s.ldapMgr.RemoveUserFromGroup(p.Context, uid, groupCn)
	return err == nil, err
}
