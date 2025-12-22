package e2e

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pvcchonkerv1alpha1 "github.com/LogicIQ/pvc-chonker/api/v1alpha1"
)

func createInodePVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-inode-pvc",
			Annotations: map[string]string{
				"pvc-chonker.io/enabled":           "true",
				"pvc-chonker.io/threshold":         "90%",
				"pvc-chonker.io/inodes-threshold":  "5%",
				"pvc-chonker.io/increase":          "50%",
				"pvc-chonker.io/cooldown":          "1m",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}
}

func createMaxSizePVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-max-size-pvc",
			Annotations: map[string]string{
				"pvc-chonker.io/enabled":   "true",
				"pvc-chonker.io/threshold": "5%",
				"pvc-chonker.io/increase":  "100%",
				"pvc-chonker.io/max-size":  "2Gi",
				"pvc-chonker.io/cooldown":  "1m",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}
}

func createCooldownPVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cooldown-pvc",
			Annotations: map[string]string{
				"pvc-chonker.io/enabled":   "true",
				"pvc-chonker.io/threshold": "5%",
				"pvc-chonker.io/increase":  "50%",
				"pvc-chonker.io/cooldown":  "10m",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}
}

func createTestPVCPolicy() *pvcchonkerv1alpha1.PVCPolicy {
	enabled := true
	threshold := "85%"
	inodesThreshold := "90%"
	increase := "25%"
	maxSize := resource.MustParse("10Gi")
	minScaleUp := resource.MustParse("1Gi")
	cooldown := metav1.Duration{Duration: 5 * time.Minute}
	
	return &pvcchonkerv1alpha1.PVCPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: testNamespace,
		},
		Spec: pvcchonkerv1alpha1.PVCPolicySpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-policy": "enabled",
				},
			},
			Template: pvcchonkerv1alpha1.PVCPolicyTemplate{
				Enabled:         &enabled,
				Threshold:       &threshold,
				InodesThreshold: &inodesThreshold,
				Increase:        &increase,
				MaxSize:         &maxSize,
				MinScaleUp:      &minScaleUp,
				Cooldown:        &cooldown,
			},
		},
	}
}

func createPolicyManagedPVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy-pvc",
			Labels: map[string]string{
				"test-policy": "enabled", // Matches policy selector
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}
}

func createPolicyOverridePVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-override-pvc",
			Labels: map[string]string{
				"test-policy": "enabled", // Matches policy selector
			},
			Annotations: map[string]string{
				"pvc-chonker.io/enabled":   "true",
				"pvc-chonker.io/threshold": "95%", // Override policy's 85%
				"pvc-chonker.io/increase":  "50%", // Override policy's 25%
				"pvc-chonker.io/cooldown":  "1m",  // Override policy's 5m
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}
}