package annotations

import (
	"context"
	"testing"
	"time"

	pvcchonkerv1alpha1 "github.com/logicIQ/pvc-chonker/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPolicyResolver_ResolvePVCConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pvcchonkerv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	globalConfig := &GlobalConfig{
		Threshold:       80.0,
		InodesThreshold: 80.0,
		Increase:        "10%",
		Cooldown:        15 * time.Minute,
		MinScaleUp:      resource.MustParse("1Gi"),
		MaxSize:         resource.MustParse("1000Gi"),
	}

	tests := []struct {
		name        string
		pvc         *corev1.PersistentVolumeClaim
		policies    []pvcchonkerv1alpha1.PVCPolicy
		expectError bool
		expected    *PVCConfig
	}{
		{
			name: "annotation takes precedence over policy",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "database"},
					Annotations: map[string]string{
						AnnotationEnabled:   "true",
						AnnotationThreshold: "90%",
					},
				},
			},
			policies: []pvcchonkerv1alpha1.PVCPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-policy",
						Namespace: "default",
					},
					Spec: pvcchonkerv1alpha1.PVCPolicySpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "database"},
						},
						Template: pvcchonkerv1alpha1.PVCPolicyTemplate{
							Threshold: ptr.To("85%"),
						},
					},
				},
			},
			expected: &PVCConfig{
				Enabled:         true,
				Threshold:       90.0, // From annotation, not policy
				InodesThreshold: 80.0,
				Increase:        "10%",
				Cooldown:        15 * time.Minute,
				MinScaleUp:      resource.MustParse("1Gi"),
				MaxSize:         resource.MustParse("1000Gi"),
			},
		},
		{
			name: "policy applies when no annotations",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "database"},
				},
			},
			policies: []pvcchonkerv1alpha1.PVCPolicy{
				{
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
							Increase:  ptr.To("25%"),
						},
					},
				},
			},
			expected: &PVCConfig{
				Enabled:         true,
				Threshold:       85.0, // From policy
				InodesThreshold: 80.0,
				Increase:        "25%", // From policy
				Cooldown:        15 * time.Minute,
				MinScaleUp:      resource.MustParse("1Gi"),
				MaxSize:         resource.MustParse("1000Gi"),
			},
		},
		{
			name: "no matching policy returns error",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "web"},
				},
			},
			policies: []pvcchonkerv1alpha1.PVCPolicy{
				{
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
			},
			expectError: true,
		},
		{
			name: "policy with all fields",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"tier": "production"},
				},
			},
			policies: []pvcchonkerv1alpha1.PVCPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "full-policy",
						Namespace: "default",
					},
					Spec: pvcchonkerv1alpha1.PVCPolicySpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"tier": "production"},
						},
						Template: pvcchonkerv1alpha1.PVCPolicyTemplate{
							Enabled:         ptr.To(true),
							Threshold:       ptr.To("75%"),
							InodesThreshold: ptr.To("85%"),
							Increase:        ptr.To("50Gi"),
							MaxSize:         ptr.To(resource.MustParse("2000Gi")),
							MinScaleUp:      ptr.To(resource.MustParse("10Gi")),
							Cooldown:        ptr.To(metav1.Duration{Duration: 30 * time.Minute}),
						},
					},
				},
			},
			expected: &PVCConfig{
				Enabled:         true,
				Threshold:       75.0,
				InodesThreshold: 85.0,
				Increase:        "50Gi",
				MaxSize:         resource.MustParse("2000Gi"),
				MinScaleUp:      resource.MustParse("10Gi"),
				Cooldown:        30 * time.Minute,
			},
		},
		{
			name: "disabled annotation overrides policy",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "database"},
					Annotations: map[string]string{
						AnnotationEnabled: "false",
					},
				},
			},
			policies: []pvcchonkerv1alpha1.PVCPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-policy",
						Namespace: "default",
					},
					Spec: pvcchonkerv1alpha1.PVCPolicySpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "database"},
						},
						Template: pvcchonkerv1alpha1.PVCPolicyTemplate{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			expected: &PVCConfig{
				Enabled:         false, // Disabled by annotation
				Threshold:       80.0,
				InodesThreshold: 80.0,
				Increase:        "10%",
				Cooldown:        15 * time.Minute,
				MinScaleUp:      resource.MustParse("1Gi"),
				MaxSize:         resource.MustParse("1000Gi"),
			},
		},
		{
			name: "namespace isolation - policy in different namespace",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "database"},
				},
			},
			policies: []pvcchonkerv1alpha1.PVCPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-policy",
						Namespace: "other", // Different namespace
					},
					Spec: pvcchonkerv1alpha1.PVCPolicySpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "database"},
						},
						Template: pvcchonkerv1alpha1.PVCPolicyTemplate{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			expectError: true, // No policy in same namespace
		},
		{
			name: "first matching policy wins",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "database", "tier": "production"},
				},
			},
			policies: []pvcchonkerv1alpha1.PVCPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "first-policy",
						Namespace: "default",
					},
					Spec: pvcchonkerv1alpha1.PVCPolicySpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "database"},
						},
						Template: pvcchonkerv1alpha1.PVCPolicyTemplate{
							Enabled:   ptr.To(true),
							Threshold: ptr.To("70%"),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "second-policy",
						Namespace: "default",
					},
					Spec: pvcchonkerv1alpha1.PVCPolicySpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"tier": "production"},
						},
						Template: pvcchonkerv1alpha1.PVCPolicyTemplate{
							Enabled:   ptr.To(true),
							Threshold: ptr.To("90%"),
						},
					},
				},
			},
			expected: &PVCConfig{
				Enabled:         true,
				Threshold:       70.0, // From first matching policy
				InodesThreshold: 80.0,
				Increase:        "10%",
				Cooldown:        15 * time.Minute,
				MinScaleUp:      resource.MustParse("1Gi"),
				MaxSize:         resource.MustParse("1000Gi"),
			},
		},
		{
			name: "policy with disabled template",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
					Labels:    map[string]string{"app": "database"},
				},
			},
			policies: []pvcchonkerv1alpha1.PVCPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "disabled-policy",
						Namespace: "default",
					},
					Spec: pvcchonkerv1alpha1.PVCPolicySpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "database"},
						},
						Template: pvcchonkerv1alpha1.PVCPolicyTemplate{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			expected: &PVCConfig{
				Enabled:         false, // Disabled by policy
				Threshold:       80.0,
				InodesThreshold: 80.0,
				Increase:        "10%",
				Cooldown:        15 * time.Minute,
				MinScaleUp:      resource.MustParse("1Gi"),
				MaxSize:         resource.MustParse("1000Gi"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []runtime.Object{tt.pvc}
			for i := range tt.policies {
				objs = append(objs, &tt.policies[i])
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				Build()

			resolver := NewPolicyResolver(client)
			config, err := resolver.ResolvePVCConfig(context.TODO(), tt.pvc, globalConfig)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if config.Enabled != tt.expected.Enabled {
				t.Errorf("expected Enabled=%v, got %v", tt.expected.Enabled, config.Enabled)
			}
			if config.Threshold != tt.expected.Threshold {
				t.Errorf("expected Threshold=%v, got %v", tt.expected.Threshold, config.Threshold)
			}
			if config.InodesThreshold != tt.expected.InodesThreshold {
				t.Errorf("expected InodesThreshold=%v, got %v", tt.expected.InodesThreshold, config.InodesThreshold)
			}
			if config.Increase != tt.expected.Increase {
				t.Errorf("expected Increase=%v, got %v", tt.expected.Increase, config.Increase)
			}
			if !config.MaxSize.Equal(tt.expected.MaxSize) {
				t.Errorf("expected MaxSize=%v, got %v", tt.expected.MaxSize, config.MaxSize)
			}
			if !config.MinScaleUp.Equal(tt.expected.MinScaleUp) {
				t.Errorf("expected MinScaleUp=%v, got %v", tt.expected.MinScaleUp, config.MinScaleUp)
			}
			if config.Cooldown != tt.expected.Cooldown {
				t.Errorf("expected Cooldown=%v, got %v", tt.expected.Cooldown, config.Cooldown)
			}
		})
	}
}

func TestPolicyResolver_buildConfigFromPolicy(t *testing.T) {
	globalConfig := &GlobalConfig{
		Threshold:       80.0,
		InodesThreshold: 80.0,
		Increase:        "10%",
		Cooldown:        15 * time.Minute,
		MinScaleUp:      resource.MustParse("1Gi"),
		MaxSize:         resource.MustParse("1000Gi"),
	}

	resolver := &PolicyResolver{}

	// Test with nil values (should use global config)
	policy := &pvcchonkerv1alpha1.PVCPolicy{
		Spec: pvcchonkerv1alpha1.PVCPolicySpec{
			Template: pvcchonkerv1alpha1.PVCPolicyTemplate{},
		},
	}

	config := resolver.buildConfigFromPolicy(policy, globalConfig)

	if config.Enabled != true {
		t.Errorf("expected Enabled=true (default), got %v", config.Enabled)
	}
	if config.Threshold != 80.0 {
		t.Errorf("expected Threshold=80.0 (from global), got %v", config.Threshold)
	}
	if config.InodesThreshold != 80.0 {
		t.Errorf("expected InodesThreshold=80.0 (from global), got %v", config.InodesThreshold)
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test getBoolValue
	if getBoolValue(nil, true) != true {
		t.Error("getBoolValue with nil should return default")
	}
	if getBoolValue(&[]bool{false}[0], true) != false {
		t.Error("getBoolValue with value should return value")
	}

	// Test getFloat64Value
	if getFloat64Value(nil, 80.0) != 80.0 {
		t.Error("getFloat64Value with nil should return default")
	}
	if getFloat64Value(&[]float64{90.0}[0], 80.0) != 90.0 {
		t.Error("getFloat64Value with value should return value")
	}

	// Test getStringValue
	if getStringValue(nil, "default") != "default" {
		t.Error("getStringValue with nil should return default")
	}
	if getStringValue(&[]string{"custom"}[0], "default") != "custom" {
		t.Error("getStringValue with value should return value")
	}

	// Test getQuantityValue
	defaultQty := resource.MustParse("1Gi")
	customQty := resource.MustParse("2Gi")
	if !getQuantityValue(nil, defaultQty).Equal(defaultQty) {
		t.Error("getQuantityValue with nil should return default")
	}
	if !getQuantityValue(&customQty, defaultQty).Equal(customQty) {
		t.Error("getQuantityValue with value should return value")
	}

	// Test getDurationValue
	defaultDur := 15 * time.Minute
	customDur := metav1.Duration{Duration: 30 * time.Minute}
	if getDurationValue(nil, defaultDur) != defaultDur {
		t.Error("getDurationValue with nil should return default")
	}
	if getDurationValue(&customDur, defaultDur) != customDur.Duration {
		t.Error("getDurationValue with value should return value")
	}
}
