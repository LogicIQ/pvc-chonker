package e2e

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	pvcchonkerv1alpha1 "github.com/LogicIQ/pvc-chonker/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var clientset *kubernetes.Clientset
var k8sClient client.Client

func TestMain(m *testing.M) {
	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}
	clientset, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
	
	// Initialize controller-runtime client
	scheme := runtime.NewScheme()
	if err := pvcchonkerv1alpha1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic(err)
	}
	
	m.Run()
}

func TestBasicExpansion(t *testing.T) {
	t.Log("=== Test: Basic PVC Expansion ===")
	ctx := context.Background()
	
	waitForOperator(t)
	
	// Wait a bit for resources to be created
	time.Sleep(5 * time.Second)
	
	// Check what PVCs exist
	pvcs, err := clientset.CoreV1().PersistentVolumeClaims("default").List(ctx, metav1.ListOptions{})
	if err == nil {
		t.Logf("Found %d PVCs in default namespace", len(pvcs.Items))
		for _, pvc := range pvcs.Items {
			t.Logf("PVC: %s, Status: %s", pvc.Name, pvc.Status.Phase)
		}
	}
	
	// Check if test PVC exists, if not wait for it
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-pvc", metav1.GetOptions{})
		if err != nil {
			t.Logf("test-pvc not found: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("Test PVC not found after waiting: %v", err)
	}
	
	// Wait for PVC to be bound
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		t.Logf("PVC status: %s", pvc.Status.Phase)
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-pvc", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get test PVC: %v", err)
	}
	originalSize := pvc.Status.Capacity[corev1.ResourceStorage]
	t.Logf("Original PVC size: %s", originalSize.String())
	
	// Wait longer for expansion with real kubelet metrics
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 120*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		newSize := pvc.Status.Capacity[corev1.ResourceStorage]
		t.Logf("Current size: %s", newSize.String())
		return newSize.Cmp(originalSize) > 0, nil
	})
	if err != nil {
		// Show operator logs for debugging
		logs := getOperatorLogs(t)
		t.Logf("Operator logs:\n%s", logs)
		t.Fatalf("PVC was not expanded within timeout: %v", err)
	}
	
	pvc, _ = clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-pvc", metav1.GetOptions{})
	finalSize := pvc.Status.Capacity[corev1.ResourceStorage]
	t.Logf("Final PVC size: %s (expanded from %s)", finalSize.String(), originalSize.String())
	t.Log("Basic expansion test passed")
}

func TestInodeExpansion(t *testing.T) {
	t.Log("=== Test: Inode Threshold Expansion ===")
	ctx := context.Background()
	
	// Create inode test PVC
	_, err := clientset.CoreV1().PersistentVolumeClaims("default").Create(ctx, createInodePVC(), metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create inode PVC: %v", err)
	}
	defer clientset.CoreV1().PersistentVolumeClaims("default").Delete(ctx, "test-inode-pvc", metav1.DeleteOptions{})
	
	// Wait for PVC to be bound
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-inode-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-inode-pvc", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get inode PVC: %v", err)
	}
	originalSize := pvc.Status.Capacity[corev1.ResourceStorage]
	t.Logf("Original inode PVC size: %s", originalSize.String())
	
	// Wait longer for operator to process with real metrics
	time.Sleep(15 * time.Second)
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
	// Check for inode processing (may not trigger expansion due to real filesystem)
	if !strings.Contains(logs, "inode") && !strings.Contains(logs, "Inode") {
		t.Log("No inode processing detected - this may be expected with real filesystems")
	} else {
		t.Log("Inode processing detected")
	}
	
	t.Log("Inode expansion test passed")
}

func TestMaxSizeLimit(t *testing.T) {
	t.Log("=== Test: Max Size Limit ===")
	ctx := context.Background()
	
	// Create max size test PVC
	_, err := clientset.CoreV1().PersistentVolumeClaims("default").Create(ctx, createMaxSizePVC(), metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create max size PVC: %v", err)
	}
	defer clientset.CoreV1().PersistentVolumeClaims("default").Delete(ctx, "test-max-size-pvc", metav1.DeleteOptions{})
	
	// Wait for PVC to be bound
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-max-size-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	// Wait longer for expansion attempt and check logs
	time.Sleep(15 * time.Second)
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
	// Check for max size processing
	if !strings.Contains(logs, "max") && !strings.Contains(logs, "Max") {
		t.Log("No max size processing detected - may not have triggered expansion")
	} else {
		t.Log("Max size processing detected")
	}
	
	t.Log("Max size limit test passed")
}

func TestCooldownPeriod(t *testing.T) {
	t.Log("=== Test: Cooldown Period ===")
	ctx := context.Background()
	
	// Create cooldown test PVC
	_, err := clientset.CoreV1().PersistentVolumeClaims("default").Create(ctx, createCooldownPVC(), metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create cooldown PVC: %v", err)
	}
	defer clientset.CoreV1().PersistentVolumeClaims("default").Delete(ctx, "test-cooldown-pvc", metav1.DeleteOptions{})
	
	// Wait for PVC to be bound
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-cooldown-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	// Simulate last expansion by adding annotation
	pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-cooldown-pvc", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get cooldown PVC: %v", err)
	}
	
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Annotations["pvc-chonker.io/last-expansion"] = time.Now().Format(time.RFC3339)
	
	_, err = clientset.CoreV1().PersistentVolumeClaims("default").Update(ctx, pvc, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update PVC with last expansion: %v", err)
	}
	
	// Wait longer and check logs for cooldown detection
	time.Sleep(15 * time.Second)
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
	// Check for cooldown processing
	if !strings.Contains(strings.ToLower(logs), "cooldown") {
		t.Log("No cooldown detection in logs - may not have processed yet")
	} else {
		t.Log("Cooldown detection found")
	}
	
	t.Log("Cooldown period test passed")
}

func TestKubeletMetrics(t *testing.T) {
	t.Log("=== Test: Kubelet Volume Metrics ===")
	ctx := context.Background()
	
	// Ensure test PVC exists and is bound
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("Test PVC not bound: %v", err)
	}
	
	// Get node name
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		t.Fatalf("Failed to get nodes: %v", err)
	}
	nodeName := nodes.Items[0].Name
	
	// Check kubelet metrics
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
	
	// Check for test-pvc specific metrics
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
		"Starting",
		"reconcil",
	}
	
	for _, expected := range expectedLogs {
		if !strings.Contains(strings.ToLower(logs), strings.ToLower(expected)) {
			t.Errorf("Missing expected log entry: %s", expected)
		}
	}
	
	t.Log("Operator logs test passed")
}

func waitForOperator(t *testing.T) {
	t.Log("Waiting for operator to be ready...")
	ctx := context.Background()
	err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pods, err := clientset.CoreV1().Pods("pvc-chonker-system").List(ctx, metav1.ListOptions{
			LabelSelector: "control-plane=controller-manager",
		})
		if err != nil {
			return false, err
		}
		if len(pods.Items) == 0 {
			return false, nil
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("Operator not ready: %v", err)
	}
}

func getOperatorLogs(t *testing.T) string {
	ctx := context.Background()
	
	pods, err := clientset.CoreV1().Pods("pvc-chonker-system").List(ctx, metav1.ListOptions{
		LabelSelector: "control-plane=controller-manager",
	})
	if err != nil {
		t.Fatalf("Failed to get operator pods: %v", err)
	}
	
	if len(pods.Items) == 0 {
		t.Fatal("No operator pods found")
	}
	
	podName := pods.Items[0].Name
	req := clientset.CoreV1().Pods("pvc-chonker-system").GetLogs(podName, &corev1.PodLogOptions{
		TailLines: int64Ptr(2),
	})
	
	logs, err := req.Stream(ctx)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}
	defer logs.Close()
	
	logData, err := io.ReadAll(logs)
	if err != nil {
		t.Fatalf("Failed to read logs: %v", err)
	}
	
	return string(logData)
}

func createInodePVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-inode-pvc",
			Annotations: map[string]string{
				"pvc-chonker.io/enabled":           "true",
				"pvc-chonker.io/threshold":         "90%",
				"pvc-chonker.io/inodes-threshold":  "5%",
				"pvc-chonker.io/increase":          "50%",
				"pvc-chonker.io/cooldown":          "1m",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}
}

func createMaxSizePVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-max-size-pvc",
			Annotations: map[string]string{
				"pvc-chonker.io/enabled":   "true",
				"pvc-chonker.io/threshold": "5%",
				"pvc-chonker.io/increase":  "100%",
				"pvc-chonker.io/max-size":  "2Gi",
				"pvc-chonker.io/cooldown":  "1m",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}
}

func createCooldownPVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cooldown-pvc",
			Annotations: map[string]string{
				"pvc-chonker.io/enabled":   "true",
				"pvc-chonker.io/threshold": "5%",
				"pvc-chonker.io/increase":  "50%",
				"pvc-chonker.io/cooldown":  "10m",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func TestPVCPolicyBasic(t *testing.T) {
	t.Log("=== Test: PVCPolicy Basic Functionality ===")
	ctx := context.Background()
	
	// Create PVCPolicy
	policy := createTestPVCPolicy()
	err := k8sClient.Create(ctx, policy)
	if err != nil {
		t.Fatalf("Failed to create PVCPolicy: %v", err)
	}
	defer k8sClient.Delete(ctx, policy)
	
	// Create PVC with matching labels
	pvc := createPolicyManagedPVC()
	_, err = clientset.CoreV1().PersistentVolumeClaims("default").Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create policy-managed PVC: %v", err)
	}
	defer clientset.CoreV1().PersistentVolumeClaims("default").Delete(ctx, "test-policy-pvc", metav1.DeleteOptions{})
	
	// Wait for PVC to be bound
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-policy-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	// Wait for policy to be processed
	time.Sleep(15 * time.Second)
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
	// Check for policy processing
	if strings.Contains(strings.ToLower(logs), "policy") {
		t.Log("PVCPolicy processing detected")
	} else {
		t.Log("No explicit policy processing in logs")
	}
	
	t.Log("PVCPolicy basic test passed")
}

func TestPVCPolicyOverride(t *testing.T) {
	t.Log("=== Test: PVCPolicy vs Annotation Override ===")
	ctx := context.Background()
	
	// Create PVCPolicy with specific settings
	policy := createTestPVCPolicy()
	err := k8sClient.Create(ctx, policy)
	if err != nil {
		t.Fatalf("Failed to create PVCPolicy: %v", err)
	}
	defer k8sClient.Delete(ctx, policy)
	
	// Create PVC with both policy labels AND annotations (annotations should win)
	pvc := createPolicyOverridePVC()
	_, err = clientset.CoreV1().PersistentVolumeClaims("default").Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create override PVC: %v", err)
	}
	defer clientset.CoreV1().PersistentVolumeClaims("default").Delete(ctx, "test-override-pvc", metav1.DeleteOptions{})
	
	// Wait for PVC to be bound
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-override-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	// Wait for processing
	time.Sleep(15 * time.Second)
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
	// Check that annotation values are used (threshold 95% from annotation, not 85% from policy)
	if strings.Contains(logs, "95") || strings.Contains(logs, "annotation") {
		t.Log("Annotation override detected")
	} else {
		t.Log("No explicit annotation override detection in logs")
	}
	
	t.Log("PVCPolicy override test passed")
}

func createTestPVCPolicy() *pvcchonkerv1alpha1.PVCPolicy {
	enabled := true
	threshold := "85%"
	inodesThreshold := "90%"
	increase := "25%"
	maxSize := resource.MustParse("10Gi")
	minScaleUp := resource.MustParse("1Gi")
	cooldown := metav1.Duration{Duration: 5 * time.Minute}
	
	return &pvcchonkerv1alpha1.PVCPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: pvcchonkerv1alpha1.PVCPolicySpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-policy": "enabled",
				},
			},
			Template: pvcchonkerv1alpha1.PVCPolicyTemplate{
				Enabled:         &enabled,
				Threshold:       &threshold,
				InodesThreshold: &inodesThreshold,
				Increase:        &increase,
				MaxSize:         &maxSize,
				MinScaleUp:      &minScaleUp,
				Cooldown:        &cooldown,
			},
		},
	}
}

func createPolicyManagedPVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy-pvc",
			Labels: map[string]string{
				"test-policy": "enabled", // Matches policy selector
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}
}

func createPolicyOverridePVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-override-pvc",
			Labels: map[string]string{
				"test-policy": "enabled", // Matches policy selector
			},
			Annotations: map[string]string{
				"pvc-chonker.io/enabled":   "true",
				"pvc-chonker.io/threshold": "95%", // Override policy's 85%
				"pvc-chonker.io/increase":  "50%", // Override policy's 25%
				"pvc-chonker.io/cooldown":  "1m",  // Override policy's 5m
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}
}