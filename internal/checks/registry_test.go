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
