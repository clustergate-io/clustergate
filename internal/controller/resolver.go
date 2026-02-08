package controller

import (
	"context"
	"encoding/json"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
	"github.com/clustergate/clustergate/internal/checks"
)

// ResolvedCheck is the fully-resolved, flat representation of a check to execute.
type ResolvedCheck struct {
	// Identifier is the unique key: "dns" for built-ins, "dynamic:name" for dynamic.
	Identifier string

	// IsBuiltin is true for compiled built-in checks.
	IsBuiltin bool

	// BuiltinName is the registered name of a built-in check.
	BuiltinName string

	// GateCheckName is the metadata.name of the GateCheck CR.
	GateCheckName string

	// Severity is the resolved severity for this check.
	Severity string

	// Category is the resolved category for this check.
	Category string

	// Interval is the resolved interval for this check.
	Interval time.Duration

	// Config is raw JSON configuration for built-in checks.
	Config json.RawMessage

	// Source tracks where this check originated: "inline" or "profile:<name>".
	Source string
}

// ResolveChecks resolves profiles and inline checks into a flat list of checks to execute.
// Merge semantics:
// 1. Profiles processed in listing order; later profiles override earlier for same identifier
// 2. Inline spec.checks[] override any profile-sourced check with same identifier
func ResolveChecks(ctx context.Context, c client.Client, spec clustergatev1alpha1.ClusterReadinessSpec, defaultInterval time.Duration) ([]ResolvedCheck, error) {
	resolved := make(map[string]ResolvedCheck)

	// Process profiles in order
	for _, profileRef := range spec.Profiles {
		var profile clustergatev1alpha1.GateProfile
		if err := c.Get(ctx, types.NamespacedName{Name: profileRef.Name}, &profile); err != nil {
			return nil, err
		}

		for _, checkRef := range profile.Spec.Checks {
			if !checkRef.IsEnabled() {
				// Explicitly disabled in profile — remove if previously added
				delete(resolved, checkRef.Identifier())
				continue
			}

			rc := resolveProfileCheckRef(checkRef, profile.Name, defaultInterval)
			resolved[rc.Identifier] = rc
		}
	}

	// Process inline checks — these override profile entries with same identifier
	for _, cs := range spec.Checks {
		if !cs.IsEnabled() {
			// Explicitly disabled inline — remove if previously added
			id := inlineIdentifier(cs)
			delete(resolved, id)
			continue
		}

		rc := resolveInlineCheck(cs, defaultInterval)

		// If overriding a profile entry, preserve defaults that aren't overridden
		if existing, ok := resolved[rc.Identifier]; ok {
			rc = mergeOverrides(existing, rc)
		}

		resolved[rc.Identifier] = rc
	}

	// Convert to slice
	result := make([]ResolvedCheck, 0, len(resolved))
	for _, rc := range resolved {
		result = append(result, rc)
	}
	return result, nil
}

// resolveProfileCheckRef converts a profile check reference to a ResolvedCheck.
func resolveProfileCheckRef(ref clustergatev1alpha1.ProfileCheckRef, profileName string, defaultInterval time.Duration) ResolvedCheck {
	rc := ResolvedCheck{
		Source:   "profile:" + profileName,
		Interval: defaultInterval,
	}

	if ref.GateCheckRef != "" {
		rc.Identifier = "dynamic:" + ref.GateCheckRef
		rc.IsBuiltin = false
		rc.GateCheckName = ref.GateCheckRef
	} else {
		rc.Identifier = ref.Name
		rc.IsBuiltin = true
		rc.BuiltinName = ref.Name
	}

	if ref.Severity != nil {
		rc.Severity = string(*ref.Severity)
	}
	rc.Category = ref.Category

	if ref.Interval != nil && ref.Interval.Duration > 0 {
		rc.Interval = ref.Interval.Duration
	}

	if ref.Config != nil {
		rc.Config = ref.Config.Raw
	}

	return rc
}

// resolveInlineCheck converts an inline CheckSpec to a ResolvedCheck.
func resolveInlineCheck(cs clustergatev1alpha1.CheckSpec, defaultInterval time.Duration) ResolvedCheck {
	rc := ResolvedCheck{
		Source:   "inline",
		Interval: defaultInterval,
	}

	if cs.GateCheckRef != "" {
		rc.Identifier = "dynamic:" + cs.GateCheckRef
		rc.IsBuiltin = false
		rc.GateCheckName = cs.GateCheckRef
	} else {
		rc.Identifier = cs.Name
		rc.IsBuiltin = true
		rc.BuiltinName = cs.Name
	}

	if cs.Severity != nil {
		rc.Severity = string(*cs.Severity)
	}
	rc.Category = cs.Category

	if cs.Interval != nil && cs.Interval.Duration > 0 {
		rc.Interval = cs.Interval.Duration
	}

	if cs.Config != nil {
		rc.Config = cs.Config.Raw
	}

	return rc
}

// mergeOverrides merges an inline override onto an existing profile entry.
// Empty fields in the override are filled with the profile entry's values.
func mergeOverrides(base, override ResolvedCheck) ResolvedCheck {
	if override.Severity == "" {
		override.Severity = base.Severity
	}
	if override.Category == "" {
		override.Category = base.Category
	}
	if override.Config == nil {
		override.Config = base.Config
	}
	return override
}

// inlineIdentifier computes the identifier for an inline CheckSpec.
func inlineIdentifier(cs clustergatev1alpha1.CheckSpec) string {
	if cs.GateCheckRef != "" {
		return "dynamic:" + cs.GateCheckRef
	}
	return cs.Name
}

// ResolveSeverityAndCategory resolves final severity and category for a check,
// falling back to checker defaults for built-ins or GateCheck defaults for dynamic.
func ResolveSeverityAndCategory(rc ResolvedCheck, ctx context.Context, c client.Client) (string, string) {
	sev := rc.Severity
	cat := rc.Category

	if rc.IsBuiltin {
		checker, ok := checks.Get(rc.BuiltinName)
		if ok {
			if sev == "" {
				sev = checker.DefaultSeverity()
			}
			if cat == "" {
				cat = checker.DefaultCategory()
			}
		} else {
			if sev == "" {
				sev = "critical"
			}
			if cat == "" {
				cat = "general"
			}
		}
	} else {
		// For dynamic checks, fetch the GateCheck CR for defaults
		var gc clustergatev1alpha1.GateCheck
		if err := c.Get(ctx, types.NamespacedName{Name: rc.GateCheckName}, &gc); err == nil {
			if sev == "" {
				sev = string(gc.Spec.Severity)
			}
			if cat == "" {
				cat = gc.Spec.Category
			}
		}
		if sev == "" {
			sev = "critical"
		}
		if cat == "" {
			cat = "custom"
		}
	}

	return sev, cat
}
