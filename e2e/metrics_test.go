package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestKubeletMetrics(t *testing.T) {
	t.Log("=== Test: Kubelet Volume Metrics ===")
	ctx := context.Background()
	
	// Wait for PVC to be bound
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("Test PVC not bound: %v", err)
	}
	
	// Wait for test pod to be running and using the volume
	waitForPod(t, "test-pod", testNamespace)
	
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		t.Fatalf("Failed to get nodes: %v", err)
	}
	nodeName := nodes.Items[0].Name
	t.Logf("Using node: %s", nodeName)
	
	// Wait for volume metrics to be populated
	waitForVolumeMetrics(t, nodeName)
	
	metricsPath := fmt.Sprintf("/api/v1/nodes/%s/proxy/metrics", nodeName)
	req := clientset.CoreV1().RESTClient().Get().AbsPath(metricsPath)
	result := req.Do(ctx)
	if result.Error() != nil {
		t.Fatalf("Failed to get kubelet metrics: %v", result.Error())
	}
	
	metricsData, err := result.Raw()
	if err != nil {
		t.Fatalf("Failed to read metrics: %v", err)
	}
	
	metricsText := string(metricsData)
	
	// Debug: log available metrics
	lines := strings.Split(metricsText, "\n")
	volumeMetricsCount := 0
	for _, line := range lines {
		if strings.Contains(line, "kubelet_volume") {
			volumeMetricsCount++
			if volumeMetricsCount <= 5 { // Log first 5 volume metrics
				t.Logf("Found volume metric: %s", line)
			}
		}
	}
	t.Logf("Total volume metrics found: %d", volumeMetricsCount)
	requiredMetrics := []string{
		"kubelet_volume_stats_capacity_bytes",
		"kubelet_volume_stats_available_bytes",
		"kubelet_volume_stats_inodes",
		"kubelet_volume_stats_inodes_used",
	}
	
	for _, metric := range requiredMetrics {
		if !strings.Contains(metricsText, metric) {
			t.Errorf("Missing required metric: %s", metric)
		} else {
			t.Logf("Found metric: %s", metric)
		}
	}
	
	if strings.Contains(metricsText, `persistentvolumeclaim="test-pvc"`) {
		t.Log("Found test-pvc metrics")
	} else {
		t.Error("Missing test-pvc specific metrics")
	}
	
	t.Log("Kubelet metrics test passed")
}

func TestOperatorLogs(t *testing.T) {
	t.Log("=== Test: Operator Logs ===")
	
	logs := getOperatorLogs(t)
	t.Logf("Recent operator logs:\n%s", logs)
	
	expectedLogs := []string{
		"reconcileAll",
		"reconcil",
	}
	
	for _, expected := range expectedLogs {
		if !strings.Contains(strings.ToLower(logs), strings.ToLower(expected)) {
			t.Errorf("Missing expected log entry: %s", expected)
		}
	}
	
	t.Log("Operator logs test passed")
}

// waitForVolumeMetrics waits for volume metrics to be available
func waitForVolumeMetrics(t *testing.T, nodeName string) {
	ctx := context.Background()
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		metricsPath := fmt.Sprintf("/api/v1/nodes/%s/proxy/metrics", nodeName)
		req := clientset.CoreV1().RESTClient().Get().AbsPath(metricsPath)
		result := req.Do(ctx)
		if result.Error() != nil {
			return false, nil
		}
		metricsData, err := result.Raw()
		if err != nil {
			return false, nil
		}
		return strings.Contains(string(metricsData), "kubelet_volume_stats_capacity_bytes"), nil
	})
	if err != nil {
		t.Logf("Warning: Volume metrics not available in time: %v", err)
	}
}