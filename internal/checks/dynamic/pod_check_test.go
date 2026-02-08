package dynamic

import (
	"context"
	"net/http"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
)

func dynamicTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(preflightv1alpha1.AddToScheme(s))
	return s
}

// newTestExecutor creates an Executor suitable for unit tests, without requiring a rest.Config.
func newTestExecutor(c client.Client) *Executor {
	return &Executor{
		client: c,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func readyPod(name, namespace string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
}

func runningButNotReadyPod(name, namespace string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			// No Ready condition
		},
	}
}

func TestPodCheck_PodsReady(t *testing.T) {
	labels := map[string]string{"app": "nginx"}
	c := fake.NewClientBuilder().
		WithScheme(dynamicTestScheme()).
		WithObjects(
			readyPod("pod-1", "ingress", labels),
			readyPod("pod-2", "ingress", labels),
		).
		Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", preflightv1alpha1.PreflightCheckSpec{
		PodCheck: &preflightv1alpha1.PodCheckSpec{
			Namespace:     "ingress",
			LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
			MinReady:      2,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected ready=true, got false: %s", result.Message)
	}
}

func TestPodCheck_InsufficientReady(t *testing.T) {
	labels := map[string]string{"app": "nginx"}
	c := fake.NewClientBuilder().
		WithScheme(dynamicTestScheme()).
		WithObjects(
			readyPod("pod-1", "ingress", labels),
		).
		Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", preflightv1alpha1.PreflightCheckSpec{
		PodCheck: &preflightv1alpha1.PodCheckSpec{
			Namespace:     "ingress",
			LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
			MinReady:      3,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false for insufficient pods")
	}
}

func TestPodCheck_NoPods(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", preflightv1alpha1.PreflightCheckSpec{
		PodCheck: &preflightv1alpha1.PodCheckSpec{
			Namespace:     "ingress",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
			MinReady:      1,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false when no pods found")
	}
}

func TestPodCheck_RunningButNotReady(t *testing.T) {
	labels := map[string]string{"app": "nginx"}
	c := fake.NewClientBuilder().
		WithScheme(dynamicTestScheme()).
		WithObjects(
			runningButNotReadyPod("pod-1", "ingress", labels),
			readyPod("pod-2", "ingress", labels),
		).
		Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", preflightv1alpha1.PreflightCheckSpec{
		PodCheck: &preflightv1alpha1.PodCheckSpec{
			Namespace:     "ingress",
			LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
			MinReady:      2,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false: one pod is running but not ready")
	}
}

func TestPodCheck_NilLabelSelector(t *testing.T) {
	c := fake.NewClientBuilder().
		WithScheme(dynamicTestScheme()).
		WithObjects(
			readyPod("pod-1", "default", map[string]string{"app": "a"}),
			readyPod("pod-2", "default", map[string]string{"app": "b"}),
		).
		Build()

	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", preflightv1alpha1.PreflightCheckSpec{
		PodCheck: &preflightv1alpha1.PodCheckSpec{
			Namespace: "default",
			MinReady:  2,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected ready=true with nil selector matching all pods: %s", result.Message)
	}
}
