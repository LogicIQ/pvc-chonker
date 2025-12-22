package controller

import (
	"context"
	"testing"

	"github.com/logicIQ/pvc-chonker/pkg/annotations"
	"github.com/logicIQ/pvc-chonker/pkg/cache"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsPVCEligible(t *testing.T) {
	reconciler := &PersistentVolumeClaimReconciler{}

	tests := []struct {
		name     string
		pvc      *corev1.PersistentVolumeClaim
		expected bool
	}{
		{
			name: "eligible PVC",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeMode: func() *corev1.PersistentVolumeMode {
						mode := corev1.PersistentVolumeFilesystem
						return &mode
					}(),
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimBound,
				},
			},
			expected: true,
		},
		{
			name: "block volume mode",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeMode: func() *corev1.PersistentVolumeMode {
						mode := corev1.PersistentVolumeBlock
						return &mode
					}(),
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimBound,
				},
			},
			expected: false,
		},
		{
			name: "not bound",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimPending,
				},
			},
			expected: false,
		},
		{
			name: "nil volume mode (defaults to filesystem)",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeMode: nil,
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimBound,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconciler.IsPVCEligible(tt.pvc)
			if result != tt.expected {
				t.Errorf("isPVCEligible() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsStorageClassExpandable(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = storagev1.AddToScheme(scheme)

	tests := []struct {
		name         string
		pvc          *corev1.PersistentVolumeClaim
		storageClass *storagev1.StorageClass
		expected     bool
	}{
		{
			name: "expandable storage class",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: func() *string { s := "expandable-sc"; return &s }(),
				},
			},
			storageClass: &storagev1.StorageClass{
				ObjectMeta:           metav1.ObjectMeta{Name: "expandable-sc"},
				AllowVolumeExpansion: func() *bool { b := true; return &b }(),
			},
			expected: true,
		},
		{
			name: "non-expandable storage class",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: func() *string { s := "non-expandable-sc"; return &s }(),
				},
			},
			storageClass: &storagev1.StorageClass{
				ObjectMeta:           metav1.ObjectMeta{Name: "non-expandable-sc"},
				AllowVolumeExpansion: func() *bool { b := false; return &b }(),
			},
			expected: false,
		},
		{
			name: "no storage class name",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: nil,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object
			if tt.storageClass != nil {
				objects = append(objects, tt.storageClass)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			reconciler := &PersistentVolumeClaimReconciler{
				Client:       fakeClient,
				storageCache: cache.NewStorageClassCache(),
			}

			ctx := context.Background()
			result := reconciler.IsStorageClassExpandable(ctx, tt.pvc)
			if result != tt.expected {
				t.Errorf("isStorageClassExpandable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExpandPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name         string
		pvc          *corev1.PersistentVolumeClaim
		config       *annotations.PVCConfig
		dryRun       bool
		wantErr      bool
		expectUpdate bool
	}{
		{
			name: "successful expansion",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("10Gi"),
						},
					},
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			},
			config: &annotations.PVCConfig{
				Increase:   "20%",
				MinScaleUp: resource.MustParse("1Gi"),
			},
			dryRun:       false,
			wantErr:      false,
			expectUpdate: true,
		},
		{
			name: "dry run mode",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			},
			config: &annotations.PVCConfig{
				Increase:   "20%",
				MinScaleUp: resource.MustParse("1Gi"),
			},
			dryRun:       true,
			wantErr:      false,
			expectUpdate: false,
		},
		{
			name: "exceeds max size",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			},
			config: &annotations.PVCConfig{
				Increase:   "20%",
				MinScaleUp: resource.MustParse("1Gi"),
				MaxSize:    resource.MustParse("11Gi"),
			},
			dryRun:  false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.pvc).
				Build()

			reconciler := &PersistentVolumeClaimReconciler{
				Client: fakeClient,
				DryRun: tt.dryRun,
			}

			ctx := context.Background()
			err := reconciler.ExpandPVC(ctx, tt.pvc, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("expandPVC() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.expectUpdate && !tt.wantErr {
				// Verify PVC was updated
				var updatedPVC corev1.PersistentVolumeClaim
				err := fakeClient.Get(ctx, types.NamespacedName{
					Name:      tt.pvc.Name,
					Namespace: tt.pvc.Namespace,
				}, &updatedPVC)
				if err != nil {
					t.Errorf("Failed to get updated PVC: %v", err)
				}

				// Check if size was increased
				originalSize := tt.pvc.Status.Capacity[corev1.ResourceStorage]
				newSize := updatedPVC.Spec.Resources.Requests[corev1.ResourceStorage]
				if newSize.Cmp(originalSize) <= 0 {
					t.Errorf("PVC size was not increased: original=%s, new=%s",
						originalSize.String(), newSize.String())
				}
			}
		})
	}
}

func TestReconcilePVCLogic(t *testing.T) {
	// This test focuses on the individual components that can be easily unit tested
	// The full reconcilePVC method would be better tested with integration tests
	t.Log("ReconcilePVC logic is tested through individual component tests")
	t.Log("Full integration testing would require a more complex test setup")
}

func TestNeedLeaderElection(t *testing.T) {
	reconciler := &PersistentVolumeClaimReconciler{}
	if !reconciler.NeedLeaderElection() {
		t.Error("NeedLeaderElection() should return true")
	}
}
