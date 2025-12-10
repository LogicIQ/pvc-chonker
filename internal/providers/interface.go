package providers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type CloudProvider interface {
	Name() string
	CanExpand(ctx context.Context, pvc *corev1.PersistentVolumeClaim) (bool, error)
	GetVolumeConstraints(ctx context.Context, pvc *corev1.PersistentVolumeClaim) (*VolumeConstraints, error)
	ValidateExpansion(ctx context.Context, pvc *corev1.PersistentVolumeClaim, newSize resource.Quantity) error
}

type VolumeConstraints struct {
	MinSize    resource.Quantity
	MaxSize    resource.Quantity
	StepSize   resource.Quantity
	VolumeType string
}
