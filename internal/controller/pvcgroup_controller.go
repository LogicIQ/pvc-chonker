package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	pvcchonkerv1alpha1 "github.com/logicIQ/pvc-chonker/api/v1alpha1"
)

// PVCGroupReconciler reconciles a PVCGroup object
type PVCGroupReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
	// Mutex to prevent concurrent status updates for the same PVCGroup
	statusLocks sync.Map // map[string]*sync.Mutex
}

//+kubebuilder:rbac:groups=pvc-chonker.io,resources=pvcgroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pvc-chonker.io,resources=pvcgroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pvc-chonker.io,resources=pvcgroups/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *PVCGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get or create a mutex for this specific PVCGroup
	lockKey := req.NamespacedName.String()
	mutexInterface, _ := r.statusLocks.LoadOrStore(lockKey, &sync.Mutex{})
	mutex := mutexInterface.(*sync.Mutex)

	// Lock to prevent concurrent reconciliation of the same PVCGroup
	mutex.Lock()
	defer mutex.Unlock()

	// Fetch the PVCGroup instance
	var pvcGroup pvcchonkerv1alpha1.PVCGroup
	if err := r.Get(ctx, req.NamespacedName, &pvcGroup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get all PVCs in the namespace (we'll filter by annotation)
	var pvcList corev1.PersistentVolumeClaimList
	if err := r.List(ctx, &pvcList, &client.ListOptions{
		Namespace: pvcGroup.Namespace,
	}); err != nil {
		logger.Error(err, "Failed to list PVCs")
		return ctrl.Result{}, err
	}

	// Only process PVCs that have the group annotation and are enabled
	var activePVCs []corev1.PersistentVolumeClaim
	for _, pvc := range pvcList.Items {
		// Must have group annotation matching this group
		if pvc.Annotations == nil || pvc.Annotations["pvc-chonker.io/group"] != pvcGroup.Name {
			continue
		}

		// Must be enabled
		if enabled, exists := pvc.Annotations["pvc-chonker.io/enabled"]; !exists || enabled != "true" {
			logger.V(1).Info("PVC excluded from group", "pvc", pvc.Name, "enabled", enabled, "exists", exists)
			continue
		}

		logger.V(1).Info("PVC included in group", "pvc", pvc.Name, "group", pvc.Annotations["pvc-chonker.io/group"], "enabled", pvc.Annotations["pvc-chonker.io/enabled"])
		activePVCs = append(activePVCs, pvc)
	}

	// Update status
	now := metav1.Now()
	pvcGroup.Status.MemberCount = int32(len(activePVCs))
	pvcGroup.Status.LastUpdated = &now

	if len(activePVCs) > 0 {
		coordinatedSize := r.calculateLargestSize(activePVCs)
		pvcGroup.Status.CurrentSize = &coordinatedSize

		// Apply coordination if needed
		if err := r.coordinatePVCSizes(ctx, activePVCs, coordinatedSize, &pvcGroup); err != nil {
			logger.Error(err, "Failed to coordinate PVC sizes")
			r.EventRecorder.Event(&pvcGroup, corev1.EventTypeWarning, "CoordinationFailed", err.Error())
			return ctrl.Result{RequeueAfter: time.Minute * 5}, err
		}
	}

	// Update status
	if err := r.Status().Update(ctx, &pvcGroup); err != nil {
		logger.Error(err, "Failed to update PVCGroup status")
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	logger.Info("PVCGroup reconciled successfully", "memberCount", pvcGroup.Status.MemberCount, "activePVCs", len(activePVCs))
	return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
}

func (r *PVCGroupReconciler) calculateLargestSize(pvcs []corev1.PersistentVolumeClaim) resource.Quantity {
	var largest resource.Quantity
	for _, pvc := range pvcs {
		if size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; size.Cmp(largest) > 0 {
			largest = size
		}
	}
	return largest
}

func (r *PVCGroupReconciler) coordinatePVCSizes(ctx context.Context, pvcs []corev1.PersistentVolumeClaim, targetSize resource.Quantity, group *pvcchonkerv1alpha1.PVCGroup) error {
	logger := log.FromContext(ctx)

	for _, pvc := range pvcs {
		currentSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]

		// Skip if PVC is already at target size or larger
		if currentSize.Cmp(targetSize) >= 0 {
			continue
		}

		// Update PVC size to match group coordination
		pvcCopy := pvc.DeepCopy()
		pvcCopy.Spec.Resources.Requests[corev1.ResourceStorage] = targetSize

		if err := r.Update(ctx, pvcCopy); err != nil {
			return fmt.Errorf("failed to update PVC %s: %w", pvc.Name, err)
		}

		logger.Info("Coordinated PVC size", "pvc", pvc.Name, "oldSize", currentSize.String(), "newSize", targetSize.String())
		r.EventRecorder.Eventf(group, corev1.EventTypeNormal, "PVCCoordinated",
			"PVC %s size coordinated from %s to %s", pvc.Name, currentSize.String(), targetSize.String())
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PVCGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pvcchonkerv1alpha1.PVCGroup{}).
		Watches(&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(r.findPVCGroupForPVC)).
		Complete(r)
}

func (r *PVCGroupReconciler) findPVCGroupForPVC(ctx context.Context, obj client.Object) []reconcile.Request {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return nil
	}

	// Only process PVCs with group annotation
	groupName, exists := pvc.Annotations["pvc-chonker.io/group"]
	if !exists {
		return nil
	}

	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name:      groupName,
			Namespace: pvc.Namespace,
		},
	}}
}
