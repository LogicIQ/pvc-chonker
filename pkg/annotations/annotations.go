package annotations

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	AnnotationEnabled       = "pvc-chonker.io/enabled"
	AnnotationThreshold     = "pvc-chonker.io/threshold"
	AnnotationIncrease      = "pvc-chonker.io/increase"
	AnnotationMaxSize       = "pvc-chonker.io/max-size"
	AnnotationCooldown      = "pvc-chonker.io/cooldown"
	AnnotationMinScaleUp    = "pvc-chonker.io/min-scale-up"
	AnnotationLastExpansion = "pvc-chonker.io/last-expansion"

	DefaultThreshold  = 80.0
	DefaultIncrease   = "10%"
	DefaultCooldown   = 15 * time.Minute
	DefaultMinScaleUp = 1 * 1024 * 1024 * 1024
)

type GlobalConfig struct {
	Threshold  float64
	Increase   string
	Cooldown   time.Duration
	MinScaleUp resource.Quantity
	MaxSize    resource.Quantity
}

type PVCConfig struct {
	Enabled       bool
	Threshold     float64
	Increase      string
	MaxSize       resource.Quantity
	Cooldown      time.Duration
	MinScaleUp    resource.Quantity
	LastExpansion *time.Time
}

func ParsePVCAnnotations(pvc *corev1.PersistentVolumeClaim, global *GlobalConfig) (*PVCConfig, error) {
	if pvc.Annotations == nil {
		return nil, fmt.Errorf("no annotations found")
	}

	config := &PVCConfig{}

	enabled, exists := pvc.Annotations[AnnotationEnabled]
	if !exists {
		return nil, fmt.Errorf("pvc-chonker not enabled")
	}
	config.Enabled = strings.ToLower(enabled) == "true"
	if !config.Enabled {
		return nil, fmt.Errorf("pvc-chonker disabled")
	}

	if threshold, exists := pvc.Annotations[AnnotationThreshold]; exists {
		t, err := parsePercentage(threshold)
		if err != nil {
			return nil, fmt.Errorf("invalid threshold: %w", err)
		}
		config.Threshold = t
	} else {
		config.Threshold = global.Threshold
	}

	if increase, exists := pvc.Annotations[AnnotationIncrease]; exists {
		config.Increase = increase
	} else {
		config.Increase = global.Increase
	}

	if maxSize, exists := pvc.Annotations[AnnotationMaxSize]; exists {
		size, err := resource.ParseQuantity(maxSize)
		if err != nil {
			return nil, fmt.Errorf("invalid max-size: %w", err)
		}
		config.MaxSize = size
	} else {
		config.MaxSize = global.MaxSize
	}

	if cooldown, exists := pvc.Annotations[AnnotationCooldown]; exists {
		duration, err := time.ParseDuration(cooldown)
		if err != nil {
			return nil, fmt.Errorf("invalid cooldown: %w", err)
		}
		config.Cooldown = duration
	} else {
		config.Cooldown = global.Cooldown
	}

	if minScaleUp, exists := pvc.Annotations[AnnotationMinScaleUp]; exists {
		size, err := resource.ParseQuantity(minScaleUp)
		if err != nil {
			return nil, fmt.Errorf("invalid min-scale-up: %w", err)
		}
		config.MinScaleUp = size
	} else {
		config.MinScaleUp = global.MinScaleUp
	}

	if lastExpansion, exists := pvc.Annotations[AnnotationLastExpansion]; exists {
		t, err := time.Parse(time.RFC3339, lastExpansion)
		if err != nil {
			return nil, fmt.Errorf("invalid last-expansion time: %w", err)
		}
		config.LastExpansion = &t
	}

	return config, nil
}

func (c *PVCConfig) CalculateNewSize(currentSize resource.Quantity) (resource.Quantity, error) {
	increase := strings.TrimSpace(c.Increase)
	
	var increaseBytes int64
	if strings.HasSuffix(increase, "%") {
		percentStr := strings.TrimSuffix(increase, "%")
		percent, err := strconv.ParseFloat(percentStr, 64)
		if err != nil {
			return resource.Quantity{}, fmt.Errorf("invalid percentage: %s", increase)
		}
		currentBytes := currentSize.Value()
		increaseBytes = int64(float64(currentBytes) * percent / 100)
	} else {
		increaseSize, err := resource.ParseQuantity(increase)
		if err != nil {
			return resource.Quantity{}, fmt.Errorf("invalid size: %s", increase)
		}
		increaseBytes = increaseSize.Value()
	}
	
	minScaleUpBytes := c.MinScaleUp.Value()
	if increaseBytes < minScaleUpBytes {
		increaseBytes = minScaleUpBytes
	}
	
	currentBytes := currentSize.Value()
	newBytes := currentBytes + increaseBytes
	
	gibBoundary := int64(1024 * 1024 * 1024)
	roundedBytes := ((newBytes + gibBoundary - 1) / gibBoundary) * gibBoundary
	
	newSize := resource.NewQuantity(roundedBytes, resource.BinarySI)
	return *newSize, nil
}

func (c *PVCConfig) IsInCooldown() bool {
	if c.LastExpansion == nil {
		return false
	}
	return time.Since(*c.LastExpansion) < c.Cooldown
}

func (c *PVCConfig) ExceedsMaxSize(newSize resource.Quantity) bool {
	if c.MaxSize.IsZero() {
		return false
	}
	return newSize.Cmp(c.MaxSize) > 0
}

func IsPvcResizing(pvc *corev1.PersistentVolumeClaim) bool {
	return len(pvc.Status.Conditions) > 0
}

func UpdateLastExpansion(pvc *corev1.PersistentVolumeClaim) {
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Annotations[AnnotationLastExpansion] = time.Now().Format(time.RFC3339)
}

func parsePercentage(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "%") {
		return 0, fmt.Errorf("percentage must end with %%")
	}
	
	percentStr := strings.TrimSuffix(s, "%")
	percent, err := strconv.ParseFloat(percentStr, 64)
	if err != nil {
		return 0, err
	}
	
	if percent < 0 || percent > 100 {
		return 0, fmt.Errorf("percentage must be between 0 and 100")
	}
	
	return percent, nil
}

func NewGlobalConfig(threshold float64, increase string, cooldown time.Duration, minScaleUp resource.Quantity, maxSize resource.Quantity) *GlobalConfig {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	if increase == "" {
		increase = DefaultIncrease
	}
	if cooldown <= 0 {
		cooldown = DefaultCooldown
	}
	if minScaleUp.IsZero() {
		minScaleUp = *resource.NewQuantity(DefaultMinScaleUp, resource.BinarySI)
	}
	
	return &GlobalConfig{
		Threshold:  threshold,
		Increase:   increase,
		Cooldown:   cooldown,
		MinScaleUp: minScaleUp,
		MaxSize:    maxSize,
	}
}