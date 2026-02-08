package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
)

func TestPreflightCheckReconciler_ValidPodCheck(t *testing.T) {
	pc := &preflightv1alpha1.PreflightCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod-check"},
		Spec: preflightv1alpha1.PreflightCheckSpec{
			Severity: preflightv1alpha1.SeverityCritical,
			Category: "compute",
			PodCheck: &preflightv1alpha1.PodCheckSpec{
				Namespace:     "default",
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				MinReady:      1,
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc).
		WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
		Build()

	r := &PreflightCheckReconciler{Client: c}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-pod-check"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated preflightv1alpha1.PreflightCheck
	if err := c.Get(context.Background(), types.NamespacedName{Name: "test-pod-check"}, &updated); err != nil {
		t.Fatalf("failed to get updated CR: %v", err)
	}
	if !updated.Status.Valid {
		t.Errorf("expected valid=true, got false: %s", updated.Status.Message)
	}
}

func TestPreflightCheckReconciler_ValidHTTPCheck(t *testing.T) {
	pc := &preflightv1alpha1.PreflightCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "test-http-check"},
		Spec: preflightv1alpha1.PreflightCheckSpec{
			Severity: preflightv1alpha1.SeverityCritical,
			Category: "networking",
			HTTPCheck: &preflightv1alpha1.HTTPCheckSpec{
				URL: "https://example.com/health",
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc).
		WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
		Build()

	r := &PreflightCheckReconciler{Client: c}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-http-check"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated preflightv1alpha1.PreflightCheck
	if err := c.Get(context.Background(), types.NamespacedName{Name: "test-http-check"}, &updated); err != nil {
		t.Fatalf("failed to get updated CR: %v", err)
	}
	if !updated.Status.Valid {
		t.Errorf("expected valid=true, got false: %s", updated.Status.Message)
	}
}

func TestPreflightCheckReconciler_ValidResourceCheck(t *testing.T) {
	pc := &preflightv1alpha1.PreflightCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "test-resource-check"},
		Spec: preflightv1alpha1.PreflightCheckSpec{
			Severity: preflightv1alpha1.SeverityCritical,
			Category: "control-plane",
			ResourceCheck: &preflightv1alpha1.ResourceCheckSpec{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "my-deploy",
				Namespace:  "default",
				Conditions: []preflightv1alpha1.ResourceConditionCheck{
					{Type: "Available", Status: "True"},
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc).
		WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
		Build()

	r := &PreflightCheckReconciler{Client: c}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-resource-check"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated preflightv1alpha1.PreflightCheck
	if err := c.Get(context.Background(), types.NamespacedName{Name: "test-resource-check"}, &updated); err != nil {
		t.Fatalf("failed to get updated CR: %v", err)
	}
	if !updated.Status.Valid {
		t.Errorf("expected valid=true, got false: %s", updated.Status.Message)
	}
}

func TestPreflightCheckReconciler_ValidPromQLCheck(t *testing.T) {
	pc := &preflightv1alpha1.PreflightCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "test-promql-check"},
		Spec: preflightv1alpha1.PreflightCheckSpec{
			Severity: preflightv1alpha1.SeverityCritical,
			Category: "monitoring",
			PromQLCheck: &preflightv1alpha1.PromQLCheckSpec{
				Endpoint: "http://prometheus:9090",
				Query:    "up{job=\"etcd\"} == 1",
				Condition: preflightv1alpha1.PromQLCondition{
					Type:      "resultCount",
					Operator:  "gte",
					Threshold: 3,
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc).
		WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
		Build()

	r := &PreflightCheckReconciler{Client: c}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-promql-check"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated preflightv1alpha1.PreflightCheck
	if err := c.Get(context.Background(), types.NamespacedName{Name: "test-promql-check"}, &updated); err != nil {
		t.Fatalf("failed to get updated CR: %v", err)
	}
	if !updated.Status.Valid {
		t.Errorf("expected valid=true, got false: %s", updated.Status.Message)
	}
}

func TestPreflightCheckReconciler_NoCheckType(t *testing.T) {
	pc := &preflightv1alpha1.PreflightCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "test-no-type"},
		Spec: preflightv1alpha1.PreflightCheckSpec{
			Severity: preflightv1alpha1.SeverityCritical,
			Category: "general",
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc).
		WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
		Build()

	r := &PreflightCheckReconciler{Client: c}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-no-type"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated preflightv1alpha1.PreflightCheck
	if err := c.Get(context.Background(), types.NamespacedName{Name: "test-no-type"}, &updated); err != nil {
		t.Fatalf("failed to get updated CR: %v", err)
	}
	if updated.Status.Valid {
		t.Error("expected valid=false for no check type")
	}
}

func TestPreflightCheckReconciler_MultipleCheckTypes(t *testing.T) {
	pc := &preflightv1alpha1.PreflightCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "test-multi-type"},
		Spec: preflightv1alpha1.PreflightCheckSpec{
			Severity: preflightv1alpha1.SeverityCritical,
			Category: "general",
			PodCheck: &preflightv1alpha1.PodCheckSpec{
				Namespace:     "default",
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				MinReady:      1,
			},
			HTTPCheck: &preflightv1alpha1.HTTPCheckSpec{
				URL: "http://example.com",
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc).
		WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
		Build()

	r := &PreflightCheckReconciler{Client: c}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-multi-type"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated preflightv1alpha1.PreflightCheck
	if err := c.Get(context.Background(), types.NamespacedName{Name: "test-multi-type"}, &updated); err != nil {
		t.Fatalf("failed to get updated CR: %v", err)
	}
	if updated.Status.Valid {
		t.Error("expected valid=false for multiple check types")
	}
}

func TestPreflightCheckReconciler_PodCheckMissingNamespace(t *testing.T) {
	pc := &preflightv1alpha1.PreflightCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod-no-ns"},
		Spec: preflightv1alpha1.PreflightCheckSpec{
			Severity: preflightv1alpha1.SeverityCritical,
			Category: "general",
			PodCheck: &preflightv1alpha1.PodCheckSpec{
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				MinReady:      1,
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc).
		WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
		Build()

	r := &PreflightCheckReconciler{Client: c}
	r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-pod-no-ns"},
	})

	var updated preflightv1alpha1.PreflightCheck
	c.Get(context.Background(), types.NamespacedName{Name: "test-pod-no-ns"}, &updated)
	if updated.Status.Valid {
		t.Error("expected valid=false for missing namespace")
	}
}

func TestPreflightCheckReconciler_PodCheckMissingLabelSelector(t *testing.T) {
	pc := &preflightv1alpha1.PreflightCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod-no-selector"},
		Spec: preflightv1alpha1.PreflightCheckSpec{
			Severity: preflightv1alpha1.SeverityCritical,
			Category: "general",
			PodCheck: &preflightv1alpha1.PodCheckSpec{
				Namespace: "default",
				MinReady:  1,
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc).
		WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
		Build()

	r := &PreflightCheckReconciler{Client: c}
	r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-pod-no-selector"},
	})

	var updated preflightv1alpha1.PreflightCheck
	c.Get(context.Background(), types.NamespacedName{Name: "test-pod-no-selector"}, &updated)
	if updated.Status.Valid {
		t.Error("expected valid=false for missing labelSelector")
	}
}

func TestPreflightCheckReconciler_HTTPCheckMissingURL(t *testing.T) {
	pc := &preflightv1alpha1.PreflightCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "test-http-no-url"},
		Spec: preflightv1alpha1.PreflightCheckSpec{
			Severity:  preflightv1alpha1.SeverityCritical,
			Category:  "general",
			HTTPCheck: &preflightv1alpha1.HTTPCheckSpec{},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(pc).
		WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
		Build()

	r := &PreflightCheckReconciler{Client: c}
	r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-http-no-url"},
	})

	var updated preflightv1alpha1.PreflightCheck
	c.Get(context.Background(), types.NamespacedName{Name: "test-http-no-url"}, &updated)
	if updated.Status.Valid {
		t.Error("expected valid=false for missing URL")
	}
}

func TestPreflightCheckReconciler_ResourceCheckMissingFields(t *testing.T) {
	tests := []struct {
		name string
		spec preflightv1alpha1.ResourceCheckSpec
	}{
		{
			name: "missing apiVersion",
			spec: preflightv1alpha1.ResourceCheckSpec{
				Kind: "Deployment",
				Name: "test",
				Conditions: []preflightv1alpha1.ResourceConditionCheck{
					{Type: "Available", Status: "True"},
				},
			},
		},
		{
			name: "missing name and labelSelector",
			spec: preflightv1alpha1.ResourceCheckSpec{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Conditions: []preflightv1alpha1.ResourceConditionCheck{
					{Type: "Available", Status: "True"},
				},
			},
		},
		{
			name: "missing conditions",
			spec: preflightv1alpha1.ResourceCheckSpec{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &preflightv1alpha1.PreflightCheck{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rc-" + tt.name},
				Spec: preflightv1alpha1.PreflightCheckSpec{
					Severity:      preflightv1alpha1.SeverityCritical,
					Category:      "general",
					ResourceCheck: &tt.spec,
				},
			}

			c := fake.NewClientBuilder().
				WithScheme(testScheme()).
				WithObjects(pc).
				WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
				Build()

			r := &PreflightCheckReconciler{Client: c}
			r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-rc-" + tt.name},
			})

			var updated preflightv1alpha1.PreflightCheck
			c.Get(context.Background(), types.NamespacedName{Name: "test-rc-" + tt.name}, &updated)
			if updated.Status.Valid {
				t.Errorf("expected valid=false for %s", tt.name)
			}
		})
	}
}

func TestPreflightCheckReconciler_PromQLCheckMissingFields(t *testing.T) {
	tests := []struct {
		name string
		spec preflightv1alpha1.PromQLCheckSpec
	}{
		{
			name: "missing endpoint",
			spec: preflightv1alpha1.PromQLCheckSpec{
				Query: "up == 1",
				Condition: preflightv1alpha1.PromQLCondition{
					Type: "resultCount", Operator: "gte", Threshold: 1,
				},
			},
		},
		{
			name: "missing query",
			spec: preflightv1alpha1.PromQLCheckSpec{
				Endpoint: "http://prometheus:9090",
				Condition: preflightv1alpha1.PromQLCondition{
					Type: "resultCount", Operator: "gte", Threshold: 1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &preflightv1alpha1.PreflightCheck{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pq-" + tt.name},
				Spec: preflightv1alpha1.PreflightCheckSpec{
					Severity:    preflightv1alpha1.SeverityCritical,
					Category:    "general",
					PromQLCheck: &tt.spec,
				},
			}

			c := fake.NewClientBuilder().
				WithScheme(testScheme()).
				WithObjects(pc).
				WithStatusSubresource(&preflightv1alpha1.PreflightCheck{}).
				Build()

			r := &PreflightCheckReconciler{Client: c}
			r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "test-pq-" + tt.name},
			})

			var updated preflightv1alpha1.PreflightCheck
			c.Get(context.Background(), types.NamespacedName{Name: "test-pq-" + tt.name}, &updated)
			if updated.Status.Valid {
				t.Errorf("expected valid=false for %s", tt.name)
			}
		})
	}
}

func TestPreflightCheckReconciler_NotFound(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()

	r := &PreflightCheckReconciler{Client: c}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	if err != nil {
		t.Errorf("expected no error for not-found CR, got %v", err)
	}
}
