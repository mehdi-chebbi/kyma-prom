package prometheus

import (
	promclient "github.com/prometheus/client_golang/prometheus"
)

// Business-level metrics for LDAP Manager
// These track actual business operations, not just HTTP requests

var (
	// ═══════════════════════════════════════════════════════════════════════════
	// USER METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// UsersCreatedTotal - Counter of users created, labeled by department
	UsersCreatedTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "ldap_users_created_total",
			Help: "Total number of users created in LDAP",
		},
		[]string{"department"},
	)

	// UsersDeletedTotal - Counter of users deleted
	UsersDeletedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "ldap_users_deleted_total",
			Help: "Total number of users deleted from LDAP",
		},
	)

	// UsersTotal - Gauge of current total users (updated on each change)
	UsersTotal = promclient.NewGauge(
		promclient.GaugeOpts{
			Name: "ldap_users_total",
			Help: "Current total number of users in LDAP",
		},
	)

	// UsersUpdatedTotal - Counter of user updates
	UsersUpdatedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "ldap_users_updated_total",
			Help: "Total number of user updates in LDAP",
		},
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// GROUP METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// GroupsCreatedTotal - Counter of groups created
	GroupsCreatedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "ldap_groups_created_total",
			Help: "Total number of groups created in LDAP",
		},
	)

	// GroupsDeletedTotal - Counter of groups deleted
	GroupsDeletedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "ldap_groups_deleted_total",
			Help: "Total number of groups deleted from LDAP",
		},
	)

	// GroupsTotal - Gauge of current total groups
	GroupsTotal = promclient.NewGauge(
		promclient.GaugeOpts{
			Name: "ldap_groups_total",
			Help: "Current total number of groups in LDAP",
		},
	)

	// GroupMembershipsChanged - Counter of group membership changes
	GroupMembershipsChanged = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "ldap_group_memberships_changed_total",
			Help: "Total number of group membership changes",
		},
		[]string{"action"}, // "add" or "remove"
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// DEPARTMENT METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// DepartmentsCreatedTotal - Counter of departments created
	DepartmentsCreatedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "ldap_departments_created_total",
			Help: "Total number of departments created in LDAP",
		},
	)

	// DepartmentsDeletedTotal - Counter of departments deleted
	DepartmentsDeletedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "ldap_departments_deleted_total",
			Help: "Total number of departments deleted from LDAP",
		},
	)

	// DepartmentsTotal - Gauge of current total departments
	DepartmentsTotal = promclient.NewGauge(
		promclient.GaugeOpts{
			Name: "ldap_departments_total",
			Help: "Current total number of departments in LDAP",
		},
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// OPERATION DURATION METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// OperationDuration - Histogram of LDAP operation durations
	OperationDuration = promclient.NewHistogramVec(
		promclient.HistogramOpts{
			Name:    "ldap_operation_duration_seconds",
			Help:    "Duration of LDAP operations in seconds",
			Buckets: promclient.DefBuckets, // .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
		},
		[]string{"operation", "success"}, // operation name, "true" or "false"
	)

	// OperationsTotal - Counter of all LDAP operations
	OperationsTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "ldap_operations_total",
			Help: "Total number of LDAP operations",
		},
		[]string{"operation", "success"}, // operation name, "true" or "false"
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// AUTHENTICATION METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// AuthAttemptsTotal - Counter of authentication attempts
	AuthAttemptsTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "ldap_auth_attempts_total",
			Help: "Total number of LDAP authentication attempts",
		},
		[]string{"success"}, // "true" or "false"
	)

	// AuthDuration - Histogram of authentication duration
	AuthDuration = promclient.NewHistogram(
		promclient.HistogramOpts{
			Name:    "ldap_auth_duration_seconds",
			Help:    "Duration of LDAP authentication attempts in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// CONNECTION POOL METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// PoolActiveConnections - Gauge of active (in-use) connections
	PoolActiveConnections = promclient.NewGauge(
		promclient.GaugeOpts{
			Name: "ldap_pool_active_connections",
			Help: "Number of active (in-use) LDAP connections",
		},
	)

	// PoolIdleConnections - Gauge of idle (available) connections
	PoolIdleConnections = promclient.NewGauge(
		promclient.GaugeOpts{
			Name: "ldap_pool_idle_connections",
			Help: "Number of idle (available) LDAP connections",
		},
	)

	// PoolTotalRequests - Counter of total pool requests
	PoolTotalRequests = promclient.NewGauge(
		promclient.GaugeOpts{
			Name: "ldap_pool_total_requests",
			Help: "Total number of LDAP connection pool requests",
		},
	)

	// PoolSize - Gauge of pool size
	PoolSize = promclient.NewGauge(
		promclient.GaugeOpts{
			Name: "ldap_pool_size",
			Help: "Size of the LDAP connection pool",
		},
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// REPOSITORY ASSIGNMENT METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// RepoAssignmentsTotal - Counter of repository assignments
	RepoAssignmentsTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "ldap_repo_assignments_total",
			Help: "Total number of repository assignments",
		},
		[]string{"entity_type"}, // "user", "group", "department"
	)
)

// Init registers all metrics with Prometheus
func Init() {
	promclient.MustRegister(
		UsersCreatedTotal,
		UsersDeletedTotal,
		UsersTotal,
		UsersUpdatedTotal,
		GroupsCreatedTotal,
		GroupsDeletedTotal,
		GroupsTotal,
		GroupMembershipsChanged,
		DepartmentsCreatedTotal,
		DepartmentsDeletedTotal,
		DepartmentsTotal,
		OperationDuration,
		OperationsTotal,
		AuthAttemptsTotal,
		AuthDuration,
		PoolActiveConnections,
		PoolIdleConnections,
		PoolTotalRequests,
		PoolSize,
		RepoAssignmentsTotal,
	)
}
