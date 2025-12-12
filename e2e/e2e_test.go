package e2e

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var clientset *kubernetes.Clientset

func TestMain(m *testing.M) {
	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}
	clientset, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
	m.Run()
}

func TestBasicExpansion(t *testing.T) {
	t.Log("=== Test: Basic PVC Expansion ===")
	ctx := context.Background()
	
	waitForOperator(t)
	
	pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-pvc", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get test PVC: %v", err)
	}
	originalSize := pvc.Status.Capacity[corev1.ResourceStorage]
	t.Logf("Original PVC size: %s", originalSize.String())
	
	err = wait.PollImmediate(2*time.Second, 20*time.Second, func() (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		newSize := pvc.Status.Capacity[corev1.ResourceStorage]
		t.Logf("Current size: %s", newSize.String())
		return newSize.Cmp(originalSize) > 0, nil
	})
	if err != nil {
		t.Fatalf("PVC was not expanded within timeout: %v", err)
	}
	
	pvc, _ = clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-pvc", metav1.GetOptions{})
	finalSize := pvc.Status.Capacity[corev1.ResourceStorage]
	t.Logf("Final PVC size: %s (expanded from %s)", finalSize.String(), originalSize.String())
	t.Log("✅ Basic expansion test passed")
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
	err = wait.PollImmediate(2*time.Second, 15*time.Second, func() (bool, error) {
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
	
	// Check operator logs for inode threshold detection
	time.Sleep(8 * time.Second) // Wait for operator to process
	
	logs := getOperatorLogs(t)
	if !strings.Contains(logs, "Inode threshold reached") {
		t.Logf("Operator logs:\n%s", logs)
		t.Error("Expected inode threshold detection in logs")
	}
	
	t.Log("✅ Inode expansion test passed")
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
	err = wait.PollImmediate(2*time.Second, 15*time.Second, func() (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(ctx, "test-max-size-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	// Wait for expansion attempt and check logs
	time.Sleep(8 * time.Second)
	
	logs := getOperatorLogs(t)
	if !strings.Contains(logs, "would exceed max size") && !strings.Contains(logs, "max-size") {
		t.Logf("Operator logs:\n%s", logs)
		t.Error("Expected max size limit detection in logs")
	}
	
	t.Log("✅ Max size limit test passed")
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
	err = wait.PollImmediate(2*time.Second, 15*time.Second, func() (bool, error) {
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
	
	// Wait and check logs for cooldown detection
	time.Sleep(8 * time.Second)
	
	logs := getOperatorLogs(t)
	if !strings.Contains(logs, "cooldown") {
		t.Logf("Operator logs:\n%s", logs)
		t.Error("Expected cooldown detection in logs")
	}
	
	t.Log("✅ Cooldown period test passed")
}

func TestOperatorLogs(t *testing.T) {
	t.Log("=== Test: Operator Logs ===")
	
	logs := getOperatorLogs(t)
	t.Logf("Recent operator logs:\n%s", logs)
	
	expectedLogs := []string{
		"Starting periodic reconciliation loop",
		"interval",
	}
	
	for _, expected := range expectedLogs {
		if !strings.Contains(logs, expected) {
			t.Errorf("Missing expected log entry: %s", expected)
		}
	}
	
	t.Log("✅ Operator logs test passed")
}

func waitForOperator(t *testing.T) {
	t.Log("Waiting for operator to be ready...")
	ctx := context.Background()
	err := wait.PollImmediate(3*time.Second, 30*time.Second, func() (bool, error) {
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
		TailLines: int64Ptr(50),
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
				"pvc-chonker.io/inodes-threshold":  "15%",
				"pvc-chonker.io/increase":          "50%",
				"pvc-chonker.io/cooldown":          "5m",
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
				"pvc-chonker.io/threshold": "15%",
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
				"pvc-chonker.io/threshold": "15%",
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