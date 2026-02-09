package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
)

func TestCheckSchedule(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	interval := 60 * time.Second

	tests := []struct {
		name             string
		resolved         []ResolvedCheck
		existingStatuses []clustergatev1alpha1.CheckStatus
		wantDueCount     int
		wantCarriedCount int
		wantRequeue      time.Duration
	}{
		{
			name: "no prior statuses - all due",
			resolved: []ResolvedCheck{
				{Identifier: "dns", Interval: interval},
				{Identifier: "dynamic:ingress", Interval: interval},
			},
			existingStatuses: nil,
			wantDueCount:     2,
			wantCarriedCount: 0,
			wantRequeue:      interval,
		},
		{
			name: "all stale - all due",
			resolved: []ResolvedCheck{
				{Identifier: "dns", Interval: interval},
				{Identifier: "dynamic:ingress", Interval: interval},
			},
			existingStatuses: []clustergatev1alpha1.CheckStatus{
				{Name: "dns", LastChecked: timePtr(now.Add(-2 * time.Minute))},
				{Name: "dynamic:ingress", LastChecked: timePtr(now.Add(-5 * time.Minute))},
			},
			wantDueCount:     2,
			wantCarriedCount: 0,
			wantRequeue:      interval,
		},
		{
			name: "all fresh - none due",
			resolved: []ResolvedCheck{
				{Identifier: "dns", Interval: interval},
				{Identifier: "dynamic:ingress", Interval: 2 * time.Minute},
			},
			existingStatuses: []clustergatev1alpha1.CheckStatus{
				{Name: "dns", LastChecked: timePtr(now.Add(-30 * time.Second))},
				{Name: "dynamic:ingress", LastChecked: timePtr(now.Add(-30 * time.Second))},
			},
			wantDueCount:     0,
			wantCarriedCount: 2,
			wantRequeue:      30 * time.Second, // dns has 30s remaining
		},
		{
			name: "mixed - some due some carried",
			resolved: []ResolvedCheck{
				{Identifier: "dns", Interval: interval},
				{Identifier: "dynamic:ingress", Interval: 5 * time.Minute},
			},
			existingStatuses: []clustergatev1alpha1.CheckStatus{
				{Name: "dns", LastChecked: timePtr(now.Add(-2 * time.Minute))},              // stale
				{Name: "dynamic:ingress", LastChecked: timePtr(now.Add(-30 * time.Second))}, // fresh (4m30s remaining)
			},
			wantDueCount:     1,
			wantCarriedCount: 1,
			wantRequeue:      4*time.Minute + 30*time.Second,
		},
		{
			name:             "empty resolved list",
			resolved:         nil,
			existingStatuses: nil,
			wantDueCount:     0,
			wantCarriedCount: 0,
			wantRequeue:      0,
		},
		{
			name: "exactly at interval boundary - treated as due",
			resolved: []ResolvedCheck{
				{Identifier: "dns", Interval: interval},
			},
			existingStatuses: []clustergatev1alpha1.CheckStatus{
				{Name: "dns", LastChecked: timePtr(now.Add(-interval))},
			},
			wantDueCount:     1,
			wantCarriedCount: 0,
			wantRequeue:      interval,
		},
		{
			name: "nil LastChecked - treated as due",
			resolved: []ResolvedCheck{
				{Identifier: "dns", Interval: interval},
			},
			existingStatuses: []clustergatev1alpha1.CheckStatus{
				{Name: "dns", LastChecked: nil},
			},
			wantDueCount:     1,
			wantCarriedCount: 0,
			wantRequeue:      interval,
		},
		{
			name: "different intervals - requeue uses shortest remaining",
			resolved: []ResolvedCheck{
				{Identifier: "dns", Interval: 2 * time.Minute},
				{Identifier: "dynamic:ingress", Interval: 5 * time.Minute},
			},
			existingStatuses: []clustergatev1alpha1.CheckStatus{
				{Name: "dns", LastChecked: timePtr(now.Add(-time.Minute))},             // 1m remaining
				{Name: "dynamic:ingress", LastChecked: timePtr(now.Add(-time.Minute))}, // 4m remaining
			},
			wantDueCount:     0,
			wantCarriedCount: 2,
			wantRequeue:      time.Minute, // shortest remaining
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			due, carried, requeue := CheckSchedule(tt.resolved, tt.existingStatuses, now)

			if len(due) != tt.wantDueCount {
				t.Errorf("due count = %d, want %d", len(due), tt.wantDueCount)
			}
			if len(carried) != tt.wantCarriedCount {
				t.Errorf("carried count = %d, want %d", len(carried), tt.wantCarriedCount)
			}
			if requeue != tt.wantRequeue {
				t.Errorf("requeue = %v, want %v", requeue, tt.wantRequeue)
			}
		})
	}
}

func TestCheckScheduleCarriedStatusIntegrity(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	lastChecked := now.Add(-10 * time.Second)

	resolved := []ResolvedCheck{
		{Identifier: "dns", Interval: 60 * time.Second},
	}
	existing := []clustergatev1alpha1.CheckStatus{
		{
			Name:        "dns",
			Status:      "Passing",
			Severity:    clustergatev1alpha1.SeverityCritical,
			Message:     "DNS operational",
			LastChecked: timePtr(lastChecked),
		},
	}

	_, carried, _ := CheckSchedule(resolved, existing, now)

	if len(carried) != 1 {
		t.Fatalf("expected 1 carried status, got %d", len(carried))
	}

	cs := carried[0]
	if cs.Name != "dns" {
		t.Errorf("carried name = %q, want %q", cs.Name, "dns")
	}
	if cs.Status != "Passing" {
		t.Errorf("carried status = %q, want %q", cs.Status, "Passing")
	}
	if cs.Severity != clustergatev1alpha1.SeverityCritical {
		t.Errorf("carried severity = %q, want %q", cs.Severity, clustergatev1alpha1.SeverityCritical)
	}
	if cs.Message != "DNS operational" {
		t.Errorf("carried message = %q, want %q", cs.Message, "DNS operational")
	}
}

func TestShortestInterval(t *testing.T) {
	tests := []struct {
		name   string
		checks []ResolvedCheck
		want   time.Duration
	}{
		{
			name:   "empty list returns default",
			checks: nil,
			want:   defaultInterval,
		},
		{
			name: "single check",
			checks: []ResolvedCheck{
				{Interval: 30 * time.Second},
			},
			want: 30 * time.Second,
		},
		{
			name: "multiple checks returns shortest",
			checks: []ResolvedCheck{
				{Interval: 5 * time.Minute},
				{Interval: 30 * time.Second},
				{Interval: 2 * time.Minute},
			},
			want: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortestInterval(tt.checks)
			if got != tt.want {
				t.Errorf("shortestInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func timePtr(t time.Time) *metav1.Time {
	mt := metav1.NewTime(t)
	return &mt
}
