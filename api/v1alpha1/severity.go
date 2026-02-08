package v1alpha1

// Severity indicates how a check result affects overall cluster readiness.
// +kubebuilder:validation:Enum=critical;warning;info
type Severity string

const (
	// SeverityCritical indicates the check blocks cluster readiness when failing.
	SeverityCritical Severity = "critical"

	// SeverityWarning indicates the check is reported but does not block readiness.
	SeverityWarning Severity = "warning"

	// SeverityInfo indicates the check is purely diagnostic.
	SeverityInfo Severity = "info"
)
