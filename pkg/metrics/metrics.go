package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ExpansionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pvc_chonker_expansions_total",
			Help: "Total number of PVC expansions performed",
		},
		[]string{"pvc", "namespace"},
	)

	ExpansionFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pvc_chonker_expansion_failures_total",
			Help: "Total number of failed PVC expansions",
		},
		[]string{"pvc", "namespace", "reason"},
	)

	LoopDurationSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "pvc_chonker_loop_duration_seconds",
			Help: "Time spent in reconciliation loop",
		},
	)

	ThresholdReachedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pvc_chonker_threshold_reached_total",
			Help: "Total number of times threshold was reached",
		},
		[]string{"pvc", "namespace"},
	)

	MaxSizeReachedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pvc_chonker_max_size_reached_total",
			Help: "Total number of times max size was reached",
		},
		[]string{"pvc", "namespace"},
	)

	PvcUnhealthyTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pvc_chonker_pvc_unhealthy_total",
			Help: "Total number of unhealthy PVCs encountered",
		},
		[]string{"pvc", "namespace"},
	)

	LastReconciliationTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pvc_chonker_last_reconciliation_timestamp_seconds",
			Help: "Timestamp of the last reconciliation loop",
		},
	)

	ReconciliationStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pvc_chonker_reconciliation_status",
			Help: "Status of last reconciliation (1=success, 0=failure)",
		},
		[]string{"status"},
	)

	LastUpsizeTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pvc_chonker_last_upsize_timestamp_seconds",
			Help: "Timestamp of the last upsize event per PVC",
		},
		[]string{"pvc", "namespace"},
	)

	UpsizeStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pvc_chonker_upsize_status",
			Help: "Status of last upsize event per PVC (1=success, 0=failure)",
		},
		[]string{"pvc", "namespace", "status"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		ExpansionsTotal,
		ExpansionFailuresTotal,
		LoopDurationSeconds,
		ThresholdReachedTotal,
		MaxSizeReachedTotal,
		PvcUnhealthyTotal,
		LastReconciliationTime,
		ReconciliationStatus,
		LastUpsizeTime,
		UpsizeStatus,
		// Standard Go runtime metrics
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}