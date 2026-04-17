package prometheus

import (
        "context"
        "time"

        "github.com/devplatform/ldap-manager/internal/models"
)

// LDAPCollector wraps an LDAPInterface and records metrics for all operations
type LDAPCollector struct {
        next LDAPInterface
}

// NewLDAPCollector creates a new instrumented wrapper around an LDAPInterface
func NewLDAPCollector(next LDAPInterface) *LDAPCollector {
        return &LDAPCollector{next: next}
}

// recordOperation records duration and count for an operation
func recordOperation(operation string, start time.Time, err error) {
        success := "true"
        if err != nil {
                success = "false"
        }

        OperationDuration.WithLabelValues(operation, success).Observe(time.Since(start).Seconds())
        OperationsTotal.WithLabelValues(operation, success).Inc()
}

// updatePoolMetrics updates connection pool gauges from stats
func updatePoolMetrics(stats *models.Stats) {
        if stats == nil {
                return
        }
        PoolSize.Set(float64(stats.PoolSize))
        PoolIdleConnections.Set(float64(stats.Available))
        PoolActiveConnections.Set(float64(stats.InUse))
        PoolTotalRequests.Set(float64(stats.TotalRequests))
}

// updateEntityCounts updates user, group, department counts
// This is called after mutations to keep gauges up-to-date
func (c *LDAPCollector) updateEntityCounts(ctx context.Context) {
        // Update users count
        users, _ := c.next.ListUsers(ctx, nil)
        if users != nil {
                UsersTotal.Set(float64(len(users)))
        }

        // Update groups count
        groups, _ := c.next.ListGroups(ctx)
        if groups != nil {
                GroupsTotal.Set(float64(len(groups)))
        }

        // Update departments count
        depts, _ := c.next.ListDepartments(ctx)
        if depts != nil {
                DepartmentsTotal.Set(float64(len(depts)))
        }
}

// ═══════════════════════════════════════════════════════════════════════════
// USER OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *LDAPCollector) CreateUser(ctx context.Context, input *models.CreateUserInput) (*models.User, error) {
        start := time.Now()
        user, err := c.next.CreateUser(ctx, input)
        recordOperation("create_user", start, err)

        if err == nil {
                // Increment counter with department label
                dept := input.Department
                if dept == "" {
                        dept = "unknown"
                }
                UsersCreatedTotal.WithLabelValues(dept).Inc()

                // Update total count
                go c.updateEntityCounts(context.Background())
        }

        return user, err
}

func (c *LDAPCollector) GetUser(ctx context.Context, uid string) (*models.User, error) {
        start := time.Now()
        user, err := c.next.GetUser(ctx, uid)
        recordOperation("get_user", start, err)
        return user, err
}

func (c *LDAPCollector) ListUsers(ctx context.Context, filter *models.SearchFilter) ([]*models.User, error) {
        start := time.Now()
        users, err := c.next.ListUsers(ctx, filter)
        recordOperation("list_users", start, err)

        // Update gauge on list (read operations can update the count too)
        if err == nil && filter == nil {
                UsersTotal.Set(float64(len(users)))
        }

        return users, err
}

func (c *LDAPCollector) UpdateUser(ctx context.Context, input *models.UpdateUserInput) (*models.User, error) {
        start := time.Now()
        user, err := c.next.UpdateUser(ctx, input)
        recordOperation("update_user", start, err)

        if err == nil {
                UsersUpdatedTotal.Inc()
        }

        return user, err
}

func (c *LDAPCollector) DeleteUser(ctx context.Context, uid string) error {
        start := time.Now()
        err := c.next.DeleteUser(ctx, uid)
        recordOperation("delete_user", start, err)

        if err == nil {
                UsersDeletedTotal.Inc()
                go c.updateEntityCounts(context.Background())
        }

        return err
}

func (c *LDAPCollector) Authenticate(ctx context.Context, uid, password string) (*models.User, error) {
        start := time.Now()
        user, err := c.next.Authenticate(ctx, uid, password)

        // Record auth-specific metrics
        AuthDuration.Observe(time.Since(start).Seconds())

        success := "true"
        if err != nil {
                success = "false"
        }
        AuthAttemptsTotal.WithLabelValues(success).Inc()

        // Also record as general operation
        recordOperation("authenticate", start, err)

        return user, err
}

// ═══════════════════════════════════════════════════════════════════════════
// GROUP OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *LDAPCollector) CreateGroup(ctx context.Context, cn, description string) (*models.Group, error) {
        start := time.Now()
        group, err := c.next.CreateGroup(ctx, cn, description)
        recordOperation("create_group", start, err)

        if err == nil {
                GroupsCreatedTotal.Inc()
                go c.updateEntityCounts(context.Background())
        }

        return group, err
}

func (c *LDAPCollector) GetGroup(ctx context.Context, cn string) (*models.Group, error) {
        start := time.Now()
        group, err := c.next.GetGroup(ctx, cn)
        recordOperation("get_group", start, err)
        return group, err
}

func (c *LDAPCollector) ListGroups(ctx context.Context) ([]*models.Group, error) {
        start := time.Now()
        groups, err := c.next.ListGroups(ctx)
        recordOperation("list_groups", start, err)

        // Update gauge
        if err == nil {
                GroupsTotal.Set(float64(len(groups)))
        }

        return groups, err
}

func (c *LDAPCollector) DeleteGroup(ctx context.Context, cn string) error {
        start := time.Now()
        err := c.next.DeleteGroup(ctx, cn)
        recordOperation("delete_group", start, err)

        if err == nil {
                GroupsDeletedTotal.Inc()
                go c.updateEntityCounts(context.Background())
        }

        return err
}

func (c *LDAPCollector) AddUserToGroup(ctx context.Context, uid, groupCN string) error {
        start := time.Now()
        err := c.next.AddUserToGroup(ctx, uid, groupCN)
        recordOperation("add_user_to_group", start, err)

        if err == nil {
                GroupMembershipsChanged.WithLabelValues("add").Inc()
        }

        return err
}

func (c *LDAPCollector) RemoveUserFromGroup(ctx context.Context, uid, groupCN string) error {
        start := time.Now()
        err := c.next.RemoveUserFromGroup(ctx, uid, groupCN)
        recordOperation("remove_user_from_group", start, err)

        if err == nil {
                GroupMembershipsChanged.WithLabelValues("remove").Inc()
        }

        return err
}

func (c *LDAPCollector) AssignRepositoriesToGroup(ctx context.Context, cn string, repositories []string) (*models.Group, error) {
        start := time.Now()
        group, err := c.next.AssignRepositoriesToGroup(ctx, cn, repositories)
        recordOperation("assign_repos_to_group", start, err)

        if err == nil {
                RepoAssignmentsTotal.WithLabelValues("group").Add(float64(len(repositories)))
        }

        return group, err
}

// ═══════════════════════════════════════════════════════════════════════════
// DEPARTMENT OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *LDAPCollector) CreateDepartment(ctx context.Context, input *models.CreateDepartmentInput) (*models.Department, error) {
        start := time.Now()
        dept, err := c.next.CreateDepartment(ctx, input)
        recordOperation("create_department", start, err)

        if err == nil {
                DepartmentsCreatedTotal.Inc()
                go c.updateEntityCounts(context.Background())
        }

        return dept, err
}

func (c *LDAPCollector) GetDepartment(ctx context.Context, ou string) (*models.Department, error) {
        start := time.Now()
        dept, err := c.next.GetDepartment(ctx, ou)
        recordOperation("get_department", start, err)
        return dept, err
}

func (c *LDAPCollector) ListDepartments(ctx context.Context) ([]*models.Department, error) {
        start := time.Now()
        depts, err := c.next.ListDepartments(ctx)
        recordOperation("list_departments", start, err)

        // Update gauge
        if err == nil {
                DepartmentsTotal.Set(float64(len(depts)))
        }

        return depts, err
}

func (c *LDAPCollector) DeleteDepartment(ctx context.Context, ou string) error {
        start := time.Now()
        err := c.next.DeleteDepartment(ctx, ou)
        recordOperation("delete_department", start, err)

        if err == nil {
                DepartmentsDeletedTotal.Inc()
                go c.updateEntityCounts(context.Background())
        }

        return err
}

func (c *LDAPCollector) AssignRepositoryToDepartment(ctx context.Context, ou string, repos []string) error {
        start := time.Now()
        err := c.next.AssignRepositoryToDepartment(ctx, ou, repos)
        recordOperation("assign_repos_to_department", start, err)

        if err == nil {
                RepoAssignmentsTotal.WithLabelValues("department").Add(float64(len(repos)))
        }

        return err
}

func (c *LDAPCollector) GetUsersByDepartment(ctx context.Context, department string) ([]*models.User, error) {
        start := time.Now()
        users, err := c.next.GetUsersByDepartment(ctx, department)
        recordOperation("get_users_by_department", start, err)
        return users, err
}

// ═══════════════════════════════════════════════════════════════════════════
// HEALTH & STATS
// ═══════════════════════════════════════════════════════════════════════════

func (c *LDAPCollector) HealthCheck(ctx context.Context) error {
        start := time.Now()
        err := c.next.HealthCheck(ctx)
        recordOperation("health_check", start, err)
        return err
}

func (c *LDAPCollector) GetStats() *models.Stats {
        start := time.Now()
        stats := c.next.GetStats()

        // Record operation (GetStats doesn't return error)
        OperationDuration.WithLabelValues("get_stats", "true").Observe(time.Since(start).Seconds())
        OperationsTotal.WithLabelValues("get_stats", "true").Inc()

        // Update pool metrics gauges
        updatePoolMetrics(stats)

        return stats
}

// ═══════════════════════════════════════════════════════════════════════════
// HELPER METHODS FOR EXTERNAL USE
// ═══════════════════════════════════════════════════════════════════════════

// RefreshCounts forces an update of all entity count gauges
// This can be called periodically or on-demand
func (c *LDAPCollector) RefreshCounts(ctx context.Context) {
        c.updateEntityCounts(ctx)
}

// UpdatePoolMetrics manually updates pool metrics from a Stats object
// This can be called from outside if needed
func UpdatePoolMetrics(stats *models.Stats) {
        updatePoolMetrics(stats)
}


