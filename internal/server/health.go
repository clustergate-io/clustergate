package server

import (
	"encoding/json"
	"net/http"
	"sync"
)

// ReadinessState holds the latest readiness status, updated by the controller.
type ReadinessState struct {
	mu     sync.RWMutex
	states map[string]*ClusterState // keyed by ClusterReadiness CR name
}

// ClusterState represents readiness for a single ClusterReadiness CR.
type ClusterState struct {
	State             string                 `json:"state"`
	Summary           *ReadinessSummaryView  `json:"summary,omitempty"`
	CategorySummaries []CategorySummaryView  `json:"categorySummaries,omitempty"`
	Checks            map[string]*CheckState `json:"checks,omitempty"`
}

// ReadinessSummaryView provides aggregated check counts for the HTTP response.
type ReadinessSummaryView struct {
	Total           int `json:"total"`
	Passing         int `json:"passing"`
	Failing         int `json:"failing"`
	CriticalTotal   int `json:"criticalTotal"`
	CriticalPassing int `json:"criticalPassing"`
	WarningFailing  int `json:"warningFailing"`
}

// CategorySummaryView provides per-category check counts for the HTTP response.
type CategorySummaryView struct {
	Category string `json:"category"`
	Ready    bool   `json:"ready"`
	Total    int    `json:"total"`
	Passing  int    `json:"passing"`
	Failing  int    `json:"failing"`
}

// CheckState represents readiness for a single check.
type CheckState struct {
	Ready    bool   `json:"ready"`
	Message  string `json:"message,omitempty"`
	Severity string `json:"severity"`
	Category string `json:"category"`
}

// NewReadinessState creates a new ReadinessState store.
func NewReadinessState() *ReadinessState {
	return &ReadinessState{
		states: make(map[string]*ClusterState),
	}
}

// Update sets the readiness state for a given ClusterReadiness CR.
func (rs *ReadinessState) Update(name string, state string, checks map[string]*CheckState, summary *ReadinessSummaryView, categorySummaries []CategorySummaryView) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.states[name] = &ClusterState{
		State:             state,
		Summary:           summary,
		CategorySummaries: categorySummaries,
		Checks:            checks,
	}
}

// Remove deletes the readiness state for a given ClusterReadiness CR.
func (rs *ReadinessState) Remove(name string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	delete(rs.states, name)
}

// IsReady returns true if all tracked ClusterReadiness CRs are ready.
func (rs *ReadinessState) IsReady() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if len(rs.states) == 0 {
		return false
	}
	for _, state := range rs.states {
		if state.State == "Unhealthy" {
			return false
		}
	}
	return true
}

// snapshot returns a copy of the current state for serialization.
func (rs *ReadinessState) snapshot() map[string]*ClusterState {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	snap := make(map[string]*ClusterState, len(rs.states))
	for k, v := range rs.states {
		snap[k] = v
	}
	return snap
}

// ReadyzHandler returns an HTTP handler for the /readyz endpoint.
// Returns 200 if all clusters are ready, 503 otherwise.
// Supports query parameters:
//
//	category - filter checks by category
//	severity - filter checks by severity
func ReadyzHandler(state *ReadinessState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap := state.snapshot()
		categoryFilter := r.URL.Query().Get("category")
		severityFilter := r.URL.Query().Get("severity")

		// Apply filters if present
		if categoryFilter != "" || severityFilter != "" {
			snap = filterSnapshot(snap, categoryFilter, severityFilter)
		}

		healthy := len(snap) > 0
		for _, cs := range snap {
			if cs.State == "Unhealthy" {
				healthy = false
				break
			}
		}

		resp := struct {
			State    string                   `json:"state"`
			Clusters map[string]*ClusterState `json:"clusters,omitempty"`
		}{
			Clusters: snap,
		}
		if !healthy {
			resp.State = "Unhealthy"
		} else {
			// Use worst state across clusters
			resp.State = "Healthy"
			for _, cs := range snap {
				if cs.State == "Degraded" {
					resp.State = "Degraded"
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if healthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(resp)
	}
}

// filterSnapshot creates a filtered copy of the snapshot based on category and severity.
func filterSnapshot(snap map[string]*ClusterState, categoryFilter, severityFilter string) map[string]*ClusterState {
	filtered := make(map[string]*ClusterState, len(snap))

	for crName, cs := range snap {
		filteredChecks := make(map[string]*CheckState)
		for checkName, check := range cs.Checks {
			if categoryFilter != "" && check.Category != categoryFilter {
				continue
			}
			if severityFilter != "" && check.Severity != severityFilter {
				continue
			}
			filteredChecks[checkName] = check
		}

		// Recompute state from filtered checks
		state := "Healthy"
		for _, check := range filteredChecks {
			if !check.Ready && check.Severity == "critical" {
				state = "Unhealthy"
				break
			}
			if !check.Ready && check.Severity == "warning" {
				state = "Degraded"
			}
		}
		if len(filteredChecks) == 0 {
			state = "Unhealthy"
		}

		filtered[crName] = &ClusterState{
			State:  state,
			Checks: filteredChecks,
		}
	}

	return filtered
}
