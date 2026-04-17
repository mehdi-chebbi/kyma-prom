package graphql

import (
	"fmt"
	"strings"

	"github.com/devplatform/ldap-manager/internal/models"
	"github.com/graphql-go/graphql"
)

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// defineDepartmentType defines the Department GraphQL type
func (s *Schema) defineDepartmentType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Department",
		Fields: graphql.Fields{
			"ou":           &graphql.Field{Type: graphql.String},
			"description":  &graphql.Field{Type: graphql.String},
			"manager":      &graphql.Field{Type: graphql.String},
			"members":      &graphql.Field{Type: graphql.NewList(graphql.String)},
			"repositories": &graphql.Field{Type: graphql.NewList(graphql.String)},
			"dn":           &graphql.Field{Type: graphql.String},
		},
	})
}

// definePaginatedDepartmentsType defines the PaginatedDepartments GraphQL type
func (s *Schema) definePaginatedDepartmentsType(departmentType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "PaginatedDepartments",
		Fields: graphql.Fields{
			"departments": &graphql.Field{
				Type:        graphql.NewList(departmentType),
				Description: "List of departments",
			},
			"total": &graphql.Field{
				Type:        graphql.Int,
				Description: "Total number of departments",
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

// defineCreateDepartmentInput defines the CreateDepartmentInput GraphQL input type
func (s *Schema) defineCreateDepartmentInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CreateDepartmentInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"ou":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"description":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"manager":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"repositories": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
		},
	})
}

// defineDepartmentFilterInput defines the DepartmentFilterInput GraphQL input type
func (s *Schema) defineDepartmentFilterInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "DepartmentFilterInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"ou":          &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Filter by department name (partial match)"},
			"description": &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Filter by description (partial match)"},
		},
	})
}

// ============================================================================
// DEPARTMENT QUERY RESOLVERS
// ============================================================================

func (s *Schema) resolveDepartment(p graphql.ResolveParams) (interface{}, error) {
	ou := p.Args["ou"].(string)
	return s.ldapMgr.GetDepartment(p.Context, ou)
}

func (s *Schema) resolveDepartments(p graphql.ResolveParams) (interface{}, error) {
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
	var filterOU, filterDesc string
	if filterInput, ok := p.Args["filter"].(map[string]interface{}); ok {
		if ou, ok := filterInput["ou"].(string); ok {
			filterOU = ou
		}
		if desc, ok := filterInput["description"].(string); ok {
			filterDesc = desc
		}
	}

	// Get all departments
	allDepartments, err := s.ldapMgr.ListDepartments(p.Context)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list departments")
		return nil, fmt.Errorf("failed to list departments: %w", err)
	}

	// Apply filters
	var filteredDepartments []*models.Department
	for _, dept := range allDepartments {
		// Filter by OU (partial match, case-insensitive)
		if filterOU != "" {
			if !containsIgnoreCase(dept.OU, filterOU) {
				continue
			}
		}
		// Filter by description (partial match, case-insensitive)
		if filterDesc != "" {
			if !containsIgnoreCase(dept.Description, filterDesc) {
				continue
			}
		}
		filteredDepartments = append(filteredDepartments, dept)
	}

	// Apply pagination
	total := len(filteredDepartments)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedDepartments := filteredDepartments[start:end]

	return map[string]interface{}{
		"items":   paginatedDepartments,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"hasMore": end < total,
	}, nil
}

func (s *Schema) resolveDepartmentsAll(p graphql.ResolveParams) (interface{}, error) {
	// Get all departments without pagination (for microservice calls)
	allDepartments, err := s.ldapMgr.ListDepartments(p.Context)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list all departments")
		return nil, fmt.Errorf("failed to list all departments: %w", err)
	}
	return allDepartments, nil
}

func (s *Schema) resolveDepartmentUsers(p graphql.ResolveParams) (interface{}, error) {
	department := p.Args["department"].(string)

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

	// Get all users in department
	allUsers, err := s.ldapMgr.GetUsersByDepartment(p.Context, department)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get department users")
		return nil, fmt.Errorf("failed to get department users: %w", err)
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

// ============================================================================
// DEPARTMENT MUTATION RESOLVERS
// ============================================================================

func (s *Schema) resolveCreateDepartment(p graphql.ResolveParams) (interface{}, error) {
	inputMap := p.Args["input"].(map[string]interface{})

	input := &models.CreateDepartmentInput{
		OU: inputMap["ou"].(string),
	}

	if desc, ok := inputMap["description"].(string); ok {
		input.Description = desc
	}
	if mgr, ok := inputMap["manager"].(string); ok {
		input.Manager = mgr
	}
	if repos, ok := inputMap["repositories"].([]interface{}); ok {
		input.Repositories = make([]string, len(repos))
		for i, r := range repos {
			input.Repositories[i] = r.(string)
		}
	}

	return s.ldapMgr.CreateDepartment(p.Context, input)
}

func (s *Schema) resolveDeleteDepartment(p graphql.ResolveParams) (interface{}, error) {
	ou := p.Args["ou"].(string)
	err := s.ldapMgr.DeleteDepartment(p.Context, ou)
	return err == nil, err
}

func (s *Schema) resolveAssignRepoToDepartment(p graphql.ResolveParams) (interface{}, error) {
	ou := p.Args["ou"].(string)
	repoInterfaces := p.Args["repositories"].([]interface{})

	repos := make([]string, len(repoInterfaces))
	for i, r := range repoInterfaces {
		repos[i] = r.(string)
	}

	if err := s.ldapMgr.AssignRepositoryToDepartment(p.Context, ou, repos); err != nil {
		return nil, err
	}

	return s.ldapMgr.GetDepartment(p.Context, ou)
}
