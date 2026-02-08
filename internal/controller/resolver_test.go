package controller

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
	"github.com/clustergate/clustergate/internal/checks"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(clustergatev1alpha1.AddToScheme(s))
	return s
}

func TestResolveChecks_InlineBuiltin(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()

	spec := clustergatev1alpha1.ClusterReadinessSpec{
		Checks: []clustergatev1alpha1.CheckSpec{
			{Name: "dns"},
		},
	}

	result, err := ResolveChecks(context.Background(), c, spec, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 check, got %d", len(result))
	}
	if result[0].Identifier != "dns" {
		t.Errorf("identifier = %q, want %q", result[0].Identifier, "dns")
	}
	if !result[0].IsBuiltin {
		t.Error("expected IsBuiltin = true")
	}
	if result[0].BuiltinName != "dns" {
		t.Errorf("BuiltinName = %q, want %q", result[0].BuiltinName, "dns")
	}
	if result[0].Source != "inline" {
		t.Errorf("Source = %q, want %q", result[0].Source, "inline")
	}
}

func TestResolveChecks_InlineDynamic(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()

	spec := clustergatev1alpha1.ClusterReadinessSpec{
		Checks: []clustergatev1alpha1.CheckSpec{
			{GateCheckRef: "ingress-controller-ready"},
		},
	}

	result, err := ResolveChecks(context.Background(), c, spec, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 check, got %d", len(result))
	}
	if result[0].Identifier != "dynamic:ingress-controller-ready" {
		t.Errorf("identifier = %q, want %q", result[0].Identifier, "dynamic:ingress-controller-ready")
	}
	if result[0].IsBuiltin {
		t.Error("expected IsBuiltin = false")
	}
}

func TestResolveChecks_InlineWithOverrides(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()

	sev := clustergatev1alpha1.SeverityWarning
	spec := clustergatev1alpha1.ClusterReadinessSpec{
		Checks: []clustergatev1alpha1.CheckSpec{
			{
				Name:     "dns",
				Severity: &sev,
				Category: "custom-networking",
				Interval: &metav1.Duration{Duration: 5 * time.Minute},
			},
		},
	}

	result, err := ResolveChecks(context.Background(), c, spec, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 check, got %d", len(result))
	}
	rc := result[0]
	if rc.Severity != "warning" {
		t.Errorf("severity = %q, want %q", rc.Severity, "warning")
	}
	if rc.Category != "custom-networking" {
		t.Errorf("category = %q, want %q", rc.Category, "custom-networking")
	}
	if rc.Interval != 5*time.Minute {
		t.Errorf("interval = %v, want %v", rc.Interval, 5*time.Minute)
	}
}

func TestResolveChecks_DisabledInlineExcluded(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()

	enabled := false
	spec := clustergatev1alpha1.ClusterReadinessSpec{
		Checks: []clustergatev1alpha1.CheckSpec{
			{Name: "dns", Enabled: &enabled},
		},
	}

	result, err := ResolveChecks(context.Background(), c, spec, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 checks, got %d", len(result))
	}
}

func TestResolveChecks_ProfileResolution(t *testing.T) {
	profile := &clustergatev1alpha1.GateProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "prod-baseline"},
		Spec: clustergatev1alpha1.GateProfileSpec{
			Checks: []clustergatev1alpha1.ProfileCheckRef{
				{Name: "dns"},
				{GateCheckRef: "ingress-controller-ready"},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(profile).
		Build()

	spec := clustergatev1alpha1.ClusterReadinessSpec{
		Profiles: []clustergatev1alpha1.ProfileRef{
			{Name: "prod-baseline"},
		},
	}

	result, err := ResolveChecks(context.Background(), c, spec, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(result))
	}

	// Verify sources
	for _, rc := range result {
		if rc.Source != "profile:prod-baseline" {
			t.Errorf("Source = %q, want %q", rc.Source, "profile:prod-baseline")
		}
	}
}

func TestResolveChecks_ProfileDisabledExcluded(t *testing.T) {
	disabled := false
	profile := &clustergatev1alpha1.GateProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "test-profile"},
		Spec: clustergatev1alpha1.GateProfileSpec{
			Checks: []clustergatev1alpha1.ProfileCheckRef{
				{Name: "dns"},
				{GateCheckRef: "ingress", Enabled: &disabled},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(profile).
		Build()

	spec := clustergatev1alpha1.ClusterReadinessSpec{
		Profiles: []clustergatev1alpha1.ProfileRef{
			{Name: "test-profile"},
		},
	}

	result, err := ResolveChecks(context.Background(), c, spec, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 check, got %d", len(result))
	}
	if result[0].Identifier != "dns" {
		t.Errorf("identifier = %q, want %q", result[0].Identifier, "dns")
	}
}

func TestResolveChecks_LaterProfileOverridesEarlier(t *testing.T) {
	sevWarning := clustergatev1alpha1.SeverityWarning
	sevCritical := clustergatev1alpha1.SeverityCritical

	profile1 := &clustergatev1alpha1.GateProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "profile-a"},
		Spec: clustergatev1alpha1.GateProfileSpec{
			Checks: []clustergatev1alpha1.ProfileCheckRef{
				{Name: "dns", Severity: &sevWarning},
			},
		},
	}
	profile2 := &clustergatev1alpha1.GateProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "profile-b"},
		Spec: clustergatev1alpha1.GateProfileSpec{
			Checks: []clustergatev1alpha1.ProfileCheckRef{
				{Name: "dns", Severity: &sevCritical},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(profile1, profile2).
		Build()

	spec := clustergatev1alpha1.ClusterReadinessSpec{
		Profiles: []clustergatev1alpha1.ProfileRef{
			{Name: "profile-a"},
			{Name: "profile-b"},
		},
	}

	result, err := ResolveChecks(context.Background(), c, spec, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 check (merged), got %d", len(result))
	}
	if result[0].Severity != "critical" {
		t.Errorf("severity = %q, want %q (from later profile)", result[0].Severity, "critical")
	}
	if result[0].Source != "profile:profile-b" {
		t.Errorf("source = %q, want %q", result[0].Source, "profile:profile-b")
	}
}

func TestResolveChecks_InlineOverridesProfile(t *testing.T) {
	sevCritical := clustergatev1alpha1.SeverityCritical
	sevWarning := clustergatev1alpha1.SeverityWarning

	profile := &clustergatev1alpha1.GateProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "base-profile"},
		Spec: clustergatev1alpha1.GateProfileSpec{
			Checks: []clustergatev1alpha1.ProfileCheckRef{
				{Name: "dns", Severity: &sevCritical, Category: "networking"},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(profile).
		Build()

	// Inline overrides severity but not category
	spec := clustergatev1alpha1.ClusterReadinessSpec{
		Profiles: []clustergatev1alpha1.ProfileRef{
			{Name: "base-profile"},
		},
		Checks: []clustergatev1alpha1.CheckSpec{
			{Name: "dns", Severity: &sevWarning},
		},
	}

	result, err := ResolveChecks(context.Background(), c, spec, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 check, got %d", len(result))
	}
	rc := result[0]
	if rc.Severity != "warning" {
		t.Errorf("severity = %q, want %q (inline override)", rc.Severity, "warning")
	}
	if rc.Category != "networking" {
		t.Errorf("category = %q, want %q (preserved from profile)", rc.Category, "networking")
	}
}

func TestResolveChecks_ProfileNotFound(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()

	spec := clustergatev1alpha1.ClusterReadinessSpec{
		Profiles: []clustergatev1alpha1.ProfileRef{
			{Name: "does-not-exist"},
		},
	}

	_, err := ResolveChecks(context.Background(), c, spec, 60*time.Second)
	if err == nil {
		t.Error("expected error for missing profile, got nil")
	}
}

// Register a stub checker for ResolveSeverityAndCategory tests.
type resolverStubChecker struct{}

func (s *resolverStubChecker) Name() string            { return "resolver-test-check" }
func (s *resolverStubChecker) DefaultSeverity() string { return "warning" }
func (s *resolverStubChecker) DefaultCategory() string { return "test-category" }
func (s *resolverStubChecker) Run(_ context.Context, _ json.RawMessage) (checks.Result, error) {
	return checks.Result{Ready: true}, nil
}

func init() {
	checks.Register(&resolverStubChecker{})
}

func TestResolveSeverityAndCategory_BuiltinRegistered(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	rc := ResolvedCheck{
		IsBuiltin:   true,
		BuiltinName: "resolver-test-check",
	}

	sev, cat := ResolveSeverityAndCategory(rc, context.Background(), c)
	if sev != "warning" {
		t.Errorf("severity = %q, want %q", sev, "warning")
	}
	if cat != "test-category" {
		t.Errorf("category = %q, want %q", cat, "test-category")
	}
}

func TestResolveSeverityAndCategory_BuiltinNotRegistered(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	rc := ResolvedCheck{
		IsBuiltin:   true,
		BuiltinName: "nonexistent-check",
	}

	sev, cat := ResolveSeverityAndCategory(rc, context.Background(), c)
	if sev != "critical" {
		t.Errorf("severity = %q, want %q (fallback)", sev, "critical")
	}
	if cat != "general" {
		t.Errorf("category = %q, want %q (fallback)", cat, "general")
	}
}

func TestResolveSeverityAndCategory_DynamicFromCR(t *testing.T) {
	pc := &clustergatev1alpha1.GateCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "my-check"},
		Spec: clustergatev1alpha1.GateCheckSpec{
			Severity: clustergatev1alpha1.SeverityWarning,
			Category: "security",
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc).
		Build()

	rc := ResolvedCheck{
		IsBuiltin:     false,
		GateCheckName: "my-check",
	}

	sev, cat := ResolveSeverityAndCategory(rc, context.Background(), c)
	if sev != "warning" {
		t.Errorf("severity = %q, want %q", sev, "warning")
	}
	if cat != "security" {
		t.Errorf("category = %q, want %q", cat, "security")
	}
}

func TestResolveSeverityAndCategory_DynamicCRNotFound(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()

	rc := ResolvedCheck{
		IsBuiltin:     false,
		GateCheckName: "missing-check",
	}

	sev, cat := ResolveSeverityAndCategory(rc, context.Background(), c)
	if sev != "critical" {
		t.Errorf("severity = %q, want %q (fallback)", sev, "critical")
	}
	if cat != "custom" {
		t.Errorf("category = %q, want %q (fallback)", cat, "custom")
	}
}
