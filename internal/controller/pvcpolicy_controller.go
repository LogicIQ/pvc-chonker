package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	pvcchonkerv1alpha1 "github.com/logicIQ/pvc-chonker/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PVCPolicyReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
	// Channel-based semaphore to limit concurrent reconciliations
	semaphore chan struct{}
}

//+kubebuilder:rbac:groups=pvc-chonker.io,resources=pvcpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pvc-chonker.io,resources=pvcpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pvc-chonker.io,resources=pvcpolicies/finalizers,verbs=update

func (r *PVCPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Acquire semaphore
	r.semaphore <- struct{}{}
	defer func() { <-r.semaphore }()

	var policy pvcchonkerv1alpha1.PVCPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var pvcs corev1.PersistentVolumeClaimList
	if err := r.List(ctx, &pvcs, client.InNamespace(policy.Namespace)); err != nil {
		log.Error(err, "Failed to list PVCs")
		return ctrl.Result{}, err
	}

	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.Selector)
	if err != nil {
		log.Error(err, "Invalid label selector")
		return ctrl.Result{}, err
	}

	matchedCount := int32(0)
	for _, pvc := range pvcs.Items {
		if selector.Matches(labels.Set(pvc.Labels)) {
			matchedCount++
		}
	}

	policy.Status.MatchedPVCs = matchedCount
	now := metav1.Now()
	policy.Status.LastUpdated = &now

	if err := r.Status().Update(ctx, &policy); err != nil {
		log.Error(err, "Failed to update PVCPolicy status")
		return ctrl.Result{}, err
	}

	log.Info("PVCPolicy reconciled", "matchedPVCs", matchedCount)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *PVCPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize semaphore before any reconciliation occurs
	r.semaphore = make(chan struct{}, 10)

	return ctrl.NewControllerManagedBy(mgr).
		For(&pvcchonkerv1alpha1.PVCPolicy{}).
		Watches(&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(r.findPVCPoliciesForPVC)).
		Complete(r)
}

func (r *PVCPolicyReconciler) findPVCPoliciesForPVC(ctx context.Context, obj client.Object) []reconcile.Request {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return nil
	}

	// List all PVCPolicies in the same namespace
	var policies pvcchonkerv1alpha1.PVCPolicyList
	if err := r.List(ctx, &policies, client.InNamespace(pvc.Namespace)); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "Failed to list PVCPolicies for PVC", "pvc", pvc.Name, "namespace", pvc.Namespace)
		return nil
	}

	var requests []reconcile.Request
	for _, policy := range policies.Items {
		// Check if this PVC matches the policy selector
		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.Selector)
		if err != nil {
			log := log.FromContext(ctx)
			log.Error(err, "Invalid label selector in PVCPolicy", "policy", policy.Name, "namespace", policy.Namespace)
			continue
		}
		if selector.Matches(labels.Set(pvc.Labels)) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      policy.Name,
					Namespace: policy.Namespace,
				},
			})
		}
	}
	return requests
}
