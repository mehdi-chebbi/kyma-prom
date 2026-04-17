package prometheus

import (
	"context"
	"time"

	"github.com/devplatform/codeserver-service/internal/gitea"
	"github.com/devplatform/codeserver-service/internal/models"
	corev1 "k8s.io/api/core/v1"
)

// KubernetesClientInterface defines all methods from kubernetes.Client that are used by GraphQL
// This allows us to wrap the Client with metrics collection
type KubernetesClientInterface interface {
	// ═══════════════════════════════════════════════════════════════════════════
	// WORKSPACE (POD) OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// CreateCodeServerPod creates a new code-server pod for a user
	CreateCodeServerPod(ctx context.Context, userID, repoURL, repoName, repoOwner, branch string) (*corev1.Pod, error)

	// GetCodeServerPod gets the current pod for a user
	GetCodeServerPod(ctx context.Context, userID string) (*corev1.Pod, error)

	// DeleteCodeServerPod deletes a user's code-server pod
	DeleteCodeServerPod(ctx context.Context, userID string) error

	// GetPodStatus returns the current status of a pod
	GetPodStatus(ctx context.Context, userID string) (models.InstanceStatus, string, error)

	// GetPodLogs returns logs from a user's pod
	GetPodLogs(ctx context.Context, userID string, lines int64) (string, error)

	// WaitForPodReady waits for pod to be in Running state
	WaitForPodReady(ctx context.Context, userID string, timeout time.Duration) error

	// ListUserInstances lists all code-server instances
	ListUserInstances(ctx context.Context, userID string) ([]corev1.Pod, error)

	// PodToInstance converts a Kubernetes pod to a CodeServerInstance
	PodToInstance(pod *corev1.Pod) *models.CodeServerInstance

	// ═══════════════════════════════════════════════════════════════════════════
	// SERVICE OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// EnsureService creates a Service for the user's pod
	EnsureService(ctx context.Context, userID string) (*corev1.Service, error)

	// DeleteService removes the user's Service
	DeleteService(ctx context.Context, userID string) error

	// ═══════════════════════════════════════════════════════════════════════════
	// PVC OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// EnsurePVC creates a PVC if it doesn't exist
	EnsurePVC(ctx context.Context, userID string) (*corev1.PersistentVolumeClaim, error)

	// GetPVC gets the PVC for a user
	GetPVC(ctx context.Context, userID string) (*corev1.PersistentVolumeClaim, error)

	// DeletePVC deletes a user's PVC
	DeletePVC(ctx context.Context, userID string) error

	// ListPVCs lists all PVCs managed by this service
	ListPVCs(ctx context.Context) ([]corev1.PersistentVolumeClaim, error)

	// ═══════════════════════════════════════════════════════════════════════════
	// ISTIO OPERATIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// EnsureVirtualService creates/updates Istio VirtualService
	EnsureVirtualService(ctx context.Context, userID string) error

	// DeleteVirtualService removes the user's VirtualService
	DeleteVirtualService(ctx context.Context, userID string) error

	// EnsureDestinationRule creates/updates DestinationRule
	EnsureDestinationRule(ctx context.Context, userID string) error

	// DeleteDestinationRule removes the user's DestinationRule
	DeleteDestinationRule(ctx context.Context, userID string) error

	// ═══════════════════════════════════════════════════════════════════════════
	// NAMESPACE & HEALTH
	// ═══════════════════════════════════════════════════════════════════════════

	// EnsureNamespace ensures the code-server namespace exists
	EnsureNamespace(ctx context.Context) error

	// HealthCheck verifies the Kubernetes connection
	HealthCheck(ctx context.Context) error

	// GetAccessURL returns the URL to access the user's code-server
	GetAccessURL(userID string) string
}

// GiteaClientInterface defines methods from gitea.Client that are used by GraphQL
type GiteaClientInterface interface {
	// GetUserRepositories gets repositories accessible by the user
	GetUserRepositories(ctx context.Context, token string) ([]*gitea.Repository, error)

	// ValidateRepoAccess checks if user can access the repository
	ValidateRepoAccess(ctx context.Context, token, owner, repoName string) (bool, error)

	// GetRepoCloneURL returns the clone URL with embedded token for private repos
	GetRepoCloneURL(ctx context.Context, token, owner, repoName string) (string, error)

	// HealthCheck checks if gitea-service is accessible
	HealthCheck(ctx context.Context) error
}
