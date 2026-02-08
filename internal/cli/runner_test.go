package cli

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/clustergate/clustergate/internal/checks"
)

type stubChecker struct {
	name     string
	severity string
	category string
	result   checks.Result
	err      error
}

func (s *stubChecker) Name() string            { return s.name }
func (s *stubChecker) DefaultSeverity() string { return s.severity }
func (s *stubChecker) DefaultCategory() string { return s.category }
func (s *stubChecker) Run(_ context.Context, _ json.RawMessage) (checks.Result, error) {
	return s.result, s.err
}

func TestRunChecks_AllPass(t *testing.T) {
	checkers := []checks.Checker{
		&stubChecker{name: "a", severity: "critical", category: "cat1", result: checks.Result{Ready: true, Message: "ok"}},
		&stubChecker{name: "b", severity: "warning", category: "cat2", result: checks.Result{Ready: true, Message: "ok"}},
	}

	report := RunChecks(context.Background(), checkers, nil)

	if !report.Ready {
		t.Fatal("expected Ready=true")
	}
	if report.Total != 2 {
		t.Fatalf("expected Total=2, got %d", report.Total)
	}
	if report.Passed != 2 {
		t.Fatalf("expected Passed=2, got %d", report.Passed)
	}
	if report.Failed != 0 {
		t.Fatalf("expected Failed=0, got %d", report.Failed)
	}
}

func TestRunChecks_SomeFail(t *testing.T) {
	checkers := []checks.Checker{
		&stubChecker{name: "a", severity: "critical", category: "cat1", result: checks.Result{Ready: true, Message: "ok"}},
		&stubChecker{name: "b", severity: "critical", category: "cat2", result: checks.Result{Ready: false, Message: "down"}},
	}

	report := RunChecks(context.Background(), checkers, nil)

	if report.Ready {
		t.Fatal("expected Ready=false")
	}
	if report.Passed != 1 {
		t.Fatalf("expected Passed=1, got %d", report.Passed)
	}
	if report.Failed != 1 {
		t.Fatalf("expected Failed=1, got %d", report.Failed)
	}
}

func TestRunChecks_WithFilter(t *testing.T) {
	checkers := []checks.Checker{
		&stubChecker{name: "a", severity: "critical", category: "cat1", result: checks.Result{Ready: true, Message: "ok"}},
		&stubChecker{name: "b", severity: "critical", category: "cat2", result: checks.Result{Ready: true, Message: "ok"}},
		&stubChecker{name: "c", severity: "critical", category: "cat3", result: checks.Result{Ready: true, Message: "ok"}},
	}

	filter := map[string]bool{"a": true, "c": true}
	report := RunChecks(context.Background(), checkers, filter)

	if report.Total != 2 {
		t.Fatalf("expected Total=2, got %d", report.Total)
	}
	if len(report.Checks) != 2 {
		t.Fatalf("expected 2 check results, got %d", len(report.Checks))
	}
	for _, c := range report.Checks {
		if c.Name != "a" && c.Name != "c" {
			t.Fatalf("unexpected check in results: %s", c.Name)
		}
	}
}

func TestRunChecks_CheckError(t *testing.T) {
	checkers := []checks.Checker{
		&stubChecker{name: "a", severity: "critical", category: "cat1", result: checks.Result{Ready: true, Message: "ok"}},
		&stubChecker{name: "b", severity: "critical", category: "cat2", err: errors.New("connection refused")},
	}

	report := RunChecks(context.Background(), checkers, nil)

	if report.Ready {
		t.Fatal("expected Ready=false when a check errors")
	}
	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(report.Errors))
	}
	if report.Errors[0].Name != "b" {
		t.Fatalf("expected error for check 'b', got '%s'", report.Errors[0].Name)
	}
	if report.Failed != 1 {
		t.Fatalf("expected Failed=1, got %d", report.Failed)
	}
}

func TestRunChecks_Empty(t *testing.T) {
	report := RunChecks(context.Background(), nil, nil)

	if !report.Ready {
		t.Fatal("expected Ready=true for empty check list")
	}
	if report.Total != 0 {
		t.Fatalf("expected Total=0, got %d", report.Total)
	}
}

func TestRunChecks_DeterministicOrder(t *testing.T) {
	checkers := []checks.Checker{
		&stubChecker{name: "c", severity: "critical", category: "cat", result: checks.Result{Ready: true, Message: "ok"}},
		&stubChecker{name: "a", severity: "critical", category: "cat", result: checks.Result{Ready: true, Message: "ok"}},
		&stubChecker{name: "b", severity: "critical", category: "cat", result: checks.Result{Ready: true, Message: "ok"}},
	}

	report := RunChecks(context.Background(), checkers, nil)

	if len(report.Checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(report.Checks))
	}
	if report.Checks[0].Name != "a" || report.Checks[1].Name != "b" || report.Checks[2].Name != "c" {
		t.Fatalf("expected alphabetical order, got %s, %s, %s",
			report.Checks[0].Name, report.Checks[1].Name, report.Checks[2].Name)
	}
}
