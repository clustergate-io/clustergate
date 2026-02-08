package controller

import (
	"time"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
)

// CheckSchedule determines which resolved checks are due for execution based on
// their individual intervals and existing status timestamps.
// Returns the checks that need to run and the shortest remaining interval for requeue.
func CheckSchedule(resolved []ResolvedCheck, existingStatuses []clustergatev1alpha1.CheckStatus, now time.Time) (due []ResolvedCheck, carried []clustergatev1alpha1.CheckStatus, nextRequeue time.Duration) {
	// Build a lookup map from existing statuses
	statusMap := make(map[string]clustergatev1alpha1.CheckStatus, len(existingStatuses))
	for _, s := range existingStatuses {
		statusMap[s.Name] = s
	}

	nextRequeue = 0

	for _, rc := range resolved {
		existing, hasExisting := statusMap[rc.Identifier]

		if !hasExisting || existing.LastChecked == nil {
			// No prior result — must run
			due = append(due, rc)
			continue
		}

		elapsed := now.Sub(existing.LastChecked.Time)
		if elapsed >= rc.Interval {
			// Stale — must run
			due = append(due, rc)
			continue
		}

		// Not yet due — carry forward existing result
		carried = append(carried, existing)

		remaining := rc.Interval - elapsed
		if nextRequeue == 0 || remaining < nextRequeue {
			nextRequeue = remaining
		}
	}

	// If all checks are due (nothing carried forward), use the shortest interval
	if nextRequeue == 0 && len(resolved) > 0 {
		nextRequeue = shortestInterval(resolved)
	}

	return due, carried, nextRequeue
}

// shortestInterval returns the shortest interval from a set of resolved checks.
func shortestInterval(checks []ResolvedCheck) time.Duration {
	if len(checks) == 0 {
		return defaultInterval
	}
	shortest := checks[0].Interval
	for _, rc := range checks[1:] {
		if rc.Interval < shortest {
			shortest = rc.Interval
		}
	}
	return shortest
}
