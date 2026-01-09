package kubelet

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/logicIQ/pvc-chonker/pkg/metrics"
	"k8s.io/apimachinery/pkg/types"
)

type VolumeMetrics struct {
	CapacityBytes      int64
	AvailableBytes     int64
	UsedBytes          int64
	UsagePercent       float64
	InodesTotal        int64
	InodesUsed         int64
	InodesFree         int64
	InodesUsagePercent float64
}

func (mc *MetricsCollector) GetVolumeMetrics(ctx context.Context, namespacedName types.NamespacedName) (*VolumeMetrics, error) {
	startTime := time.Now()
	defer func() {
		metrics.KubeletClientResponseTime.Observe(time.Since(startTime).Seconds())
	}()

	cache, err := mc.GetAllVolumeMetrics(ctx)
	if err != nil {
		return nil, err
	}

	vm, exists := cache.Get(namespacedName)
	if !exists {
		return nil, fmt.Errorf("volume metrics not found for %s/%s", namespacedName.Namespace, namespacedName.Name)
	}

	return vm, nil
}

var (
	capacityRegex    = regexp.MustCompile(`kubelet_volume_stats_capacity_bytes\{.*namespace="([^"]+)".*persistentvolumeclaim="([^"]+)".*\}\s+(\d+)`)
	availableRegex   = regexp.MustCompile(`kubelet_volume_stats_available_bytes\{.*namespace="([^"]+)".*persistentvolumeclaim="([^"]+)".*\}\s+(\d+)`)
	inodesRegex      = regexp.MustCompile(`kubelet_volume_stats_inodes\{.*namespace="([^"]+)".*persistentvolumeclaim="([^"]+)".*\}\s+(\d+)`)
	inodesUsedRegex  = regexp.MustCompile(`kubelet_volume_stats_inodes_used\{.*namespace="([^"]+)".*persistentvolumeclaim="([^"]+)".*\}\s+(\d+)`)
)

func (mc *MetricsCollector) parseVolumeMetrics(metricsText string, namespacedName types.NamespacedName) (*VolumeMetrics, error) {
	var capacity, available, inodesTotal, inodesUsed int64
	var err error

	// Find capacity
	for _, match := range capacityRegex.FindAllStringSubmatch(metricsText, -1) {
		if len(match) >= 4 && match[1] == namespacedName.Namespace && match[2] == namespacedName.Name {
			if capacity, err = strconv.ParseInt(match[3], 10, 64); err != nil {
				return nil, fmt.Errorf("invalid capacity value: %w", err)
			}
			break
		}
	}

	// Find available
	for _, match := range availableRegex.FindAllStringSubmatch(metricsText, -1) {
		if len(match) >= 4 && match[1] == namespacedName.Namespace && match[2] == namespacedName.Name {
			if available, err = strconv.ParseInt(match[3], 10, 64); err != nil {
				return nil, fmt.Errorf("invalid available value: %w", err)
			}
			break
		}
	}

	if capacity == 0 || available == 0 {
		return nil, fmt.Errorf("volume metrics not found for %s/%s", namespacedName.Namespace, namespacedName.Name)
	}

	// Find inodes
	for _, match := range inodesRegex.FindAllStringSubmatch(metricsText, -1) {
		if len(match) >= 4 && match[1] == namespacedName.Namespace && match[2] == namespacedName.Name {
			if inodesTotal, err = strconv.ParseInt(match[3], 10, 64); err == nil {
				break
			}
		}
	}

	// Find inodes used
	for _, match := range inodesUsedRegex.FindAllStringSubmatch(metricsText, -1) {
		if len(match) >= 4 && match[1] == namespacedName.Namespace && match[2] == namespacedName.Name {
			if inodesUsed, err = strconv.ParseInt(match[3], 10, 64); err == nil {
				break
			}
		}
	}

	used := capacity - available
	usagePercent := float64(used) / float64(capacity) * 100

	var inodesFree int64
	var inodesUsagePercent float64
	if inodesTotal > 0 {
		inodesFree = inodesTotal - inodesUsed
		inodesUsagePercent = float64(inodesUsed) / float64(inodesTotal) * 100
	}

	return &VolumeMetrics{
		CapacityBytes:      capacity,
		AvailableBytes:     available,
		UsedBytes:          used,
		UsagePercent:       usagePercent,
		InodesTotal:        inodesTotal,
		InodesUsed:         inodesUsed,
		InodesFree:         inodesFree,
		InodesUsagePercent: inodesUsagePercent,
	}, nil
}
