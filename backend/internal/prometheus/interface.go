package prometheus

import (
	"context"

	"github.com/devplatform/ldap-manager/internal/models"
)

// LDAPInterface defines all methods from ldap.Manager that are used by GraphQL
// This allows us to wrap the Manager with metrics collection
type LDAPInterface interface {
	// ═══════════════════════════════════════════════════════════════════════════
	// USER OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// CreateUser creates a new user in LDAP
	CreateUser(ctx context.Context, input *models.CreateUserInput) (*models.User, error)

	// GetUser retrieves a user by UID
	GetUser(ctx context.Context, uid string) (*models.User, error)

	// ListUsers lists users with optional filtering
	ListUsers(ctx context.Context, filter *models.SearchFilter) ([]*models.User, error)

	// UpdateUser updates user attributes
	UpdateUser(ctx context.Context, input *models.UpdateUserInput) (*models.User, error)

	// DeleteUser deletes a user from LDAP
	DeleteUser(ctx context.Context, uid string) error

	// Authenticate authenticates a user with their password
	Authenticate(ctx context.Context, uid, password string) (*models.User, error)

	// ═══════════════════════════════════════════════════════════════════════════
	// GROUP OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// CreateGroup creates a new group
	CreateGroup(ctx context.Context, cn, description string) (*models.Group, error)

	// GetGroup retrieves a group by CN
	GetGroup(ctx context.Context, cn string) (*models.Group, error)

	// ListGroups lists all groups
	ListGroups(ctx context.Context) ([]*models.Group, error)

	// DeleteGroup deletes a group by CN
	DeleteGroup(ctx context.Context, cn string) error

	// AddUserToGroup adds a user to a group
	AddUserToGroup(ctx context.Context, uid, groupCN string) error

	// RemoveUserFromGroup removes a user from a group
	RemoveUserFromGroup(ctx context.Context, uid, groupCN string) error

	// AssignRepositoriesToGroup assigns repositories to a group
	AssignRepositoriesToGroup(ctx context.Context, cn string, repositories []string) (*models.Group, error)

	// ═══════════════════════════════════════════════════════════════════════════
	// DEPARTMENT OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// CreateDepartment creates a new department
	CreateDepartment(ctx context.Context, input *models.CreateDepartmentInput) (*models.Department, error)

	// GetDepartment retrieves a department by OU
	GetDepartment(ctx context.Context, ou string) (*models.Department, error)

	// ListDepartments lists all departments
	ListDepartments(ctx context.Context) ([]*models.Department, error)

	// DeleteDepartment deletes a department
	DeleteDepartment(ctx context.Context, ou string) error

	// AssignRepositoryToDepartment assigns repositories to a department
	AssignRepositoryToDepartment(ctx context.Context, ou string, repos []string) error

	// GetUsersByDepartment retrieves all users in a department
	GetUsersByDepartment(ctx context.Context, department string) ([]*models.User, error)

	// ═══════════════════════════════════════════════════════════════════════════
	// HEALTH & STATS
	// ═══════════════════════════════════════════════════════════════════════════

	// HealthCheck performs a health check on the LDAP connection
	HealthCheck(ctx context.Context) error

	// GetStats returns connection pool statistics
	GetStats() *models.Stats
}
