package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GateCheckSpec defines the desired state of GateCheck.
// Exactly one check type must be specified.
type GateCheckSpec struct {
	// Description is a human-readable description of what this check validates.
	// +optional
	Description string `json:"description,omitempty"`

	// Severity indicates how a failing result affects cluster readiness.
	// +optional
	// +kubebuilder:default=critical
	Severity Severity `json:"severity,omitempty"`

	// Category groups related checks for filtering and reporting.
	// +optional
	Category string `json:"category,omitempty"`

	// Interval overrides the default check interval.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// PodCheck verifies that pods matching a label selector are running and ready.
	// +optional
	PodCheck *PodCheckSpec `json:"podCheck,omitempty"`

	// HTTPCheck performs an HTTP request and validates the response status code.
	// +optional
	HTTPCheck *HTTPCheckSpec `json:"httpCheck,omitempty"`

	// ResourceCheck asserts conditions on any Kubernetes resource.
	// +optional
	ResourceCheck *ResourceCheckSpec `json:"resourceCheck,omitempty"`

	// PromQLCheck queries a Prometheus endpoint and evaluates the result.
	// +optional
	PromQLCheck *PromQLCheckSpec `json:"promqlCheck,omitempty"`

	// ScriptCheck runs a custom script as a Kubernetes Job.
	// +optional
	ScriptCheck *ScriptCheckSpec `json:"scriptCheck,omitempty"`
}

// GateCheckStatus defines the observed state of GateCheck.
type GateCheckStatus struct {
	// Conditions represent the latest available observations of the GateCheck's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=gchk
// +kubebuilder:printcolumn:name="Severity",type=string,JSONPath=`.spec.severity`
// +kubebuilder:printcolumn:name="Category",type=string,JSONPath=`.spec.category`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.description`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GateCheck is the Schema for the gatechecks API.
type GateCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GateCheckSpec   `json:"spec,omitempty"`
	Status GateCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GateCheckList contains a list of GateCheck.
type GateCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GateCheck `json:"items"`
}

// --- Dynamic check type specs ---

// PodCheckSpec defines a check that verifies pods matching a label selector are running and ready.
type PodCheckSpec struct {
	// Namespace to search for pods.
	Namespace string `json:"namespace"`

	// LabelSelector selects the pods to check.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// MinReady is the minimum number of ready pods required for the check to pass.
	// +optional
	// +kubebuilder:default=1
	MinReady int32 `json:"minReady,omitempty"`
}

// HTTPCheckSpec defines a check that performs an HTTP request and validates the response.
type HTTPCheckSpec struct {
	// URL is the HTTP endpoint to probe.
	URL string `json:"url"`

	// Method is the HTTP method to use.
	// +optional
	// +kubebuilder:default=GET
	Method string `json:"method,omitempty"`

	// ExpectedStatusCodes is the list of acceptable HTTP status codes.
	// +optional
	ExpectedStatusCodes []int `json:"expectedStatusCodes,omitempty"`

	// TimeoutSeconds is the request timeout.
	// +optional
	// +kubebuilder:default=10
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// InsecureSkipTLSVerify disables TLS certificate verification.
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`

	// Headers to include in the request.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`
}

// ResourceCheckSpec defines a check that asserts conditions on a Kubernetes resource.
type ResourceCheckSpec struct {
	// APIVersion of the resource (e.g. "apps/v1").
	APIVersion string `json:"apiVersion"`

	// Kind of the resource (e.g. "Deployment").
	Kind string `json:"kind"`

	// Namespace of the resource. Empty for cluster-scoped resources.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the resource. Mutually exclusive with LabelSelector.
	// +optional
	Name string `json:"name,omitempty"`

	// LabelSelector selects resources to check. Mutually exclusive with Name.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Conditions to assert on the resource.
	Conditions []ResourceConditionCheck `json:"conditions"`
}

// ResourceConditionCheck defines an expected condition on a resource.
type ResourceConditionCheck struct {
	// Type is the condition type to check.
	Type string `json:"type"`

	// Status is the expected condition status (e.g. "True", "False").
	Status string `json:"status"`
}

// PromQLCheckSpec defines a check that queries Prometheus and evaluates the result.
type PromQLCheckSpec struct {
	// Endpoint is the Prometheus server URL.
	Endpoint string `json:"endpoint"`

	// Query is the PromQL expression to evaluate.
	Query string `json:"query"`

	// Condition defines how to evaluate the query result.
	Condition PromQLCondition `json:"condition"`

	// TimeoutSeconds is the query timeout.
	// +optional
	// +kubebuilder:default=10
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

// PromQLCondition defines how to evaluate a PromQL query result.
type PromQLCondition struct {
	// Type is either "resultCount" or "value".
	// +kubebuilder:validation:Enum=resultCount;value
	Type string `json:"type"`

	// Operator is the comparison operator: gte, lte, eq, gt, lt.
	// +kubebuilder:validation:Enum=gte;lte;eq;gt;lt
	Operator string `json:"operator"`

	// Threshold is the value to compare against.
	Threshold float64 `json:"threshold"`
}

// ScriptCheckSpec defines a check that runs a script as a Kubernetes Job.
type ScriptCheckSpec struct {
	// Image is the container image to run.
	Image string `json:"image"`

	// Command is the entrypoint for the container.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args are the arguments to the entrypoint.
	// +optional
	Args []string `json:"args,omitempty"`

	// TimeoutSeconds is the maximum time the job may run.
	// +optional
	// +kubebuilder:default=30
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// ServiceAccountName for the job pod.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Env is a list of environment variables for the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// --- ProfileCheckRef for GateProfile ---

// ProfileCheckRef is a reference to a built-in or dynamic check within a GateProfile.
type ProfileCheckRef struct {
	// Name is the identifier for a built-in check (e.g. "dns").
	// Mutually exclusive with GateCheckRef.
	// +optional
	Name string `json:"name,omitempty"`

	// GateCheckRef references a GateCheck CR by metadata.name.
	// Mutually exclusive with Name.
	// +optional
	GateCheckRef string `json:"gateCheckRef,omitempty"`

	// Severity overrides the check's default severity.
	// +optional
	Severity *Severity `json:"severity,omitempty"`

	// Category overrides the check's default category.
	// +optional
	Category string `json:"category,omitempty"`

	// Interval overrides the default check interval.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Enabled controls whether this check is active.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Config holds check-specific configuration as arbitrary JSON.
	// +optional
	Config *apiextensionsv1.JSON `json:"config,omitempty"`
}

// Identifier returns a unique key for this check reference.
func (r *ProfileCheckRef) Identifier() string {
	if r.GateCheckRef != "" {
		return "dynamic:" + r.GateCheckRef
	}
	return r.Name
}

// IsEnabled returns true if the check is enabled (defaults to true if not set).
func (r *ProfileCheckRef) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}
