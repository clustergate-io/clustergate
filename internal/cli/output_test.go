package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatText_AllPass(t *testing.T) {
	report := &Report{
		State:  "Healthy",
		Total:  2,
		Passed: 2,
		Failed: 0,
		Checks: []CheckResult{
			{Name: "dns", Category: "networking", Severity: "critical", Status: "Passing", Message: "DNS operational"},
			{Name: "kube-apiserver", Category: "control-plane", Severity: "critical", Status: "Passing", Message: "healthy"},
		},
	}

	var buf bytes.Buffer
	FormatText(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "[PASS] dns") {
		t.Error("expected [PASS] dns in output")
	}
	if !strings.Contains(out, "[PASS] kube-apiserver") {
		t.Error("expected [PASS] kube-apiserver in output")
	}
	if strings.Contains(out, "[FAIL]") {
		t.Error("did not expect [FAIL] in output")
	}
	if !strings.Contains(out, "Cluster State: Healthy") {
		t.Error("expected Cluster State: Healthy in output")
	}
	if !strings.Contains(out, "2/2 passed") {
		t.Error("expected 2/2 passed in output")
	}
}

func TestFormatText_SomeFail(t *testing.T) {
	report := &Report{
		State:  "Unhealthy",
		Total:  2,
		Passed: 1,
		Failed: 1,
		Checks: []CheckResult{
			{Name: "dns", Category: "networking", Severity: "critical", Status: "Passing", Message: "DNS operational"},
			{Name: "kube-apiserver", Category: "control-plane", Severity: "critical", Status: "Failing", Message: "unhealthy"},
		},
	}

	var buf bytes.Buffer
	FormatText(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "[PASS] dns") {
		t.Error("expected [PASS] dns in output")
	}
	if !strings.Contains(out, "[FAIL] kube-apiserver") {
		t.Error("expected [FAIL] kube-apiserver in output")
	}
	if !strings.Contains(out, "Cluster State: Unhealthy") {
		t.Error("expected Cluster State: Unhealthy in output")
	}
	if !strings.Contains(out, "1 failed") {
		t.Error("expected '1 failed' in output")
	}
}

func TestFormatText_WithErrors(t *testing.T) {
	report := &Report{
		State:  "Unhealthy",
		Total:  1,
		Passed: 0,
		Failed: 1,
		Errors: []CheckError{
			{Name: "etcd", Error: "connection refused"},
		},
	}

	var buf bytes.Buffer
	FormatText(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "[ERR]  etcd") {
		t.Error("expected [ERR]  etcd in output")
	}
	if !strings.Contains(out, "connection refused") {
		t.Error("expected error message in output")
	}
	if !strings.Contains(out, "Cluster State: Unhealthy") {
		t.Error("expected Cluster State: Unhealthy in output")
	}
}

func TestFormatText_Empty(t *testing.T) {
	report := &Report{State: "Healthy"}

	var buf bytes.Buffer
	FormatText(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "0/0 passed") {
		t.Error("expected 0/0 passed in output")
	}
	if !strings.Contains(out, "Cluster State: Healthy") {
		t.Error("expected Cluster State: Healthy in output")
	}
}

func TestFormatJSON_AllPass(t *testing.T) {
	report := &Report{
		State:  "Healthy",
		Total:  1,
		Passed: 1,
		Failed: 0,
		Checks: []CheckResult{
			{Name: "dns", Category: "networking", Severity: "critical", Status: "Passing", Message: "ok"},
		},
	}

	var buf bytes.Buffer
	if err := FormatJSON(&buf, report); err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	var parsed Report
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if parsed.State != "Healthy" {
		t.Errorf("expected state=Healthy in JSON, got %s", parsed.State)
	}
	if len(parsed.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(parsed.Checks))
	}
}

func TestFormatJSON_SomeFail(t *testing.T) {
	report := &Report{
		State:  "Unhealthy",
		Total:  2,
		Passed: 1,
		Failed: 1,
		Checks: []CheckResult{
			{Name: "dns", Status: "Passing", Message: "ok"},
			{Name: "etcd", Status: "Failing", Message: "down"},
		},
	}

	var buf bytes.Buffer
	if err := FormatJSON(&buf, report); err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	var parsed Report
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if parsed.State != "Unhealthy" {
		t.Errorf("expected state=Unhealthy in JSON, got %s", parsed.State)
	}
	if parsed.Failed != 1 {
		t.Errorf("expected failed=1, got %d", parsed.Failed)
	}
}

func TestFormatJSON_Indented(t *testing.T) {
	report := &Report{State: "Healthy", Total: 1, Passed: 1, Checks: []CheckResult{
		{Name: "dns", Status: "Passing", Message: "ok"},
	}}

	var buf bytes.Buffer
	if err := FormatJSON(&buf, report); err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	if !strings.Contains(buf.String(), "\n  ") {
		t.Error("expected indented JSON output")
	}
}

func TestFormatText_CategoryAndSeverity(t *testing.T) {
	report := &Report{
		State:  "Healthy",
		Total:  1,
		Passed: 1,
		Checks: []CheckResult{
			{Name: "dns", Category: "networking", Severity: "critical", Status: "Passing", Message: "ok"},
		},
	}

	var buf bytes.Buffer
	FormatText(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "(networking/critical)") {
		t.Error("expected (networking/critical) in output")
	}
}
