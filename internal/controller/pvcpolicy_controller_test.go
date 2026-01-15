package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	pvcchonkerv1alpha1 "github.com/logicIQ/pvc-chonker/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPVCPolicyReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := pvcchonkerv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add pvcchonkerv1alpha1 to scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}

	tests := []struct {
		name          string
		policy        *pvcchonkerv1alpha1.PVCPolicy
		pvcs          []corev1.PersistentVolumeClaim
		expectedCount int32
		expectError   bool
		policyExists  bool
	}{
		{
			name: "policy matches single PVC",
			policy: &pvcchonkerv1alpha1.PVCPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: pvcchonkerv1alpha1.PVCPolicySpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "database"},
					},
					Template: pvcchonkerv1alpha1.PVCPolicyTemplate{
						Enabled:   ptr.To(true),
						Threshold: ptr.To("85%"),
					},
				},
			},
			pvcs: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pvc",
						Namespace: "default",
						Labels:    map[string]string{"app": "database"},
					},
				},
			},
			expectedCount: 1,
			policyExists:  true,
		},
		{
			name: "policy matches no PVCs",
			policy: &pvcchonkerv1alpha1.PVCPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: pvcchonkerv1alpha1.PVCPolicySpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "database"},
					},
				},
			},
			pvcs: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pvc",
						Namespace: "default",
						Labels:    map[string]string{"app": "web"},
					},
				},
			},
			expectedCount: 0,
			policyExists:  true,
		},
		{
			name: "policy matches multiple PVCs",
			policy: &pvcchonkerv1alpha1.PVCPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: pvcchonkerv1alpha1.PVCPolicySpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"tier": "production"},
					},
				},
			},
			pvcs: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc1",
						Namespace: "default",
						Labels:    map[string]string{"tier": "production"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc2",
						Namespace: "default",
						Labels:    map[string]string{"tier": "production"},
					},
				},
			},
			expectedCount: 2,
			policyExists:  true,
		},
		{
			name: "policy not found",
			policy: &pvcchonkerv1alpha1.PVCPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "missing-policy",
					Namespace: "default",
				},
			},
			policyExists: false,
		},
		{
			name: "invalid label selector",
			policy: &pvcchonkerv1alpha1.PVCPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-policy",
					Namespace: "default",
				},
				Spec: pvcchonkerv1alpha1.PVCPolicySpec{
					Selector: metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "invalid",
								Operator: "InvalidOperator",
							},
						},
					},
				},
			},
			expectError:  true,
			policyExists: true,
		},
		{
			name: "namespace isolation",
			policy: &pvcchonkerv1alpha1.PVCPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: pvcchonkerv1alpha1.PVCPolicySpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "database"},
					},
				},
			},
			pvcs: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc-other-ns",
						Namespace: "other",
						Labels:    map[string]string{"app": "database"},
					},
				},
			},
			expectedCount: 0,
			policyExists:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []runtime.Object
			if tt.policyExists {
				objs = append(objs, tt.policy)
			}
			for i := range tt.pvcs {
				objs = append(objs, &tt.pvcs[i])
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&pvcchonkerv1alpha1.PVCPolicy{}).
				Build()

			r := &PVCPolicyReconciler{
				Client:        client,
				Scheme:        scheme,
				EventRecorder: &record.FakeRecorder{},
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.policy.Name,
					Namespace: tt.policy.Namespace,
				},
			}

			result, err := r.Reconcile(context.TODO(), req)

			if !tt.policyExists {
				if err != nil {
					t.Errorf("expected no error for missing policy, got: %v", err)
				}
				return
			}

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify requeue
				if result.RequeueAfter != 5*time.Minute {
					t.Errorf("expected requeue after 5m, got %v", result.RequeueAfter)
				}

				// Verify status update
				var updatedPolicy pvcchonkerv1alpha1.PVCPolicy
				err = client.Get(context.TODO(), req.NamespacedName, &updatedPolicy)
				if err != nil {
					t.Fatalf("failed to get updated policy: %v", err)
				}

				if updatedPolicy.Status.MatchedPVCs != tt.expectedCount {
					t.Errorf("expected %d matched PVCs, got %d", tt.expectedCount, updatedPolicy.Status.MatchedPVCs)
				}

				if updatedPolicy.Status.LastUpdated == nil {
					t.Error("expected LastUpdated to be set")
				}
			}
		})
	}
}

func TestPVCPolicyReconciler_ErrorHandling(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := pvcchonkerv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add pvcchonkerv1alpha1 to scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}

	tests := []struct {
		name        string
		policy      *pvcchonkerv1alpha1.PVCPolicy
		setupClient func() client.Client
		expectError bool
	}{
		{
			name: "client list error",
			policy: &pvcchonkerv1alpha1.PVCPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: pvcchonkerv1alpha1.PVCPolicySpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			setupClient: func() client.Client {
				return &errorClient{}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			r := &PVCPolicyReconciler{
				Client:        client,
				Scheme:        scheme,
				EventRecorder: &record.FakeRecorder{},
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.policy.Name,
					Namespace: tt.policy.Namespace,
				},
			}

			_, err := r.Reconcile(context.TODO(), req)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestPVCPolicyReconciler_StatusUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := pvcchonkerv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add pvcchonkerv1alpha1 to scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}

	policy := &pvcchonkerv1alpha1.PVCPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: pvcchonkerv1alpha1.PVCPolicySpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		},
	}

	client := &statusUpdateErrorClient{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(policy).
			WithStatusSubresource(&pvcchonkerv1alpha1.PVCPolicy{}).
			Build(),
	}

	r := &PVCPolicyReconciler{
		Client:        client,
		Scheme:        scheme,
		EventRecorder: &record.FakeRecorder{},
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      policy.Name,
			Namespace: policy.Namespace,
		},
	}

	_, err := r.Reconcile(context.TODO(), req)
	if err == nil {
		t.Error("expected status update error")
	}
}

type errorClient struct {
	client.Client
}

func (c *errorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if policy, ok := obj.(*pvcchonkerv1alpha1.PVCPolicy); ok {
		policy.ObjectMeta = metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		}
		return nil
	}
	return fmt.Errorf("unsupported object type: %T", obj)
}

func (c *errorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return fmt.Errorf("mock list error")
}

type statusUpdateErrorClient struct {
	client.Client
}

func (c *statusUpdateErrorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *statusUpdateErrorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.Client.List(ctx, list, opts...)
}

func (c *statusUpdateErrorClient) Status() client.StatusWriter {
	return &errorStatusWriter{}
}

type errorStatusWriter struct{}

func (w *errorStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return fmt.Errorf("mock status create error")
}

func (w *errorStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return fmt.Errorf("mock status update error")
}

func (w *errorStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return fmt.Errorf("mock status patch error")
}
