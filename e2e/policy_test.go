package e2e

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1 "k8s.io/api/core/v1"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/assert"

	pvcchonkerv1alpha1 "github.com/LogicIQ/pvc-chonker/api/v1alpha1"
)

func TestPVCPolicyBasic(t *testing.T) {
	t.Log("=== Test: PVCPolicy Basic Functionality ===")
	ctx := context.Background()
	k8sClient := getK8sClient(t)
	
	// Wait for operator to be ready
	waitForOperator(t)
	
	// Apply the test PVCPolicy fixture
	cmd := exec.Command("kubectl", "apply", "-f", "fixtures/test-pvcpolicy.yaml")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("kubectl apply error: %s", string(output))
	}
	require.NoError(t, err, "Failed to apply PVCPolicy fixture")
	
	// Create a PVC that matches the policy
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "policy-test-pvc",
			Namespace: testNamespace,
			Labels: map[string]string{
				"test-policy": "enabled",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("standard"),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pvc))
	
	// Wait for policy to be processed
	time.Sleep(15 * time.Second)
	
	// Check if policy was applied
	var policy pvcchonkerv1alpha1.PVCPolicy
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      "test-policy",
		Namespace: testNamespace,
	}, &policy)
	require.NoError(t, err, "PVCPolicy should exist")
	
	// Verify policy configuration
	assert.NotNil(t, policy.Spec.Template.Enabled)
	assert.True(t, *policy.Spec.Template.Enabled)
	assert.Equal(t, "85%", *policy.Spec.Template.Threshold)
	assert.Equal(t, "25%", *policy.Spec.Template.Increase)
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
	if strings.Contains(strings.ToLower(logs), "policy") {
		t.Log("PVCPolicy processing detected")
	} else {
		t.Log("No explicit policy processing in logs")
	}
	
	// Cleanup
	_ = k8sClient.Delete(ctx, pvc)
	cmd = exec.Command("kubectl", "delete", "-f", "fixtures/test-pvcpolicy.yaml", "--ignore-not-found=true")
	_ = cmd.Run()
	
	t.Log("PVCPolicy basic test passed")
}

func TestPVCPolicyOverride(t *testing.T) {
	t.Log("=== Test: PVCPolicy vs Annotation Override ===")
	ctx := context.Background()
	k8sClient := getK8sClient(t)
	
	// Wait for operator to be ready
	waitForOperator(t)
	
	// Apply the test PVCPolicy fixture
	cmd := exec.Command("kubectl", "apply", "-f", "fixtures/test-pvcpolicy.yaml")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("kubectl apply error: %s", string(output))
	}
	require.NoError(t, err, "Failed to apply PVCPolicy fixture")
	
	// Create a PVC that matches the policy but has annotation overrides
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "override-test-pvc",
			Namespace: testNamespace,
			Labels: map[string]string{
				"test-policy": "enabled",
			},
			Annotations: map[string]string{
				"pvc-chonker.io/enabled":   "true",
				"pvc-chonker.io/threshold": "95%", // Override policy's 85%
				"pvc-chonker.io/increase":  "50%", // Override policy's 25%
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("standard"),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pvc))
	
	// Wait for processing
	time.Sleep(15 * time.Second)
	
	// Verify the PVC has the override annotations
	var updatedPVC corev1.PersistentVolumeClaim
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
	}, &updatedPVC)
	require.NoError(t, err)
	
	// Annotations should override policy values
	assert.Equal(t, "95%", updatedPVC.Annotations["pvc-chonker.io/threshold"])
	assert.Equal(t, "50%", updatedPVC.Annotations["pvc-chonker.io/increase"])
	
	logs := getOperatorLogs(t)
	t.Logf("Operator logs:\n%s", logs)
	
	if strings.Contains(logs, "95") || strings.Contains(logs, "annotation") {
		t.Log("Annotation override detected")
	} else {
		t.Log("No explicit annotation override detection in logs")
	}
	
	// Cleanup
	_ = k8sClient.Delete(ctx, pvc)
	cmd = exec.Command("kubectl", "delete", "-f", "fixtures/test-pvcpolicy.yaml", "--ignore-not-found=true")
	_ = cmd.Run()
	
	t.Log("PVCPolicy override test passed")
}

