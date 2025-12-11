package kubelet

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
)

type VolumeMetrics struct {
	CapacityBytes   int64
	AvailableBytes  int64
	UsedBytes       int64
	UsagePercent    float64
}

type MetricsCollector struct {
	httpClient *http.Client
	kubeletURL string
}

func NewMetricsCollector(kubeletURL string) *MetricsCollector {
	return &MetricsCollector{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		kubeletURL: kubeletURL,
	}
}

func (mc *MetricsCollector) GetVolumeMetrics(ctx context.Context, namespacedName types.NamespacedName) (*VolumeMetrics, error) {
	metricsURL := fmt.Sprintf("%s/metrics", mc.kubeletURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", metricsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kubelet metrics returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return mc.parseVolumeMetrics(string(body), namespacedName)
}

func (mc *MetricsCollector) parseVolumeMetrics(metricsText string, namespacedName types.NamespacedName) (*VolumeMetrics, error) {
	var capacity, available int64
	var found bool

	lines := strings.Split(metricsText, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}

		if strings.Contains(line, "kubelet_volume_stats_capacity_bytes") &&
			strings.Contains(line, fmt.Sprintf(`namespace="%s"`, namespacedName.Namespace)) &&
			strings.Contains(line, fmt.Sprintf(`persistentvolumeclaim="%s"`, namespacedName.Name)) {
			
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if val, err := strconv.ParseInt(parts[len(parts)-1], 10, 64); err == nil {
					capacity = val
					found = true
				}
			}
		}

		if strings.Contains(line, "kubelet_volume_stats_available_bytes") &&
			strings.Contains(line, fmt.Sprintf(`namespace="%s"`, namespacedName.Namespace)) &&
			strings.Contains(line, fmt.Sprintf(`persistentvolumeclaim="%s"`, namespacedName.Name)) {
			
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if val, err := strconv.ParseInt(parts[len(parts)-1], 10, 64); err == nil {
					available = val
				}
			}
		}
	}

	if !found || capacity == 0 {
		return nil, fmt.Errorf("volume metrics not found for %s/%s", namespacedName.Namespace, namespacedName.Name)
	}

	used := capacity - available
	usagePercent := float64(used) / float64(capacity) * 100

	return &VolumeMetrics{
		CapacityBytes:  capacity,
		AvailableBytes: available,
		UsedBytes:      used,
		UsagePercent:   usagePercent,
	}, nil
}