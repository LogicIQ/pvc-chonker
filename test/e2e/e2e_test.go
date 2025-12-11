package e2e

import (
	"context"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	k8sClient   client.Client
	clientset   *kubernetes.Clientset
	ctx         context.Context
	cancel      context.CancelFunc
	clusterName = "pvc-chonker-test"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Test Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())

	cmd := exec.Command("kind", "get", "clusters")
	output, err := cmd.Output()
	if err != nil || !contains(string(output), clusterName) {
		Skip("Kind cluster not found. Run task e2e-setup first")
	}

	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	cancel()
})

var _ = Describe("End-to-End PVC Expansion", func() {
	Context("Real Kubernetes Cluster", func() {
		It("should expand PVC when storage threshold is reached", func() {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "e2e-test-pvc",
					Namespace: "default",
					Annotations: map[string]string{
						"pvc-chonker.io/enabled":   "true",
						"pvc-chonker.io/threshold": "80%",
						"pvc-chonker.io/increase":  "100%",
						"pvc-chonker.io/max-size":  "10Gi",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
					StorageClassName: stringPtr("local-path"),
				},
			}

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				var currentPVC corev1.PersistentVolumeClaim
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "e2e-test-pvc",
					Namespace: "default",
				}, &currentPVC)
				if err != nil {
					return false
				}
				return currentPVC.Status.Phase == corev1.ClaimBound
			}, 2*time.Minute, 5*time.Second).Should(BeTrue())

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "storage-filler",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "filler",
							Image:   "busybox:1.35",
							Command: []string{"/bin/sh"},
							Args:    []string{"-c", "dd if=/dev/zero of=/data/bigfile bs=1M count=800; sleep 30"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "e2e-test-pvc",
								},
							},
						},
					},
				},
			}

			err = k8sClient.Create(ctx, pod)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				var currentPVC corev1.PersistentVolumeClaim
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "e2e-test-pvc",
					Namespace: "default",
				}, &currentPVC)
				if err != nil {
					return false
				}

				requestedSize := currentPVC.Spec.Resources.Requests[corev1.ResourceStorage]
				originalSize := resource.MustParse("1Gi")

				return requestedSize.Cmp(originalSize) > 0
			}, 5*time.Minute, 10*time.Second).Should(BeTrue())

			var finalPVC corev1.PersistentVolumeClaim
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "e2e-test-pvc",
				Namespace: "default",
			}, &finalPVC)
			Expect(err).NotTo(HaveOccurred())

			finalSize := finalPVC.Spec.Resources.Requests[corev1.ResourceStorage]
			expectedSize := resource.MustParse("2Gi")
			Expect(finalSize.Cmp(expectedSize)).To(Equal(0))

			err = k8sClient.Delete(ctx, pod)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(ctx, &finalPVC)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should respect cooldown periods", func() {
			now := time.Now()
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cooldown-test-pvc",
					Namespace: "default",
					Annotations: map[string]string{
						"pvc-chonker.io/enabled":        "true",
						"pvc-chonker.io/threshold":      "50%",
						"pvc-chonker.io/increase":       "100%",
						"pvc-chonker.io/cooldown":       "5m",
						"pvc-chonker.io/last-expansion": now.Format(time.RFC3339),
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
					StorageClassName: stringPtr("local-path"),
				},
			}

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				var currentPVC corev1.PersistentVolumeClaim
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "cooldown-test-pvc",
					Namespace: "default",
				}, &currentPVC)
				if err != nil {
					return false
				}
				return currentPVC.Status.Phase == corev1.ClaimBound
			}, 2*time.Minute, 5*time.Second).Should(BeTrue())

			Consistently(func() bool {
				var currentPVC corev1.PersistentVolumeClaim
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "cooldown-test-pvc",
					Namespace: "default",
				}, &currentPVC)
				if err != nil {
					return false
				}

				currentSize := currentPVC.Spec.Resources.Requests[corev1.ResourceStorage]
				originalSize := resource.MustParse("1Gi")

				return currentSize.Cmp(originalSize) == 0
			}, 30*time.Second, 5*time.Second).Should(BeTrue())

			var finalPVC corev1.PersistentVolumeClaim
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "cooldown-test-pvc",
				Namespace: "default",
			}, &finalPVC)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(ctx, &finalPVC)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringPtr(s string) *string {
	return &s
}