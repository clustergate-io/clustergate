package dynamic

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
)

func deploymentWithConditions(name, namespace string, conditions []interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["status"] = map[string]interface{}{
		"conditions": conditions,
	}
	return obj
}

func TestResourceCheck_NamedResourceMatching(t *testing.T) {
	deploy := deploymentWithConditions("cert-manager", "cert-manager", []interface{}{
		map[string]interface{}{"type": "Available", "status": "True"},
	})

	c := fake.NewClientBuilder().
		WithScheme(dynamicTestScheme()).
		WithObjects(deploy).
		Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		ResourceCheck: &clustergatev1alpha1.ResourceCheckSpec{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "cert-manager",
			Name:       "cert-manager",
			Conditions: []clustergatev1alpha1.ResourceConditionCheck{
				{Type: "Available", Status: "True"},
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected ready=true, got false: %s", result.Message)
	}
}

func TestResourceCheck_ConditionMismatch(t *testing.T) {
	deploy := deploymentWithConditions("cert-manager", "cert-manager", []interface{}{
		map[string]interface{}{"type": "Available", "status": "False"},
	})

	c := fake.NewClientBuilder().
		WithScheme(dynamicTestScheme()).
		WithObjects(deploy).
		Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		ResourceCheck: &clustergatev1alpha1.ResourceCheckSpec{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "cert-manager",
			Name:       "cert-manager",
			Conditions: []clustergatev1alpha1.ResourceConditionCheck{
				{Type: "Available", Status: "True"},
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false for condition mismatch")
	}
}

func TestResourceCheck_ResourceNotFound(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		ResourceCheck: &clustergatev1alpha1.ResourceCheckSpec{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "default",
			Name:       "does-not-exist",
			Conditions: []clustergatev1alpha1.ResourceConditionCheck{
				{Type: "Available", Status: "True"},
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false for missing resource")
	}
}

func TestResourceCheck_NoConditions(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "apps", Version: "v1", Kind: "Deployment",
	})
	obj.SetName("no-conditions")
	obj.SetNamespace("default")
	// No status.conditions field

	c := fake.NewClientBuilder().
		WithScheme(dynamicTestScheme()).
		WithObjects(obj).
		Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		ResourceCheck: &clustergatev1alpha1.ResourceCheckSpec{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "default",
			Name:       "no-conditions",
			Conditions: []clustergatev1alpha1.ResourceConditionCheck{
				{Type: "Available", Status: "True"},
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false when resource has no conditions")
	}
}

func TestResourceCheck_NeitherNameNorSelector(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		ResourceCheck: &clustergatev1alpha1.ResourceCheckSpec{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Conditions: []clustergatev1alpha1.ResourceConditionCheck{
				{Type: "Available", Status: "True"},
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false when neither name nor labelSelector provided")
	}
}

func TestResourceCheck_LabelSelectorMultiple(t *testing.T) {
	deploy1 := deploymentWithConditions("deploy-a", "default", []interface{}{
		map[string]interface{}{"type": "Available", "status": "True"},
	})
	deploy1.SetLabels(map[string]string{"tier": "frontend"})

	deploy2 := deploymentWithConditions("deploy-b", "default", []interface{}{
		map[string]interface{}{"type": "Available", "status": "True"},
	})
	deploy2.SetLabels(map[string]string{"tier": "frontend"})

	c := fake.NewClientBuilder().
		WithScheme(dynamicTestScheme()).
		WithObjects(deploy1, deploy2).
		Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		ResourceCheck: &clustergatev1alpha1.ResourceCheckSpec{
			APIVersion:    "apps/v1",
			Kind:          "Deployment",
			Namespace:     "default",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "frontend"}},
			Conditions: []clustergatev1alpha1.ResourceConditionCheck{
				{Type: "Available", Status: "True"},
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected ready=true for multiple matching resources: %s", result.Message)
	}
}
