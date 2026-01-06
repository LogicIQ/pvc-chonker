package e2e

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createInodePVC() (*corev1.PersistentVolumeClaim, error) {
	storageQty, err := resource.ParseQuantity("1Gi")
	if err != nil {
		return nil, err
	}
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
					corev1.ResourceStorage: storageQty,
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}, nil
}

func createMaxSizePVC() (*corev1.PersistentVolumeClaim, error) {
	storageQty, err := resource.ParseQuantity("1Gi")
	if err != nil {
		return nil, err
	}
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
					corev1.ResourceStorage: storageQty,
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}, nil
}

func createCooldownPVC() (*corev1.PersistentVolumeClaim, error) {
	storageQty, err := resource.ParseQuantity("1Gi")
	if err != nil {
		return nil, err
	}
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
					corev1.ResourceStorage: storageQty,
				},
			},
			StorageClassName: stringPtr("expandable-local"),
		},
	}, nil
}

