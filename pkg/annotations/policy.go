package annotations

import (
	"context"
	"strings"
	"time"

	pvcchonkerv1alpha1 "github.com/logicIQ/pvc-chonker/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PolicyResolver struct {
	client.Client
}

func NewPolicyResolver(client client.Client) *PolicyResolver {
	return &PolicyResolver{Client: client}
}

func (r *PolicyResolver) ResolvePVCConfig(ctx context.Context, pvc *corev1.PersistentVolumeClaim, globalConfig *GlobalConfig) (*PVCConfig, error) {
	if pvc.Annotations != nil {
		if enabled, exists := pvc.Annotations[AnnotationEnabled]; exists {
			if strings.ToLower(enabled) == "false" {
				return &PVCConfig{
					Enabled:         false,
					Threshold:       globalConfig.Threshold,
					InodesThreshold: globalConfig.InodesThreshold,
					Increase:        globalConfig.Increase,
					MaxSize:         globalConfig.MaxSize,
					MinScaleUp:      globalConfig.MinScaleUp,
					Cooldown:        globalConfig.Cooldown,
				}, nil
			}
		}
	}

	config, err := ParsePVCAnnotations(pvc, globalConfig)
	if err == nil {
		return config, nil
	}

	var policies pvcchonkerv1alpha1.PVCPolicyList
	if err := r.List(ctx, &policies, client.InNamespace(pvc.Namespace)); err != nil {
		return nil, err
	}

	for _, policy := range policies.Items {
		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.Selector)
		if err != nil {
			return nil, err
		}

		if selector.Matches(labels.Set(pvc.Labels)) {
			return r.buildConfigFromPolicy(&policy, globalConfig), nil
		}
	}

	return nil, ErrPVCNotManaged
}

func (r *PolicyResolver) buildConfigFromPolicy(policy *pvcchonkerv1alpha1.PVCPolicy, globalConfig *GlobalConfig) *PVCConfig {
	config := &PVCConfig{
		Enabled:         getBoolValue(policy.Spec.Template.Enabled, true),
		Threshold:       getThresholdValue(policy.Spec.Template.Threshold, globalConfig.Threshold),
		InodesThreshold: getThresholdValue(policy.Spec.Template.InodesThreshold, globalConfig.InodesThreshold),
		Increase:        getStringValue(policy.Spec.Template.Increase, globalConfig.Increase),
		MaxSize:         getQuantityValue(policy.Spec.Template.MaxSize, globalConfig.MaxSize),
		MinScaleUp:      getQuantityValue(policy.Spec.Template.MinScaleUp, globalConfig.MinScaleUp),
		Cooldown:        getDurationValue(policy.Spec.Template.Cooldown, globalConfig.Cooldown),
	}
	return config
}

func getBoolValue(ptr *bool, defaultVal bool) bool {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

func getThresholdValue(ptr *string, defaultVal float64) float64 {
	if ptr != nil {
		if val, err := parsePercentage(*ptr); err == nil {
			return val
		}
	}
	return defaultVal
}

func getStringValue(ptr *string, defaultVal string) string {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

func getQuantityValue(ptr *resource.Quantity, defaultVal resource.Quantity) resource.Quantity {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

func getDurationValue(ptr *metav1.Duration, defaultVal time.Duration) time.Duration {
	if ptr != nil {
		return ptr.Duration
	}
	return defaultVal
}
