package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/logicIQ/pvc-chonker/pkg/annotations"
	"github.com/logicIQ/pvc-chonker/pkg/kubelet"
	"github.com/logicIQ/pvc-chonker/pkg/metrics"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"k8s.io/client-go/tools/record"
)

type PersistentVolumeClaimReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	GlobalConfig     *annotations.GlobalConfig
	MetricsCollector *kubelet.MetricsCollector
	WatchInterval    time.Duration
	EventRecorder    record.EventRecorder
	DryRun           bool
	MaxParallel      int
}

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;update;patch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
func (r *PersistentVolumeClaimReconciler) Start(ctx context.Context) error {
	log := log.FromContext(ctx).WithName("pvcReconciler")
	log.Info("Starting periodic reconciliation loop", "interval", r.WatchInterval, "dryRun", r.DryRun)

	ticker := time.NewTicker(r.WatchInterval)
	defer ticker.Stop()


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


func (r *PersistentVolumeClaimReconciler) reconcileAll(ctx context.Context) {
	log := log.FromContext(ctx).WithName("reconcileAll")
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds()
		metrics.LoopDurationSeconds.Observe(duration)
		metrics.LastReconciliationTime.SetToCurrentTime()
	}()

	log.V(1).Info("Starting reconciliation cycle", "dryRun", r.DryRun)

	var pvcs corev1.PersistentVolumeClaimList
	if err := r.Client.List(ctx, &pvcs); err != nil {
		log.Error(err, "Failed to list PVCs")
		metrics.ReconciliationStatus.WithLabelValues("failure").Set(1)
		metrics.ReconciliationStatus.WithLabelValues("success").Set(0)
		return
	}

	if r.MaxParallel <= 0 {
		r.MaxParallel = 4
	}

	semaphore := make(chan struct{}, r.MaxParallel)
	var wg sync.WaitGroup

	for _, pvc := range pvcs.Items {
		wg.Add(1)
		go func(pvc corev1.PersistentVolumeClaim) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			r.reconcilePVC(ctx, &pvc)
		}(pvc)
	}

	wg.Wait()

	metrics.ReconciliationStatus.WithLabelValues("success").Set(1)
	metrics.ReconciliationStatus.WithLabelValues("failure").Set(0)
	log.V(1).Info("Completed reconciliation cycle", "pvcCount", len(pvcs.Items), "duration", time.Since(startTime))
}


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

	if !r.isStorageClassExpandable(ctx, pvc) {
		log.V(2).Info("Storage class does not allow volume expansion")
		metrics.PvcUnhealthyTotal.WithLabelValues(pvc.Name, pvc.Namespace).Inc()
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
		log.V(2).Info("Threshold not reached", "usagePercent", fmt.Sprintf("%.1f%%", volumeMetrics.UsagePercent), "threshold", fmt.Sprintf("%.1f%%", config.Threshold))
		return
	}

	metrics.ThresholdReachedTotal.WithLabelValues(pvc.Name, pvc.Namespace).Inc()
	log.Info("Threshold reached - initiating expansion", "usagePercent", fmt.Sprintf("%.1f%%", volumeMetrics.UsagePercent), "threshold", fmt.Sprintf("%.1f%%", config.Threshold), "dryRun", r.DryRun)

	if err := r.expandPVC(ctx, pvc, config); err != nil {
		metrics.ExpansionFailuresTotal.WithLabelValues(pvc.Name, pvc.Namespace, "expansion_failed").Inc()
		metrics.LastUpsizeTime.WithLabelValues(pvc.Name, pvc.Namespace).SetToCurrentTime()
		metrics.UpsizeStatus.WithLabelValues(pvc.Name, pvc.Namespace, "failure").Set(1)
		metrics.UpsizeStatus.WithLabelValues(pvc.Name, pvc.Namespace, "success").Set(0)
		r.EventRecorder.Eventf(pvc, corev1.EventTypeWarning, "ExpansionFailed", "Failed to expand PVC: %v", err)
		log.Error(err, "PVC expansion failed")
		return
	}

	metrics.ExpansionsTotal.WithLabelValues(pvc.Name, pvc.Namespace).Inc()
	metrics.LastUpsizeTime.WithLabelValues(pvc.Name, pvc.Namespace).SetToCurrentTime()
	metrics.UpsizeStatus.WithLabelValues(pvc.Name, pvc.Namespace, "success").Set(1)
	metrics.UpsizeStatus.WithLabelValues(pvc.Name, pvc.Namespace, "failure").Set(0)
	currentSize := pvc.Status.Capacity[corev1.ResourceStorage]
	newSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	r.EventRecorder.Eventf(pvc, corev1.EventTypeNormal, "Expanded", "PVC expanded from %s to %s (usage: %.1f%%)", 
		currentSize.String(), newSize.String(), volumeMetrics.UsagePercent)
	log.Info("PVC expansion completed successfully", "from", currentSize.String(), "to", newSize.String())
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

func (r *PersistentVolumeClaimReconciler) isStorageClassExpandable(ctx context.Context, pvc *corev1.PersistentVolumeClaim) bool {
	if pvc.Spec.StorageClassName == nil {
		return false
	}

	var sc storagev1.StorageClass
	if err := r.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, &sc); err != nil {
		return false
	}

	return sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
}

func (r *PersistentVolumeClaimReconciler) expandPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, config *annotations.PVCConfig) error {
	log := log.FromContext(ctx).WithValues("pvc", pvc.Name, "namespace", pvc.Namespace)
	currentSize := pvc.Status.Capacity[corev1.ResourceStorage]
	newSize, err := config.CalculateNewSize(currentSize)
	if err != nil {
		return fmt.Errorf("failed to calculate new size: %w", err)
	}

	if config.ExceedsMaxSize(newSize) {
		metrics.MaxSizeReachedTotal.WithLabelValues(pvc.Name, pvc.Namespace).Inc()
		return fmt.Errorf("new size %s exceeds max size %s", newSize.String(), config.MaxSize.String())
	}

	if r.DryRun {
		log.Info("DRY RUN: Would expand PVC", "currentSize", currentSize.String(), "newSize", newSize.String())
		return nil
	}


	pvcCopy := pvc.DeepCopy()
	pvcCopy.Spec.Resources.Requests[corev1.ResourceStorage] = newSize
	annotations.UpdateLastExpansion(pvcCopy)

	if err := r.Update(ctx, pvcCopy); err != nil {

		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to update PVC spec: %w", err)
		}
		return fmt.Errorf("PVC not found during update: %w", err)
	}

	return nil
}

func (r *PersistentVolumeClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	
	return nil
}
