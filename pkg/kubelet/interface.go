package kubelet

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
)

type MetricsCollectorInterface interface {
	GetVolumeMetrics(ctx context.Context, namespacedName types.NamespacedName) (*VolumeMetrics, error)
	GetAllVolumeMetrics(ctx context.Context) (*MetricsCache, error)
}

var _ MetricsCollectorInterface = (*MetricsCollector)(nil)
