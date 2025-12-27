package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	pvcchonkerv1alpha1 "github.com/logicIQ/pvc-chonker/api/v1alpha1"
)

// PVCGroupMutator handles PVC mutations based on PVCGroup membership
type PVCGroupMutator struct {
	Client  client.Client
	decoder *admission.Decoder
}

// Handle implements the admission.Handler interface
func (m *PVCGroupMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx)

	// Only handle PVC creation and updates
	if req.Kind.Kind != "PersistentVolumeClaim" {
		return admission.Allowed("not a PVC")
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := (*m.decoder).Decode(req, pvc); err != nil {
		logger.Error(err, "Failed to decode PVC")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Skip if PVC is explicitly disabled
	if enabled, exists := pvc.Annotations["pvc-chonker.io/enabled"]; exists && enabled == "false" {
		return admission.Allowed("PVC expansion disabled")
	}

	// Find matching PVCGroups
	var pvcGroupList pvcchonkerv1alpha1.PVCGroupList
	if err := m.Client.List(ctx, &pvcGroupList, &client.ListOptions{
		Namespace: pvc.Namespace,
	}); err != nil {
		logger.Error(err, "Failed to list PVCGroups")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	var matchingGroup *pvcchonkerv1alpha1.PVCGroup
	for _, group := range pvcGroupList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&group.Spec.Selector)
		if err != nil {
			continue
		}
		if selector.Matches(labels.Set(pvc.Labels)) {
			matchingGroup = &group
			break
		}
	}

	if matchingGroup == nil {
		return admission.Allowed("no matching PVCGroup")
	}

	// Apply group template settings as annotations if not already present
	patches := []map[string]interface{}{}

	if pvc.Annotations == nil {
		patches = append(patches, map[string]interface{}{
			"op":    "add",
			"path":  "/metadata/annotations",
			"value": map[string]string{},
		})
	}

	template := matchingGroup.Spec.Template

	// Add group-based annotations only if not already set
	if _, exists := pvc.Annotations["pvc-chonker.io/group"]; !exists {
		patches = append(patches, map[string]interface{}{
			"op":    "add",
			"path":  "/metadata/annotations/pvc-chonker.io~1group",
			"value": matchingGroup.Name,
		})
	}

	if template.Enabled != nil {
		if _, exists := pvc.Annotations["pvc-chonker.io/enabled"]; !exists {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  "/metadata/annotations/pvc-chonker.io~1enabled",
				"value": fmt.Sprintf("%t", *template.Enabled),
			})
		}
	}

	if template.Threshold != nil {
		if _, exists := pvc.Annotations["pvc-chonker.io/threshold"]; !exists {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  "/metadata/annotations/pvc-chonker.io~1threshold",
				"value": *template.Threshold,
			})
		}
	}

	if template.Increase != nil {
		if _, exists := pvc.Annotations["pvc-chonker.io/increase"]; !exists {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  "/metadata/annotations/pvc-chonker.io~1increase",
				"value": *template.Increase,
			})
		}
	}

	if template.MaxSize != nil {
		if _, exists := pvc.Annotations["pvc-chonker.io/max-size"]; !exists {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  "/metadata/annotations/pvc-chonker.io~1max-size",
				"value": template.MaxSize.String(),
			})
		}
	}

	if template.Cooldown != nil {
		if _, exists := pvc.Annotations["pvc-chonker.io/cooldown"]; !exists {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  "/metadata/annotations/pvc-chonker.io~1cooldown",
				"value": template.Cooldown.Duration.String(),
			})
		}
	}

	if len(patches) == 0 {
		return admission.Allowed("no patches needed")
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		logger.Error(err, "Failed to marshal patches")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	logger.Info("Applied PVCGroup template to PVC", "pvc", pvc.Name, "group", matchingGroup.Name, "patches", len(patches))

	return admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			UID:     req.UID,
			Allowed: true,
			PatchType: func() *admissionv1.PatchType {
				pt := admissionv1.PatchTypeJSONPatch
				return &pt
			}(),
			Patch: patchBytes,
		},
	}
}

// Default implements the admission.CustomDefaulter interface
func (m *PVCGroupMutator) Default(ctx context.Context, obj runtime.Object) error {
	// This method is required by the CustomDefaulter interface but we handle
	// mutations in the Handle method instead
	return nil
}

// InjectDecoder implements the admission.DecoderInjector interface
func (m *PVCGroupMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}

// SetupWebhookWithManager sets up the webhook with the manager
func SetupPVCGroupWebhook(mgr ctrl.Manager) error {
	mutator := &PVCGroupMutator{
		Client: mgr.GetClient(),
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		WithDefaulter(mutator).
		Complete()
}
