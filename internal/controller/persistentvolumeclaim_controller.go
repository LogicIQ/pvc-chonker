package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/logicIQ/pvc-chonker/pkg/annotations"
	"github.com/logicIQ/pvc-chonker/pkg/cache"
	"github.com/logicIQ/pvc-chonker/pkg/kubelet"
	"github.com/logicIQ/pvc-chonker/pkg/metrics"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get;patch;update
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes/proxy,verbs=get;list;watch

type PersistentVolumeClaimReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	GlobalConfig     *annotations.GlobalConfig
	MetricsCollector kubelet.MetricsCollectorInterface
	WatchInterval    time.Duration
	EventRecorder    record.EventRecorder
	DryRun           bool
	MaxParallel      int
	metricsCache     *kubelet.MetricsCache
	storageCache     *cache.StorageClassCache
	policyResolver   *annotations.PolicyResolver
}

func (r *PersistentVolumeClaimReconciler) Start(ctx context.Context) error {
	log := log.FromContext(ctx).WithName("pvcReconciler")
	log.Info("Starting periodic reconciliation loop", "interval", r.WatchInterval, "dryRun", r.DryRun)

	// Initialize storage class cache
	r.storageCache = cache.NewStorageClassCache()
	r.policyResolver = annotations.NewPolicyResolver(r.Client)

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
		metrics.LastReconciliationTime.SetToCurrentTime()
	}()

	log.Info("Starting reconciliation cycle", "dryRun", r.DryRun, "time", startTime.Format(time.RFC3339))

	r.storageCache.Clear()

	var pvcs corev1.PersistentVolumeClaimList
	if err := r.Client.List(ctx, &pvcs); err != nil {
		log.Error(err, "Failed to list PVCs")
		metrics.RecordKubernetesClientRequest("list_pvcs", "failed")
		metrics.ReconciliationStatus.WithLabelValues("failure").Set(1)
		metrics.ReconciliationStatus.WithLabelValues("success").Set(0)
		return
	}
	metrics.RecordKubernetesClientRequest("list_pvcs", "success")

	managedPVCs := make([]corev1.PersistentVolumeClaim, 0, len(pvcs.Items))
	for i := range pvcs.Items {
		if _, err := r.policyResolver.ResolvePVCConfig(ctx, &pvcs.Items[i], r.GlobalConfig); err == nil {
			managedPVCs = append(managedPVCs, pvcs.Items[i])
		}
	}
	pvcs.Items = managedPVCs

	log.Info("Found PVCs", "total", len(pvcs.Items), "managed", len(managedPVCs))

	log.V(1).Info("Fetching kubelet metrics")
	metricsCache, err := r.MetricsCollector.GetAllVolumeMetrics(ctx)
	if err != nil {
		log.Error(err, "Failed to fetch kubelet metrics")
		metrics.RecordKubeletClientRequest("failed")
		metrics.ReconciliationStatus.WithLabelValues("failure").Set(1)
		metrics.ReconciliationStatus.WithLabelValues("success").Set(0)
		return
	}
	log.V(1).Info("Successfully fetched kubelet metrics", "volumeCount", len(metricsCache.GetAll()))
	for key, vm := range metricsCache.GetAll() {
		if vm == nil {
			log.V(1).Info("Skipping nil volume metrics", "pvc", key)
			continue
		}
		log.V(1).Info("Found volume metrics", "pvc", key, "usage", vm.UsagePercent, "capacity", vm.CapacityBytes)
	}
	metrics.RecordKubeletClientRequest("success")
	if metricsCache == nil {
		log.Error(nil, "Metrics cache is nil after successful fetch")
		metrics.ReconciliationStatus.WithLabelValues("failure").Set(1)
		metrics.ReconciliationStatus.WithLabelValues("success").Set(0)
		return
	}
	r.metricsCache = metricsCache

	metrics.ManagedPVCsTotal.Set(float64(len(managedPVCs)))

	if r.MaxParallel <= 0 {
		r.MaxParallel = 4
	}

	semaphore := make(chan struct{}, r.MaxParallel)
	var wg sync.WaitGroup

	for i := range pvcs.Items {
		wg.Add(1)
		go func(pvc corev1.PersistentVolumeClaim) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			r.reconcilePVC(ctx, &pvc)
		}(pvcs.Items[i])
	}

	wg.Wait()

	duration := time.Since(startTime)
	metrics.RecordLoopDuration(duration.Seconds())
	metrics.ReconciliationStatus.WithLabelValues("success").Set(1)
	metrics.ReconciliationStatus.WithLabelValues("failure").Set(0)
	log.Info("Completed reconciliation cycle", "totalPVCs", len(pvcs.Items), "managedPVCs", len(managedPVCs), "duration", duration, "nextCycle", startTime.Add(r.WatchInterval).Format(time.RFC3339))
}

func (r *PersistentVolumeClaimReconciler) reconcilePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) {
	log := log.FromContext(ctx).WithValues("pvc", pvc.Name, "namespace", pvc.Namespace)

	log.V(1).Info("Processing PVC", "phase", pvc.Status.Phase, "size", pvc.Status.Capacity[corev1.ResourceStorage])

	config, err := r.policyResolver.ResolvePVCConfig(ctx, pvc, r.GlobalConfig)
	if err != nil {
		log.V(2).Info("PVC not managed", "reason", err.Error())
		return
	}

	if !r.IsPVCEligible(pvc) {
		log.V(2).Info("PVC not eligible for expansion")
		return
	}

	if !r.IsStorageClassExpandable(ctx, pvc) {
		log.V(2).Info("Storage class does not allow volume expansion")
		metrics.RecordFailedResize(pvc.Name, pvc.Namespace, "storage_class_not_expandable")
		return
	}

	if annotations.IsPvcResizing(pvc) {
		log.V(1).Info("PVC is currently resizing, skipping")
		metrics.RecordResizeInProgress(pvc.Name, pvc.Namespace)
		return
	}

	if config.IsInCooldown() {
		log.V(2).Info("PVC is in cooldown period")
		metrics.RecordCooldownSkipped(pvc.Name, pvc.Namespace)
		return
	}

	namespacedName := types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}
	volumeMetrics, exists := r.metricsCache.Get(namespacedName)
	if !exists {
		log.V(1).Info("Volume metrics not found in cache", "availableMetrics", len(r.metricsCache.GetAll()))
		metrics.RecordFailedResize(pvc.Name, pvc.Namespace, "metrics_not_found")
		return
	}

	log.V(1).Info("Found volume metrics", "storageUsage", volumeMetrics.UsagePercent, "inodesUsage", volumeMetrics.InodesUsagePercent, "storageThreshold", config.Threshold, "inodesThreshold", config.InodesThreshold)

	currentSize := pvc.Status.Capacity[corev1.ResourceStorage]
	metrics.UpdatePVCMetrics(pvc.Name, pvc.Namespace, volumeMetrics.UsagePercent, currentSize.Value())
	metrics.UpdatePVCInodesMetrics(pvc.Name, pvc.Namespace, volumeMetrics.InodesUsagePercent, volumeMetrics.InodesTotal)

	thresholdReached := volumeMetrics.UsagePercent >= config.Threshold
	if volumeMetrics.InodesTotal > 0 {
		thresholdReached = thresholdReached || volumeMetrics.InodesUsagePercent >= config.InodesThreshold
		if volumeMetrics.InodesUsagePercent >= config.InodesThreshold {
			fsType := r.getFilesystemType(ctx, pvc)
			if fsType == "ext3" || fsType == "ext4" {
				log.Info("Inode threshold reached on fixed-inode filesystem - expansion will not resolve inode pressure",
					"filesystem", fsType,
					"inodesUsage", volumeMetrics.InodesUsagePercent,
					"inodesThreshold", config.InodesThreshold)
			} else {
				log.Info("Inode threshold reached",
					"filesystem", fsType,
					"inodesUsage", volumeMetrics.InodesUsagePercent,
					"inodesThreshold", config.InodesThreshold)
			}
		}
	}

	if !thresholdReached {
		log.V(3).Info("Threshold not reached", "storageUsage", volumeMetrics.UsagePercent, "inodesUsage", volumeMetrics.InodesUsagePercent, "storageThreshold", config.Threshold, "inodesThreshold", config.InodesThreshold)
		return
	}

	metrics.RecordThresholdReached(pvc.Name, pvc.Namespace)
	log.Info("Threshold reached - initiating expansion",
		"storageUsage", volumeMetrics.UsagePercent,
		"inodesUsage", volumeMetrics.InodesUsagePercent,
		"storageThreshold", config.Threshold,
		"inodesThreshold", config.InodesThreshold,
		"dryRun", r.DryRun)

	if err := r.ExpandPVC(ctx, pvc, config); err != nil {
		metrics.RecordFailedResize(pvc.Name, pvc.Namespace, "expansion_failed")
		r.EventRecorder.Eventf(pvc, corev1.EventTypeWarning, "ExpansionFailed", "Failed to expand PVC: %v", err)
		log.Error(err, "PVC expansion failed")
		return
	}

	metrics.RecordSuccessfulResize(pvc.Name, pvc.Namespace)
	newSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if volumeMetrics.InodesTotal > 0 {
		if volumeMetrics.InodesUsagePercent >= config.InodesThreshold {
			fsType := r.getFilesystemType(ctx, pvc)
			if fsType == "ext3" || fsType == "ext4" {
				r.EventRecorder.Eventf(pvc, corev1.EventTypeWarning, "ExpandedInodePressure",
					"PVC expanded from %s to %s due to inode pressure (storage: %.1f%%, inodes: %.1f%%) - WARNING: %s filesystem has fixed inode count, expansion will not resolve inode pressure",
					currentSize.String(), newSize.String(), volumeMetrics.UsagePercent, volumeMetrics.InodesUsagePercent, fsType)
			} else {
				r.EventRecorder.Eventf(pvc, corev1.EventTypeNormal, "ExpandedInodePressure",
					"PVC expanded from %s to %s due to inode pressure (storage: %.1f%%, inodes: %.1f%%) - %s filesystem",
					currentSize.String(), newSize.String(), volumeMetrics.UsagePercent, volumeMetrics.InodesUsagePercent, fsType)
			}
		} else {
			r.EventRecorder.Eventf(pvc, corev1.EventTypeNormal, "Expanded",
				"PVC expanded from %s to %s (storage: %.1f%%, inodes: %.1f%%)",
				currentSize.String(), newSize.String(), volumeMetrics.UsagePercent, volumeMetrics.InodesUsagePercent)
		}
	} else {
		r.EventRecorder.Eventf(pvc, corev1.EventTypeNormal, "Expanded",
			"PVC expanded from %s to %s (storage: %.1f%%)",
			currentSize.String(), newSize.String(), volumeMetrics.UsagePercent)
	}
	log.Info("PVC expansion completed successfully", "from", currentSize.String(), "to", newSize.String())
}

func (r *PersistentVolumeClaimReconciler) IsPVCEligible(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode != corev1.PersistentVolumeFilesystem {
		return false
	}
	if pvc.Status.Phase != corev1.ClaimBound {
		return false
	}
	return true
}

func (r *PersistentVolumeClaimReconciler) IsStorageClassExpandable(ctx context.Context, pvc *corev1.PersistentVolumeClaim) bool {
	if pvc.Spec.StorageClassName == nil {
		return false
	}

	scName := *pvc.Spec.StorageClassName
	if expandable, exists := r.storageCache.Get(scName); exists {
		return expandable
	}

	var sc storagev1.StorageClass
	if err := r.Get(ctx, types.NamespacedName{Name: scName}, &sc); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "Failed to get storage class", "storageClass", scName)
		metrics.RecordKubernetesClientRequest("get_storageclass", "failed")
		return false
	}
	metrics.RecordKubernetesClientRequest("get_storageclass", "success")

	expandable := sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
	r.storageCache.Set(scName, expandable)
	return expandable
}

func (r *PersistentVolumeClaimReconciler) getFilesystemType(ctx context.Context, pvc *corev1.PersistentVolumeClaim) string {
	if pvc.Spec.StorageClassName == nil {
		return "unknown"
	}

	var sc storagev1.StorageClass
	if err := r.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, &sc); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "Failed to get storage class for filesystem type detection", "storageClass", *pvc.Spec.StorageClassName)
		metrics.RecordKubernetesClientRequest("get_storageclass_fstype", "failed")
		return "unknown"
	}
	metrics.RecordKubernetesClientRequest("get_storageclass_fstype", "success")

	if fsType, exists := sc.Parameters["fsType"]; exists {
		return fsType
	}
	if fsType, exists := sc.Parameters["csi.storage.k8s.io/fstype"]; exists {
		return fsType
	}
	return "ext4"
}

func (r *PersistentVolumeClaimReconciler) ExpandPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, config *annotations.PVCConfig) error {
	log := log.FromContext(ctx).WithValues("pvc", pvc.Name, "namespace", pvc.Namespace)
	currentSize := pvc.Status.Capacity[corev1.ResourceStorage]
	newSize, err := config.CalculateNewSize(currentSize)
	if err != nil {
		return fmt.Errorf("failed to calculate new size: %w", err)
	}

	if config.ExceedsMaxSize(newSize) {
		metrics.RecordLimitReached(pvc.Name, pvc.Namespace)
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
		metrics.RecordKubernetesClientRequest("update_pvc", "failed")
		return fmt.Errorf("failed to update PVC spec: %w", err)
	}
	metrics.RecordKubernetesClientRequest("update_pvc", "success")

	return nil
}

func (r *PersistentVolumeClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// This reconciler uses a custom Start method instead of the standard controller pattern
	// The Start method handles periodic reconciliation of all PVCs
	return mgr.Add(r)
}
