package kubernetes

import (
	"context"
	"fmt"
	"time"

	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworking "istio.io/client-go/pkg/apis/networking/v1beta1"
	"google.golang.org/protobuf/types/known/durationpb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EnsureService creates a Service for the user's pod
func (c *Client) EnsureService(ctx context.Context, userID string) (*corev1.Service, error) {
	serviceName := c.config.GetServiceName(userID)

	// Check if service exists
	existing, err := c.clientset.CoreV1().Services(c.config.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err == nil {
		c.logger.WithField("service", serviceName).Debug("Service already exists")
		return existing, nil
	}

	if !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	// Create service
	svc := c.buildService(userID)
	created, err := c.clientset.CoreV1().Services(c.config.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	c.logger.WithFields(map[string]interface{}{
		"service": serviceName,
		"user":    userID,
	}).Info("Created service")

	return created, nil
}

// DeleteService removes the user's Service
func (c *Client) DeleteService(ctx context.Context, userID string) error {
	serviceName := c.config.GetServiceName(userID)

	err := c.clientset.CoreV1().Services(c.config.Namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete service: %w", err)
	}

	c.logger.WithFields(map[string]interface{}{
		"service": serviceName,
		"user":    userID,
	}).Info("Deleted service")

	return nil
}

// EnsureVirtualService creates/updates Istio VirtualService for routing with WebSocket support
func (c *Client) EnsureVirtualService(ctx context.Context, userID string) error {
	if c.istioClient == nil {
		c.logger.Warn("Istio client not available, skipping VirtualService creation")
		return nil
	}

	vsName := fmt.Sprintf("code-server-%s", sanitizeUserID(userID))
	serviceName := c.config.GetServiceName(userID)
	host := fmt.Sprintf("code-%s.%s", sanitizeUserID(userID), c.config.BaseDomain)

	vs := &istionetworking.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vsName,
			Namespace: c.config.Namespace,
			Labels: map[string]string{
				"app":        "code-server",
				"user":       sanitizeUserID(userID),
				"managed-by": "codeserver-service",
			},
			Annotations: map[string]string{
				"codeserver.devplatform/user-id": userID,
			},
		},
		Spec: networkingv1beta1.VirtualService{
			Hosts:    []string{host},
			Gateways: []string{"codeserver-gateway"},
			Http: []*networkingv1beta1.HTTPRoute{
				{
					Name: "websocket",
					Match: []*networkingv1beta1.HTTPMatchRequest{
						{
							Headers: map[string]*networkingv1beta1.StringMatch{
								"upgrade": {
									MatchType: &networkingv1beta1.StringMatch_Exact{
										Exact: "websocket",
									},
								},
							},
						},
					},
					Route: []*networkingv1beta1.HTTPRouteDestination{
						{
							Destination: &networkingv1beta1.Destination{
								Host: fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, c.config.Namespace),
								Port: &networkingv1beta1.PortSelector{
									Number: 80,
								},
							},
						},
					},
					Timeout: durationpb.New(120 * time.Second),
					Retries: &networkingv1beta1.HTTPRetry{
						Attempts: 3,
					},
				},
				{
					Name: "http",
					Route: []*networkingv1beta1.HTTPRouteDestination{
						{
							Destination: &networkingv1beta1.Destination{
								Host: fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, c.config.Namespace),
								Port: &networkingv1beta1.PortSelector{
									Number: 80,
								},
							},
						},
					},
					Timeout: durationpb.New(60 * time.Second),
				},
			},
		},
	}

	// Try to update existing, create if not found
	_, err := c.istioClient.NetworkingV1beta1().VirtualServices(c.config.Namespace).Update(ctx, vs, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = c.istioClient.NetworkingV1beta1().VirtualServices(c.config.Namespace).Create(ctx, vs, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create VirtualService: %w", err)
			}
			c.logger.WithFields(map[string]interface{}{
				"virtualService": vsName,
				"host":           host,
				"user":           userID,
			}).Info("Created VirtualService with WebSocket support")
			return nil
		}
		return fmt.Errorf("failed to update VirtualService: %w", err)
	}

	c.logger.WithField("virtualService", vsName).Debug("Updated VirtualService")
	return nil
}

// DeleteVirtualService removes the user's VirtualService
func (c *Client) DeleteVirtualService(ctx context.Context, userID string) error {
	if c.istioClient == nil {
		return nil
	}

	vsName := fmt.Sprintf("code-server-%s", sanitizeUserID(userID))

	err := c.istioClient.NetworkingV1beta1().VirtualServices(c.config.Namespace).Delete(ctx, vsName, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete VirtualService: %w", err)
	}

	c.logger.WithFields(map[string]interface{}{
		"virtualService": vsName,
		"user":           userID,
	}).Info("Deleted VirtualService")

	return nil
}

// EnsureDestinationRule creates/updates DestinationRule for connection pooling
func (c *Client) EnsureDestinationRule(ctx context.Context, userID string) error {
	if c.istioClient == nil {
		return nil
	}

	drName := fmt.Sprintf("code-server-%s", sanitizeUserID(userID))
	serviceName := c.config.GetServiceName(userID)

	dr := &istionetworking.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      drName,
			Namespace: c.config.Namespace,
			Labels: map[string]string{
				"app":        "code-server",
				"user":       sanitizeUserID(userID),
				"managed-by": "codeserver-service",
			},
		},
		Spec: networkingv1beta1.DestinationRule{
			Host: fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, c.config.Namespace),
			TrafficPolicy: &networkingv1beta1.TrafficPolicy{
				ConnectionPool: &networkingv1beta1.ConnectionPoolSettings{
					Tcp: &networkingv1beta1.ConnectionPoolSettings_TCPSettings{
						MaxConnections: 100,
					},
					Http: &networkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						H2UpgradePolicy:          networkingv1beta1.ConnectionPoolSettings_HTTPSettings_UPGRADE,
						MaxRequestsPerConnection: 0, // Unlimited for WebSocket
					},
				},
			},
		},
	}

	_, err := c.istioClient.NetworkingV1beta1().DestinationRules(c.config.Namespace).Update(ctx, dr, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = c.istioClient.NetworkingV1beta1().DestinationRules(c.config.Namespace).Create(ctx, dr, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create DestinationRule: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to update DestinationRule: %w", err)
	}

	return nil
}

// DeleteDestinationRule removes the user's DestinationRule
func (c *Client) DeleteDestinationRule(ctx context.Context, userID string) error {
	if c.istioClient == nil {
		return nil
	}

	drName := fmt.Sprintf("code-server-%s", sanitizeUserID(userID))

	err := c.istioClient.NetworkingV1beta1().DestinationRules(c.config.Namespace).Delete(ctx, drName, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete DestinationRule: %w", err)
	}

	return nil
}

// buildService creates a service specification
func (c *Client) buildService(userID string) *corev1.Service {
	serviceName := c.config.GetServiceName(userID)
	sanitizedUser := sanitizeUserID(userID)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: c.config.Namespace,
			Labels: map[string]string{
				"app":        "code-server",
				"user":       sanitizedUser,
				"managed-by": "codeserver-service",
			},
			Annotations: map[string]string{
				"codeserver.devplatform/user-id": userID,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app":  "code-server",
				"user": sanitizedUser,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// GetAccessURL returns the URL to access the user's code-server
func (c *Client) GetAccessURL(userID string) string {
	return c.config.GetCodeServerURL(userID)
}
