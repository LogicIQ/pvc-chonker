package annotations

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParsePVCAnnotations(t *testing.T) {
	global := NewGlobalConfig(80.0, "10%", 15*time.Minute, *resource.NewQuantity(1024*1024*1024, resource.BinarySI), resource.Quantity{})

	tests := []struct {
		name        string
		annotations map[string]string
		wantErr     bool
		expected    *PVCConfig
	}{
		{
			name:        "no annotations",
			annotations: nil,
			wantErr:     true,
		},
		{
			name:        "not enabled",
			annotations: map[string]string{AnnotationEnabled: "false"},
			wantErr:     true,
		},
		{
			name:        "enabled with defaults",
			annotations: map[string]string{AnnotationEnabled: "true"},
			wantErr:     false,
			expected: &PVCConfig{
				Enabled:    true,
				Threshold:  80.0,
				Increase:   "10%",
				Cooldown:   15 * time.Minute,
				MinScaleUp: *resource.NewQuantity(1024*1024*1024, resource.BinarySI),
			},
		},
		{
			name: "custom values",
			annotations: map[string]string{
				AnnotationEnabled:    "true",
				AnnotationThreshold:  "85%",
				AnnotationIncrease:   "20%",
				AnnotationMaxSize:    "100Gi",
				AnnotationCooldown:   "30m",
				AnnotationMinScaleUp: "2Gi",
			},
			wantErr: false,
			expected: &PVCConfig{
				Enabled:    true,
				Threshold:  85.0,
				Increase:   "20%",
				MaxSize:    resource.MustParse("100Gi"),
				Cooldown:   30 * time.Minute,
				MinScaleUp: resource.MustParse("2Gi"),
			},
		},
		{
			name: "invalid threshold",
			annotations: map[string]string{
				AnnotationEnabled:   "true",
				AnnotationThreshold: "invalid",
			},
			wantErr: true,
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
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePVCAnnotations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && config != nil {
				if config.Enabled != tt.expected.Enabled {
					t.Errorf("Enabled = %v, want %v", config.Enabled, tt.expected.Enabled)
				}
				if config.Threshold != tt.expected.Threshold {
					t.Errorf("Threshold = %v, want %v", config.Threshold, tt.expected.Threshold)
				}
			}
		})
	}
}

func TestCalculateNewSize(t *testing.T) {
	tests := []struct {
		name        string
		config      *PVCConfig
		currentSize string
		expected    string
		wantErr     bool
	}{
		{
			name: "percentage increase",
			config: &PVCConfig{
				Increase:   "20%",
				MinScaleUp: resource.MustParse("1Gi"),
			},
			currentSize: "10Gi",
			expected:    "12Gi", // 10Gi + 20% = 12Gi, already GiB aligned
		},
		{
			name: "fixed size increase",
			config: &PVCConfig{
				Increase:   "5Gi",
				MinScaleUp: resource.MustParse("1Gi"),
			},
			currentSize: "10Gi",
			expected:    "15Gi",
		},
		{
			name: "min scale up enforced",
			config: &PVCConfig{
				Increase:   "1%",
				MinScaleUp: resource.MustParse("2Gi"),
			},
			currentSize: "10Gi",
			expected:    "12Gi", // 1% would be ~100Mi, but min is 2Gi
		},
		{
			name: "invalid percentage",
			config: &PVCConfig{
				Increase: "invalid%",
			},
			currentSize: "10Gi",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentSize := resource.MustParse(tt.currentSize)
			newSize, err := tt.config.CalculateNewSize(currentSize)

			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateNewSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				expected := resource.MustParse(tt.expected)
				if newSize.Cmp(expected) != 0 {
					t.Errorf("CalculateNewSize() = %v, want %v", newSize.String(), expected.String())
				}
			}
		})
	}
}

func TestIsInCooldown(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		lastExpansion  *time.Time
		cooldown       time.Duration
		expected       bool
	}{
		{
			name:          "no last expansion",
			lastExpansion: nil,
			cooldown:      15 * time.Minute,
			expected:      false,
		},
		{
			name:          "in cooldown",
			lastExpansion: &now,
			cooldown:      15 * time.Minute,
			expected:      true,
		},
		{
			name:          "out of cooldown",
			lastExpansion: func() *time.Time { t := now.Add(-30 * time.Minute); return &t }(),
			cooldown:      15 * time.Minute,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &PVCConfig{
				LastExpansion: tt.lastExpansion,
				Cooldown:      tt.cooldown,
			}

			result := config.IsInCooldown()
			if result != tt.expected {
				t.Errorf("IsInCooldown() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExceedsMaxSize(t *testing.T) {
	tests := []struct {
		name     string
		maxSize  string
		newSize  string
		expected bool
	}{
		{
			name:     "no max size",
			maxSize:  "",
			newSize:  "100Gi",
			expected: false,
		},
		{
			name:     "under max size",
			maxSize:  "100Gi",
			newSize:  "50Gi",
			expected: false,
		},
		{
			name:     "exceeds max size",
			maxSize:  "100Gi",
			newSize:  "150Gi",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &PVCConfig{}
			if tt.maxSize != "" {
				config.MaxSize = resource.MustParse(tt.maxSize)
			}

			newSize := resource.MustParse(tt.newSize)
			result := config.ExceedsMaxSize(newSize)
			if result != tt.expected {
				t.Errorf("ExceedsMaxSize() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParsePercentage(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		wantErr  bool
	}{
		{"80%", 80.0, false},
		{"85.5%", 85.5, false},
		{"0%", 0.0, false},
		{"100%", 100.0, false},
		{"80", 0, true},
		{"150%", 0, true},
		{"-10%", 0, true},
		{"invalid%", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parsePercentage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePercentage(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("parsePercentage(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewGlobalConfig(t *testing.T) {
	tests := []struct {
		name       string
		threshold  float64
		increase   string
		cooldown   time.Duration
		minScaleUp resource.Quantity
		maxSize    resource.Quantity
		expected   *GlobalConfig
	}{
		{
			name:       "all defaults",
			threshold:  0,
			increase:   "",
			cooldown:   0,
			minScaleUp: resource.Quantity{},
			maxSize:    resource.Quantity{},
			expected: &GlobalConfig{
				Threshold:  DefaultThreshold,
				Increase:   DefaultIncrease,
				Cooldown:   DefaultCooldown,
				MinScaleUp: *resource.NewQuantity(DefaultMinScaleUp, resource.BinarySI),
				MaxSize:    resource.Quantity{},
			},
		},
		{
			name:       "custom values",
			threshold:  85.0,
			increase:   "20%",
			cooldown:   30 * time.Minute,
			minScaleUp: resource.MustParse("2Gi"),
			maxSize:    resource.MustParse("1000Gi"),
			expected: &GlobalConfig{
				Threshold:  85.0,
				Increase:   "20%",
				Cooldown:   30 * time.Minute,
				MinScaleUp: resource.MustParse("2Gi"),
				MaxSize:    resource.MustParse("1000Gi"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewGlobalConfig(tt.threshold, tt.increase, tt.cooldown, tt.minScaleUp, tt.maxSize)
			
			if config.Threshold != tt.expected.Threshold {
				t.Errorf("Threshold = %v, want %v", config.Threshold, tt.expected.Threshold)
			}
			if config.Increase != tt.expected.Increase {
				t.Errorf("Increase = %v, want %v", config.Increase, tt.expected.Increase)
			}
			if config.Cooldown != tt.expected.Cooldown {
				t.Errorf("Cooldown = %v, want %v", config.Cooldown, tt.expected.Cooldown)
			}
		})
	}
}