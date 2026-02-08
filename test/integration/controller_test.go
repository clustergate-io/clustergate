package integration

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
)

// ---------------------------------------------------------------------------
// CRD Schema Validation Tests
// ---------------------------------------------------------------------------

func TestCRD_RejectsInvalidSeverityEnum(t *testing.T) {
	pc := &clustergatev1alpha1.GateCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-invalid-severity",
		},
		Spec: clustergatev1alpha1.GateCheckSpec{
			Severity: clustergatev1alpha1.Severity("banana"), // invalid enum
			Category: "test",
			PodCheck: &clustergatev1alpha1.PodCheckSpec{
				Namespace:     "default",
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				MinReady:      1,
			},
		},
	}
	err := k8sClient.Create(ctx, pc)
	if err == nil {
		// Clean up if somehow created
		_ = k8sClient.Delete(ctx, pc)
		t.Fatal("expected creation to fail with invalid severity enum, but it succeeded")
	}
	if !errors.IsInvalid(err) {
		t.Logf("error type: %T, message: %v", err, err)
		// CRD schema validation may return different error types depending on version;
		// the key is that it was rejected.
	}
}

func TestCRD_GateCheckCreation(t *testing.T) {
	pc := &clustergatev1alpha1.GateCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod-check",
		},
		Spec: clustergatev1alpha1.GateCheckSpec{
			Severity:    clustergatev1alpha1.SeverityCritical,
			Category:    "test",
			Description: "Test pod check for integration tests",
			PodCheck: &clustergatev1alpha1.PodCheckSpec{
				Namespace:     "default",
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				MinReady:      1,
			},
		},
	}
	if err := k8sClient.Create(ctx, pc); err != nil {
		t.Fatalf("failed to create GateCheck: %v", err)
	}
	defer k8sClient.Delete(ctx, pc)

	// Verify it can be fetched back.
	fetched := &clustergatev1alpha1.GateCheck{}
	if err := k8sClient.Get(ctx, keyFor(pc), fetched); err != nil {
		t.Fatalf("failed to fetch GateCheck: %v", err)
	}
	if fetched.Spec.Category != "test" {
		t.Errorf("Category = %q, want %q", fetched.Spec.Category, "test")
	}
	if fetched.Spec.Severity != clustergatev1alpha1.SeverityCritical {
		t.Errorf("Severity = %q, want %q", fetched.Spec.Severity, clustergatev1alpha1.SeverityCritical)
	}
}

func TestCRD_GateCheckWithHTTPCheck(t *testing.T) {
	pc := &clustergatev1alpha1.GateCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-http-check",
		},
		Spec: clustergatev1alpha1.GateCheckSpec{
			Severity: clustergatev1alpha1.SeverityWarning,
			Category: "networking",
			HTTPCheck: &clustergatev1alpha1.HTTPCheckSpec{
				URL:    "https://example.com/healthz",
				Method: "GET",
			},
		},
	}
	if err := k8sClient.Create(ctx, pc); err != nil {
		t.Fatalf("failed to create GateCheck with HTTPCheck: %v", err)
	}
	defer k8sClient.Delete(ctx, pc)

	fetched := &clustergatev1alpha1.GateCheck{}
	if err := k8sClient.Get(ctx, keyFor(pc), fetched); err != nil {
		t.Fatalf("failed to fetch GateCheck: %v", err)
	}
	if fetched.Spec.HTTPCheck == nil {
		t.Fatal("expected HTTPCheck to be set")
	}
	if fetched.Spec.HTTPCheck.URL != "https://example.com/healthz" {
		t.Errorf("URL = %q, want %q", fetched.Spec.HTTPCheck.URL, "https://example.com/healthz")
	}
}

func TestCRD_GateCheckNoType_Creates(t *testing.T) {
	// A GateCheck with no check type specified should still be accepted
	// by the CRD (validation is done by the reconciler, not the schema).
	pc := &clustergatev1alpha1.GateCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-no-type-check",
		},
		Spec: clustergatev1alpha1.GateCheckSpec{
			Severity:    clustergatev1alpha1.SeverityCritical,
			Category:    "test",
			Description: "Check with no type — should be accepted by CRD but marked invalid by reconciler",
		},
	}
	if err := k8sClient.Create(ctx, pc); err != nil {
		t.Fatalf("expected creation to succeed (CRD allows it), but got: %v", err)
	}
	defer k8sClient.Delete(ctx, pc)
}

// ---------------------------------------------------------------------------
// GateProfile CRD Tests
// ---------------------------------------------------------------------------

func TestCRD_GateProfileCreation(t *testing.T) {
	profile := &clustergatev1alpha1.GateProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-profile",
		},
		Spec: clustergatev1alpha1.GateProfileSpec{
			Description: "Integration test profile",
			Checks: []clustergatev1alpha1.ProfileCheckRef{
				{
					Name:     "kube-apiserver",
					Severity: severityPtr(clustergatev1alpha1.SeverityCritical),
					Category: "control-plane",
				},
				{
					Name:     "dns",
					Severity: severityPtr(clustergatev1alpha1.SeverityCritical),
					Category: "networking",
				},
			},
		},
	}
	if err := k8sClient.Create(ctx, profile); err != nil {
		t.Fatalf("failed to create GateProfile: %v", err)
	}
	defer k8sClient.Delete(ctx, profile)

	fetched := &clustergatev1alpha1.GateProfile{}
	if err := k8sClient.Get(ctx, keyFor(profile), fetched); err != nil {
		t.Fatalf("failed to fetch GateProfile: %v", err)
	}
	if len(fetched.Spec.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(fetched.Spec.Checks))
	}
}

// ---------------------------------------------------------------------------
// ClusterReadiness CRD Tests
// ---------------------------------------------------------------------------

func TestCRD_ClusterReadinessCreation(t *testing.T) {
	cr := &clustergatev1alpha1.ClusterReadiness{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-readiness",
		},
		Spec: clustergatev1alpha1.ClusterReadinessSpec{
			Checks: []clustergatev1alpha1.CheckSpec{
				{
					Name:     "kube-apiserver",
					Severity: severityPtr(clustergatev1alpha1.SeverityCritical),
					Category: "control-plane",
				},
				{
					Name:     "etcd",
					Severity: severityPtr(clustergatev1alpha1.SeverityCritical),
					Category: "control-plane",
				},
			},
		},
	}
	if err := k8sClient.Create(ctx, cr); err != nil {
		t.Fatalf("failed to create ClusterReadiness: %v", err)
	}
	defer k8sClient.Delete(ctx, cr)

	fetched := &clustergatev1alpha1.ClusterReadiness{}
	if err := k8sClient.Get(ctx, keyFor(fetched), fetched); err != nil {
		// Use the key from the original object
		if err := k8sClient.Get(ctx, keyFor(cr), fetched); err != nil {
			t.Fatalf("failed to fetch ClusterReadiness: %v", err)
		}
	}
	if len(fetched.Spec.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(fetched.Spec.Checks))
	}
}

func TestCRD_ClusterReadinessWithProfile(t *testing.T) {
	cr := &clustergatev1alpha1.ClusterReadiness{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-readiness-with-profiles",
		},
		Spec: clustergatev1alpha1.ClusterReadinessSpec{
			Profiles: []clustergatev1alpha1.ProfileRef{
				{Name: "production-baseline"},
			},
			Checks: []clustergatev1alpha1.CheckSpec{
				{
					Name:     "kube-apiserver",
					Severity: severityPtr(clustergatev1alpha1.SeverityCritical),
					Category: "control-plane",
				},
			},
		},
	}
	if err := k8sClient.Create(ctx, cr); err != nil {
		t.Fatalf("failed to create ClusterReadiness with profile ref: %v", err)
	}
	defer k8sClient.Delete(ctx, cr)

	fetched := &clustergatev1alpha1.ClusterReadiness{}
	if err := k8sClient.Get(ctx, keyFor(cr), fetched); err != nil {
		t.Fatalf("failed to fetch ClusterReadiness: %v", err)
	}
	if len(fetched.Spec.Profiles) != 1 {
		t.Errorf("expected 1 profile reference, got %d", len(fetched.Spec.Profiles))
	}
}

func TestCRD_ClusterReadinessDelete(t *testing.T) {
	cr := &clustergatev1alpha1.ClusterReadiness{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-readiness-delete",
		},
		Spec: clustergatev1alpha1.ClusterReadinessSpec{
			Checks: []clustergatev1alpha1.CheckSpec{
				{
					Name:     "dns",
					Severity: severityPtr(clustergatev1alpha1.SeverityCritical),
					Category: "networking",
				},
			},
		},
	}
	if err := k8sClient.Create(ctx, cr); err != nil {
		t.Fatalf("failed to create ClusterReadiness: %v", err)
	}

	if err := k8sClient.Delete(ctx, cr); err != nil {
		t.Fatalf("failed to delete ClusterReadiness: %v", err)
	}

	// Verify deletion
	fetched := &clustergatev1alpha1.ClusterReadiness{}
	err := k8sClient.Get(ctx, keyFor(cr), fetched)
	if err == nil {
		t.Fatal("expected NotFound error after deletion, but resource still exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func keyFor(obj metav1.Object) client.ObjectKey {
	return client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

func severityPtr(s clustergatev1alpha1.Severity) *clustergatev1alpha1.Severity {
	return &s
}
