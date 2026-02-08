package checks

import (
	"context"
	"encoding/json"
	"testing"
)

// stubChecker implements the Checker interface for testing.
type stubChecker struct {
	name     string
	severity string
	category string
}

func (s *stubChecker) Name() string            { return s.name }
func (s *stubChecker) DefaultSeverity() string { return s.severity }
func (s *stubChecker) DefaultCategory() string { return s.category }
func (s *stubChecker) Run(_ context.Context, _ json.RawMessage) (Result, error) {
	return Result{Ready: true}, nil
}

func TestRegisterAndGet(t *testing.T) {
	// Use a unique name to avoid collisions with other test runs
	checker := &stubChecker{name: "test-register-get", severity: "critical", category: "test"}
	Register(checker)

	got, ok := Get("test-register-get")
	if !ok {
		t.Fatal("expected Get to return true for registered check")
	}
	if got.Name() != "test-register-get" {
		t.Errorf("Name() = %q, want %q", got.Name(), "test-register-get")
	}
}

func TestGetNonExistent(t *testing.T) {
	_, ok := Get("does-not-exist-xyz")
	if ok {
		t.Error("expected Get to return false for non-existent check")
	}
}

func TestListContainsRegistered(t *testing.T) {
	checker := &stubChecker{name: "test-list-check", severity: "warning", category: "test"}
	Register(checker)

	names := List()
	found := false
	for _, n := range names {
		if n == "test-list-check" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("List() does not contain %q, got %v", "test-list-check", names)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	checker := &stubChecker{name: "test-dup-panic", severity: "critical", category: "test"}
	Register(checker)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()

	// Register again with same name — should panic
	Register(checker)
}

func TestAll(t *testing.T) {
	Reset()
	Register(&stubChecker{name: "all-a", severity: "critical", category: "test"})
	Register(&stubChecker{name: "all-b", severity: "warning", category: "test"})

	all := All()
	if len(all) != 2 {
		t.Fatalf("expected 2 checkers, got %d", len(all))
	}

	names := map[string]bool{}
	for _, c := range all {
		names[c.Name()] = true
	}
	if !names["all-a"] || !names["all-b"] {
		t.Errorf("expected all-a and all-b, got %v", names)
	}
}

func TestReset(t *testing.T) {
	Reset()
	Register(&stubChecker{name: "reset-test", severity: "critical", category: "test"})

	if len(All()) != 1 {
		t.Fatal("expected 1 checker before reset")
	}

	Reset()

	if len(All()) != 0 {
		t.Fatal("expected 0 checkers after reset")
	}

	// Should be able to re-register the same name after reset.
	Register(&stubChecker{name: "reset-test", severity: "critical", category: "test"})
	if len(All()) != 1 {
		t.Fatal("expected 1 checker after re-registration")
	}
	Reset()
}
