package cli

import (
	"context"
	"sort"

	"github.com/clustergate/clustergate/internal/checks"
)

// CheckResult holds a single check's outcome.
type CheckResult struct {
	Name     string            `json:"name"`
	Category string            `json:"category"`
	Severity string            `json:"severity"`
	Status   string            `json:"status"`
	Message  string            `json:"message"`
	Details  map[string]string `json:"details,omitempty"`
}

// CheckError captures a check that returned an execution error.
type CheckError struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

// Report holds the aggregate result of running all checks.
type Report struct {
	State  string        `json:"state"`
	Total  int           `json:"total"`
	Passed int           `json:"passed"`
	Failed int           `json:"failed"`
	Checks []CheckResult `json:"checks"`
	Errors []CheckError  `json:"errors,omitempty"`
}

// RunChecks executes the given checkers and returns a Report.
// If filter is non-empty, only checks whose names are in filter are executed.
func RunChecks(ctx context.Context, checkers []checks.Checker, filter map[string]bool) *Report {
	report := &Report{State: "Healthy"}

	// Sort checkers by name for deterministic output.
	sorted := make([]checks.Checker, len(checkers))
	copy(sorted, checkers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name() < sorted[j].Name()
	})

	hasCriticalFailure := false
	hasWarningFailure := false

	for _, c := range sorted {
		if len(filter) > 0 && !filter[c.Name()] {
			continue
		}

		report.Total++

		result, err := c.Run(ctx, nil)
		if err != nil {
			report.Errors = append(report.Errors, CheckError{
				Name:  c.Name(),
				Error: err.Error(),
			})
			report.Failed++
			hasCriticalFailure = true
			continue
		}

		report.Checks = append(report.Checks, CheckResult{
			Name:     c.Name(),
			Category: c.DefaultCategory(),
			Severity: c.DefaultSeverity(),
			Status:   statusStr(result.Ready),
			Message:  result.Message,
			Details:  result.Details,
		})

		if result.Ready {
			report.Passed++
		} else {
			report.Failed++
			if c.DefaultSeverity() == "critical" {
				hasCriticalFailure = true
			} else if c.DefaultSeverity() == "warning" {
				hasWarningFailure = true
			}
		}
	}

	// Compute the health state
	if hasCriticalFailure {
		report.State = "Unhealthy"
	} else if hasWarningFailure {
		report.State = "Degraded"
	} else {
		report.State = "Healthy"
	}

	return report
}

// statusStr converts a ready bool to a human-readable status string.
func statusStr(ready bool) string {
	if ready {
		return "Passing"
	}
	return "Failing"
}
