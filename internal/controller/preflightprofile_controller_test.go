package controller

import (
	"context"
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
	"github.com/camcast3/platform-preflight/internal/checks"
)

// profileStubChecker is registered so that the profile controller can validate
// built-in check references in tests. It uses "profile-test-dns" to avoid
// conflicting with the "resolver-test-check" registered in resolver_test.go.
type profileStubChecker struct{}

func (s *profileStubChecker) Name() string            { return "profile-test-dns" }
func (s *profileStubChecker) DefaultSeverity() string { return "critical" }
func (s *profileStubChecker) DefaultCategory() string { return "networking" }
func (s *profileStubChecker) Run(_ context.Context, _ json.RawMessage) (checks.Result, error) {
	return checks.Result{Ready: true}, nil
}

func init() {
	checks.Register(&profileStubChecker{})
}

func TestPreflightProfileReconciler_ValidBuiltin(t *testing.T) {
	pp := &preflightv1alpha1.PreflightProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "test-valid-builtin"},
		Spec: preflightv1alpha1.PreflightProfileSpec{
			Checks: []preflightv1alpha1.ProfileCheckRef{
				{Name: "profile-test-dns"},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pp).
		WithStatusSubresource(&preflightv1alpha1.PreflightProfile{}).
		Build()

	r := &PreflightProfileReconciler{Client: c}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-valid-builtin"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated preflightv1alpha1.PreflightProfile
	c.Get(context.Background(), types.NamespacedName{Name: "test-valid-builtin"}, &updated)
	if !updated.Status.Valid {
		t.Errorf("expected valid=true, got false: %s", updated.Status.Message)
	}
	if updated.Status.CheckCount != 1 {
		t.Errorf("checkCount = %d, want 1", updated.Status.CheckCount)
	}
}

func TestPreflightProfileReconciler_ValidDynamicRef(t *testing.T) {
	pc := &preflightv1alpha1.PreflightCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "ingress-ready"},
		Spec: preflightv1alpha1.PreflightCheckSpec{
			Severity: preflightv1alpha1.SeverityCritical,
			Category: "networking",
			PodCheck: &preflightv1alpha1.PodCheckSpec{
				Namespace:     "ingress",
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "ingress"}},
				MinReady:      1,
			},
		},
	}

	pp := &preflightv1alpha1.PreflightProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "test-valid-dynamic"},
		Spec: preflightv1alpha1.PreflightProfileSpec{
			Checks: []preflightv1alpha1.ProfileCheckRef{
				{PreflightCheckRef: "ingress-ready"},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc, pp).
		WithStatusSubresource(&preflightv1alpha1.PreflightProfile{}).
		Build()

	r := &PreflightProfileReconciler{Client: c}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-valid-dynamic"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated preflightv1alpha1.PreflightProfile
	c.Get(context.Background(), types.NamespacedName{Name: "test-valid-dynamic"}, &updated)
	if !updated.Status.Valid {
		t.Errorf("expected valid=true, got false: %s", updated.Status.Message)
	}
}

func TestPreflightProfileReconciler_NeitherNameNorRef(t *testing.T) {
	pp := &preflightv1alpha1.PreflightProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "test-no-name-or-ref"},
		Spec: preflightv1alpha1.PreflightProfileSpec{
			Checks: []preflightv1alpha1.ProfileCheckRef{
				{}, // neither name nor ref
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pp).
		WithStatusSubresource(&preflightv1alpha1.PreflightProfile{}).
		Build()

	r := &PreflightProfileReconciler{Client: c}
	r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-no-name-or-ref"},
	})

	var updated preflightv1alpha1.PreflightProfile
	c.Get(context.Background(), types.NamespacedName{Name: "test-no-name-or-ref"}, &updated)
	if updated.Status.Valid {
		t.Error("expected valid=false when neither name nor ref specified")
	}
}

func TestPreflightProfileReconciler_BothNameAndRef(t *testing.T) {
	pp := &preflightv1alpha1.PreflightProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "test-both-name-ref"},
		Spec: preflightv1alpha1.PreflightProfileSpec{
			Checks: []preflightv1alpha1.ProfileCheckRef{
				{Name: "profile-test-dns", PreflightCheckRef: "some-ref"},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pp).
		WithStatusSubresource(&preflightv1alpha1.PreflightProfile{}).
		Build()

	r := &PreflightProfileReconciler{Client: c}
	r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-both-name-ref"},
	})

	var updated preflightv1alpha1.PreflightProfile
	c.Get(context.Background(), types.NamespacedName{Name: "test-both-name-ref"}, &updated)
	if updated.Status.Valid {
		t.Error("expected valid=false when both name and ref specified")
	}
}

func TestPreflightProfileReconciler_BuiltinNotRegistered(t *testing.T) {
	pp := &preflightv1alpha1.PreflightProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "test-unknown-builtin"},
		Spec: preflightv1alpha1.PreflightProfileSpec{
			Checks: []preflightv1alpha1.ProfileCheckRef{
				{Name: "nonexistent-builtin-check"},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pp).
		WithStatusSubresource(&preflightv1alpha1.PreflightProfile{}).
		Build()

	r := &PreflightProfileReconciler{Client: c}
	r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-unknown-builtin"},
	})

	var updated preflightv1alpha1.PreflightProfile
	c.Get(context.Background(), types.NamespacedName{Name: "test-unknown-builtin"}, &updated)
	if updated.Status.Valid {
		t.Error("expected valid=false for unregistered built-in check")
	}
}

func TestPreflightProfileReconciler_PreflightCheckCRNotFound(t *testing.T) {
	pp := &preflightv1alpha1.PreflightProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "test-missing-cr"},
		Spec: preflightv1alpha1.PreflightProfileSpec{
			Checks: []preflightv1alpha1.ProfileCheckRef{
				{PreflightCheckRef: "does-not-exist"},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pp).
		WithStatusSubresource(&preflightv1alpha1.PreflightProfile{}).
		Build()

	r := &PreflightProfileReconciler{Client: c}
	r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-missing-cr"},
	})

	var updated preflightv1alpha1.PreflightProfile
	c.Get(context.Background(), types.NamespacedName{Name: "test-missing-cr"}, &updated)
	if updated.Status.Valid {
		t.Error("expected valid=false for missing PreflightCheck CR")
	}
}

func TestPreflightProfileReconciler_AllDisabled(t *testing.T) {
	disabled := false
	pp := &preflightv1alpha1.PreflightProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "test-all-disabled"},
		Spec: preflightv1alpha1.PreflightProfileSpec{
			Checks: []preflightv1alpha1.ProfileCheckRef{
				{Name: "profile-test-dns", Enabled: &disabled},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pp).
		WithStatusSubresource(&preflightv1alpha1.PreflightProfile{}).
		Build()

	r := &PreflightProfileReconciler{Client: c}
	r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-all-disabled"},
	})

	var updated preflightv1alpha1.PreflightProfile
	c.Get(context.Background(), types.NamespacedName{Name: "test-all-disabled"}, &updated)
	if updated.Status.Valid {
		t.Error("expected valid=false when all checks disabled")
	}
}

func TestPreflightProfileReconciler_NotFound(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()

	r := &PreflightProfileReconciler{Client: c}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	if err != nil {
		t.Errorf("expected no error for not-found CR, got %v", err)
	}
}
