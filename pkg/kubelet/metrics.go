package kubelet

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
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
	capacityPattern := regexp.MustCompile(fmt.Sprintf(
		`kubelet_volume_stats_capacity_bytes\{.*namespace="%s".*persistentvolumeclaim="%s".*\}\s+(\d+)`,
		namespacedName.Namespace, namespacedName.Name))
	availablePattern := regexp.MustCompile(fmt.Sprintf(
		`kubelet_volume_stats_available_bytes\{.*namespace="%s".*persistentvolumeclaim="%s".*\}\s+(\d+)`,
		namespacedName.Namespace, namespacedName.Name))

	capacityMatch := capacityPattern.FindStringSubmatch(metricsText)
	availableMatch := availablePattern.FindStringSubmatch(metricsText)

	if len(capacityMatch) < 2 || len(availableMatch) < 2 {
		return nil, fmt.Errorf("volume metrics not found for %s/%s", namespacedName.Namespace, namespacedName.Name)
	}

	capacity, err := strconv.ParseInt(capacityMatch[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid capacity value: %w", err)
	}

	available, err := strconv.ParseInt(availableMatch[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid available value: %w", err)
	}

	if capacity == 0 {
		return nil, fmt.Errorf("zero capacity for %s/%s", namespacedName.Namespace, namespacedName.Name)
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