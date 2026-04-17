package prometheus

import (
	promclient "github.com/prometheus/client_golang/prometheus"
)

// Business-level metrics for CodeServer Service
// These track workspace provisioning, storage, and Kubernetes operations

var (
	// ═══════════════════════════════════════════════════════════════════════════
	// WORKSPACE METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// WorkspacesCreatedTotal - Counter of workspaces created
	WorkspacesCreatedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "codeserver_workspaces_created_total",
			Help: "Total number of code-server workspaces created",
		},
	)

	// WorkspacesDeletedTotal - Counter of workspaces deleted
	WorkspacesDeletedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "codeserver_workspaces_deleted_total",
			Help: "Total number of code-server workspaces deleted",
		},
	)

	// WorkspacesStartedTotal - Counter of workspaces started
	WorkspacesStartedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "codeserver_workspaces_started_total",
			Help: "Total number of code-server workspaces started",
		},
	)

	// WorkspacesStoppedTotal - Counter of workspaces stopped
	WorkspacesStoppedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "codeserver_workspaces_stopped_total",
			Help: "Total number of code-server workspaces stopped",
		},
	)

	// WorkspacesActiveTotal - Gauge of currently active workspaces
	WorkspacesActiveTotal = promclient.NewGauge(
		promclient.GaugeOpts{
			Name: "codeserver_workspaces_active_total",
			Help: "Current number of active code-server workspaces",
		},
	)

	// WorkspaceProvisionDuration - Histogram of workspace provisioning duration
	WorkspaceProvisionDuration = promclient.NewHistogram(
		promclient.HistogramOpts{
			Name:    "codeserver_workspace_provision_duration_seconds",
			Help:    "Duration of workspace provisioning operations in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
		},
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// STORAGE (PVC) METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// PVCCreatedTotal - Counter of PVCs created
	PVCCreatedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "codeserver_pvc_created_total",
			Help: "Total number of persistent volume claims created",
		},
	)

	// PVCDeletedTotal - Counter of PVCs deleted
	PVCDeletedTotal = promclient.NewCounter(
		promclient.CounterOpts{
			Name: "codeserver_pvc_deleted_total",
			Help: "Total number of persistent volume claims deleted",
		},
	)

	// PVCSizeBytes - Gauge of PVC size per user
	PVCSizeBytes = promclient.NewGaugeVec(
		promclient.GaugeOpts{
			Name: "codeserver_pvc_size_bytes",
			Help: "Size of persistent volume claims in bytes",
		},
		[]string{"user"},
	)

	// PVCTotalBytes - Gauge of total PVC storage used
	PVCTotalBytes = promclient.NewGauge(
		promclient.GaugeOpts{
			Name: "codeserver_pvc_total_bytes",
			Help: "Total bytes of persistent storage used by all workspaces",
		},
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// KUBERNETES OPERATION METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// K8sPodCreatesTotal - Counter of pod creation operations
	K8sPodCreatesTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "k8s_pod_creates_total",
			Help: "Total number of Kubernetes pod creation operations",
		},
		[]string{"success"},
	)

	// K8sPodDeletesTotal - Counter of pod deletion operations
	K8sPodDeletesTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "k8s_pod_deletes_total",
			Help: "Total number of Kubernetes pod deletion operations",
		},
		[]string{"success"},
	)

	// K8sServiceCreatesTotal - Counter of service creation operations
	K8sServiceCreatesTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "k8s_service_creates_total",
			Help: "Total number of Kubernetes service creation operations",
		},
		[]string{"success"},
	)

	// K8sServiceDeletesTotal - Counter of service deletion operations
	K8sServiceDeletesTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "k8s_service_deletes_total",
			Help: "Total number of Kubernetes service deletion operations",
		},
		[]string{"success"},
	)

	// K8sPVCCreatesTotal - Counter of PVC creation operations
	K8sPVCCreatesTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "k8s_pvc_creates_total",
			Help: "Total number of Kubernetes PVC creation operations",
		},
		[]string{"success"},
	)

	// K8sPVCDeletesTotal - Counter of PVC deletion operations
	K8sPVCDeletesTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "k8s_pvc_deletes_total",
			Help: "Total number of Kubernetes PVC deletion operations",
		},
		[]string{"success"},
	)

	// K8sVirtualServiceCreatesTotal - Counter of VirtualService creation operations
	K8sVirtualServiceCreatesTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "k8s_virtualservice_creates_total",
			Help: "Total number of Istio VirtualService creation operations",
		},
		[]string{"success"},
	)

	// K8sOperationDuration - Histogram of K8s operation durations
	K8sOperationDuration = promclient.NewHistogramVec(
		promclient.HistogramOpts{
			Name:    "k8s_operation_duration_seconds",
			Help:    "Duration of Kubernetes operations in seconds",
			Buckets: promclient.DefBuckets,
		},
		[]string{"operation", "success"},
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// REPOSITORY SYNC METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// ReposSyncedTotal - Counter of repository sync operations
	ReposSyncedTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "codeserver_repos_synced_total",
			Help: "Total number of repository sync operations",
		},
		[]string{"success"},
	)

	// RepoCloneDuration - Histogram of repository clone duration
	RepoCloneDuration = promclient.NewHistogram(
		promclient.HistogramOpts{
			Name:    "codeserver_repo_clone_duration_seconds",
			Help:    "Duration of repository clone operations during provisioning",
			Buckets: []float64{1, 5, 10, 30, 60, 120},
		},
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// GITEA INTEGRATION METRICS
	// ═══════════════════════════════════════════════════════════════════════════

	// GiteaAPICallsTotal - Counter of Gitea API calls
	GiteaAPICallsTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "codeserver_gitea_api_calls_total",
			Help: "Total number of Gitea API calls",
		},
		[]string{"operation", "success"},
	)

	// GiteaAPIDuration - Histogram of Gitea API call durations
	GiteaAPIDuration = promclient.NewHistogramVec(
		promclient.HistogramOpts{
			Name:    "codeserver_gitea_api_duration_seconds",
			Help:    "Duration of Gitea API calls in seconds",
			Buckets: promclient.DefBuckets,
		},
		[]string{"operation"},
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// BUSINESS OPERATION METRICS (High-level)
	// ═══════════════════════════════════════════════════════════════════════════

	// OperationDuration - Histogram of business operation durations
	OperationDuration = promclient.NewHistogramVec(
		promclient.HistogramOpts{
			Name:    "codeserver_business_operation_duration_seconds",
			Help:    "Duration of business operations in seconds",
			Buckets: promclient.DefBuckets,
		},
		[]string{"operation", "success"},
	)

	// OperationsTotal - Counter of all business operations
	OperationsTotal = promclient.NewCounterVec(
		promclient.CounterOpts{
			Name: "codeserver_business_operations_total",
			Help: "Total number of business operations",
		},
		[]string{"operation", "success"},
	)
)

// Init registers all metrics with Prometheus
func Init() {
	promclient.MustRegister(
		WorkspacesCreatedTotal,
		WorkspacesDeletedTotal,
		WorkspacesStartedTotal,
		WorkspacesStoppedTotal,
		WorkspacesActiveTotal,
		WorkspaceProvisionDuration,
		PVCCreatedTotal,
		PVCDeletedTotal,
		PVCSizeBytes,
		PVCTotalBytes,
		K8sPodCreatesTotal,
		K8sPodDeletesTotal,
		K8sServiceCreatesTotal,
		K8sServiceDeletesTotal,
		K8sPVCCreatesTotal,
		K8sPVCDeletesTotal,
		K8sVirtualServiceCreatesTotal,
		K8sOperationDuration,
		ReposSyncedTotal,
		RepoCloneDuration,
		GiteaAPICallsTotal,
		GiteaAPIDuration,
		OperationDuration,
		OperationsTotal,
	)
}
