package kubernetes

import (
        "context"
        "fmt"

        "github.com/devplatform/codeserver-service/internal/config"
        "github.com/sirupsen/logrus"
        istionetworking "istio.io/client-go/pkg/apis/networking/v1beta1"
        istioclient "istio.io/client-go/pkg/clientset/versioned"
        corev1 "k8s.io/api/core/v1"
        "k8s.io/apimachinery/pkg/api/errors"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/client-go/kubernetes"
        "k8s.io/client-go/rest"
        "k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes clientset
type Client struct {
        clientset   *kubernetes.Clientset
        istioClient *istioclient.Clientset
        config      *config.Config
        logger      *logrus.Logger
}

// NewClient creates a new Kubernetes client
func NewClient(cfg *config.Config, logger *logrus.Logger) (*Client, error) {
        var kubeConfig *rest.Config
        var err error

        if cfg.KubeConfig != "" {
                // Out-of-cluster (development)
                logger.Info("Using out-of-cluster Kubernetes configuration")
                kubeConfig, err = clientcmd.BuildConfigFromFlags("", cfg.KubeConfig)
        } else {
                // In-cluster (production)
                logger.Info("Using in-cluster Kubernetes configuration")
                kubeConfig, err = rest.InClusterConfig()
        }

        if err != nil {
                return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
        }

        clientset, err := kubernetes.NewForConfig(kubeConfig)
        if err != nil {
                return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
        }

        istioClient, err := istioclient.NewForConfig(kubeConfig)
        if err != nil {
                logger.WithError(err).Warn("Failed to create Istio client, VirtualService management will be disabled")
        }

        return &Client{
                clientset:   clientset,
                istioClient: istioClient,
                config:      cfg,
                logger:      logger,
        }, nil
}

// EnsureNamespace ensures the code-server namespace exists
func (c *Client) EnsureNamespace(ctx context.Context) error {
        ns := &corev1.Namespace{
                ObjectMeta: metav1.ObjectMeta{
                        Name: c.config.Namespace,
                        Labels: map[string]string{
                                "app":             "codeserver-instances",
                                "istio-injection": "enabled",
                                "managed-by":      "codeserver-service",
                        },
                },
        }

        _, err := c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
        if err != nil {
                if errors.IsAlreadyExists(err) {
                        c.logger.Debug("Namespace already exists")
                        return nil
                }
                return fmt.Errorf("failed to create namespace: %w", err)
        }

        c.logger.WithField("namespace", c.config.Namespace).Info("Created namespace")
        return nil
}

// HealthCheck verifies the Kubernetes connection
func (c *Client) HealthCheck(ctx context.Context) error {
        _, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
        if err != nil {
                return fmt.Errorf("kubernetes health check failed: %w", err)
        }
        return nil
}

// GetIstioClient returns the Istio client (may be nil if not available)
func (c *Client) GetIstioClient() *istioclient.Clientset {
        return c.istioClient
}

// ListUserInstances lists all code-server instances
func (c *Client) ListUserInstances(ctx context.Context, userID string) ([]corev1.Pod, error) {
        labelSelector := fmt.Sprintf("app=code-server,managed-by=codeserver-service")
        if userID != "" {
                sanitizedUser := sanitizeUserID(userID)
                labelSelector = fmt.Sprintf("app=code-server,user=%s,managed-by=codeserver-service", sanitizedUser)
        }

        pods, err := c.clientset.CoreV1().Pods(c.config.Namespace).List(ctx, metav1.ListOptions{
                LabelSelector: labelSelector,
        })
        if err != nil {
                return nil, fmt.Errorf("failed to list pods: %w", err)
        }

        return pods.Items, nil
}

// GetVirtualService gets the VirtualService for a user
func (c *Client) GetVirtualService(ctx context.Context, userID string) (*istionetworking.VirtualService, error) {
        if c.istioClient == nil {
                return nil, fmt.Errorf("istio client not available")
        }

        vsName := fmt.Sprintf("code-server-%s", sanitizeUserID(userID))
        vs, err := c.istioClient.NetworkingV1beta1().VirtualServices(c.config.Namespace).Get(ctx, vsName, metav1.GetOptions{})
        if err != nil {
                return nil, err
        }
        return vs, nil
}

// sanitizeUserID converts a user ID to a valid Kubernetes resource name
func sanitizeUserID(userID string) string {
        result := make([]byte, 0, len(userID))
        for i := 0; i < len(userID); i++ {
                c := userID[i]
                switch {
                case c >= 'a' && c <= 'z':
                        result = append(result, c)
                case c >= 'A' && c <= 'Z':
                        result = append(result, c+32)
                case c >= '0' && c <= '9':
                        result = append(result, c)
                case c == '.' || c == '_' || c == '@':
                        result = append(result, '-')
                case c == '-':
                        result = append(result, c)
                }
        }
        if len(result) > 0 && result[0] == '-' {
                result = result[1:]
        }
        if len(result) > 0 && result[len(result)-1] == '-' {
                result = result[:len(result)-1]
        }
        if len(result) > 63 {
                result = result[:63]
        }
        return string(result)
}
