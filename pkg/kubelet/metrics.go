package kubelet

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
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

var metricLineRegex = regexp.MustCompile(`^(kubelet_volume_stats_(?:capacity_bytes|available_bytes|inodes|inodes_used))\{.*namespace="([^"]+)".*persistentvolumeclaim="([^"]+)".*\}\s+(\d+)`)

func (mc *MetricsCollector) parseVolumeMetrics(metricsText string, namespacedName types.NamespacedName) (*VolumeMetrics, error) {
	var capacity, available, inodesTotal, inodesUsed int64

	scanner := bufio.NewScanner(strings.NewReader(metricsText))
	for scanner.Scan() {
		line := scanner.Text()
		match := metricLineRegex.FindStringSubmatch(line)
		if len(match) != 5 || match[2] != namespacedName.Namespace || match[3] != namespacedName.Name {
			continue
		}

		value, err := strconv.ParseInt(match[4], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid metric value: %w", err)
		}

		switch match[1] {
		case "kubelet_volume_stats_capacity_bytes":
			capacity = value
		case "kubelet_volume_stats_available_bytes":
			available = value
		case "kubelet_volume_stats_inodes":
			inodesTotal = value
		case "kubelet_volume_stats_inodes_used":
			inodesUsed = value
		}
	}

	if capacity == 0 || available == 0 {
		return nil, fmt.Errorf("volume metrics not found for %s/%s", namespacedName.Namespace, namespacedName.Name)
	}

	var used int64
	var usagePercent float64
	if available > capacity {
		used = 0
		usagePercent = 0.0
	} else {
		used = capacity - available
		usagePercent = float64(used) / float64(capacity) * 100
	}

	var inodesFree int64
	var inodesUsagePercent float64
	if inodesTotal > 0 {
		if inodesUsed > inodesTotal {
			inodesFree = 0
			inodesUsagePercent = 100.0
		} else {
			inodesFree = inodesTotal - inodesUsed
			inodesUsagePercent = float64(inodesUsed) / float64(inodesTotal) * 100
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
