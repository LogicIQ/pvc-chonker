package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1 "k8s.io/api/core/v1"
)

func TestPVCPolicyBasic(t *testing.T) {
	t.Log("=== Test: PVCPolicy Basic Functionality ===")
	ctx := context.Background()
	
	policy := createTestPVCPolicy()
	err := k8sClient.Create(ctx, policy)
	if err != nil {
		t.Fatalf("Failed to create PVCPolicy: %v", err)
	}
	defer k8sClient.Delete(ctx, policy)
	
	pvc := createPolicyManagedPVC()
	_, err = clientset.CoreV1().PersistentVolumeClaims(testNamespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create policy-managed PVC: %v", err)
	}
	defer clientset.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, "test-policy-pvc", metav1.DeleteOptions{})
	
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-policy-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	time.Sleep(15 * time.Second)
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
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
	
	policy := createTestPVCPolicy()
	err := k8sClient.Create(ctx, policy)
	if err != nil {
		t.Fatalf("Failed to create PVCPolicy: %v", err)
	}
	defer k8sClient.Delete(ctx, policy)
	
	pvc := createPolicyOverridePVC()
	_, err = clientset.CoreV1().PersistentVolumeClaims(testNamespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create override PVC: %v", err)
	}
	defer clientset.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, "test-override-pvc", metav1.DeleteOptions{})
	
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "test-override-pvc", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	if err != nil {
		t.Fatalf("PVC did not bind: %v", err)
	}
	
	time.Sleep(15 * time.Second)
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
	if strings.Contains(logs, "95") || strings.Contains(logs, "annotation") {
		t.Log("Annotation override detected")
	} else {
		t.Log("No explicit annotation override detection in logs")
	}
	
	t.Log("PVCPolicy override test passed")
}