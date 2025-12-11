package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are properly registered
	registry := prometheus.NewRegistry()
	
	// Register all metrics including Go runtime metrics
	registry.MustRegister(
		ExpansionsTotal,
		ExpansionFailuresTotal,
		ThresholdReachedTotal,
		MaxSizeReachedTotal,
		PvcUnhealthyTotal,
		ReconciliationStatus,
		LoopDurationSeconds,
		LastReconciliationTime,
		LastUpsizeTime,
		UpsizeStatus,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	// Verify metrics can be gathered without error
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Errorf("Failed to gather metrics: %v", err)
	}

	// Verify we have metrics registered (should include both custom and Go runtime metrics)
	if len(metricFamilies) < 10 {
		t.Errorf("Expected at least 10 metric families, got %d", len(metricFamilies))
	}
	t.Logf("Registered %d metric families (including Go runtime metrics)", len(metricFamilies))
}

func TestExpansionsTotal(t *testing.T) {
	// Reset metric
	ExpansionsTotal.Reset()

	// Test increment
	ExpansionsTotal.WithLabelValues("test-pvc", "default").Inc()
	ExpansionsTotal.WithLabelValues("test-pvc", "default").Inc()
	ExpansionsTotal.WithLabelValues("other-pvc", "kube-system").Inc()

	// Verify values
	if testutil.ToFloat64(ExpansionsTotal.WithLabelValues("test-pvc", "default")) != 2 {
		t.Error("Expected ExpansionsTotal for test-pvc to be 2")
	}
	if testutil.ToFloat64(ExpansionsTotal.WithLabelValues("other-pvc", "kube-system")) != 1 {
		t.Error("Expected ExpansionsTotal for other-pvc to be 1")
	}
}

func TestExpansionFailuresTotal(t *testing.T) {
	// Reset metric
	ExpansionFailuresTotal.Reset()

	// Test increment with different reasons
	ExpansionFailuresTotal.WithLabelValues("test-pvc", "default", "max_size_exceeded").Inc()
	ExpansionFailuresTotal.WithLabelValues("test-pvc", "default", "calculation_error").Inc()

	// Verify values
	if testutil.ToFloat64(ExpansionFailuresTotal.WithLabelValues("test-pvc", "default", "max_size_exceeded")) != 1 {
		t.Error("Expected ExpansionFailuresTotal for max_size_exceeded to be 1")
	}
	if testutil.ToFloat64(ExpansionFailuresTotal.WithLabelValues("test-pvc", "default", "calculation_error")) != 1 {
		t.Error("Expected ExpansionFailuresTotal for calculation_error to be 1")
	}
}

func TestReconciliationStatus(t *testing.T) {
	// Reset metric
	ReconciliationStatus.Reset()

	// Test setting status
	ReconciliationStatus.WithLabelValues("success").Set(1)
	ReconciliationStatus.WithLabelValues("failure").Set(0)

	// Verify values
	if testutil.ToFloat64(ReconciliationStatus.WithLabelValues("success")) != 1 {
		t.Error("Expected ReconciliationStatus success to be 1")
	}
	if testutil.ToFloat64(ReconciliationStatus.WithLabelValues("failure")) != 0 {
		t.Error("Expected ReconciliationStatus failure to be 0")
	}
}

func TestLoopDurationSeconds(t *testing.T) {
	// Test observation
	LoopDurationSeconds.Observe(1.5)
	LoopDurationSeconds.Observe(2.0)
	LoopDurationSeconds.Observe(0.8)

	// Basic test that observations were recorded without error
	t.Log("LoopDurationSeconds observations recorded successfully")
}

func TestUpsizeStatus(t *testing.T) {
	// Reset metric
	UpsizeStatus.Reset()

	// Test setting different statuses
	UpsizeStatus.WithLabelValues("test-pvc", "default", "success").Set(1)
	UpsizeStatus.WithLabelValues("test-pvc", "default", "failure").Set(0)
	UpsizeStatus.WithLabelValues("other-pvc", "kube-system", "success").Set(0)
	UpsizeStatus.WithLabelValues("other-pvc", "kube-system", "failure").Set(1)

	// Verify values
	if testutil.ToFloat64(UpsizeStatus.WithLabelValues("test-pvc", "default", "success")) != 1 {
		t.Error("Expected UpsizeStatus success for test-pvc to be 1")
	}
	if testutil.ToFloat64(UpsizeStatus.WithLabelValues("test-pvc", "default", "failure")) != 0 {
		t.Error("Expected UpsizeStatus failure for test-pvc to be 0")
	}
	if testutil.ToFloat64(UpsizeStatus.WithLabelValues("other-pvc", "kube-system", "failure")) != 1 {
		t.Error("Expected UpsizeStatus failure for other-pvc to be 1")
	}
}

func TestMetricLabels(t *testing.T) {
	// Test that metrics accept the expected labels
	tests := []struct {
		name   string
		metric prometheus.Collector
		labels []string
	}{
		{
			name:   "ExpansionsTotal",
			metric: ExpansionsTotal,
			labels: []string{"pvc-name", "namespace"},
		},
		{
			name:   "ExpansionFailuresTotal", 
			metric: ExpansionFailuresTotal,
			labels: []string{"pvc-name", "namespace", "reason"},
		},
		{
			name:   "UpsizeStatus",
			metric: UpsizeStatus,
			labels: []string{"pvc-name", "namespace", "status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test verifies that the metrics can be created with expected labels
			// without panicking. More detailed label validation would require
			// inspecting the metric descriptor.
			switch m := tt.metric.(type) {
			case *prometheus.CounterVec:
				_ = m.WithLabelValues(make([]string, len(tt.labels))...)
			case *prometheus.GaugeVec:
				_ = m.WithLabelValues(make([]string, len(tt.labels))...)
			}
		})
	}
}

func TestGoRuntimeMetrics(t *testing.T) {
	// Test that Go runtime metrics are available
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather Go runtime metrics: %v", err)
	}

	// Check for expected Go runtime metrics
	expectedMetrics := []string{
		"go_memstats_",
		"go_goroutines",
		"process_cpu_seconds_total",
		"process_resident_memory_bytes",
	}

	found := make(map[string]bool)
	for _, mf := range metricFamilies {
		for _, expected := range expectedMetrics {
			if strings.HasPrefix(mf.GetName(), expected) || mf.GetName() == expected {
				found[expected] = true
			}
		}
	}

	for _, expected := range expectedMetrics {
		if !found[expected] {
			t.Errorf("Expected Go runtime metric %s not found", expected)
		}
	}

	t.Logf("Found %d Go runtime metric families", len(metricFamilies))
}