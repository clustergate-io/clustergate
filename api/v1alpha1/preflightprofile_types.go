package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PreflightProfileSpec defines a reusable bundle of readiness checks.
type PreflightProfileSpec struct {
	// Description is a human-readable summary of what this profile covers.
	// +optional
	Description string `json:"description,omitempty"`

	// Checks is the list of check references included in this profile.
	Checks []ProfileCheckRef `json:"checks"`
}

// ProfileCheckRef references either a built-in check or a PreflightCheck CR,
// with optional overrides for severity, category, interval, and enabled state.
type ProfileCheckRef struct {
	// Name is the identifier for a built-in check (e.g. "dns").
	// Mutually exclusive with PreflightCheckRef.
	// +optional
	Name string `json:"name,omitempty"`

	// PreflightCheckRef references a PreflightCheck CR by metadata.name.
	// Mutually exclusive with Name.
	// +optional
	PreflightCheckRef string `json:"preflightCheckRef,omitempty"`

	// Severity overrides the check's default severity.
	// +optional
	Severity *Severity `json:"severity,omitempty"`

	// Category overrides the check's default category.
	// +optional
	Category string `json:"category,omitempty"`

	// Interval overrides the default interval for this check.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Enabled controls whether this check is active. Defaults to true.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Config holds check-specific configuration. Only applicable for built-in checks.
	// +optional
	Config *apiextensionsv1.JSON `json:"config,omitempty"`
}

// IsEnabled returns true if the check ref is enabled (defaults to true if not set).
func (r *ProfileCheckRef) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

// Identifier returns a unique string for this check reference.
func (r *ProfileCheckRef) Identifier() string {
	if r.PreflightCheckRef != "" {
		return "dynamic:" + r.PreflightCheckRef
	}
	return r.Name
}

// PreflightProfileStatus reports the observed state of a PreflightProfile.
type PreflightProfileStatus struct {
	// Valid indicates whether all referenced checks exist and the profile is well-formed.
	Valid bool `json:"valid"`

	// CheckCount is the number of checks included in this profile.
	// +optional
	CheckCount int `json:"checkCount,omitempty"`

	// Message describes the validation result.
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=pp
// +kubebuilder:printcolumn:name="Checks",type=integer,JSONPath=`.status.checkCount`
// +kubebuilder:printcolumn:name="Valid",type=boolean,JSONPath=`.status.valid`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PreflightProfile defines a reusable bundle of readiness checks.
type PreflightProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PreflightProfileSpec   `json:"spec,omitempty"`
	Status            PreflightProfileStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PreflightProfileList contains a list of PreflightProfile.
type PreflightProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PreflightProfile `json:"items"`
}
