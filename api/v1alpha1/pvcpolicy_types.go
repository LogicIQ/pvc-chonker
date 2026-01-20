package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCPolicySpec defines the desired state of PVCPolicy
// +kubebuilder:object:generate=true
type PVCPolicySpec struct {
	// Selector specifies which PVCs this policy applies to
	Selector metav1.LabelSelector `json:"selector"`

	// Template defines the expansion configuration
	Template PVCPolicyTemplate `json:"template"`
}

// PVCPolicyTemplate defines the expansion configuration template
// +kubebuilder:object:generate=true
// +kubebuilder:validation:MinProperties=1
type PVCPolicyTemplate struct {
	// Enabled controls whether auto-expansion is enabled
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Threshold is the storage usage percentage that triggers expansion
	// +optional
	// +kubebuilder:validation:Pattern=`^([1-9][0-9]*(\.[0-9]*)?|0\.[1-9][0-9]*)%$`
	Threshold *string `json:"threshold,omitempty"`

	// InodesThreshold is the inode usage percentage that triggers expansion
	// +optional
	// +kubebuilder:validation:Pattern=`^([1-9][0-9]*(\.[0-9]*)?|0\.[1-9][0-9]*)%$`
	InodesThreshold *string `json:"inodesThreshold,omitempty"`

	// Increase specifies the expansion amount (percentage or absolute)
	// +optional
	// +kubebuilder:validation:Pattern=`^([1-9][0-9]*(\.[0-9]+)?%|0\.[1-9][0-9]*%|[0-9]+(\.[0-9]+)?[KMGTPE]i)$`
	Increase *string `json:"increase,omitempty"`

	// MaxSize is the maximum size limit for the PVC
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`

	// MinScaleUp is the minimum expansion amount
	// +optional
	MinScaleUp *resource.Quantity `json:"minScaleUp,omitempty"`

	// Cooldown is the minimum time between expansions
	// +optional
	Cooldown *metav1.Duration `json:"cooldown,omitempty"`
}

// PVCPolicyStatus defines the observed state of PVCPolicy
// +kubebuilder:object:generate=true
type PVCPolicyStatus struct {
	// MatchedPVCs is the number of PVCs currently matched by this policy
	// +optional
	MatchedPVCs int32 `json:"matchedPVCs,omitempty"`

	// LastUpdated is the timestamp when the policy was last processed
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

// PVCPolicy is the Schema for the pvcpolicies API
type PVCPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PVCPolicySpec   `json:"spec,omitempty"`
	Status PVCPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type PVCPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PVCPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PVCPolicy{}, &PVCPolicyList{})
}
