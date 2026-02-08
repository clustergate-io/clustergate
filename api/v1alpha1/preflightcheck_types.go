package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PreflightCheckSpec defines a dynamic readiness check.
type PreflightCheckSpec struct {
	// Description is a human-readable summary of what this check verifies.
	// +optional
	Description string `json:"description,omitempty"`

	// Severity determines how this check affects overall readiness.
	// +kubebuilder:default=critical
	Severity Severity `json:"severity"`

	// Category groups this check for filtering and reporting.
	// +kubebuilder:default=custom
	Category string `json:"category"`

	// Interval overrides the default check interval from ClusterReadiness.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Exactly one of the following check types must be specified.

	// PodCheck verifies that pods matching criteria exist and are ready.
	// +optional
	PodCheck *PodCheckSpec `json:"podCheck,omitempty"`

	// HTTPCheck performs an HTTP request and validates the response.
	// +optional
	HTTPCheck *HTTPCheckSpec `json:"httpCheck,omitempty"`

	// ResourceCheck verifies that a Kubernetes resource has expected conditions.
	// +optional
	ResourceCheck *ResourceCheckSpec `json:"resourceCheck,omitempty"`

	// PromQLCheck queries a Prometheus instance and evaluates the result.
	// +optional
	PromQLCheck *PromQLCheckSpec `json:"promqlCheck,omitempty"`

	// ScriptCheck runs a script as a Kubernetes Job and uses the exit code as the result.
	// +optional
	ScriptCheck *ScriptCheckSpec `json:"scriptCheck,omitempty"`
}

// PodCheckSpec checks for pod existence and readiness.
type PodCheckSpec struct {
	// Namespace to search for pods.
	Namespace string `json:"namespace"`

	// LabelSelector to match pods.
	LabelSelector *metav1.LabelSelector `json:"labelSelector"`

	// MinReady is the minimum number of pods that must be Running and Ready.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	MinReady int32 `json:"minReady"`
}

// HTTPCheckSpec performs an HTTP health check.
type HTTPCheckSpec struct {
	// URL to send the request to.
	URL string `json:"url"`

	// Method is the HTTP method. Defaults to GET.
	// +kubebuilder:default=GET
	// +optional
	Method string `json:"method,omitempty"`

	// ExpectedStatusCodes is the set of acceptable HTTP response codes.
	// Defaults to [200].
	// +optional
	ExpectedStatusCodes []int `json:"expectedStatusCodes,omitempty"`

	// TimeoutSeconds is the per-request timeout. Defaults to 10.
	// +kubebuilder:default=10
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// Headers to include in the request.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// InsecureSkipTLSVerify disables TLS certificate verification.
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`
}

// ResourceCheckSpec verifies conditions on a Kubernetes resource.
type ResourceCheckSpec struct {
	// APIVersion of the resource (e.g. "apps/v1").
	APIVersion string `json:"apiVersion"`

	// Kind of the resource (e.g. "Deployment").
	Kind string `json:"kind"`

	// Namespace of the resource. Empty for cluster-scoped resources.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of a specific resource. Mutually exclusive with labelSelector.
	// +optional
	Name string `json:"name,omitempty"`

	// LabelSelector to match resources. Mutually exclusive with name.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Conditions to verify on the matched resource(s).
	Conditions []ResourceConditionCheck `json:"conditions"`
}

// ResourceConditionCheck specifies an expected condition on a resource.
type ResourceConditionCheck struct {
	// Type is the condition type to check (e.g. "Available", "Ready").
	Type string `json:"type"`

	// Status is the expected condition status: "True", "False", or "Unknown".
	// +kubebuilder:validation:Enum=True;False;Unknown
	Status string `json:"status"`
}

// PromQLCheckSpec queries a Prometheus instance and evaluates the result.
type PromQLCheckSpec struct {
	// Endpoint is the Prometheus server URL (e.g. "http://prometheus.monitoring.svc:9090").
	Endpoint string `json:"endpoint"`

	// Query is the PromQL expression to execute.
	Query string `json:"query"`

	// Condition defines how to evaluate the query result.
	Condition PromQLCondition `json:"condition"`

	// TimeoutSeconds is the query timeout. Defaults to 10.
	// +kubebuilder:default=10
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

// PromQLCondition defines the assertion to apply to query results.
type PromQLCondition struct {
	// Type is the condition type: "resultCount" or "value".
	// resultCount asserts on the number of time series returned.
	// value asserts that all returned sample values satisfy the comparison.
	// +kubebuilder:validation:Enum=resultCount;value
	Type string `json:"type"`

	// Operator is the comparison operator: gte, lte, eq, gt, lt.
	// +kubebuilder:validation:Enum=gte;lte;eq;gt;lt
	Operator string `json:"operator"`

	// Threshold is the value to compare against.
	Threshold float64 `json:"threshold"`
}

// ScriptCheckSpec runs a script as a Kubernetes Job and interprets the exit code.
// Exit code 0 means Ready, non-zero means not Ready.
type ScriptCheckSpec struct {
	// Image is the container image to run (e.g., "busybox:latest").
	Image string `json:"image"`

	// Command is the entrypoint for the container (e.g., ["sh", "-c"]).
	Command []string `json:"command"`

	// Args are arguments to the command.
	// +optional
	Args []string `json:"args,omitempty"`

	// TimeoutSeconds is how long to wait for the Job to complete. Defaults to 30.
	// +kubebuilder:default=30
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// ServiceAccountName to use for the Job pod.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Env variables to set in the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Volumes to attach to the Job pod.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// VolumeMounts for the script container.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// PreflightCheckStatus reports the observed state of a PreflightCheck.
type PreflightCheckStatus struct {
	// Valid indicates whether this check definition is well-formed and executable.
	Valid bool `json:"valid"`

	// Message describes the validation result.
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=pc
// +kubebuilder:printcolumn:name="Severity",type=string,JSONPath=`.spec.severity`
// +kubebuilder:printcolumn:name="Category",type=string,JSONPath=`.spec.category`
// +kubebuilder:printcolumn:name="Valid",type=boolean,JSONPath=`.status.valid`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PreflightCheck defines a dynamic readiness check that can be created without recompiling.
type PreflightCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PreflightCheckSpec   `json:"spec,omitempty"`
	Status            PreflightCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PreflightCheckList contains a list of PreflightCheck.
type PreflightCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PreflightCheck `json:"items"`
}
