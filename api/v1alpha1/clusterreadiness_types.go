package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterReadinessSpec defines the desired state of ClusterReadiness.
type ClusterReadinessSpec struct {
	// Interval is the default interval for checks that don't specify their own (e.g. "60s", "5m").
	// +optional
	Interval metav1.Duration `json:"interval,omitempty"`

	// Profiles references GateProfile CRs to include in this readiness evaluation.
	// +optional
	Profiles []ProfileRef `json:"profiles,omitempty"`

	// Checks is the list of inline readiness checks to run.
	// Inline checks override profile checks with the same name/ref.
	// +optional
	Checks []CheckSpec `json:"checks,omitempty"`
}

// ProfileRef references a GateProfile CR by name.
type ProfileRef struct {
	// Name is the metadata.name of the GateProfile CR.
	Name string `json:"name"`

	// ExcludeChecks is a list of check names or gateCheckRefs to exclude from this profile.
	// +optional
	ExcludeChecks []string `json:"excludeChecks,omitempty"`
}

// CheckSpec defines a single readiness check to run.
type CheckSpec struct {
	// Name is the identifier for a built-in check (e.g. "dns").
	// Mutually exclusive with GateCheckRef.
	// +optional
	Name string `json:"name,omitempty"`

	// GateCheckRef references a GateCheck CR by metadata.name.
	// Mutually exclusive with Name.
	// +optional
	GateCheckRef string `json:"gateCheckRef,omitempty"`

	// Severity overrides the check's default severity.
	// Defaults to "critical" for built-in checks, or the GateCheck's severity.
	// +optional
	Severity *Severity `json:"severity,omitempty"`

	// Category overrides the check's default category.
	// +optional
	Category string `json:"category,omitempty"`

	// Interval overrides the default interval for this specific check.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Enabled controls whether this check is active.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Config holds check-specific configuration as arbitrary JSON.
	// Only applicable for built-in checks.
	// +optional
	Config *apiextensionsv1.JSON `json:"config,omitempty"`
}

// IsEnabled returns true if the check is enabled (defaults to true if not set).
func (c *CheckSpec) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// ClusterReadinessStatus defines the observed state of ClusterReadiness.
type ClusterReadinessStatus struct {
	// Ready indicates whether all critical-severity checks are passing.
	Ready bool `json:"ready"`

	// Summary provides aggregated counts across all checks.
	// +optional
	Summary *ReadinessSummary `json:"summary,omitempty"`

	// CategorySummaries provides per-category aggregation.
	// +optional
	CategorySummaries []CategorySummary `json:"categorySummaries,omitempty"`

	// LastChecked is the last time any check was evaluated.
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`

	// Checks contains the per-check results.
	// +optional
	Checks []CheckStatus `json:"checks,omitempty"`

	// Conditions represent the latest available observations of the resource's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ReadinessSummary aggregates check results across the entire ClusterReadiness.
type ReadinessSummary struct {
	// Total is the total number of enabled checks.
	Total int `json:"total"`

	// Passing is the number of checks currently passing.
	Passing int `json:"passing"`

	// Failing is the number of checks currently failing.
	Failing int `json:"failing"`

	// CriticalTotal is the number of critical-severity checks.
	CriticalTotal int `json:"criticalTotal"`

	// CriticalPassing is the number of critical checks currently passing.
	CriticalPassing int `json:"criticalPassing"`

	// WarningTotal is the number of warning-severity checks.
	WarningTotal int `json:"warningTotal"`

	// WarningFailing is the number of warning checks currently failing.
	WarningFailing int `json:"warningFailing"`
}

// CategorySummary aggregates check results for one category.
type CategorySummary struct {
	// Category name.
	Category string `json:"category"`

	// Ready indicates all critical checks in this category are passing.
	Ready bool `json:"ready"`

	// Total number of checks in this category.
	Total int `json:"total"`

	// Passing checks in this category.
	Passing int `json:"passing"`

	// Failing checks in this category.
	Failing int `json:"failing"`
}

// CheckStatus reports the result of a single readiness check.
type CheckStatus struct {
	// Name matches the check identifier (built-in name or GateCheck ref).
	Name string `json:"name"`

	// Source indicates where this check originated: "builtin", "dynamic", or "profile:<name>".
	// +optional
	Source string `json:"source,omitempty"`

	// Ready indicates whether this check is passing.
	Ready bool `json:"ready"`

	// Severity of this check.
	Severity Severity `json:"severity"`

	// Category of this check.
	Category string `json:"category"`

	// Message is a human-readable description of the check result.
	// +optional
	Message string `json:"message,omitempty"`

	// LastChecked is when this check was last evaluated.
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Passing",type=integer,JSONPath=`.status.summary.passing`
// +kubebuilder:printcolumn:name="Failing",type=integer,JSONPath=`.status.summary.failing`
// +kubebuilder:printcolumn:name="Total",type=integer,JSONPath=`.status.summary.total`
// +kubebuilder:printcolumn:name="Last Checked",type=date,JSONPath=`.status.lastChecked`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterReadiness is the Schema for the clusterreadiness API.
type ClusterReadiness struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterReadinessSpec   `json:"spec,omitempty"`
	Status ClusterReadinessStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterReadinessList contains a list of ClusterReadiness.
type ClusterReadinessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterReadiness `json:"items"`
}
