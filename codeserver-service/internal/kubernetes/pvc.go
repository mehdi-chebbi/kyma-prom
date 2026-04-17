package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsurePVC creates a PVC if it doesn't exist, returns existing if it does
func (c *Client) EnsurePVC(ctx context.Context, userID string) (*corev1.PersistentVolumeClaim, error) {
	pvcName := c.config.GetPVCName(userID)

	// Try to get existing PVC
	existing, err := c.clientset.CoreV1().PersistentVolumeClaims(c.config.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		c.logger.WithField("pvc", pvcName).Debug("PVC already exists")
		return existing, nil
	}

	if !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get PVC: %w", err)
	}

	// Create new PVC
	pvc := c.buildPVC(userID)
	created, err := c.clientset.CoreV1().PersistentVolumeClaims(c.config.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create PVC: %w", err)
	}

	c.logger.WithFields(map[string]interface{}{
		"pvc":  pvcName,
		"user": userID,
		"size": c.config.PVCSize,
	}).Info("Created PVC")

	return created, nil
}

// GetPVC gets the PVC for a user
func (c *Client) GetPVC(ctx context.Context, userID string) (*corev1.PersistentVolumeClaim, error) {
	pvcName := c.config.GetPVCName(userID)
	return c.clientset.CoreV1().PersistentVolumeClaims(c.config.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
}

// DeletePVC deletes a user's PVC (and all their data)
func (c *Client) DeletePVC(ctx context.Context, userID string) error {
	pvcName := c.config.GetPVCName(userID)

	err := c.clientset.CoreV1().PersistentVolumeClaims(c.config.Namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete PVC: %w", err)
	}

	c.logger.WithFields(map[string]interface{}{
		"pvc":  pvcName,
		"user": userID,
	}).Info("Deleted PVC")

	return nil
}

// ListPVCs lists all PVCs managed by this service
func (c *Client) ListPVCs(ctx context.Context) ([]corev1.PersistentVolumeClaim, error) {
	pvcs, err := c.clientset.CoreV1().PersistentVolumeClaims(c.config.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=code-server,managed-by=codeserver-service",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}
	return pvcs.Items, nil
}

// buildPVC creates a PVC specification for a user
func (c *Client) buildPVC(userID string) *corev1.PersistentVolumeClaim {
	pvcName := c.config.GetPVCName(userID)
	storageClass := c.config.PVCStorageClass

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
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
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(c.config.PVCSize),
				},
			},
		},
	}
}
