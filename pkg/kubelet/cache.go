package kubelet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/logicIQ/pvc-chonker/pkg/metrics"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MetricsCache struct {
	data  map[string]*VolumeMetrics
	mutex sync.RWMutex
}

type MetricsCollector struct {
	client     client.Client
	clientset  *kubernetes.Clientset
	kubeletURL string
}

func NewMetricsCache() *MetricsCache {
	return &MetricsCache{
		data: make(map[string]*VolumeMetrics),
	}
}

func NewMetricsCollector(kubeletURL string) *MetricsCollector {
	return &MetricsCollector{
		kubeletURL: kubeletURL,
	}
}

func (mc *MetricsCollector) SetClient(client client.Client, clientset *kubernetes.Clientset) {
	mc.client = client
	mc.clientset = clientset
}

func (mc *MetricsCollector) GetAllVolumeMetrics(ctx context.Context) (*MetricsCache, error) {
	startTime := time.Now()
	defer func() {
		metrics.KubeletClientResponseTime.Observe(time.Since(startTime).Seconds())
	}()

	cache := NewMetricsCache()
	if err := mc.fetchAllNodeMetrics(ctx, cache); err != nil {
		return nil, err
	}

	cache.calculateUsagePercentages()
	return cache, nil
}

func (mc *MetricsCollector) fetchAllNodeMetrics(ctx context.Context, cache *MetricsCache) error {
	// Get list of nodes
	var nodes corev1.NodeList
	if err := mc.client.List(ctx, &nodes); err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return fmt.Errorf("no nodes found")
	}

	// Use errgroup to fetch metrics from all nodes in parallel
	eg, ectx := errgroup.WithContext(ctx)
	for _, node := range nodes.Items {
		nodeName := node.Name
		eg.Go(func() error {
			return mc.fetchNodeMetrics(ectx, nodeName, cache)
		})
	}

	return eg.Wait()
}

func (mc *MetricsCollector) fetchNodeMetrics(ctx context.Context, nodeName string, cache *MetricsCache) error {
	var respBody []byte
	var err error

	// Use custom kubelet URL if provided (for e2e testing), otherwise use K8s API proxy
	if mc.kubeletURL != "" {
		respBody, err = mc.fetchFromCustomURL(ctx)
	} else {
		// Use Kubernetes API proxy to access kubelet metrics
		req := mc.clientset.
			CoreV1().
			RESTClient().
			Get().
			Resource("nodes").
			Name(nodeName).
			SubResource("proxy").
			Suffix("metrics")

		respBody, err = req.DoRaw(ctx)
	}

	if err != nil {
		return fmt.Errorf("failed to get metrics from node %s: %w", nodeName, err)
	}

	// Parse Prometheus metrics exactly like ava-labs
	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(bytes.NewReader(respBody))
	if err != nil {
		return fmt.Errorf("failed to parse metrics from node %s: %w", nodeName, err)
	}

	// Process metrics using ava-labs constants
	if gauge, ok := metricFamilies["kubelet_volume_stats_capacity_bytes"]; ok {
		for _, m := range gauge.Metric {
			pvcName, value := mc.parseMetric(m)
			if pvcName.Name != "" && pvcName.Namespace != "" {
				cache.setCapacity(pvcName, value)
			}
		}
	}

	if gauge, ok := metricFamilies["kubelet_volume_stats_available_bytes"]; ok {
		for _, m := range gauge.Metric {
			pvcName, value := mc.parseMetric(m)
			if pvcName.Name != "" && pvcName.Namespace != "" {
				cache.setAvailable(pvcName, value)
			}
		}
	}

	if gauge, ok := metricFamilies["kubelet_volume_stats_inodes"]; ok {
		for _, m := range gauge.Metric {
			pvcName, value := mc.parseMetric(m)
			if pvcName.Name != "" && pvcName.Namespace != "" {
				cache.setInodesTotal(pvcName, value)
			}
		}
	}

	if gauge, ok := metricFamilies["kubelet_volume_stats_inodes_used"]; ok {
		for _, m := range gauge.Metric {
			pvcName, value := mc.parseMetric(m)
			if pvcName.Name != "" && pvcName.Namespace != "" {
				cache.setInodesUsed(pvcName, value)
			}
		}
	}

	return nil
}

func (mc *MetricsCollector) fetchFromCustomURL(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", mc.kubeletURL+"/metrics", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metrics: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log the close error but don't override the main error
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (mc *MetricsCollector) parseMetric(m *dto.Metric) (pvcName types.NamespacedName, value int64) {
	for _, label := range m.GetLabel() {
		if label.GetName() == "namespace" {
			pvcName.Namespace = label.GetValue()
		} else if label.GetName() == "persistentvolumeclaim" {
			pvcName.Name = label.GetValue()
		}
	}
	value = int64(m.GetGauge().GetValue())
	return pvcName, value
}

func (cache *MetricsCache) Get(namespacedName types.NamespacedName) (*VolumeMetrics, bool) {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	key := namespacedName.Namespace + "/" + namespacedName.Name
	metrics, exists := cache.data[key]
	return metrics, exists
}

func (cache *MetricsCache) GetAll() map[string]*VolumeMetrics {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	return cache.data
}

func (cache *MetricsCache) setCapacity(pvcName types.NamespacedName, value int64) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	key := pvcName.Namespace + "/" + pvcName.Name
	if cache.data[key] == nil {
		cache.data[key] = &VolumeMetrics{}
	}
	cache.data[key].CapacityBytes = value
}

func (cache *MetricsCache) setAvailable(pvcName types.NamespacedName, value int64) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	key := pvcName.Namespace + "/" + pvcName.Name
	if cache.data[key] == nil {
		cache.data[key] = &VolumeMetrics{}
	}
	cache.data[key].AvailableBytes = value
}

func (cache *MetricsCache) setInodesTotal(pvcName types.NamespacedName, value int64) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	key := pvcName.Namespace + "/" + pvcName.Name
	if cache.data[key] == nil {
		cache.data[key] = &VolumeMetrics{}
	}
	cache.data[key].InodesTotal = value
}

func (cache *MetricsCache) setInodesUsed(pvcName types.NamespacedName, value int64) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	key := pvcName.Namespace + "/" + pvcName.Name
	if cache.data[key] == nil {
		cache.data[key] = &VolumeMetrics{}
	}
	cache.data[key].InodesUsed = value
}

func (cache *MetricsCache) calculateUsagePercentages() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	for key, vm := range cache.data {
		if vm.CapacityBytes > 0 {
			vm.UsedBytes = vm.CapacityBytes - vm.AvailableBytes
			vm.UsagePercent = float64(vm.UsedBytes) / float64(vm.CapacityBytes) * 100
		}
		if vm.InodesTotal > 0 {
			vm.InodesFree = vm.InodesTotal - vm.InodesUsed
			vm.InodesUsagePercent = float64(vm.InodesUsed) / float64(vm.InodesTotal) * 100
		}
		cache.data[key] = vm
	}
}
