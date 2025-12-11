package kubelet

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
)

// MetricsCollectorInterface defines the interface for collecting volume metrics
type MetricsCollectorInterface interface {
	GetVolumeMetrics(ctx context.Context, namespacedName types.NamespacedName) (*VolumeMetrics, error)
	GetAllVolumeMetrics(ctx context.Context) (*MetricsCache, error)
}

// Ensure MetricsCollector implements the interface
var _ MetricsCollectorInterface = (*MetricsCollector)(nil)
