package integration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/logicIQ/pvc-chonker/pkg/annotations"
)

var _ = Describe("Annotations Integration", func() {
	var globalConfig *annotations.GlobalConfig

	BeforeEach(func() {
		globalConfig = annotations.NewGlobalConfig(
			80.0,                                                        // threshold
			"10%",                                                       // increase
			15*time.Minute,                                              // cooldown
			*resource.NewQuantity(1024*1024*1024, resource.BinarySI),   // minScaleUp (1Gi)
			*resource.NewQuantity(100*1024*1024*1024, resource.BinarySI), // maxSize (100Gi)
		)
	})

	Context("PVC Annotation Parsing", func() {
		It("should parse enabled PVC with default values", func() {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "pvc-chonker-test",
					Annotations: map[string]string{
						annotations.AnnotationEnabled: "true",
					},
				},
			}

			config, err := annotations.ParsePVCAnnotations(pvc, globalConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Enabled).To(BeTrue())
			Expect(config.Threshold).To(Equal(80.0))
			Expect(config.Increase).To(Equal("10%"))
			Expect(config.Cooldown).To(Equal(15 * time.Minute))
		})

		It("should parse PVC with custom annotation values", func() {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "pvc-chonker-test",
					Annotations: map[string]string{
						annotations.AnnotationEnabled:    "true",
						annotations.AnnotationThreshold:  "85%",
						annotations.AnnotationIncrease:   "20%",
						annotations.AnnotationMaxSize:    "50Gi",
						annotations.AnnotationCooldown:   "30m",
						annotations.AnnotationMinScaleUp: "2Gi",
					},
				},
			}

			config, err := annotations.ParsePVCAnnotations(pvc, globalConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Enabled).To(BeTrue())
			Expect(config.Threshold).To(Equal(85.0))
			Expect(config.Increase).To(Equal("20%"))
			Expect(config.MaxSize).To(Equal(resource.MustParse("50Gi")))
			Expect(config.Cooldown).To(Equal(30 * time.Minute))
			Expect(config.MinScaleUp).To(Equal(resource.MustParse("2Gi")))
		})

		It("should reject disabled PVC", func() {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "pvc-chonker-test",
					Annotations: map[string]string{
						annotations.AnnotationEnabled: "false",
					},
				},
			}

			_, err := annotations.ParsePVCAnnotations(pvc, globalConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("disabled"))
		})

		It("should reject PVC without annotations", func() {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "pvc-chonker-test",
				},
			}

			_, err := annotations.ParsePVCAnnotations(pvc, globalConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no annotations"))
		})

		It("should handle invalid threshold values", func() {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "pvc-chonker-test",
					Annotations: map[string]string{
						annotations.AnnotationEnabled:   "true",
						annotations.AnnotationThreshold: "invalid",
					},
				},
			}

			_, err := annotations.ParsePVCAnnotations(pvc, globalConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid threshold"))
		})
	})

	Context("Size Calculations", func() {
		var config *annotations.PVCConfig

		BeforeEach(func() {
			config = &annotations.PVCConfig{
				Enabled:    true,
				Threshold:  80.0,
				Increase:   "20%",
				MinScaleUp: resource.MustParse("1Gi"),
				MaxSize:    resource.MustParse("100Gi"),
			}
		})

		It("should calculate percentage-based increases correctly", func() {
			currentSize := resource.MustParse("10Gi")
			newSize, err := config.CalculateNewSize(currentSize)
			Expect(err).NotTo(HaveOccurred())
			
			// 10Gi + 20% = 12Gi (already GiB aligned)
			expectedSize := resource.MustParse("12Gi")
			Expect(newSize.Cmp(expectedSize)).To(Equal(0))
		})

		It("should calculate fixed-size increases correctly", func() {
			config.Increase = "5Gi"
			currentSize := resource.MustParse("10Gi")
			newSize, err := config.CalculateNewSize(currentSize)
			Expect(err).NotTo(HaveOccurred())
			
			expectedSize := resource.MustParse("15Gi")
			Expect(newSize.Cmp(expectedSize)).To(Equal(0))
		})

		It("should enforce minimum scale-up amounts", func() {
			config.Increase = "1%"  // Very small percentage
			config.MinScaleUp = resource.MustParse("2Gi")
			currentSize := resource.MustParse("10Gi")
			newSize, err := config.CalculateNewSize(currentSize)
			Expect(err).NotTo(HaveOccurred())
			
			// Should use MinScaleUp instead of 1% of 10Gi
			expectedSize := resource.MustParse("12Gi")
			Expect(newSize.Cmp(expectedSize)).To(Equal(0))
		})

		It("should round up to GiB boundaries", func() {
			config.Increase = "1.5Gi"
			currentSize := resource.MustParse("10Gi")
			newSize, err := config.CalculateNewSize(currentSize)
			Expect(err).NotTo(HaveOccurred())
			
			// Should round up to next GiB boundary
			expectedSize := resource.MustParse("12Gi")
			Expect(newSize.Cmp(expectedSize)).To(Equal(0))
		})
	})

	Context("Cooldown Management", func() {
		It("should detect when PVC is in cooldown", func() {
			now := time.Now()
			config := &annotations.PVCConfig{
				Cooldown:      30 * time.Minute,
				LastExpansion: &now,
			}

			Expect(config.IsInCooldown()).To(BeTrue())
		})

		It("should detect when PVC is out of cooldown", func() {
			pastTime := time.Now().Add(-45 * time.Minute)
			config := &annotations.PVCConfig{
				Cooldown:      30 * time.Minute,
				LastExpansion: &pastTime,
			}

			Expect(config.IsInCooldown()).To(BeFalse())
		})

		It("should handle PVC with no previous expansion", func() {
			config := &annotations.PVCConfig{
				Cooldown:      30 * time.Minute,
				LastExpansion: nil,
			}

			Expect(config.IsInCooldown()).To(BeFalse())
		})
	})

	Context("Max Size Validation", func() {
		It("should detect when new size exceeds maximum", func() {
			config := &annotations.PVCConfig{
				MaxSize: resource.MustParse("50Gi"),
			}

			newSize := resource.MustParse("60Gi")
			Expect(config.ExceedsMaxSize(newSize)).To(BeTrue())
		})

		It("should allow sizes under maximum", func() {
			config := &annotations.PVCConfig{
				MaxSize: resource.MustParse("50Gi"),
			}

			newSize := resource.MustParse("40Gi")
			Expect(config.ExceedsMaxSize(newSize)).To(BeFalse())
		})

		It("should handle unlimited size when MaxSize is zero", func() {
			config := &annotations.PVCConfig{
				MaxSize: resource.Quantity{},
			}

			newSize := resource.MustParse("1000Gi")
			Expect(config.ExceedsMaxSize(newSize)).To(BeFalse())
		})
	})
})