package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pvcchonkerv1alpha1 "github.com/logicIQ/pvc-chonker/api/v1alpha1"
)

// PVCGroupReconciler reconciles a PVCGroup object
type PVCGroupReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

//+kubebuilder:rbac:groups=pvc-chonker.io,resources=pvcgroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pvc-chonker.io,resources=pvcgroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pvc-chonker.io,resources=pvcgroups/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *PVCGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the PVCGroup instance
	var pvcGroup pvcchonkerv1alpha1.PVCGroup
	if err := r.Get(ctx, req.NamespacedName, &pvcGroup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get PVCs matching the selector
	selector, err := metav1.LabelSelectorAsSelector(&pvcGroup.Spec.Selector)
	if err != nil {
		logger.Error(err, "Failed to convert label selector")
		return ctrl.Result{}, err
	}

	var pvcList corev1.PersistentVolumeClaimList
	if err := r.List(ctx, &pvcList, &client.ListOptions{
		Namespace:     pvcGroup.Namespace,
		LabelSelector: selector,
	}); err != nil {
		logger.Error(err, "Failed to list PVCs")
		return ctrl.Result{}, err
	}

	// Filter out PVCs that are disabled via annotation
	var activePVCs []corev1.PersistentVolumeClaim
	for _, pvc := range pvcList.Items {
		if enabled, exists := pvc.Annotations["pvc-chonker.io/enabled"]; exists && enabled == "false" {
			continue
		}
		activePVCs = append(activePVCs, pvc)
	}

	// Update status
	now := metav1.Now()
	pvcGroup.Status.MemberCount = int32(len(activePVCs))
	pvcGroup.Status.LastUpdated = &now

	if len(activePVCs) > 0 {
		coordinatedSize := r.calculateCoordinatedSize(activePVCs, pvcGroup.Spec.CoordinationPolicy)
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
		return ctrl.Result{}, err
	}

	logger.Info("PVCGroup reconciled successfully", "memberCount", pvcGroup.Status.MemberCount)
	return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
}

func (r *PVCGroupReconciler) calculateCoordinatedSize(pvcs []corev1.PersistentVolumeClaim, policy pvcchonkerv1alpha1.CoordinationPolicy) resource.Quantity {
	if len(pvcs) == 0 {
		return resource.Quantity{}
	}

	switch policy {
	case pvcchonkerv1alpha1.CoordinationPolicyLargest:
		var largest resource.Quantity
		for _, pvc := range pvcs {
			if size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; size.Cmp(largest) > 0 {
				largest = size
			}
		}
		return largest

	case pvcchonkerv1alpha1.CoordinationPolicyAverage:
		var total int64
		for _, pvc := range pvcs {
			size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			total += size.Value()
		}
		avgValue := total / int64(len(pvcs))
		return *resource.NewQuantity(avgValue, resource.BinarySI)

	case pvcchonkerv1alpha1.CoordinationPolicySum:
		var total int64
		for _, pvc := range pvcs {
			size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			total += size.Value()
		}
		evenValue := total / int64(len(pvcs))
		return *resource.NewQuantity(evenValue, resource.BinarySI)

	default:
		// Default to largest
		var largest resource.Quantity
		for _, pvc := range pvcs {
			if size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; size.Cmp(largest) > 0 {
				largest = size
			}
		}
		return largest
	}
}

func (r *PVCGroupReconciler) coordinatePVCSizes(ctx context.Context, pvcs []corev1.PersistentVolumeClaim, targetSize resource.Quantity, group *pvcchonkerv1alpha1.PVCGroup) error {
	logger := log.FromContext(ctx)

	for _, pvc := range pvcs {
		currentSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]

		// Skip if PVC is already at target size or larger
		if currentSize.Cmp(targetSize) >= 0 {
			continue
		}

		// Check if PVC has individual annotations that override group settings
		if threshold, exists := pvc.Annotations["pvc-chonker.io/threshold"]; exists {
			logger.Info("PVC has individual threshold annotation, skipping group coordination", "pvc", pvc.Name, "threshold", threshold)
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
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
