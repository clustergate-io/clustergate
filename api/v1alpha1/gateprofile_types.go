package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GateProfileSpec defines the desired state of GateProfile.
type GateProfileSpec struct {
	// Description is a human-readable description of this profile.
	// +optional
	Description string `json:"description,omitempty"`

	// Checks is the list of check references included in this profile.
	Checks []ProfileCheckRef `json:"checks"`
}

// GateProfileStatus defines the observed state of GateProfile.
type GateProfileStatus struct {
	// Conditions represent the latest available observations of the GateProfile's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=gp
// +kubebuilder:printcolumn:name="Checks",type=integer,JSONPath=`.spec.checks`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GateProfile is the Schema for the gateprofiles API.
type GateProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GateProfileSpec   `json:"spec,omitempty"`
	Status GateProfileStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GateProfileList contains a list of GateProfile.
type GateProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GateProfile `json:"items"`
}
