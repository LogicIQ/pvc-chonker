package annotations

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParsePVCAnnotations_InodesThreshold(t *testing.T) {
	global := NewGlobalConfig(0, 0, "", 0, resource.Quantity{}, resource.Quantity{})

	tests := []struct {
		name        string
		annotations map[string]string
		expected    float64
		expectError bool
	}{
		{
			name: "custom inodes threshold",
			annotations: map[string]string{
				AnnotationEnabled:         "true",
				AnnotationInodesThreshold: "90%",
			},
			expected: 90.0,
		},
		{
			name: "default inodes threshold",
			annotations: map[string]string{
				AnnotationEnabled: "true",
			},
			expected: DefaultInodesThreshold,
		},
		{
			name: "invalid inodes threshold",
			annotations: map[string]string{
				AnnotationEnabled:         "true",
				AnnotationInodesThreshold: "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}

			config, err := ParsePVCAnnotations(pvc, global)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if config.InodesThreshold != tt.expected {
				t.Errorf("expected InodesThreshold %f, got %f", tt.expected, config.InodesThreshold)
			}
		})
	}
}

func TestParsePVCAnnotations_SeparateThresholds(t *testing.T) {
	global := NewGlobalConfig(0, 0, "", 0, resource.Quantity{}, resource.Quantity{})

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				AnnotationEnabled:         "true",
				AnnotationThreshold:       "85%",
				AnnotationInodesThreshold: "95%",
			},
		},
	}

	config, err := ParsePVCAnnotations(pvc, global)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Threshold != 85.0 {
		t.Errorf("expected Threshold 85.0, got %f", config.Threshold)
	}

	if config.InodesThreshold != 95.0 {
		t.Errorf("expected InodesThreshold 95.0, got %f", config.InodesThreshold)
	}
}

func TestNewGlobalConfig_InodesThreshold(t *testing.T) {
	config := NewGlobalConfig(0, 0, "", 0, resource.Quantity{}, resource.Quantity{})

	if config.InodesThreshold != DefaultInodesThreshold {
		t.Errorf("expected InodesThreshold %f, got %f", DefaultInodesThreshold, config.InodesThreshold)
	}
}
