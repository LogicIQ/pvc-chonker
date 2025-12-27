package webhook

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	pvcchonkerv1alpha1 "github.com/logicIQ/pvc-chonker/api/v1alpha1"
)

func TestPVCGroupMutator_Handle(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, pvcchonkerv1alpha1.AddToScheme(scheme))

	tests := []struct {
		name         string
		pvc          *corev1.PersistentVolumeClaim
		pvcGroups    []pvcchonkerv1alpha1.PVCGroup
		expectPatch  bool
		expectedKeys []string
	}{
		{
			name: "PVC matches group selector",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "test"},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("100Gi"),
						},
					},
				},
			},
			pvcGroups: []pvcchonkerv1alpha1.PVCGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-group",
						Namespace: "default",
					},
					Spec: pvcchonkerv1alpha1.PVCGroupSpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						Template: pvcchonkerv1alpha1.PVCGroupTemplate{
							Enabled:   boolPtr(true),
							Threshold: stringPtr("80%"),
							Increase:  stringPtr("20%"),
						},
					},
				},
			},
			expectPatch: true,
			expectedKeys: []string{
				"pvc-chonker.io/group",
				"pvc-chonker.io/enabled",
				"pvc-chonker.io/threshold",
				"pvc-chonker.io/increase",
			},
		},
		{
			name: "PVC with existing annotations not overridden",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "test"},
					Annotations: map[string]string{
						"pvc-chonker.io/threshold": "90%",
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
			pvcGroups: []pvcchonkerv1alpha1.PVCGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-group",
						Namespace: "default",
					},
					Spec: pvcchonkerv1alpha1.PVCGroupSpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						Template: pvcchonkerv1alpha1.PVCGroupTemplate{
							Enabled:   boolPtr(true),
							Threshold: stringPtr("80%"),
						},
					},
				},
			},
			expectPatch: true,
			expectedKeys: []string{
				"pvc-chonker.io/group",
				"pvc-chonker.io/enabled",
			},
		},
		{
			name: "PVC disabled via annotation",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "test"},
					Annotations: map[string]string{
						"pvc-chonker.io/enabled": "false",
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
			pvcGroups: []pvcchonkerv1alpha1.PVCGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-group",
						Namespace: "default",
					},
					Spec: pvcchonkerv1alpha1.PVCGroupSpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						Template: pvcchonkerv1alpha1.PVCGroupTemplate{
							Threshold: stringPtr("80%"),
						},
					},
				},
			},
			expectPatch: false,
		},
		{
			name: "No matching group",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "other"},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("100Gi"),
						},
					},
				},
			},
			pvcGroups: []pvcchonkerv1alpha1.PVCGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-group",
						Namespace: "default",
					},
					Spec: pvcchonkerv1alpha1.PVCGroupSpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						Template: pvcchonkerv1alpha1.PVCGroupTemplate{
							Threshold: stringPtr("80%"),
						},
					},
				},
			},
			expectPatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []runtime.Object{}
			for i := range tt.pvcGroups {
				objs = append(objs, &tt.pvcGroups[i])
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
			decoder := admission.NewDecoder(scheme)

			mutator := &PVCGroupMutator{
				Client:  client,
				decoder: &decoder,
			}

			pvcBytes, err := json.Marshal(tt.pvc)
			require.NoError(t, err)

			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{
						Kind: "PersistentVolumeClaim",
					},
					Object: runtime.RawExtension{
						Raw: pvcBytes,
					},
				},
			}

			resp := mutator.Handle(context.Background(), req)
			assert.True(t, resp.Allowed)

			if tt.expectPatch {
				assert.NotNil(t, resp.Patch)

				var patches []map[string]interface{}
				err = json.Unmarshal(resp.Patch, &patches)
				require.NoError(t, err)

				// Check that expected annotations are being added
				patchedKeys := make(map[string]bool)
				for _, patch := range patches {
					if path, ok := patch["path"].(string); ok {
						// Convert JSON patch path to annotation key
						if len(path) > len("/metadata/annotations/") &&
							path[:len("/metadata/annotations/")] == "/metadata/annotations/" {
							key := path[len("/metadata/annotations/"):]
							key = jsonPathToAnnotationKey(key)
							patchedKeys[key] = true
						}
					}
				}

				for _, expectedKey := range tt.expectedKeys {
					assert.True(t, patchedKeys[expectedKey], "expected key %s not found in patches", expectedKey)
				}
			} else {
				assert.Nil(t, resp.Patch)
			}
		})
	}
}

func TestPVCGroupMutator_HandleNonPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	decoder := admission.NewDecoder(scheme)

	mutator := &PVCGroupMutator{
		Client:  client,
		decoder: &decoder,
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Kind: metav1.GroupVersionKind{
				Kind: "Pod",
			},
		},
	}

	resp := mutator.Handle(context.Background(), req)
	assert.True(t, resp.Allowed)
	assert.Nil(t, resp.Patch)
}

func jsonPathToAnnotationKey(jsonPath string) string {
	// Convert JSON patch path encoding back to annotation key
	// e.g., "pvc-chonker.io~1enabled" -> "pvc-chonker.io/enabled"
	key := jsonPath
	key = string([]rune(key)) // Handle any unicode issues
	// Replace ~1 with /
	result := ""
	for i := 0; i < len(key); i++ {
		if i < len(key)-1 && key[i] == '~' && key[i+1] == '1' {
			result += "/"
			i++ // Skip the '1'
		} else {
			result += string(key[i])
		}
	}
	return result
}

func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}
