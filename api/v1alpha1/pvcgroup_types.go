package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCGroupSpec defines the desired state of PVCGroup
// +kubebuilder:object:generate=true
type PVCGroupSpec struct {
	// Template defines the expansion configuration for the group
	Template PVCGroupTemplate `json:"template"`
}

// PVCGroupTemplate defines the expansion configuration template for groups
// +kubebuilder:object:generate=true
// +kubebuilder:validation:MinProperties=1
type PVCGroupTemplate struct {
	// Enabled controls whether auto-expansion is enabled
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Threshold is the storage usage percentage that triggers expansion
	// +optional
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]*)?)%$`
	Threshold *string `json:"threshold,omitempty"`

	// InodesThreshold is the inode usage percentage that triggers expansion
	// +optional
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]*)?)%$`
	InodesThreshold *string `json:"inodesThreshold,omitempty"`

	// Increase specifies the expansion amount (percentage or absolute)
	// +optional
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]+)?%|[0-9]+(\.[0-9]+)?[KMGTPE]i)$`
	Increase *string `json:"increase,omitempty"`

	// MaxSize is the maximum size limit for PVCs in the group
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`

	// MinScaleUp is the minimum expansion amount
	// +optional
	MinScaleUp *resource.Quantity `json:"minScaleUp,omitempty"`

	// Cooldown is the minimum time between expansions
	// +optional
	Cooldown *metav1.Duration `json:"cooldown,omitempty"`
}

// PVCGroupStatus defines the observed state of PVCGroup
// +kubebuilder:object:generate=true
type PVCGroupStatus struct {
	// MemberCount is the number of PVCs currently in this group
	// +optional
	MemberCount int32 `json:"memberCount,omitempty"`

	// CurrentSize is the current coordinated size of PVCs in the group
	// +optional
	CurrentSize *resource.Quantity `json:"currentSize,omitempty"`

	// LastExpansion is the timestamp of the last expansion
	// +optional
	LastExpansion *metav1.Time `json:"lastExpansion,omitempty"`

	// LastUpdated is the timestamp when the group was last processed
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

// PVCGroup is the Schema for the pvcgroups API
type PVCGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PVCGroupSpec   `json:"spec,omitempty"`
	Status PVCGroupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type PVCGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PVCGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PVCGroup{}, &PVCGroupList{})
}
