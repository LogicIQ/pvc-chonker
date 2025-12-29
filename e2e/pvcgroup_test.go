package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	pvcchonkerv1alpha1 "github.com/LogicIQ/pvc-chonker/api/v1alpha1"
)

func TestPVCGroupCoordination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx := context.Background()
	k8sClient := getK8sClient(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvcgroup-test-" + generateRandomString(8),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ns))
	defer func() {
		_ = k8sClient.Delete(ctx, ns)
	}()

	// Create PVCGroup
	pvcGroup := &pvcchonkerv1alpha1.PVCGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-group",
			Namespace: ns.Name,
		},
		Spec: pvcchonkerv1alpha1.PVCGroupSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "test-app",
					"tier": "database",
				},
			},
			CoordinationPolicy: pvcchonkerv1alpha1.CoordinationPolicyLargest,
			Template: pvcchonkerv1alpha1.PVCGroupTemplate{
				Enabled:   boolPtr(true),
				Threshold: stringPtr("80%"),
				Increase:  stringPtr("25%"),
				MaxSize:   &[]resource.Quantity{resource.MustParse("1000Gi")}[0],
				Cooldown:  &metav1.Duration{Duration: 30 * time.Second},
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pvcGroup))

	// Create PVCs with different sizes
	pvc1 := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-1",
			Namespace: ns.Name,
			Labels: map[string]string{
				"app":  "test-app",
				"tier": "database",
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
	require.NoError(t, k8sClient.Create(ctx, pvc1))

	pvc2 := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-2",
			Namespace: ns.Name,
			Labels: map[string]string{
				"app":  "test-app",
				"tier": "database",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("200Gi"),
				},
			},
			StorageClassName: stringPtr("standard"),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pvc2))

	// Create disabled PVC
	pvcDisabled := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-disabled",
			Namespace: ns.Name,
			Labels: map[string]string{
				"app":  "test-app",
				"tier": "database",
			},
			Annotations: map[string]string{
				"pvc-chonker.io/enabled": "false",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("50Gi"),
				},
			},
			StorageClassName: stringPtr("standard"),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pvcDisabled))

	// Wait for PVCGroup controller to process
	time.Sleep(15 * time.Second)

	// Check PVCGroup status
	var updatedGroup pvcchonkerv1alpha1.PVCGroup
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      pvcGroup.Name,
		Namespace: pvcGroup.Namespace,
	}, &updatedGroup))

	// Should have 2 active members (disabled PVC excluded)
	assert.Equal(t, int32(2), updatedGroup.Status.MemberCount)
	
	// Current size should be 200Gi (largest policy)
	if assert.NotNil(t, updatedGroup.Status.CurrentSize) {
		expected := resource.MustParse("200Gi")
		assert.True(t, expected.Equal(*updatedGroup.Status.CurrentSize))
	}

	// Check that PVC1 was coordinated to match PVC2 size
	var updatedPVC1 corev1.PersistentVolumeClaim
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      pvc1.Name,
		Namespace: pvc1.Namespace,
	}, &updatedPVC1))

	expectedSize := resource.MustParse("200Gi")
	actualSize := updatedPVC1.Spec.Resources.Requests[corev1.ResourceStorage]
	assert.True(t, expectedSize.Equal(actualSize), 
		"PVC1 should be coordinated to 200Gi, got %s", actualSize.String())

	// Check that disabled PVC was not modified
	var updatedPVCDisabled corev1.PersistentVolumeClaim
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      pvcDisabled.Name,
		Namespace: pvcDisabled.Namespace,
	}, &updatedPVCDisabled))

	originalSize := resource.MustParse("50Gi")
	disabledSize := updatedPVCDisabled.Spec.Resources.Requests[corev1.ResourceStorage]
	assert.True(t, originalSize.Equal(disabledSize),
		"Disabled PVC should remain 50Gi, got %s", disabledSize.String())
}

func TestPVCGroupWebhook(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx := context.Background()
	k8sClient := getK8sClient(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvcgroup-webhook-test-" + generateRandomString(8),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ns))
	defer func() {
		_ = k8sClient.Delete(ctx, ns)
	}()

	// Create PVCGroup first
	pvcGroup := &pvcchonkerv1alpha1.PVCGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-test-group",
			Namespace: ns.Name,
		},
		Spec: pvcchonkerv1alpha1.PVCGroupSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"webhook-test": "true",
				},
			},
			Template: pvcchonkerv1alpha1.PVCGroupTemplate{
				Enabled:   boolPtr(true),
				Threshold: stringPtr("75%"),
				Increase:  stringPtr("30%"),
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pvcGroup))

	// Create PVC that should match the group
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-test-pvc",
			Namespace: ns.Name,
			Labels: map[string]string{
				"webhook-test": "true",
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

	// Check that webhook applied group annotations
	var createdPVC corev1.PersistentVolumeClaim
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
	}, &createdPVC))

	// Verify webhook added group annotations
	assert.Equal(t, pvcGroup.Name, createdPVC.Annotations["pvc-chonker.io/group"])
	assert.Equal(t, "true", createdPVC.Annotations["pvc-chonker.io/enabled"])
	assert.Equal(t, "75%", createdPVC.Annotations["pvc-chonker.io/threshold"])
	assert.Equal(t, "30%", createdPVC.Annotations["pvc-chonker.io/increase"])
}

func TestPVCGroupCoordinationPolicies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx := context.Background()
	k8sClient := getK8sClient(t)

	tests := []struct {
		name           string
		policy         pvcchonkerv1alpha1.CoordinationPolicy
		pvcSizes       []string
		expectedSize   string
	}{
		{
			name:         "largest policy",
			policy:       pvcchonkerv1alpha1.CoordinationPolicyLargest,
			pvcSizes:     []string{"100Gi", "200Gi", "150Gi"},
			expectedSize: "200Gi",
		},
		{
			name:         "average policy",
			policy:       pvcchonkerv1alpha1.CoordinationPolicyAverage,
			pvcSizes:     []string{"100Gi", "200Gi", "150Gi"},
			expectedSize: "150Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy-test-" + generateRandomString(8),
				},
			}
			require.NoError(t, k8sClient.Create(ctx, ns))
			defer func() {
				_ = k8sClient.Delete(ctx, ns)
			}()

			// Create PVCGroup with specific policy
			pvcGroup := &pvcchonkerv1alpha1.PVCGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy-test-group",
					Namespace: ns.Name,
				},
				Spec: pvcchonkerv1alpha1.PVCGroupSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"policy-test": tt.name,
						},
					},
					CoordinationPolicy: tt.policy,
					Template: pvcchonkerv1alpha1.PVCGroupTemplate{
						Enabled: boolPtr(true),
					},
				},
			}
			require.NoError(t, k8sClient.Create(ctx, pvcGroup))

			for i, size := range tt.pvcSizes {
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("policy-test-pvc-%d", i),
						Namespace: ns.Name,
						Labels: map[string]string{
							"policy-test": tt.name,
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(size),
							},
						},
						StorageClassName: stringPtr("standard"),
					},
				}
				require.NoError(t, k8sClient.Create(ctx, pvc))
			}

			// Wait for coordination
			time.Sleep(15 * time.Second)

			// Check PVCGroup status
			var updatedGroup pvcchonkerv1alpha1.PVCGroup
			require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
				Name:      pvcGroup.Name,
				Namespace: pvcGroup.Namespace,
			}, &updatedGroup))

			// Verify coordinated size
			if assert.NotNil(t, updatedGroup.Status.CurrentSize) {
				expected := resource.MustParse(tt.expectedSize)
				assert.True(t, expected.Equal(*updatedGroup.Status.CurrentSize),
					"Expected %s, got %s", expected.String(), updatedGroup.Status.CurrentSize.String())
			}
		})
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
		_ = k8sClient.Delete(ctx, ns)
	}()

	// Create PVCGroup first
	pvcGroup := &pvcchonkerv1alpha1.PVCGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-e2e-group",
			Namespace: ns.Name,
		},
		Spec: pvcchonkerv1alpha1.PVCGroupSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"webhook-e2e": "true",
				},
			},
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
			Labels: map[string]string{
				"webhook-e2e": "true",
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

	// Check that webhook applied group annotations
	var createdPVC corev1.PersistentVolumeClaim
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
	}, &createdPVC))

	// Verify webhook added group annotations
	assert.Equal(t, pvcGroup.Name, createdPVC.Annotations["pvc-chonker.io/group"])
	assert.Equal(t, "true", createdPVC.Annotations["pvc-chonker.io/enabled"])
	assert.Equal(t, "75%", createdPVC.Annotations["pvc-chonker.io/threshold"])
	assert.Equal(t, "30%", createdPVC.Annotations["pvc-chonker.io/increase"])
}

func boolPtr(b bool) *bool {
	return &b
}