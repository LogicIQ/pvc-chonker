package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	Namespace                 = "pvcchonker"
	ResizerSubsystem          = "resizer"
	KubernetesClientSubsystem = "kubernetes_client"
	KubeletClientSubsystem    = "kubelet_client"
)

// Resizer metrics
var (
	SuccessResizeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: ResizerSubsystem,
			Name:      "success_resize_total",
			Help:      "Counter that indicates how many volume expansion processing resized succeed",
		},
		[]string{"persistentvolumeclaim", "namespace"},
	)

	FailedResizeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: ResizerSubsystem,
			Name:      "failed_resize_total",
			Help:      "Counter that indicates how many volume expansion processing resizes fail",
		},
		[]string{"persistentvolumeclaim", "namespace", "reason"},
	)

	LoopSecondsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: ResizerSubsystem,
			Name:      "loop_seconds_total",
			Help:      "Counter that indicates the sum of seconds spent on volume expansion processing loops",
		},
	)

	LimitReachedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: ResizerSubsystem,
			Name:      "limit_reached_total",
			Help:      "Counter that indicates how many storage limits were reached",
		},
		[]string{"persistentvolumeclaim", "namespace"},
	)

	ThresholdReachedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: ResizerSubsystem,
			Name:      "threshold_reached_total",
			Help:      "Counter that indicates how many times threshold was reached",
		},
		[]string{"persistentvolumeclaim", "namespace"},
	)

	CooldownSkippedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: ResizerSubsystem,
			Name:      "cooldown_skipped_total",
			Help:      "Counter that indicates how many PVCs were skipped due to cooldown",
		},
		[]string{"persistentvolumeclaim", "namespace"},
	)

	ResizeInProgressTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: ResizerSubsystem,
			Name:      "resize_in_progress_total",
			Help:      "Counter that indicates how many PVCs were skipped due to ongoing resize",
		},
		[]string{"persistentvolumeclaim", "namespace"},
	)
)

// Kubernetes client metrics
var (
	KubernetesClientFailTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: KubernetesClientSubsystem,
			Name:      "fail_total",
			Help:      "Counter that indicates how many API requests to kube-api server are failed",
		},
		[]string{"operation"},
	)

	KubernetesClientRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: KubernetesClientSubsystem,
			Name:      "requests_total",
			Help:      "Counter that indicates total API requests to kube-api server",
		},
		[]string{"operation", "status"},
	)
)

// Kubelet client metrics
var (
	KubeletClientFailTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: KubeletClientSubsystem,
			Name:      "fail_total",
			Help:      "Counter that indicates how many requests to kubelet metrics are failed",
		},
	)

	KubeletClientRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: KubeletClientSubsystem,
			Name:      "requests_total",
			Help:      "Counter that indicates total requests to kubelet metrics",
		},
		[]string{"status"},
	)

	KubeletClientResponseTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: KubeletClientSubsystem,
			Name:      "response_time_seconds",
			Help:      "Histogram of kubelet client response times",
			Buckets:   prometheus.DefBuckets,
		},
	)
)

// Operational metrics
var (
	LastReconciliationTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "last_reconciliation_timestamp_seconds",
			Help:      "Timestamp of the last reconciliation loop",
		},
	)

	ReconciliationStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "reconciliation_status",
			Help:      "Status of last reconciliation (1=success, 0=failure)",
		},
		[]string{"status"},
	)

	ManagedPVCsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "managed_pvcs_total",
			Help:      "Total number of PVCs currently managed by pvc-chonker",
		},
	)

	PVCUsagePercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "pvc_usage_percent",
			Help:      "Current usage percentage of managed PVCs",
		},
		[]string{"persistentvolumeclaim", "namespace"},
	)

	PVCCapacityBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "pvc_capacity_bytes",
			Help:      "Current capacity of managed PVCs in bytes",
		},
		[]string{"persistentvolumeclaim", "namespace"},
	)

	PVCInodesUsagePercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "pvc_inodes_usage_percent",
			Help:      "Current inode usage percentage of managed PVCs",
		},
		[]string{"persistentvolumeclaim", "namespace"},
	)

	PVCInodesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "pvc_inodes_total",
			Help:      "Total inodes available in managed PVCs",
		},
		[]string{"persistentvolumeclaim", "namespace"},
	)
)

// Helper functions for consistent metric updates
func RecordSuccessfulResize(pvcName, namespace string) {
	SuccessResizeTotal.WithLabelValues(pvcName, namespace).Inc()
}

func RecordFailedResize(pvcName, namespace, reason string) {
	FailedResizeTotal.WithLabelValues(pvcName, namespace, reason).Inc()
}

func RecordThresholdReached(pvcName, namespace string) {
	ThresholdReachedTotal.WithLabelValues(pvcName, namespace).Inc()
}

func RecordLimitReached(pvcName, namespace string) {
	LimitReachedTotal.WithLabelValues(pvcName, namespace).Inc()
}

func RecordCooldownSkipped(pvcName, namespace string) {
	CooldownSkippedTotal.WithLabelValues(pvcName, namespace).Inc()
}

func RecordResizeInProgress(pvcName, namespace string) {
	ResizeInProgressTotal.WithLabelValues(pvcName, namespace).Inc()
}

func RecordKubernetesClientRequest(operation, status string) {
	KubernetesClientRequestsTotal.WithLabelValues(operation, status).Inc()
	if status == "failed" {
		KubernetesClientFailTotal.WithLabelValues(operation).Inc()
	}
}

func RecordKubeletClientRequest(status string) {
	KubeletClientRequestsTotal.WithLabelValues(status).Inc()
	if status == "failed" {
		KubeletClientFailTotal.Inc()
	}
}

func RecordLoopDuration(seconds float64) {
	LoopSecondsTotal.Add(seconds)
}

func UpdatePVCMetrics(pvcName, namespace string, usagePercent float64, capacityBytes int64) {
	PVCUsagePercent.WithLabelValues(pvcName, namespace).Set(usagePercent)
	PVCCapacityBytes.WithLabelValues(pvcName, namespace).Set(float64(capacityBytes))
}

func UpdatePVCInodesMetrics(pvcName, namespace string, inodesUsagePercent float64, inodesTotal int64) {
	if inodesTotal > 0 {
		PVCInodesUsagePercent.WithLabelValues(pvcName, namespace).Set(inodesUsagePercent)
		PVCInodesTotal.WithLabelValues(pvcName, namespace).Set(float64(inodesTotal))
	}
}

var registerOnce sync.Once

func init() {
	registerOnce.Do(func() {
		metrics.Registry.MustRegister(
			// Resizer metrics
			SuccessResizeTotal,
			FailedResizeTotal,
			LoopSecondsTotal,
			LimitReachedTotal,
			ThresholdReachedTotal,
			CooldownSkippedTotal,
			ResizeInProgressTotal,
			// Client metrics
			KubernetesClientFailTotal,
			KubernetesClientRequestsTotal,
			KubeletClientFailTotal,
			KubeletClientRequestsTotal,
			KubeletClientResponseTime,
			// Operational metrics
			LastReconciliationTime,
			ReconciliationStatus,
			ManagedPVCsTotal,
			PVCUsagePercent,
			PVCCapacityBytes,
			PVCInodesUsagePercent,
			PVCInodesTotal,
		)
	})
}
