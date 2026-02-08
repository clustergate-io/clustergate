package controller

import (
	"testing"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
)

func TestAggregateCheck(t *testing.T) {
	tests := []struct {
		name         string
		severity     string
		category     string
		ready        bool
		wantSummary  preflightv1alpha1.ReadinessSummary
		wantCatReady bool
	}{
		{
			name:     "critical passing",
			severity: "critical",
			category: "networking",
			ready:    true,
			wantSummary: preflightv1alpha1.ReadinessSummary{
				Total: 1, Passing: 1, CriticalTotal: 1, CriticalPassing: 1,
			},
			wantCatReady: true,
		},
		{
			name:     "critical failing",
			severity: "critical",
			category: "networking",
			ready:    false,
			wantSummary: preflightv1alpha1.ReadinessSummary{
				Total: 1, Failing: 1, CriticalTotal: 1,
			},
			wantCatReady: false,
		},
		{
			name:     "warning failing",
			severity: "warning",
			category: "observability",
			ready:    false,
			wantSummary: preflightv1alpha1.ReadinessSummary{
				Total: 1, Failing: 1, WarningTotal: 1, WarningFailing: 1,
			},
			wantCatReady: true, // warning doesn't block category
		},
		{
			name:     "warning passing",
			severity: "warning",
			category: "observability",
			ready:    true,
			wantSummary: preflightv1alpha1.ReadinessSummary{
				Total: 1, Passing: 1, WarningTotal: 1,
			},
			wantCatReady: true,
		},
		{
			name:     "info check",
			severity: "info",
			category: "diagnostics",
			ready:    true,
			wantSummary: preflightv1alpha1.ReadinessSummary{
				Total: 1, Passing: 1,
			},
			wantCatReady: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := &preflightv1alpha1.ReadinessSummary{}
			categoryMap := make(map[string]*categoryAgg)

			aggregateCheck(summary, categoryMap, tt.severity, tt.category, tt.ready)

			if summary.Total != tt.wantSummary.Total {
				t.Errorf("Total = %d, want %d", summary.Total, tt.wantSummary.Total)
			}
			if summary.Passing != tt.wantSummary.Passing {
				t.Errorf("Passing = %d, want %d", summary.Passing, tt.wantSummary.Passing)
			}
			if summary.Failing != tt.wantSummary.Failing {
				t.Errorf("Failing = %d, want %d", summary.Failing, tt.wantSummary.Failing)
			}
			if summary.CriticalTotal != tt.wantSummary.CriticalTotal {
				t.Errorf("CriticalTotal = %d, want %d", summary.CriticalTotal, tt.wantSummary.CriticalTotal)
			}
			if summary.CriticalPassing != tt.wantSummary.CriticalPassing {
				t.Errorf("CriticalPassing = %d, want %d", summary.CriticalPassing, tt.wantSummary.CriticalPassing)
			}
			if summary.WarningTotal != tt.wantSummary.WarningTotal {
				t.Errorf("WarningTotal = %d, want %d", summary.WarningTotal, tt.wantSummary.WarningTotal)
			}
			if summary.WarningFailing != tt.wantSummary.WarningFailing {
				t.Errorf("WarningFailing = %d, want %d", summary.WarningFailing, tt.wantSummary.WarningFailing)
			}

			agg, exists := categoryMap[tt.category]
			if !exists {
				t.Fatal("expected category to exist in map")
			}
			if agg.ready != tt.wantCatReady {
				t.Errorf("category ready = %v, want %v", agg.ready, tt.wantCatReady)
			}
		})
	}
}

func TestAggregateCheck_MultipleCalls(t *testing.T) {
	summary := &preflightv1alpha1.ReadinessSummary{}
	categoryMap := make(map[string]*categoryAgg)

	// First: critical passing in networking
	aggregateCheck(summary, categoryMap, "critical", "networking", true)
	// Second: critical failing in networking
	aggregateCheck(summary, categoryMap, "critical", "networking", false)
	// Third: warning failing in networking
	aggregateCheck(summary, categoryMap, "warning", "networking", false)

	if summary.Total != 3 {
		t.Errorf("Total = %d, want 3", summary.Total)
	}
	if summary.Passing != 1 {
		t.Errorf("Passing = %d, want 1", summary.Passing)
	}
	if summary.Failing != 2 {
		t.Errorf("Failing = %d, want 2", summary.Failing)
	}
	if summary.CriticalTotal != 2 {
		t.Errorf("CriticalTotal = %d, want 2", summary.CriticalTotal)
	}
	if summary.CriticalPassing != 1 {
		t.Errorf("CriticalPassing = %d, want 1", summary.CriticalPassing)
	}
	if summary.WarningFailing != 1 {
		t.Errorf("WarningFailing = %d, want 1", summary.WarningFailing)
	}

	agg := categoryMap["networking"]
	if agg.ready {
		t.Error("expected networking category not ready (critical failing)")
	}
	if agg.total != 3 {
		t.Errorf("category total = %d, want 3", agg.total)
	}
	if agg.passing != 1 {
		t.Errorf("category passing = %d, want 1", agg.passing)
	}
	if agg.failing != 2 {
		t.Errorf("category failing = %d, want 2", agg.failing)
	}
}
