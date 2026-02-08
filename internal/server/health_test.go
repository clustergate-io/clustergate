package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadinessState_IsReady(t *testing.T) {
	tests := []struct {
		name  string
		setup func(rs *ReadinessState)
		want  bool
	}{
		{
			name:  "empty state is not ready",
			setup: func(rs *ReadinessState) {},
			want:  false,
		},
		{
			name: "single ready cluster",
			setup: func(rs *ReadinessState) {
				rs.Update("cluster-1", true, nil, nil, nil)
			},
			want: true,
		},
		{
			name: "two ready clusters",
			setup: func(rs *ReadinessState) {
				rs.Update("cluster-1", true, nil, nil, nil)
				rs.Update("cluster-2", true, nil, nil, nil)
			},
			want: true,
		},
		{
			name: "one ready one not ready",
			setup: func(rs *ReadinessState) {
				rs.Update("cluster-1", true, nil, nil, nil)
				rs.Update("cluster-2", false, nil, nil, nil)
			},
			want: false,
		},
		{
			name: "single not ready cluster",
			setup: func(rs *ReadinessState) {
				rs.Update("cluster-1", false, nil, nil, nil)
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := NewReadinessState()
			tt.setup(rs)
			if got := rs.IsReady(); got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadinessState_Remove(t *testing.T) {
	rs := NewReadinessState()
	rs.Update("cluster-1", true, nil, nil, nil)
	rs.Update("cluster-2", false, nil, nil, nil)

	// Not ready because cluster-2 is failing
	if rs.IsReady() {
		t.Error("expected not ready before remove")
	}

	rs.Remove("cluster-2")

	// Now ready because only cluster-1 (ready) remains
	if !rs.IsReady() {
		t.Error("expected ready after removing failing cluster")
	}

	rs.Remove("cluster-1")

	// Empty = not ready
	if rs.IsReady() {
		t.Error("expected not ready after removing all clusters")
	}
}

func TestReadyzHandler_Ready(t *testing.T) {
	rs := NewReadinessState()
	rs.Update("test-cluster", true, map[string]*CheckState{
		"dns": {Ready: true, Message: "ok", Severity: "critical", Category: "networking"},
	}, &ReadinessSummaryView{Total: 1, Passing: 1}, nil)

	handler := ReadyzHandler(rs)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Ready    bool                     `json:"ready"`
		Clusters map[string]*ClusterState `json:"clusters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Ready {
		t.Error("expected ready=true in response")
	}
	if len(resp.Clusters) != 1 {
		t.Errorf("expected 1 cluster, got %d", len(resp.Clusters))
	}
}

func TestReadyzHandler_NotReady(t *testing.T) {
	rs := NewReadinessState()
	rs.Update("test-cluster", false, map[string]*CheckState{
		"dns": {Ready: false, Message: "failing", Severity: "critical", Category: "networking"},
	}, nil, nil)

	handler := ReadyzHandler(rs)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp struct {
		Ready bool `json:"ready"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Ready {
		t.Error("expected ready=false in response")
	}
}

func TestReadyzHandler_Empty(t *testing.T) {
	rs := NewReadinessState()
	handler := ReadyzHandler(rs)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestReadyzHandler_CategoryFilter(t *testing.T) {
	rs := NewReadinessState()
	rs.Update("test-cluster", true, map[string]*CheckState{
		"dns":     {Ready: true, Message: "ok", Severity: "critical", Category: "networking"},
		"ingress": {Ready: false, Message: "failing", Severity: "critical", Category: "networking"},
		"vault":   {Ready: true, Message: "ok", Severity: "critical", Category: "security"},
	}, nil, nil)

	handler := ReadyzHandler(rs)
	req := httptest.NewRequest(http.MethodGet, "/readyz?category=security", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	// Security checks are all passing, so should be 200
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Ready    bool                     `json:"ready"`
		Clusters map[string]*ClusterState `json:"clusters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	cs := resp.Clusters["test-cluster"]
	if cs == nil {
		t.Fatal("expected test-cluster in response")
	}
	if len(cs.Checks) != 1 {
		t.Errorf("expected 1 filtered check, got %d", len(cs.Checks))
	}
	if _, ok := cs.Checks["vault"]; !ok {
		t.Error("expected vault check in filtered results")
	}
}

func TestReadyzHandler_SeverityFilter(t *testing.T) {
	rs := NewReadinessState()
	rs.Update("test-cluster", false, map[string]*CheckState{
		"dns":     {Ready: true, Message: "ok", Severity: "critical", Category: "networking"},
		"logging": {Ready: false, Message: "degraded", Severity: "warning", Category: "observability"},
	}, nil, nil)

	handler := ReadyzHandler(rs)
	req := httptest.NewRequest(http.MethodGet, "/readyz?severity=critical", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	// Only critical checks, and they're all passing
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Clusters map[string]*ClusterState `json:"clusters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	cs := resp.Clusters["test-cluster"]
	if len(cs.Checks) != 1 {
		t.Errorf("expected 1 filtered check, got %d", len(cs.Checks))
	}
}

func TestFilterSnapshot(t *testing.T) {
	snap := map[string]*ClusterState{
		"cluster-1": {
			Ready: true,
			Checks: map[string]*CheckState{
				"dns":     {Ready: true, Severity: "critical", Category: "networking"},
				"ingress": {Ready: false, Severity: "critical", Category: "networking"},
				"vault":   {Ready: true, Severity: "warning", Category: "security"},
			},
		},
	}

	t.Run("category filter", func(t *testing.T) {
		filtered := filterSnapshot(snap, "security", "")
		cs := filtered["cluster-1"]
		if len(cs.Checks) != 1 {
			t.Errorf("expected 1 check, got %d", len(cs.Checks))
		}
		if !cs.Ready {
			t.Error("expected cluster ready (only warning checks in security)")
		}
	})

	t.Run("severity filter", func(t *testing.T) {
		filtered := filterSnapshot(snap, "", "critical")
		cs := filtered["cluster-1"]
		if len(cs.Checks) != 2 {
			t.Errorf("expected 2 checks, got %d", len(cs.Checks))
		}
		if cs.Ready {
			t.Error("expected cluster not ready (ingress critical is failing)")
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		filtered := filterSnapshot(snap, "networking", "critical")
		cs := filtered["cluster-1"]
		if len(cs.Checks) != 2 {
			t.Errorf("expected 2 checks, got %d", len(cs.Checks))
		}
	})
}
