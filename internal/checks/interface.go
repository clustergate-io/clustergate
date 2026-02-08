package checks

import (
	"context"
	"encoding/json"
)

// Checker is the interface that all readiness checks must implement.
type Checker interface {
	// Name returns the unique identifier for this check (e.g. "dns", "ingress").
	Name() string

	// DefaultSeverity returns the check's default severity level ("critical", "warning", "info").
	DefaultSeverity() string

	// DefaultCategory returns the check's default category (e.g. "networking", "control-plane").
	DefaultCategory() string

	// Run executes the check and returns a Result.
	// The config parameter contains check-specific configuration from the CRD spec.
	Run(ctx context.Context, config json.RawMessage) (Result, error)
}

// Result holds the outcome of a single readiness check.
type Result struct {
	// Ready indicates whether the check is passing.
	Ready bool `json:"ready"`

	// Message is a human-readable summary of the result.
	Message string `json:"message"`

	// Details contains additional key-value diagnostic information.
	Details map[string]string `json:"details,omitempty"`
}
