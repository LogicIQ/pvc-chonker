package kubelet

import (
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

func TestParseVolumeMetrics_WithInodes(t *testing.T) {
	mc := &MetricsCollector{}
	namespacedName := types.NamespacedName{Namespace: "test-ns", Name: "test-pvc"}

	metricsText := `
kubelet_volume_stats_capacity_bytes{namespace="test-ns",persistentvolumeclaim="test-pvc"} 1073741824
kubelet_volume_stats_available_bytes{namespace="test-ns",persistentvolumeclaim="test-pvc"} 536870912
kubelet_volume_stats_inodes{namespace="test-ns",persistentvolumeclaim="test-pvc"} 65536
kubelet_volume_stats_inodes_used{namespace="test-ns",persistentvolumeclaim="test-pvc"} 32768
`

	metrics, err := mc.parseVolumeMetrics(metricsText, namespacedName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.CapacityBytes != 1073741824 {
		t.Errorf("expected capacity 1073741824, got %d", metrics.CapacityBytes)
	}

	if metrics.AvailableBytes != 536870912 {
		t.Errorf("expected available 536870912, got %d", metrics.AvailableBytes)
	}

	if metrics.InodesTotal != 65536 {
		t.Errorf("expected inodes total 65536, got %d", metrics.InodesTotal)
	}

	if metrics.InodesUsed != 32768 {
		t.Errorf("expected inodes used 32768, got %d", metrics.InodesUsed)
	}

	if metrics.InodesFree != 32768 {
		t.Errorf("expected inodes free 32768, got %d", metrics.InodesFree)
	}

	expectedInodesUsagePercent := 50.0
	if metrics.InodesUsagePercent != expectedInodesUsagePercent {
		t.Errorf("expected inodes usage percent %f, got %f", expectedInodesUsagePercent, metrics.InodesUsagePercent)
	}
}

func TestParseVolumeMetrics_WithoutInodes(t *testing.T) {
	mc := &MetricsCollector{}
	namespacedName := types.NamespacedName{Namespace: "test-ns", Name: "test-pvc"}

	metricsText := `
kubelet_volume_stats_capacity_bytes{namespace="test-ns",persistentvolumeclaim="test-pvc"} 1073741824
kubelet_volume_stats_available_bytes{namespace="test-ns",persistentvolumeclaim="test-pvc"} 536870912
`

	metrics, err := mc.parseVolumeMetrics(metricsText, namespacedName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.InodesTotal != 0 {
		t.Errorf("expected inodes total 0, got %d", metrics.InodesTotal)
	}

	if metrics.InodesUsagePercent != 0 {
		t.Errorf("expected inodes usage percent 0, got %f", metrics.InodesUsagePercent)
	}
}
