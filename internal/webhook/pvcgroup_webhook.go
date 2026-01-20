package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
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

	// Check if decoder is initialized
	if m.decoder == nil {
		logger.Error(nil, "Decoder not initialized")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("decoder not initialized"))
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := (*m.decoder).Decode(req, pvc); err != nil {
		logger.Error(err, "Failed to decode PVC")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Skip if PVC is explicitly disabled
	if pvc.Annotations != nil {
		if enabled, exists := pvc.Annotations["pvc-chonker.io/enabled"]; exists && enabled == "false" {
			return admission.Allowed("PVC expansion disabled")
		}
	}

	// Check if PVC has a group annotation
	if pvc.Annotations == nil {
		return admission.Allowed("no annotations")
	}
	groupName, hasGroup := pvc.Annotations["pvc-chonker.io/group"]
	if !hasGroup {
		return admission.Allowed("no group annotation")
	}

	// Find the specified PVCGroup
	var pvcGroup pvcchonkerv1alpha1.PVCGroup
	if err := m.Client.Get(ctx, client.ObjectKey{
		Name:      groupName,
		Namespace: pvc.Namespace,
	}, &pvcGroup); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return admission.Allowed("PVCGroup not found")
		}
		logger.Error(err, "Failed to get PVCGroup", "group", groupName)
		return admission.Errored(http.StatusInternalServerError, err)
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

	// Get template annotations to apply
	templateAnnotations := getTemplateAnnotations(pvcGroup.Spec.Template, pvc.Annotations)

	// Create JSON patches for missing annotations
	for key, value := range templateAnnotations {
		escapedKey := "pvc-chonker.io~1" + key[len("pvc-chonker.io/"):]
		patches = append(patches, map[string]interface{}{
			"op":    "add",
			"path":  "/metadata/annotations/" + escapedKey,
			"value": value,
		})
	}

	if len(patches) == 0 {
		return admission.Allowed("no patches needed")
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		logger.Error(err, "Failed to marshal patches")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	logger.Info("Applied PVCGroup template to PVC", "pvc", pvc.Name, "group", groupName)

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

// getTemplateAnnotations returns a map of annotations to apply from the template
// Only returns annotations that don't already exist in the provided annotations map
func getTemplateAnnotations(template pvcchonkerv1alpha1.PVCGroupTemplate, existing map[string]string) map[string]string {
	result := make(map[string]string)

	if template.Threshold != nil {
		if _, exists := existing["pvc-chonker.io/threshold"]; !exists {
			result["pvc-chonker.io/threshold"] = *template.Threshold
		}
	}

	if template.Increase != nil {
		if _, exists := existing["pvc-chonker.io/increase"]; !exists {
			result["pvc-chonker.io/increase"] = *template.Increase
		}
	}

	if template.MaxSize != nil {
		if _, exists := existing["pvc-chonker.io/max-size"]; !exists {
			result["pvc-chonker.io/max-size"] = template.MaxSize.String()
		}
	}

	if template.Cooldown != nil {
		if _, exists := existing["pvc-chonker.io/cooldown"]; !exists {
			result["pvc-chonker.io/cooldown"] = template.Cooldown.Duration.String()
		}
	}

	return result
}

// Default implements the admission.CustomDefaulter interface
func (m *PVCGroupMutator) Default(ctx context.Context, obj runtime.Object) error {
	logger := log.FromContext(ctx)
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return nil
	}

	// Check if PVC has annotations and a group annotation
	if pvc.Annotations == nil {
		return nil
	}

	groupName, hasGroup := pvc.Annotations["pvc-chonker.io/group"]
	if !hasGroup || groupName == "" {
		return nil
	}

	// Find the specified PVCGroup
	var pvcGroup pvcchonkerv1alpha1.PVCGroup
	if err := m.Client.Get(ctx, client.ObjectKey{
		Name:      groupName,
		Namespace: pvc.Namespace,
	}, &pvcGroup); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.V(1).Info("PVCGroup not found, skipping template application", "group", groupName)
			return nil // PVCGroup not found, skip processing
		}
		logger.Error(err, "Failed to get PVCGroup", "group", groupName, "namespace", pvc.Namespace)
		return fmt.Errorf("failed to get PVCGroup %s/%s: %w", pvc.Namespace, groupName, err)
	}

	// Apply group template settings as annotations if not already present
	templateAnnotations := getTemplateAnnotations(pvcGroup.Spec.Template, pvc.Annotations)
	for key, value := range templateAnnotations {
		pvc.Annotations[key] = value
	}

	logger.Info("Applied PVCGroup template to PVC", "pvc", pvc.Name, "group", groupName)
	return nil
}

// InjectDecoder implements the admission.DecoderInjector interface
func (m *PVCGroupMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}

// SetupWebhookWithManager sets up the webhook with the manager
func SetupPVCGroupWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		WithDefaulter(&PVCGroupMutator{Client: mgr.GetClient()}).
		Complete()
}
