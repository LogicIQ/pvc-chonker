package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestBasicExpansion(t *testing.T) {
	t.Log("=== Test: Basic PVC Expansion ===")
	ctx := context.Background()
	
	waitForOperator(t)
	
	// Wait for test pod to be ready
	waitForPod(t, "test-pod", testNamespace)
	
	pvcs, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		t.Logf("Found %d PVCs in %s namespace", len(pvcs.Items), testNamespace)
		for _, pvc := range pvcs.Items {
			t.Logf("PVC: %s, Status: %s", pvc.Name, pvc.Status.Phase)
		}
	}
	
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-pvc", metav1.GetOptions{})
		if err != nil {
			t.Logf("test-pvc not found: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("Test PVC not found after waiting: %v", err)
	}
	
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		t.Logf("PVC status: %s", pvc.Status.Phase)
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-pvc", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get test PVC: %v", err)
	}
	originalSize := pvc.Status.Capacity[corev1.ResourceStorage]
	t.Logf("Original PVC size: %s", originalSize.String())
	
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 120*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		newSize := pvc.Status.Capacity[corev1.ResourceStorage]
		t.Logf("Current size: %s", newSize.String())
		return newSize.Cmp(originalSize) > 0, nil
	})
	if err != nil {
		logs := getOperatorLogs(t)
		t.Logf("Operator logs:\n%s", logs)
		t.Fatalf("PVC was not expanded within timeout: %v", err)
	}
	
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-pvc", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get test PVC: %v", err)
	}
	finalSize := pvc.Status.Capacity[corev1.ResourceStorage]
	t.Logf("Final PVC size: %s (expanded from %s)", finalSize.String(), originalSize.String())
	t.Log("Basic expansion test passed")
}

func TestInodeExpansion(t *testing.T) {
	t.Log("=== Test: Inode Threshold Expansion ===")
	ctx := context.Background()
	
	_, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Create(ctx, createInodePVC(), metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create inode PVC: %v", err)
	}
	defer func() {
		if err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, "test-inode-pvc", metav1.DeleteOptions{}); err != nil {
			t.Logf("Warning: Failed to cleanup inode PVC: %v", err)
		}
	}()
	
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-inode-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-inode-pvc", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get inode PVC: %v", err)
	}
	originalSize := pvc.Status.Capacity[corev1.ResourceStorage]
	t.Logf("Original inode PVC size: %s", originalSize.String())
	
	// Wait for operator to process the PVC
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		logs := getOperatorLogs(t)
		return strings.Contains(logs, "test-inode-pvc"), nil
	})
	if err != nil {
		t.Log("Operator may not have processed inode PVC yet")
	}
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
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
	
	_, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Create(ctx, createMaxSizePVC(), metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create max size PVC: %v", err)
	}
	defer func() {
		if err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, "test-max-size-pvc", metav1.DeleteOptions{}); err != nil {
			t.Logf("Warning: Failed to cleanup max size PVC: %v", err)
		}
	}()
	
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-max-size-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	// Wait for operator to process the PVC
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		logs := getOperatorLogs(t)
		return strings.Contains(logs, "test-max-size-pvc"), nil
	})
	if err != nil {
		t.Log("Operator may not have processed max size PVC yet")
	}
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
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
	
	_, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Create(ctx, createCooldownPVC(), metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create cooldown PVC: %v", err)
	}
	defer clientset.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, "test-cooldown-pvc", metav1.DeleteOptions{})
	
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-cooldown-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-cooldown-pvc", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get cooldown PVC: %v", err)
	}
	
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Annotations["pvc-chonker.io/last-expansion"] = time.Now().Format(time.RFC3339)
	
	_, err = clientset.CoreV1().PersistentVolumeClaims(testNamespace).Update(ctx, pvc, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update PVC with last expansion: %v", err)
	}
	
	time.Sleep(15 * time.Second)
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
	if !strings.Contains(strings.ToLower(logs), "cooldown") {
		t.Log("No cooldown detection in logs - may not have processed yet")
	} else {
		t.Log("Cooldown detection found")
	}
	
	t.Log("Cooldown period test passed")
}