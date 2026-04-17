package graphql

import (
        "github.com/devplatform/ldap-manager/internal/config"
        "github.com/devplatform/ldap-manager/internal/prometheus"
        "github.com/graphql-go/graphql"
        "github.com/sirupsen/logrus"
)

// Schema represents the GraphQL schema
type Schema struct {
        schema  graphql.Schema
        ldapMgr prometheus.LDAPInterface
        config  *config.Config
        logger  *logrus.Logger
}

// NewSchema creates a new GraphQL schema
func NewSchema(ldapMgr prometheus.LDAPInterface, cfg *config.Config, logger *logrus.Logger) *Schema {
        s := &Schema{
                ldapMgr: ldapMgr,
                config:  cfg,
                logger:  logger,
        }

        // Define types
        userType := s.defineUserType()
        departmentType := s.defineDepartmentType()
        groupType := s.defineGroupType()
        statsType := s.defineStatsType()
        healthType := s.defineHealthType()

        // Define paginated types
        paginatedUsersType := s.definePaginatedUsersType(userType)
        paginatedDepartmentsType := s.definePaginatedDepartmentsType(departmentType)
        paginatedGroupsType := s.definePaginatedGroupsType(groupType)

        // Define input types
        createUserInputType := s.defineCreateUserInput()
        updateUserInputType := s.defineUpdateUserInput()
        createDepartmentInputType := s.defineCreateDepartmentInput()
        searchFilterInputType := s.defineSearchFilterInput()
        departmentFilterInputType := s.defineDepartmentFilterInput()
        groupFilterInputType := s.defineGroupFilterInput()

        // Define root query
        queryType := graphql.NewObject(graphql.ObjectConfig{
                Name: "Query",
                Fields: graphql.Fields{
                        "me": &graphql.Field{
                                Type:    userType,
                                Resolve: s.resolveMe,
                        },
                        "user": &graphql.Field{
                                Type: userType,
                                Args: graphql.FieldConfigArgument{
                                        "uid": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                },
                                Resolve: s.resolveUser,
                        },
                        "users": &graphql.Field{
                                Type: paginatedUsersType,
                                Args: graphql.FieldConfigArgument{
                                        "filter": &graphql.ArgumentConfig{
                                                Type:        searchFilterInputType,
                                                Description: "Search filter for users",
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
                                Resolve: s.resolveUsers,
                        },
                        "usersAll": &graphql.Field{
                                Type:        graphql.NewList(userType),
                                Description: "Get all users without pagination (for microservice calls)",
                                Resolve:     s.resolveUsersAll,
                        },
                        "department": &graphql.Field{
                                Type: departmentType,
                                Args: graphql.FieldConfigArgument{
                                        "ou": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                },
                                Resolve: s.resolveDepartment,
                        },
                        "departments": &graphql.Field{
                                Type: paginatedDepartmentsType,
                                Args: graphql.FieldConfigArgument{
                                        "filter": &graphql.ArgumentConfig{
                                                Type:        departmentFilterInputType,
                                                Description: "Filter departments by name or description",
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
                                Resolve: s.resolveDepartments,
                        },
                        "departmentsAll": &graphql.Field{
                                Type:        graphql.NewList(departmentType),
                                Description: "Get all departments without pagination (for microservice calls)",
                                Resolve:     s.resolveDepartmentsAll,
                        },
                        "departmentUsers": &graphql.Field{
                                Type: paginatedUsersType,
                                Args: graphql.FieldConfigArgument{
                                        "department": &graphql.ArgumentConfig{
                                                Type:        graphql.NewNonNull(graphql.String),
                                                Description: "Department name to filter by",
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
                                Resolve: s.resolveDepartmentUsers,
                        },
                        "group": &graphql.Field{
                                Type: groupType,
                                Args: graphql.FieldConfigArgument{
                                        "cn": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                },
                                Resolve: s.resolveGroup,
                        },
                        "groups": &graphql.Field{
                                Type: paginatedGroupsType,
                                Args: graphql.FieldConfigArgument{
                                        "filter": &graphql.ArgumentConfig{
                                                Type:        groupFilterInputType,
                                                Description: "Filter groups by name",
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
                                Resolve: s.resolveGroups,
                        },
                        "groupsAll": &graphql.Field{
                                Type:        graphql.NewList(groupType),
                                Description: "Get all groups without pagination (for microservice calls)",
                                Resolve:     s.resolveGroupsAll,
                        },
                        "health": &graphql.Field{
                                Type:    healthType,
                                Resolve: s.resolveHealth,
                        },
                        "stats": &graphql.Field{
                                Type:    statsType,
                                Resolve: s.resolveStats,
                        },
                },
        })

        // Define root mutation
        mutationType := graphql.NewObject(graphql.ObjectConfig{
                Name: "Mutation",
                Fields: graphql.Fields{
                        "createUser": &graphql.Field{
                                Type: userType,
                                Args: graphql.FieldConfigArgument{
                                        "input": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(createUserInputType),
                                        },
                                },
                                Resolve: s.resolveCreateUser,
                        },
                        "updateUser": &graphql.Field{
                                Type: userType,
                                Args: graphql.FieldConfigArgument{
                                        "input": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(updateUserInputType),
                                        },
                                },
                                Resolve: s.resolveUpdateUser,
                        },
                        "deleteUser": &graphql.Field{
                                Type: graphql.Boolean,
                                Args: graphql.FieldConfigArgument{
                                        "uid": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                },
                                Resolve: s.resolveDeleteUser,
                        },
                        "createDepartment": &graphql.Field{
                                Type: departmentType,
                                Args: graphql.FieldConfigArgument{
                                        "input": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(createDepartmentInputType),
                                        },
                                },
                                Resolve: s.resolveCreateDepartment,
                        },
                        "deleteDepartment": &graphql.Field{
                                Type: graphql.Boolean,
                                Args: graphql.FieldConfigArgument{
                                        "ou": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                },
                                Resolve: s.resolveDeleteDepartment,
                        },
                        "assignRepoToDepartment": &graphql.Field{
                                Type: departmentType,
                                Args: graphql.FieldConfigArgument{
                                        "ou": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "repositories": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
                                        },
                                },
                                Resolve: s.resolveAssignRepoToDepartment,
                        },
                        "assignRepoToUser": &graphql.Field{
                                Type: userType,
                                Args: graphql.FieldConfigArgument{
                                        "uid": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "repositories": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
                                        },
                                },
                                Resolve: s.resolveAssignRepoToUser,
                        },
                        "assignRepoToGroup": &graphql.Field{
                                Type: groupType,
                                Args: graphql.FieldConfigArgument{
                                        "groupCn": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "repositories": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
                                        },
                                },
                                Resolve: s.resolveAssignRepoToGroup,
                        },
                        "createGroup": &graphql.Field{
                                Type: groupType,
                                Args: graphql.FieldConfigArgument{
                                        "cn": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "description": &graphql.ArgumentConfig{
                                                Type: graphql.String,
                                        },
                                },
                                Resolve: s.resolveCreateGroup,
                        },
                        "addUserToGroup": &graphql.Field{
                                Type: graphql.Boolean,
                                Args: graphql.FieldConfigArgument{
                                        "uid": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "groupCn": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                },
                                Resolve: s.resolveAddUserToGroup,
                        },
                        "removeUserFromGroup": &graphql.Field{
                                Type: graphql.Boolean,
                                Args: graphql.FieldConfigArgument{
                                        "uid": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                        "groupCn": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                },
                                Resolve: s.resolveRemoveUserFromGroup,
                        },
                        "deleteGroup": &graphql.Field{
                                Type: graphql.Boolean,
                                Args: graphql.FieldConfigArgument{
                                        "cn": &graphql.ArgumentConfig{
                                                Type: graphql.NewNonNull(graphql.String),
                                        },
                                },
                                Resolve: s.resolveDeleteGroup,
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

// NOTE: All resolver functions have been moved to their respective type files:
// - User resolvers → types_user.go
// - Department resolvers → types_department.go
// - Group resolvers → types_group.go
// - Common resolvers (login, health, stats) → types_common.go
