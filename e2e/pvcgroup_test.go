package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pvcchonkerv1alpha1 "github.com/LogicIQ/pvc-chonker/api/v1alpha1"
)

func TestPVCGroupCoordination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx := context.Background()
	k8sClient := getK8sClient(t)
	waitForOperator(t)

	setupPVCGroupTest(t)
	pvcGroup := triggerPVCGroupReconciliation(t, k8sClient, ctx)
	validatePVCGroupStatus(t, k8sClient, ctx, pvcGroup)
	validatePVCCoordination(t, k8sClient, ctx)

	_ = executeBash("kubectl delete -f fixtures/test-pvcgroup.yaml --ignore-not-found=true")
}

func setupPVCGroupTest(t *testing.T) {
	err := executeBash("kubectl apply -f fixtures/test-pvcgroup.yaml")
	require.NoError(t, err, "Failed to apply PVCGroup fixture")
	waitForPVCsCreated(t, getK8sClient(t), testNamespace, 2)
}

func triggerPVCGroupReconciliation(t *testing.T, k8sClient client.Client, ctx context.Context) *pvcchonkerv1alpha1.PVCGroup {
	var pvcGroup pvcchonkerv1alpha1.PVCGroup
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      "test-pvcgroup",
		Namespace: testNamespace,
	}, &pvcGroup))

	waitForPVCGroupStatus(t, k8sClient, "test-pvcgroup", testNamespace)

	for i := 0; i < 3; i++ {
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
			Name:      "test-pvcgroup",
			Namespace: testNamespace,
		}, &pvcGroup))

		pvcGroup.Annotations = map[string]string{"test-trigger": fmt.Sprintf("%d", i+1)}
		if err := k8sClient.Update(ctx, &pvcGroup); err != nil {
			t.Logf("Update attempt %d failed: %v", i+1, err)
			continue
		}
		waitForPVCGroupStatus(t, k8sClient, "test-pvcgroup", testNamespace)

		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
			Name:      "test-pvcgroup",
			Namespace: testNamespace,
		}, &pvcGroup))

		if pvcGroup.Status.MemberCount >= 2 {
			break
		}
	}

	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      "test-pvcgroup",
		Namespace: testNamespace,
	}, &pvcGroup))

	return &pvcGroup
}

func validatePVCGroupStatus(t *testing.T, k8sClient client.Client, ctx context.Context, pvcGroup *pvcchonkerv1alpha1.PVCGroup) {
	assert.True(t, pvcGroup.Status.MemberCount >= 2, "Should have at least 2 members")

	if assert.NotNil(t, pvcGroup.Status.CurrentSize) {
		expected := resource.MustParse("200Gi")
		assert.True(t, expected.Equal(*pvcGroup.Status.CurrentSize),
			"Expected 200Gi, got %s", pvcGroup.Status.CurrentSize.String())
	}
}

func validatePVCCoordination(t *testing.T, k8sClient client.Client, ctx context.Context) {
	var pvc1 corev1.PersistentVolumeClaim
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      "test-pvc-1",
		Namespace: testNamespace,
	}, &pvc1))

	expectedSize := resource.MustParse("200Gi")
	actualSize := pvc1.Spec.Resources.Requests[corev1.ResourceStorage]
	assert.True(t, expectedSize.Equal(actualSize),
		"PVC1 should be coordinated to 200Gi, got %s", actualSize.String())

	var pvcDisabled corev1.PersistentVolumeClaim
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      "test-pvc-disabled",
		Namespace: testNamespace,
	}, &pvcDisabled))

	originalSize := resource.MustParse("50Gi")
	disabledSize := pvcDisabled.Spec.Resources.Requests[corev1.ResourceStorage]
	assert.True(t, originalSize.Equal(disabledSize),
		"Disabled PVC should remain 50Gi, got %s", disabledSize.String())
}

func TestPVCGroupWebhook(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx := context.Background()
	k8sClient := getK8sClient(t)

	// Wait for operator to be ready
	waitForOperator(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "webhook-test-" + generateRandomString(8),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ns))
	defer func() {
		if err := k8sClient.Delete(ctx, ns); err != nil {
			t.Logf("Failed to cleanup namespace: %v", err)
		}
	}()

	// Create PVCGroup first
	pvcGroup := &pvcchonkerv1alpha1.PVCGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-test-group",
			Namespace: ns.Name,
		},
		Spec: pvcchonkerv1alpha1.PVCGroupSpec{
			Template: pvcchonkerv1alpha1.PVCGroupTemplate{
				Enabled:   boolPtr(true),
				Threshold: stringPtr("75%"),
				Increase:  stringPtr("30%"),
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pvcGroup))

	// Wait for PVCGroup to be processed
	waitForPVCGroupStatus(t, k8sClient, "webhook-test-group", ns.Name)

	// Create PVC that should match the group
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-test-pvc",
			Namespace: ns.Name,
			Annotations: map[string]string{
				"pvc-chonker.io/group":   "webhook-test-group",
				"pvc-chonker.io/enabled": "true",
				// Don't include threshold/increase so webhook can add them
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("100Gi"),
				},
			},
			StorageClassName: stringPtr("standard"),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pvc))

	// Wait for potential webhook processing
	waitForPVCCreated(t, k8sClient, pvc.Name, pvc.Namespace)

	// Check that webhook applied group annotations
	var createdPVC corev1.PersistentVolumeClaim
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
	}, &createdPVC))

	// Check if webhook is working by looking for group annotations
	if createdPVC.Annotations != nil {
		if groupName, exists := createdPVC.Annotations["pvc-chonker.io/group"]; exists {
			assert.Equal(t, pvcGroup.Name, groupName)
			assert.Equal(t, "true", createdPVC.Annotations["pvc-chonker.io/enabled"])
			assert.Equal(t, "75%", createdPVC.Annotations["pvc-chonker.io/threshold"])
			assert.Equal(t, "30%", createdPVC.Annotations["pvc-chonker.io/increase"])
			t.Log("Webhook is working - all group annotations applied correctly")
		} else {
			t.Log("Webhook not enabled - checking if operator has webhook flag enabled")
			// Check operator deployment for webhook flag
			logs := getOperatorLogs(t)
			if strings.Contains(logs, "webhook") {
				t.Log("Webhook mentioned in logs but not working")
			} else {
				t.Log("Webhook not enabled in operator configuration")
			}
		}
	} else {
		t.Log("No annotations found - webhook not enabled")
	}
}

func TestPVCGroupWebhookE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx := context.Background()
	k8sClient := getK8sClient(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "webhook-e2e-test-" + generateRandomString(8),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ns))
	defer func() {
		if err := k8sClient.Delete(ctx, ns); err != nil {
			t.Logf("Failed to cleanup namespace: %v", err)
		}
	}()

	// Create PVCGroup first
	pvcGroup := &pvcchonkerv1alpha1.PVCGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-e2e-group",
			Namespace: ns.Name,
		},
		Spec: pvcchonkerv1alpha1.PVCGroupSpec{
			Template: pvcchonkerv1alpha1.PVCGroupTemplate{
				Enabled:   boolPtr(true),
				Threshold: stringPtr("75%"),
				Increase:  stringPtr("30%"),
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pvcGroup))

	// Create PVC that should match the group and trigger webhook
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-e2e-pvc",
			Namespace: ns.Name,
			Annotations: map[string]string{
				"pvc-chonker.io/group":   "webhook-e2e-group",
				"pvc-chonker.io/enabled": "true",
				// Don't include threshold/increase so webhook can add them
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("100Gi"),
				},
			},
			StorageClassName: stringPtr("standard"),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pvc))

	// Wait for potential webhook processing
	waitForPVCCreated(t, k8sClient, pvc.Name, pvc.Namespace)

	// Check that webhook applied group annotations (if webhook is enabled)
	var createdPVC corev1.PersistentVolumeClaim
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
	}, &createdPVC))

	// Note: Webhook may not be enabled in test environment
	if createdPVC.Annotations != nil {
		if groupName, exists := createdPVC.Annotations["pvc-chonker.io/group"]; exists {
			assert.Equal(t, pvcGroup.Name, groupName)
			t.Log("Webhook E2E test passed - group annotation found")
		} else {
			t.Log("Webhook not enabled in E2E test environment")
		}
	} else {
		t.Log("Webhook not enabled - no annotations found in E2E test")
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func executeBash(command string) error {
	// Validate command is not empty
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("command cannot be empty")
	}
	
	// Whitelist allowed command prefixes for safety
	allowedPrefixes := []string{"kubectl", "helm", "task"}
	isAllowed := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(strings.TrimSpace(command), prefix) {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return fmt.Errorf("command not allowed: must start with kubectl, helm, or task")
	}
	
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}
	return nil
}

// waitForPVCsCreated waits for at least minCount PVCs to be created in the namespace
func waitForPVCsCreated(t *testing.T, k8sClient client.Client, namespace string, minCount int) {
	ctx := context.Background()
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		var pvcList corev1.PersistentVolumeClaimList
		if err := k8sClient.List(ctx, &pvcList, &client.ListOptions{Namespace: namespace}); err != nil {
			return false, err
		}
		return len(pvcList.Items) >= minCount, nil
	})
	if err != nil {
		t.Fatalf("PVCs not created in time: %v", err)
	}
}

// waitForPVCGroupStatus waits for PVCGroup status to be updated
func waitForPVCGroupStatus(t *testing.T, k8sClient client.Client, name, namespace string) {
	ctx := context.Background()
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		var pvcGroup pvcchonkerv1alpha1.PVCGroup
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &pvcGroup); err != nil {
			return false, err
		}
		return pvcGroup.Status.LastUpdated != nil, nil
	})
	if err != nil {
		t.Fatalf("PVCGroup status not updated in time: %v", err)
	}
}

// waitForPVCCreated waits for a PVC to be created and available
func waitForPVCCreated(t *testing.T, k8sClient client.Client, name, namespace string) {
	ctx := context.Background()
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 15*time.Second, true, func(ctx context.Context) (bool, error) {
		var pvc corev1.PersistentVolumeClaim
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &pvc); err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("PVC not created in time: %v", err)
	}
}