package graphql

import (
	"fmt"

	"github.com/devplatform/ldap-manager/internal/models"
	"github.com/graphql-go/graphql"
)

// defineUserType defines the User GraphQL type
func (s *Schema) defineUserType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"uid":          &graphql.Field{Type: graphql.String},
			"cn":           &graphql.Field{Type: graphql.String},
			"givenName":    &graphql.Field{Type: graphql.String},
			"sn":           &graphql.Field{Type: graphql.String},
			"mail":         &graphql.Field{Type: graphql.String},
			"department":   &graphql.Field{Type: graphql.String},
			"repositories": &graphql.Field{Type: graphql.NewList(graphql.String)},
			"dn":           &graphql.Field{Type: graphql.String},
		},
	})
}

// definePaginatedUsersType defines the PaginatedUsers GraphQL type
func (s *Schema) definePaginatedUsersType(userType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "PaginatedUsers",
		Fields: graphql.Fields{
			"users": &graphql.Field{
				Type:        graphql.NewList(userType),
				Description: "List of users",
			},
			"total": &graphql.Field{
				Type:        graphql.Int,
				Description: "Total number of users",
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

// defineCreateUserInput defines the CreateUserInput GraphQL input type
func (s *Schema) defineCreateUserInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CreateUserInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"uid":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"cn":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"givenName":    &graphql.InputObjectFieldConfig{Type: graphql.String},
			"sn":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"mail":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"password":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"department":   &graphql.InputObjectFieldConfig{Type: graphql.String},
			"repositories": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
		},
	})
}

// defineUpdateUserInput defines the UpdateUserInput GraphQL input type
func (s *Schema) defineUpdateUserInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "UpdateUserInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"uid":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"cn":           &graphql.InputObjectFieldConfig{Type: graphql.String},
			"givenName":    &graphql.InputObjectFieldConfig{Type: graphql.String},
			"sn":           &graphql.InputObjectFieldConfig{Type: graphql.String},
			"mail":         &graphql.InputObjectFieldConfig{Type: graphql.String},
			"password":     &graphql.InputObjectFieldConfig{Type: graphql.String},
			"department":   &graphql.InputObjectFieldConfig{Type: graphql.String},
			"repositories": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
		},
	})
}

// defineSearchFilterInput defines the SearchFilterInput GraphQL input type
func (s *Schema) defineSearchFilterInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "SearchFilterInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"uid":        &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Filter by user ID (partial match)"},
			"cn":         &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Filter by common name (partial match)"},
			"sn":         &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Filter by surname (partial match)"},
			"givenName":  &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Filter by first name (partial match)"},
			"mail":       &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Filter by email (partial match)"},
			"department": &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Filter by department"},
			"uidNumber":  &graphql.InputObjectFieldConfig{Type: graphql.Int, Description: "Filter by UID number"},
			"gidNumber":  &graphql.InputObjectFieldConfig{Type: graphql.Int, Description: "Filter by GID number"},
			"repository": &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Filter by repository access"},
		},
	})
}

// ============================================================================
// USER QUERY RESOLVERS
// ============================================================================

func (s *Schema) resolveMe(p graphql.ResolveParams) (interface{}, error) {
	user, ok := p.Context.Value("user").(*models.User)
	if !ok {
		return nil, fmt.Errorf("unauthorized")
	}
	return user, nil
}

func (s *Schema) resolveUser(p graphql.ResolveParams) (interface{}, error) {
	uid := p.Args["uid"].(string)
	return s.ldapMgr.GetUser(p.Context, uid)
}

func (s *Schema) resolveUsers(p graphql.ResolveParams) (interface{}, error) {
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
	var filter *models.SearchFilter
	if filterInput, ok := p.Args["filter"].(map[string]interface{}); ok {
		filter = &models.SearchFilter{}
		if uid, ok := filterInput["uid"].(string); ok {
			filter.UID = uid
		}
		if cn, ok := filterInput["cn"].(string); ok {
			filter.CN = cn
		}
		if sn, ok := filterInput["sn"].(string); ok {
			filter.SN = sn
		}
		if givenName, ok := filterInput["givenName"].(string); ok {
			filter.GivenName = givenName
		}
		if mail, ok := filterInput["mail"].(string); ok {
			filter.Mail = mail
		}
		if dept, ok := filterInput["department"].(string); ok {
			filter.Department = dept
		}
		if uidNumber, ok := filterInput["uidNumber"].(int); ok {
			filter.UIDNumber = uidNumber
		}
		if gidNumber, ok := filterInput["gidNumber"].(int); ok {
			filter.GIDNumber = gidNumber
		}
		if repo, ok := filterInput["repository"].(string); ok {
			filter.Repository = repo
		}
	}

	// Get all users matching filter
	allUsers, err := s.ldapMgr.ListUsers(p.Context, filter)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list users")
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// Apply pagination
	total := len(allUsers)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedUsers := allUsers[start:end]

	return map[string]interface{}{
		"items":   paginatedUsers,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"hasMore": end < total,
	}, nil
}

func (s *Schema) resolveUsersAll(p graphql.ResolveParams) (interface{}, error) {
	// Get all users without pagination or filtering (for microservice calls)
	allUsers, err := s.ldapMgr.ListUsers(p.Context, nil)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list all users")
		return nil, fmt.Errorf("failed to list all users: %w", err)
	}
	return allUsers, nil
}

// ============================================================================
// USER MUTATION RESOLVERS
// ============================================================================

func (s *Schema) resolveCreateUser(p graphql.ResolveParams) (interface{}, error) {
	inputMap := p.Args["input"].(map[string]interface{})

	input := &models.CreateUserInput{
		UID:        inputMap["uid"].(string),
		CN:         inputMap["cn"].(string),
		SN:         inputMap["sn"].(string),
		GivenName:  inputMap["givenName"].(string),
		Mail:       inputMap["mail"].(string),
		Department: inputMap["department"].(string),
		Password:   inputMap["password"].(string),
	}

	if repos, ok := inputMap["repositories"].([]interface{}); ok {
		input.Repositories = make([]string, len(repos))
		for i, r := range repos {
			input.Repositories[i] = r.(string)
		}
	}

	return s.ldapMgr.CreateUser(p.Context, input)
}

func (s *Schema) resolveUpdateUser(p graphql.ResolveParams) (interface{}, error) {
	inputMap := p.Args["input"].(map[string]interface{})

	input := &models.UpdateUserInput{
		UID: inputMap["uid"].(string),
	}

	if cn, ok := inputMap["cn"].(string); ok {
		input.CN = &cn
	}
	if sn, ok := inputMap["sn"].(string); ok {
		input.SN = &sn
	}
	if givenName, ok := inputMap["givenName"].(string); ok {
		input.GivenName = &givenName
	}
	if mail, ok := inputMap["mail"].(string); ok {
		input.Mail = &mail
	}
	if dept, ok := inputMap["department"].(string); ok {
		input.Department = &dept
	}
	if password, ok := inputMap["password"].(string); ok {
		input.Password = &password
	}
	if repos, ok := inputMap["repositories"].([]interface{}); ok {
		input.Repositories = make([]string, len(repos))
		for i, r := range repos {
			input.Repositories[i] = r.(string)
		}
	}

	return s.ldapMgr.UpdateUser(p.Context, input)
}

func (s *Schema) resolveDeleteUser(p graphql.ResolveParams) (interface{}, error) {
	uid := p.Args["uid"].(string)
	err := s.ldapMgr.DeleteUser(p.Context, uid)
	return err == nil, err
}

func (s *Schema) resolveAssignRepoToUser(p graphql.ResolveParams) (interface{}, error) {
	uid := p.Args["uid"].(string)
	repoInterfaces := p.Args["repositories"].([]interface{})

	repos := make([]string, len(repoInterfaces))
	for i, r := range repoInterfaces {
		repos[i] = r.(string)
	}

	input := &models.UpdateUserInput{
		UID:          uid,
		Repositories: repos,
	}

	return s.ldapMgr.UpdateUser(p.Context, input)
}
