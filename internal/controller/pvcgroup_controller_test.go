package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pvcchonkerv1alpha1 "github.com/logicIQ/pvc-chonker/api/v1alpha1"
)

func TestPVCGroupReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, pvcchonkerv1alpha1.AddToScheme(scheme))

	tests := []struct {
		name           string
		pvcGroup       *pvcchonkerv1alpha1.PVCGroup
		pvcs           []corev1.PersistentVolumeClaim
		expectedStatus pvcchonkerv1alpha1.PVCGroupStatus
	}{
		{
			name: "basic group with matching PVCs",
			pvcGroup: &pvcchonkerv1alpha1.PVCGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-group",
					Namespace: "default",
				},
				Spec: pvcchonkerv1alpha1.PVCGroupSpec{
					Template: pvcchonkerv1alpha1.PVCGroupTemplate{
						Threshold: stringPtr("80%"),
					},
				},
			},
			pvcs: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-1",
						Namespace: "default",
						Annotations: map[string]string{
							"pvc-chonker.io/group":   "test-group",
							"pvc-chonker.io/enabled": "true",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("100Gi"),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-2",
						Namespace: "default",
						Annotations: map[string]string{
							"pvc-chonker.io/group":   "test-group",
							"pvc-chonker.io/enabled": "true",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("200Gi"),
							},
						},
					},
				},
			},
			expectedStatus: pvcchonkerv1alpha1.PVCGroupStatus{
				MemberCount: 2,
				CurrentSize: &[]resource.Quantity{resource.MustParse("200Gi")}[0],
			},
		},
		{
			name: "group with disabled PVC",
			pvcGroup: &pvcchonkerv1alpha1.PVCGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-group",
					Namespace: "default",
				},
				Spec: pvcchonkerv1alpha1.PVCGroupSpec{
					Template: pvcchonkerv1alpha1.PVCGroupTemplate{
						Threshold: stringPtr("80%"),
					},
				},
			},
			pvcs: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-1",
						Namespace: "default",
						Annotations: map[string]string{
							"pvc-chonker.io/group":   "test-group",
							"pvc-chonker.io/enabled": "true",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("100Gi"),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-2",
						Namespace: "default",
						Annotations: map[string]string{
							"pvc-chonker.io/group":   "test-group",
							"pvc-chonker.io/enabled": "false",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("200Gi"),
							},
						},
					},
				},
			},
			expectedStatus: pvcchonkerv1alpha1.PVCGroupStatus{
				MemberCount: 1,
				CurrentSize: &[]resource.Quantity{resource.MustParse("100Gi")}[0],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []runtime.Object{tt.pvcGroup}
			for i := range tt.pvcs {
				objs = append(objs, &tt.pvcs[i])
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).WithStatusSubresource(&pvcchonkerv1alpha1.PVCGroup{}).Build()
			recorder := record.NewFakeRecorder(10)

			reconciler := &PVCGroupReconciler{
				Client:        client,
				Scheme:        scheme,
				EventRecorder: recorder,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.pvcGroup.Name,
					Namespace: tt.pvcGroup.Namespace,
				},
			}

			result, err := reconciler.Reconcile(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, time.Minute*10, result.RequeueAfter)

			// Check updated status
			var updatedGroup pvcchonkerv1alpha1.PVCGroup
			err = client.Get(context.Background(), req.NamespacedName, &updatedGroup)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus.MemberCount, updatedGroup.Status.MemberCount)
			if tt.expectedStatus.CurrentSize != nil {
				require.NotNil(t, updatedGroup.Status.CurrentSize)
				assert.True(t, tt.expectedStatus.CurrentSize.Equal(*updatedGroup.Status.CurrentSize))
			}
			assert.NotNil(t, updatedGroup.Status.LastUpdated)
		})
	}
}

func TestPVCGroupReconciler_calculateLargestSize(t *testing.T) {
	reconciler := &PVCGroupReconciler{}

	pvcs := []corev1.PersistentVolumeClaim{
		{
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("100Gi"),
					},
				},
			},
		},
		{
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("200Gi"),
					},
				},
			},
		},
		{
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("150Gi"),
					},
				},
			},
		},
	}

	result := reconciler.calculateLargestSize(pvcs)
	expected := resource.MustParse("200Gi")
	assert.True(t, expected.Equal(result), "expected %s, got %s", expected.String(), result.String())
}

func TestPVCGroupReconciler_coordinatePVCSizes(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, pvcchonkerv1alpha1.AddToScheme(scheme))

	group := &pvcchonkerv1alpha1.PVCGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-group",
			Namespace: "default",
		},
	}

	pvcs := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pvc-1",
				Namespace: "default",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("100Gi"),
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pvc-2",
				Namespace: "default",
				Annotations: map[string]string{
					"pvc-chonker.io/threshold": "90%",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("150Gi"),
					},
				},
			},
		},
	}

	objs := []runtime.Object{group}
	for i := range pvcs {
		objs = append(objs, &pvcs[i])
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).WithStatusSubresource(&pvcchonkerv1alpha1.PVCGroup{}).Build()
	recorder := record.NewFakeRecorder(10)

	reconciler := &PVCGroupReconciler{
		Client:        client,
		Scheme:        scheme,
		EventRecorder: recorder,
	}

	targetSize := resource.MustParse("200Gi")
	err := reconciler.coordinatePVCSizes(context.Background(), pvcs, targetSize, group)
	require.NoError(t, err)

	// Check that pvc-1 was updated to target size
	var updatedPVC1 corev1.PersistentVolumeClaim
	err = client.Get(context.Background(), types.NamespacedName{Name: "pvc-1", Namespace: "default"}, &updatedPVC1)
	require.NoError(t, err)
	assert.True(t, targetSize.Equal(updatedPVC1.Spec.Resources.Requests[corev1.ResourceStorage]))

	// Check that pvc-2 was also updated to target size (since 150Gi < 200Gi)
	var updatedPVC2 corev1.PersistentVolumeClaim
	err = client.Get(context.Background(), types.NamespacedName{Name: "pvc-2", Namespace: "default"}, &updatedPVC2)
	require.NoError(t, err)
	assert.True(t, targetSize.Equal(updatedPVC2.Spec.Resources.Requests[corev1.ResourceStorage]))
}

func stringPtr(s string) *string {
	return &s
}
