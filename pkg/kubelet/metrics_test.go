package kubelet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"
)

func TestGetVolumeMetrics(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		statusCode     int
		namespacedName types.NamespacedName
		wantErr        bool
		expected       *VolumeMetrics
	}{
		{
			name: "successful metrics parsing",
			responseBody: `# HELP kubelet_volume_stats_capacity_bytes Capacity in bytes of the volume
# TYPE kubelet_volume_stats_capacity_bytes gauge
kubelet_volume_stats_capacity_bytes{namespace="test-ns",persistentvolumeclaim="test-pvc"} 10737418240
# HELP kubelet_volume_stats_available_bytes Available bytes in the volume
# TYPE kubelet_volume_stats_available_bytes gauge
kubelet_volume_stats_available_bytes{namespace="test-ns",persistentvolumeclaim="test-pvc"} 2147483648`,
			statusCode:     200,
			namespacedName: types.NamespacedName{Namespace: "test-ns", Name: "test-pvc"},
			wantErr:        false,
			expected: &VolumeMetrics{
				CapacityBytes:  10737418240, // 10Gi
				AvailableBytes: 2147483648,  // 2Gi
				UsedBytes:      8589934592,  // 8Gi
				UsagePercent:   80.0,
			},
		},
		{
			name:           "volume not found",
			responseBody:   `# No metrics for the requested volume`,
			statusCode:     200,
			namespacedName: types.NamespacedName{Namespace: "test-ns", Name: "missing-pvc"},
			wantErr:        true,
		},
		{
			name:           "kubelet error",
			responseBody:   "",
			statusCode:     500,
			namespacedName: types.NamespacedName{Namespace: "test-ns", Name: "test-pvc"},
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			collector := NewMetricsCollector(server.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			metrics, err := collector.GetVolumeMetrics(ctx, tt.namespacedName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetVolumeMetrics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && metrics != nil {
				if metrics.CapacityBytes != tt.expected.CapacityBytes {
					t.Errorf("CapacityBytes = %v, want %v", metrics.CapacityBytes, tt.expected.CapacityBytes)
				}
				if metrics.AvailableBytes != tt.expected.AvailableBytes {
					t.Errorf("AvailableBytes = %v, want %v", metrics.AvailableBytes, tt.expected.AvailableBytes)
				}
				if metrics.UsedBytes != tt.expected.UsedBytes {
					t.Errorf("UsedBytes = %v, want %v", metrics.UsedBytes, tt.expected.UsedBytes)
				}
				if metrics.UsagePercent != tt.expected.UsagePercent {
					t.Errorf("UsagePercent = %v, want %v", metrics.UsagePercent, tt.expected.UsagePercent)
				}
			}
		})
	}
}

func TestParseVolumeMetrics(t *testing.T) {
	collector := NewMetricsCollector("http://localhost")
	
	tests := []struct {
		name           string
		metricsText    string
		namespacedName types.NamespacedName
		wantErr        bool
		expected       *VolumeMetrics
	}{
		{
			name: "valid metrics",
			metricsText: `kubelet_volume_stats_capacity_bytes{namespace="default",persistentvolumeclaim="data-pvc"} 5368709120
kubelet_volume_stats_available_bytes{namespace="default",persistentvolumeclaim="data-pvc"} 1073741824`,
			namespacedName: types.NamespacedName{Namespace: "default", Name: "data-pvc"},
			wantErr:        false,
			expected: &VolumeMetrics{
				CapacityBytes:  5368709120, // 5Gi
				AvailableBytes: 1073741824, // 1Gi
				UsedBytes:      4294967296, // 4Gi
				UsagePercent:   80.0,
			},
		},
		{
			name: "metrics not found",
			metricsText: `kubelet_volume_stats_capacity_bytes{namespace="other",persistentvolumeclaim="other-pvc"} 1000000000`,
			namespacedName: types.NamespacedName{Namespace: "default", Name: "data-pvc"},
			wantErr:        true,
		},
		{
			name: "zero capacity",
			metricsText: `kubelet_volume_stats_capacity_bytes{namespace="default",persistentvolumeclaim="data-pvc"} 0
kubelet_volume_stats_available_bytes{namespace="default",persistentvolumeclaim="data-pvc"} 0`,
			namespacedName: types.NamespacedName{Namespace: "default", Name: "data-pvc"},
			wantErr:        true,
		},
		{
			name: "with comments and empty lines",
			metricsText: `# HELP kubelet_volume_stats_capacity_bytes Capacity in bytes
# TYPE kubelet_volume_stats_capacity_bytes gauge

kubelet_volume_stats_capacity_bytes{namespace="default",persistentvolumeclaim="data-pvc"} 2147483648

# HELP kubelet_volume_stats_available_bytes Available bytes
kubelet_volume_stats_available_bytes{namespace="default",persistentvolumeclaim="data-pvc"} 536870912`,
			namespacedName: types.NamespacedName{Namespace: "default", Name: "data-pvc"},
			wantErr:        false,
			expected: &VolumeMetrics{
				CapacityBytes:  2147483648, // 2Gi
				AvailableBytes: 536870912,  // 512Mi
				UsedBytes:      1610612736, // 1.5Gi
				UsagePercent:   75.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics, err := collector.parseVolumeMetrics(tt.metricsText, tt.namespacedName)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseVolumeMetrics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && metrics != nil {
				if metrics.CapacityBytes != tt.expected.CapacityBytes {
					t.Errorf("CapacityBytes = %v, want %v", metrics.CapacityBytes, tt.expected.CapacityBytes)
				}
				if metrics.AvailableBytes != tt.expected.AvailableBytes {
					t.Errorf("AvailableBytes = %v, want %v", metrics.AvailableBytes, tt.expected.AvailableBytes)
				}
				if metrics.UsedBytes != tt.expected.UsedBytes {
					t.Errorf("UsedBytes = %v, want %v", metrics.UsedBytes, tt.expected.UsedBytes)
				}
				if metrics.UsagePercent != tt.expected.UsagePercent {
					t.Errorf("UsagePercent = %.1f, want %.1f", metrics.UsagePercent, tt.expected.UsagePercent)
				}
			}
		})
	}
}

func TestNewMetricsCollector(t *testing.T) {
	kubeletURL := "http://test-kubelet:10255"
	collector := NewMetricsCollector(kubeletURL)

	if collector.kubeletURL != kubeletURL {
		t.Errorf("kubeletURL = %v, want %v", collector.kubeletURL, kubeletURL)
	}

	if collector.httpClient.Timeout != 10*time.Second {
		t.Errorf("httpClient.Timeout = %v, want %v", collector.httpClient.Timeout, 10*time.Second)
	}
}