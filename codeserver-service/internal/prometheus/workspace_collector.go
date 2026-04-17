package prometheus

import (
	"context"
	"time"

	"github.com/devplatform/codeserver-service/internal/models"
	corev1 "k8s.io/api/core/v1"
)

// WorkspaceCollector wraps a KubernetesClientInterface and records metrics for all operations
type WorkspaceCollector struct {
	next KubernetesClientInterface
}

// NewWorkspaceCollector creates a new instrumented wrapper around a KubernetesClientInterface
func NewWorkspaceCollector(next KubernetesClientInterface) *WorkspaceCollector {
	return &WorkspaceCollector{next: next}
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

// recordK8sOperation records K8s-specific operation metrics
func recordK8sOperation(operation string, start time.Time, err error) {
	success := "true"
	if err != nil {
		success = "false"
	}

	K8sOperationDuration.WithLabelValues(operation, success).Observe(time.Since(start).Seconds())
}

// ═══════════════════════════════════════════════════════════════════════════
// WORKSPACE (POD) OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *WorkspaceCollector) CreateCodeServerPod(ctx context.Context, userID, repoURL, repoName, repoOwner, branch string) (*corev1.Pod, error) {
	start := time.Now()
	pod, err := c.next.CreateCodeServerPod(ctx, userID, repoURL, repoName, repoOwner, branch)

	success := "true"
	if err != nil {
		success = "false"
	}

	K8sPodCreatesTotal.WithLabelValues(success).Inc()
	recordK8sOperation("create_pod", start, err)
	recordOperation("create_code_server_pod", start, err)

	if err == nil {
		WorkspacesCreatedTotal.Inc()
	}

	return pod, err
}

func (c *WorkspaceCollector) GetCodeServerPod(ctx context.Context, userID string) (*corev1.Pod, error) {
	start := time.Now()
	pod, err := c.next.GetCodeServerPod(ctx, userID)
	recordOperation("get_code_server_pod", start, err)
	return pod, err
}

func (c *WorkspaceCollector) DeleteCodeServerPod(ctx context.Context, userID string) error {
	start := time.Now()
	err := c.next.DeleteCodeServerPod(ctx, userID)

	success := "true"
	if err != nil {
		success = "false"
	}

	K8sPodDeletesTotal.WithLabelValues(success).Inc()
	recordK8sOperation("delete_pod", start, err)
	recordOperation("delete_code_server_pod", start, err)

	if err == nil {
		WorkspacesStoppedTotal.Inc()
	}

	return err
}

func (c *WorkspaceCollector) GetPodStatus(ctx context.Context, userID string) (models.InstanceStatus, string, error) {
	start := time.Now()
	status, msg, err := c.next.GetPodStatus(ctx, userID)
	recordOperation("get_pod_status", start, err)
	return status, msg, err
}

func (c *WorkspaceCollector) GetPodLogs(ctx context.Context, userID string, lines int64) (string, error) {
	start := time.Now()
	logs, err := c.next.GetPodLogs(ctx, userID, lines)
	recordOperation("get_pod_logs", start, err)
	return logs, err
}

func (c *WorkspaceCollector) WaitForPodReady(ctx context.Context, userID string, timeout time.Duration) error {
	start := time.Now()
	err := c.next.WaitForPodReady(ctx, userID, timeout)
	recordOperation("wait_for_pod_ready", start, err)
	return err
}

func (c *WorkspaceCollector) ListUserInstances(ctx context.Context, userID string) ([]corev1.Pod, error) {
	start := time.Now()
	pods, err := c.next.ListUserInstances(ctx, userID)
	recordOperation("list_user_instances", start, err)

	// Update active workspaces gauge
	if err == nil {
		WorkspacesActiveTotal.Set(float64(len(pods)))
	}

	return pods, err
}

func (c *WorkspaceCollector) PodToInstance(pod *corev1.Pod) *models.CodeServerInstance {
	return c.next.PodToInstance(pod)
}

// ═══════════════════════════════════════════════════════════════════════════
// SERVICE OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *WorkspaceCollector) EnsureService(ctx context.Context, userID string) (*corev1.Service, error) {
	start := time.Now()
	svc, err := c.next.EnsureService(ctx, userID)

	success := "true"
	if err != nil {
		success = "false"
	}

	K8sServiceCreatesTotal.WithLabelValues(success).Inc()
	recordK8sOperation("ensure_service", start, err)
	recordOperation("ensure_service", start, err)

	return svc, err
}

func (c *WorkspaceCollector) DeleteService(ctx context.Context, userID string) error {
	start := time.Now()
	err := c.next.DeleteService(ctx, userID)

	success := "true"
	if err != nil {
		success = "false"
	}

	K8sServiceDeletesTotal.WithLabelValues(success).Inc()
	recordK8sOperation("delete_service", start, err)
	recordOperation("delete_service", start, err)

	return err
}

// ═══════════════════════════════════════════════════════════════════════════
// PVC OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *WorkspaceCollector) EnsurePVC(ctx context.Context, userID string) (*corev1.PersistentVolumeClaim, error) {
	start := time.Now()
	pvc, err := c.next.EnsurePVC(ctx, userID)

	success := "true"
	if err != nil {
		success = "false"
	}

	K8sPVCCreatesTotal.WithLabelValues(success).Inc()
	recordK8sOperation("ensure_pvc", start, err)
	recordOperation("ensure_pvc", start, err)

	if err == nil && pvc != nil {
		PVCCreatedTotal.Inc()
		// Update PVC size gauge
		if storage, ok := pvc.Status.Capacity["storage"]; ok {
			PVCSizeBytes.WithLabelValues(userID).Set(float64(storage.Value()))
		}
	}

	return pvc, err
}

func (c *WorkspaceCollector) GetPVC(ctx context.Context, userID string) (*corev1.PersistentVolumeClaim, error) {
	start := time.Now()
	pvc, err := c.next.GetPVC(ctx, userID)
	recordOperation("get_pvc", start, err)
	return pvc, err
}

func (c *WorkspaceCollector) DeletePVC(ctx context.Context, userID string) error {
	start := time.Now()
	err := c.next.DeletePVC(ctx, userID)

	success := "true"
	if err != nil {
		success = "false"
	}

	K8sPVCDeletesTotal.WithLabelValues(success).Inc()
	recordK8sOperation("delete_pvc", start, err)
	recordOperation("delete_pvc", start, err)

	if err == nil {
		PVCDeletedTotal.Inc()
		// Remove PVC size gauge for this user
		PVCSizeBytes.DeleteLabelValues(userID)
	}

	return err
}

func (c *WorkspaceCollector) ListPVCs(ctx context.Context) ([]corev1.PersistentVolumeClaim, error) {
	start := time.Now()
	pvcs, err := c.next.ListPVCs(ctx)
	recordOperation("list_pvcs", start, err)

	// Update total PVC storage gauge
	if err == nil {
		var totalBytes int64
		for _, pvc := range pvcs {
			if storage, ok := pvc.Status.Capacity["storage"]; ok {
				totalBytes += storage.Value()
			}
		}
		PVCTotalBytes.Set(float64(totalBytes))
	}

	return pvcs, err
}

// ═══════════════════════════════════════════════════════════════════════════
// ISTIO OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

func (c *WorkspaceCollector) EnsureVirtualService(ctx context.Context, userID string) error {
	start := time.Now()
	err := c.next.EnsureVirtualService(ctx, userID)

	success := "true"
	if err != nil {
		success = "false"
	}

	K8sVirtualServiceCreatesTotal.WithLabelValues(success).Inc()
	recordK8sOperation("ensure_virtualservice", start, err)
	recordOperation("ensure_virtualservice", start, err)

	return err
}

func (c *WorkspaceCollector) DeleteVirtualService(ctx context.Context, userID string) error {
	start := time.Now()
	err := c.next.DeleteVirtualService(ctx, userID)
	recordK8sOperation("delete_virtualservice", start, err)
	recordOperation("delete_virtualservice", start, err)
	return err
}

func (c *WorkspaceCollector) EnsureDestinationRule(ctx context.Context, userID string) error {
	start := time.Now()
	err := c.next.EnsureDestinationRule(ctx, userID)
	recordK8sOperation("ensure_destinationrule", start, err)
	recordOperation("ensure_destinationrule", start, err)
	return err
}

func (c *WorkspaceCollector) DeleteDestinationRule(ctx context.Context, userID string) error {
	start := time.Now()
	err := c.next.DeleteDestinationRule(ctx, userID)
	recordK8sOperation("delete_destinationrule", start, err)
	recordOperation("delete_destinationrule", start, err)
	return err
}

// ═══════════════════════════════════════════════════════════════════════════
// NAMESPACE & HEALTH
// ═══════════════════════════════════════════════════════════════════════════

func (c *WorkspaceCollector) EnsureNamespace(ctx context.Context) error {
	start := time.Now()
	err := c.next.EnsureNamespace(ctx)
	recordOperation("ensure_namespace", start, err)
	return err
}

func (c *WorkspaceCollector) HealthCheck(ctx context.Context) error {
	start := time.Now()
	err := c.next.HealthCheck(ctx)
	recordOperation("health_check", start, err)
	return err
}

func (c *WorkspaceCollector) GetAccessURL(userID string) string {
	return c.next.GetAccessURL(userID)
}
