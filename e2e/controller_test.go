package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	"github.com/logicIQ/pvc-chonker/internal/controller"
	"github.com/logicIQ/pvc-chonker/pkg/annotations"
	"github.com/logicIQ/pvc-chonker/pkg/kubelet"
)

var _ = Describe("Controller Integration", func() {
	var reconciler *controller.PersistentVolumeClaimReconciler
	var globalConfig *annotations.GlobalConfig
	var mockMetricsCollector *MockMetricsCollector

	BeforeEach(func() {
		globalConfig = annotations.NewGlobalConfig(
			80.0,                                                        // threshold
			"20%",                                                       // increase
			5*time.Second,                                               // short cooldown for testing
			*resource.NewQuantity(1024*1024*1024, resource.BinarySI),   // minScaleUp (1Gi)
			*resource.NewQuantity(100*1024*1024*1024, resource.BinarySI), // maxSize (100Gi)
		)

		mockMetricsCollector = NewMockMetricsCollector()

		reconciler = &controller.PersistentVolumeClaimReconciler{
			Client:           k8sClient,
			GlobalConfig:     globalConfig,
			MetricsCollector: mockMetricsCollector,
			WatchInterval:    1 * time.Second,
			EventRecorder:    &record.FakeRecorder{},
			DryRun:           false,
		}
	})

	Context("PVC Eligibility", func() {
		It("should identify eligible PVCs", func() {
			pvc := createTestPVC("eligible-pvc", "pvc-chonker-test", "10Gi", 
				map[string]string{
					annotations.AnnotationEnabled: "true",
				}, 
				storageClassPtr("expandable-storage"))

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			// Test eligibility
			eligible := reconciler.IsPVCEligible(pvc)
			Expect(eligible).To(BeTrue())

			// Cleanup
			err = k8sClient.Delete(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject block volume PVCs", func() {
			blockMode := corev1.PersistentVolumeBlock
			pvc := createTestPVC("block-pvc", "pvc-chonker-test", "10Gi", 
				map[string]string{
					annotations.AnnotationEnabled: "true",
				}, 
				storageClassPtr("expandable-storage"))
			pvc.Spec.VolumeMode = &blockMode

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			eligible := reconciler.IsPVCEligible(pvc)
			Expect(eligible).To(BeFalse())

			// Cleanup
			err = k8sClient.Delete(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject unbound PVCs", func() {
			pvc := createTestPVC("unbound-pvc", "pvc-chonker-test", "10Gi", 
				map[string]string{
					annotations.AnnotationEnabled: "true",
				}, 
				storageClassPtr("expandable-storage"))
			pvc.Status.Phase = corev1.ClaimPending

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			eligible := reconciler.IsPVCEligible(pvc)
			Expect(eligible).To(BeFalse())

			// Cleanup
			err = k8sClient.Delete(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Storage Class Validation", func() {
		It("should allow expandable storage classes", func() {
			pvc := createTestPVC("expandable-test", "pvc-chonker-test", "10Gi", 
				map[string]string{
					annotations.AnnotationEnabled: "true",
				}, 
				storageClassPtr("expandable-storage"))

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			expandable := reconciler.IsStorageClassExpandable(ctx, pvc)
			Expect(expandable).To(BeTrue())

			// Cleanup
			err = k8sClient.Delete(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject non-expandable storage classes", func() {
			pvc := createTestPVC("non-expandable-test", "pvc-chonker-test", "10Gi", 
				map[string]string{
					annotations.AnnotationEnabled: "true",
				}, 
				storageClassPtr("non-expandable-storage"))

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			expandable := reconciler.IsStorageClassExpandable(ctx, pvc)
			Expect(expandable).To(BeFalse())

			// Cleanup
			err = k8sClient.Delete(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("PVC Expansion", func() {
		It("should expand PVC when threshold is reached", func() {
			pvc := createTestPVC("expansion-test", "pvc-chonker-test", "10Gi", 
				map[string]string{
					annotations.AnnotationEnabled:   "true",
					annotations.AnnotationThreshold: "80%",
					annotations.AnnotationIncrease:  "20%",
				}, 
				storageClassPtr("expandable-storage"))

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			// Mock high usage (90% of 10Gi)
			mockMetricsCollector.SetVolumeMetrics("pvc-chonker-test/expansion-test", &kubelet.VolumeMetrics{
				CapacityBytes:  10 * 1024 * 1024 * 1024, // 10Gi
				AvailableBytes: 1 * 1024 * 1024 * 1024,  // 1Gi available
				UsedBytes:      9 * 1024 * 1024 * 1024,  // 9Gi used
				UsagePercent:   90.0,                     // 90% usage
			})

			// Parse annotations to get config
			config, err := annotations.ParsePVCAnnotations(pvc, globalConfig)
			Expect(err).NotTo(HaveOccurred())

			// Test expansion
			err = reconciler.ExpandPVC(ctx, pvc, config)
			Expect(err).NotTo(HaveOccurred())

			// Verify PVC was updated
			var updatedPVC corev1.PersistentVolumeClaim
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "expansion-test",
				Namespace: "pvc-chonker-test",
			}, &updatedPVC)
			Expect(err).NotTo(HaveOccurred())

			// Should be expanded from 10Gi to 12Gi (10Gi + 20%)
			expectedSize := resource.MustParse("12Gi")
			actualSize := updatedPVC.Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(actualSize.Cmp(expectedSize)).To(Equal(0))

			// Cleanup
			err = k8sClient.Delete(ctx, &updatedPVC)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should respect maximum size limits", func() {
			pvc := createTestPVC("max-size-test", "pvc-chonker-test", "10Gi", 
				map[string]string{
					annotations.AnnotationEnabled:   "true",
					annotations.AnnotationThreshold: "80%",
					annotations.AnnotationIncrease:  "50%", // Would expand to 15Gi
					annotations.AnnotationMaxSize:   "12Gi", // But max is 12Gi
				}, 
				storageClassPtr("expandable-storage"))

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			config, err := annotations.ParsePVCAnnotations(pvc, globalConfig)
			Expect(err).NotTo(HaveOccurred())

			// Should fail due to max size limit
			err = reconciler.ExpandPVC(ctx, pvc, config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exceeds max size"))

			// Cleanup
			err = k8sClient.Delete(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should work in dry run mode", func() {
			reconciler.DryRun = true

			pvc := createTestPVC("dry-run-test", "pvc-chonker-test", "10Gi", 
				map[string]string{
					annotations.AnnotationEnabled:   "true",
					annotations.AnnotationThreshold: "80%",
					annotations.AnnotationIncrease:  "20%",
				}, 
				storageClassPtr("expandable-storage"))

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			config, err := annotations.ParsePVCAnnotations(pvc, globalConfig)
			Expect(err).NotTo(HaveOccurred())

			// Should succeed without error in dry run
			err = reconciler.ExpandPVC(ctx, pvc, config)
			Expect(err).NotTo(HaveOccurred())

			// Verify PVC was NOT actually updated
			var unchangedPVC corev1.PersistentVolumeClaim
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "dry-run-test",
				Namespace: "pvc-chonker-test",
			}, &unchangedPVC)
			Expect(err).NotTo(HaveOccurred())

			// Size should remain unchanged
			originalSize := resource.MustParse("10Gi")
			actualSize := unchangedPVC.Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(actualSize.Cmp(originalSize)).To(Equal(0))

			// Cleanup
			err = k8sClient.Delete(ctx, &unchangedPVC)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Cooldown Behavior", func() {
		It("should respect cooldown periods", func() {
			now := time.Now()
			pvc := createTestPVC("cooldown-test", "pvc-chonker-test", "10Gi", 
				map[string]string{
					annotations.AnnotationEnabled:       "true",
					annotations.AnnotationThreshold:     "80%",
					annotations.AnnotationIncrease:      "20%",
					annotations.AnnotationCooldown:      "1h",
					annotations.AnnotationLastExpansion: now.Format(time.RFC3339),
				}, 
				storageClassPtr("expandable-storage"))

			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())

			config, err := annotations.ParsePVCAnnotations(pvc, globalConfig)
			Expect(err).NotTo(HaveOccurred())

			// Should be in cooldown
			Expect(config.IsInCooldown()).To(BeTrue())

			// Cleanup
			err = k8sClient.Delete(ctx, pvc)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

// MockMetricsCollector for testing
type MockMetricsCollector struct {
	metrics map[string]*kubelet.VolumeMetrics
}

func NewMockMetricsCollector() *MockMetricsCollector {
	return &MockMetricsCollector{
		metrics: make(map[string]*kubelet.VolumeMetrics),
	}
}

func (m *MockMetricsCollector) SetVolumeMetrics(key string, metrics *kubelet.VolumeMetrics) {
	m.metrics[key] = metrics
}

func (m *MockMetricsCollector) GetVolumeMetrics(ctx context.Context, namespacedName types.NamespacedName) (*kubelet.VolumeMetrics, error) {
	key := namespacedName.Namespace + "/" + namespacedName.Name
	if metrics, exists := m.metrics[key]; exists {
		return metrics, nil
	}
	return nil, ErrVolumeNotFound
}

func (m *MockMetricsCollector) GetAllVolumeMetrics(ctx context.Context) (*kubelet.MetricsCache, error) {
	cache := kubelet.NewMetricsCache()
	// Populate cache with mock data
	for key, metrics := range m.metrics {
		// This is a simplified mock - in real implementation we'd need to access private fields
		// For now, return empty cache
		_ = key
		_ = metrics
	}
	return cache, nil
}

// Add the missing error
var ErrVolumeNotFound = fmt.Errorf("volume not found")