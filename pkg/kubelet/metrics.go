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

func (mc *MetricsCollector) parseVolumeMetrics(metricsText string, namespacedName types.NamespacedName) (*VolumeMetrics, error) {
	capacityPattern := regexp.MustCompile(fmt.Sprintf(
		`kubelet_volume_stats_capacity_bytes\{.*namespace="%s".*persistentvolumeclaim="%s".*\}\s+(\d+)`,
		namespacedName.Namespace, namespacedName.Name))
	availablePattern := regexp.MustCompile(fmt.Sprintf(
		`kubelet_volume_stats_available_bytes\{.*namespace="%s".*persistentvolumeclaim="%s".*\}\s+(\d+)`,
		namespacedName.Namespace, namespacedName.Name))
	inodesPattern := regexp.MustCompile(fmt.Sprintf(
		`kubelet_volume_stats_inodes\{.*namespace="%s".*persistentvolumeclaim="%s".*\}\s+(\d+)`,
		namespacedName.Namespace, namespacedName.Name))
	inodesUsedPattern := regexp.MustCompile(fmt.Sprintf(
		`kubelet_volume_stats_inodes_used\{.*namespace="%s".*persistentvolumeclaim="%s".*\}\s+(\d+)`,
		namespacedName.Namespace, namespacedName.Name))

	capacityMatch := capacityPattern.FindStringSubmatch(metricsText)
	availableMatch := availablePattern.FindStringSubmatch(metricsText)
	inodesMatch := inodesPattern.FindStringSubmatch(metricsText)
	inodesUsedMatch := inodesUsedPattern.FindStringSubmatch(metricsText)

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

	var inodesTotal, inodesUsed, inodesFree int64
	var inodesUsagePercent float64

	if len(inodesMatch) >= 2 && len(inodesUsedMatch) >= 2 {
		if inodesTotal, err = strconv.ParseInt(inodesMatch[1], 10, 64); err == nil {
			if inodesUsed, err = strconv.ParseInt(inodesUsedMatch[1], 10, 64); err == nil {
				inodesFree = inodesTotal - inodesUsed
				if inodesTotal > 0 {
					inodesUsagePercent = float64(inodesUsed) / float64(inodesTotal) * 100
				}
			}
		}
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
