package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/logicIQ/pvc-chonker/pkg/annotations"
	"github.com/logicIQ/pvc-chonker/pkg/kubelet"
	"github.com/logicIQ/pvc-chonker/pkg/metrics"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PersistentVolumeClaimReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	GlobalConfig     *annotations.GlobalConfig
	MetricsCollector *kubelet.MetricsCollector
	WatchInterval    time.Duration
}

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;update;patch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Start implements manager.Runnable for periodic reconciliation
func (r *PersistentVolumeClaimReconciler) Start(ctx context.Context) error {
	log := log.FromContext(ctx).WithName("pvcReconciler")
	log.Info("Starting periodic reconciliation loop", "interval", r.WatchInterval)

	ticker := time.NewTicker(r.WatchInterval)
	defer ticker.Stop()

	// Initial run
	r.reconcileAll(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping periodic reconciliation loop")
			return nil
		case <-ticker.C:
			r.reconcileAll(ctx)
		}
	}
}

func (r *PersistentVolumeClaimReconciler) NeedLeaderElection() bool {
	return true
}

// reconcileAll processes all managed PVCs for usage monitoring
func (r *PersistentVolumeClaimReconciler) reconcileAll(ctx context.Context) {
	log := log.FromContext(ctx).WithName("reconcileAll")
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds()
		metrics.LoopDurationSeconds.Observe(duration)
		metrics.LastReconciliationTime.SetToCurrentTime()
	}()

	log.V(1).Info("Starting reconciliation for all PVCs")

	var pvcs corev1.PersistentVolumeClaimList
	if err := r.Client.List(ctx, &pvcs); err != nil {
		log.Error(err, "Failed to list PVCs")
		metrics.ReconciliationStatus.WithLabelValues("failure").Set(1)
		metrics.ReconciliationStatus.WithLabelValues("success").Set(0)
		return
	}

	for _, pvc := range pvcs.Items {
		r.reconcilePVC(ctx, &pvc)
	}

	metrics.ReconciliationStatus.WithLabelValues("success").Set(1)
	metrics.ReconciliationStatus.WithLabelValues("failure").Set(0)
	log.V(1).Info("Completed reconciliation for all PVCs", "count", len(pvcs.Items))
}

// reconcilePVC processes a single PVC for expansion
func (r *PersistentVolumeClaimReconciler) reconcilePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) {
	log := log.FromContext(ctx).WithValues("pvc", pvc.Name, "namespace", pvc.Namespace)

	config, err := annotations.ParsePVCAnnotations(pvc, r.GlobalConfig)
	if err != nil {
		log.V(2).Info("PVC not managed", "reason", err.Error())
		return
	}

	if !r.isPVCEligible(pvc) {
		log.V(2).Info("PVC not eligible for expansion")
		return
	}

	if annotations.IsPvcResizing(pvc) {
		log.V(1).Info("PVC is currently resizing, skipping")
		return
	}

	if config.IsInCooldown() {
		log.V(2).Info("PVC is in cooldown period")
		return
	}

	namespacedName := types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}
	volumeMetrics, err := r.MetricsCollector.GetVolumeMetrics(ctx, namespacedName)
	if err != nil {
		log.V(2).Info("Failed to get volume metrics", "error", err)
		return
	}

	if volumeMetrics.UsagePercent < config.Threshold {
		log.V(2).Info("Threshold not reached", "usage", volumeMetrics.UsagePercent, "threshold", config.Threshold)
		return
	}

	metrics.ThresholdReachedTotal.WithLabelValues(pvc.Name, pvc.Namespace).Inc()
	log.Info("Threshold reached, expanding PVC", "usage", volumeMetrics.UsagePercent, "threshold", config.Threshold)

	if err := r.expandPVC(ctx, pvc, config); err != nil {
		metrics.ExpansionFailuresTotal.WithLabelValues(pvc.Name, pvc.Namespace, "expansion_failed").Inc()
		metrics.LastUpsizeTime.WithLabelValues(pvc.Name, pvc.Namespace).SetToCurrentTime()
		metrics.UpsizeStatus.WithLabelValues(pvc.Name, pvc.Namespace, "failure").Set(1)
		metrics.UpsizeStatus.WithLabelValues(pvc.Name, pvc.Namespace, "success").Set(0)
		log.Error(err, "Failed to expand PVC")
		return
	}

	metrics.ExpansionsTotal.WithLabelValues(pvc.Name, pvc.Namespace).Inc()
	metrics.LastUpsizeTime.WithLabelValues(pvc.Name, pvc.Namespace).SetToCurrentTime()
	metrics.UpsizeStatus.WithLabelValues(pvc.Name, pvc.Namespace, "success").Set(1)
	metrics.UpsizeStatus.WithLabelValues(pvc.Name, pvc.Namespace, "failure").Set(0)
	log.Info("PVC expanded successfully")
}

func (r *PersistentVolumeClaimReconciler) isPVCEligible(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode != corev1.PersistentVolumeFilesystem {
		return false
	}
	if pvc.Status.Phase != corev1.ClaimBound {
		return false
	}
	return true
}

func (r *PersistentVolumeClaimReconciler) expandPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, config *annotations.PVCConfig) error {
	currentSize := pvc.Status.Capacity[corev1.ResourceStorage]
	newSize, err := config.CalculateNewSize(currentSize)
	if err != nil {
		return fmt.Errorf("failed to calculate new size: %w", err)
	}

	if config.ExceedsMaxSize(newSize) {
		metrics.MaxSizeReachedTotal.WithLabelValues(pvc.Name, pvc.Namespace).Inc()
		return fmt.Errorf("new size %s exceeds max size %s", newSize.String(), config.MaxSize.String())
	}

	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = newSize
	annotations.UpdateLastExpansion(pvc)

	if err := r.Update(ctx, pvc); err != nil {
		return fmt.Errorf("failed to update PVC: %w", err)
	}

	return nil
}

func (r *PersistentVolumeClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// No need to watch PVC events since we use periodic reconciliation
	return nil
}
