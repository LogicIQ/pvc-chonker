package kubelet

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

type MetricsCache struct {
	data  map[string]*VolumeMetrics
	mutex sync.RWMutex
}

func NewMetricsCache() *MetricsCache {
	return &MetricsCache{
		data: make(map[string]*VolumeMetrics),
	}
}

func (mc *MetricsCollector) GetAllVolumeMetrics(ctx context.Context) (*MetricsCache, error) {
	metricsText, err := mc.fetchMetrics(ctx)
	if err != nil {
		return nil, err
	}

	cache := NewMetricsCache()
	cache.parseAllMetrics(metricsText)
	return cache, nil
}

func (mc *MetricsCollector) fetchMetrics(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", mc.kubeletURL+"/metrics", nil)
	if err != nil {
		return "", err
	}

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (cache *MetricsCache) parseAllMetrics(metricsText string) {
	capacityPattern := regexp.MustCompile(`kubelet_volume_stats_capacity_bytes\{.*namespace="([^"]+)".*persistentvolumeclaim="([^"]+)".*\}\s+(\d+)`)
	availablePattern := regexp.MustCompile(`kubelet_volume_stats_available_bytes\{.*namespace="([^"]+)".*persistentvolumeclaim="([^"]+)".*\}\s+(\d+)`)

	capacityMatches := capacityPattern.FindAllStringSubmatch(metricsText, -1)
	availableMatches := availablePattern.FindAllStringSubmatch(metricsText, -1)

	capacityMap := make(map[string]int64)
	for _, match := range capacityMatches {
		if len(match) >= 4 {
			key := match[1] + "/" + match[2]
			if val, err := strconv.ParseInt(match[3], 10, 64); err == nil {
				capacityMap[key] = val
			}
		}
	}

	availableMap := make(map[string]int64)
	for _, match := range availableMatches {
		if len(match) >= 4 {
			key := match[1] + "/" + match[2]
			if val, err := strconv.ParseInt(match[3], 10, 64); err == nil {
				availableMap[key] = val
			}
		}
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	
	for key, capacity := range capacityMap {
		if available, exists := availableMap[key]; exists && capacity > 0 {
			used := capacity - available
			usagePercent := float64(used) / float64(capacity) * 100
			
			cache.data[key] = &VolumeMetrics{
				CapacityBytes:  capacity,
				AvailableBytes: available,
				UsedBytes:      used,
				UsagePercent:   usagePercent,
			}
		}
	}
}

func (cache *MetricsCache) Get(namespacedName types.NamespacedName) (*VolumeMetrics, bool) {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	key := namespacedName.Namespace + "/" + namespacedName.Name
	metrics, exists := cache.data[key]
	return metrics, exists
}