package v1alpha1

// ClusterHealthState represents the overall health of the cluster.
// +kubebuilder:validation:Enum=Healthy;Degraded;Unhealthy
type ClusterHealthState string

const (
	// ClusterHealthy indicates all checks (critical and warning) are passing.
	ClusterHealthy ClusterHealthState = "Healthy"

	// ClusterDegraded indicates all critical checks pass but one or more warning checks are failing.
	ClusterDegraded ClusterHealthState = "Degraded"

	// ClusterUnhealthy indicates one or more critical checks are failing.
	ClusterUnhealthy ClusterHealthState = "Unhealthy"
)
