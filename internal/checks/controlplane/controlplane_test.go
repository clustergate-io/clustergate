package controlplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// ---------------------------------------------------------------------------
// API Server Check Tests
// ---------------------------------------------------------------------------

func TestAPIServerCheck_Metadata(t *testing.T) {
	check := NewAPIServerCheck(&rest.Config{})
	if check.Name() != "kube-apiserver" {
		t.Errorf("Name() = %q, want %q", check.Name(), "kube-apiserver")
	}
	if check.DefaultSeverity() != "critical" {
		t.Errorf("DefaultSeverity() = %q, want %q", check.DefaultSeverity(), "critical")
	}
	if check.DefaultCategory() != "control-plane" {
		t.Errorf("DefaultCategory() = %q, want %q", check.DefaultCategory(), "control-plane")
	}
}

func TestAPIServerCheck_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	check := NewAPIServerCheck(&rest.Config{Host: srv.URL})
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected Ready=true, got false: %s", result.Message)
	}
	if result.Details["statusCode"] != "200" {
		t.Errorf("expected statusCode=200, got %s", result.Details["statusCode"])
	}
}

func TestAPIServerCheck_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	check := NewAPIServerCheck(&rest.Config{Host: srv.URL})
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Errorf("expected Ready=false, got true")
	}
	if result.Details["statusCode"] != "500" {
		t.Errorf("expected statusCode=500, got %s", result.Details["statusCode"])
	}
}

func TestAPIServerCheck_InvalidConfig(t *testing.T) {
	check := NewAPIServerCheck(&rest.Config{})
	_, err := check.Run(context.Background(), json.RawMessage(`{invalid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON config, got nil")
	}
}

func TestAPIServerCheck_CustomEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom-health" {
			t.Errorf("unexpected path: %s, want /custom-health", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	check := NewAPIServerCheck(&rest.Config{Host: srv.URL})
	cfg := json.RawMessage(`{"endpoint": "/custom-health"}`)
	result, err := check.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected Ready=true, got false: %s", result.Message)
	}
}

// ---------------------------------------------------------------------------
// Etcd Check Tests
// ---------------------------------------------------------------------------

func TestEtcdCheck_Metadata(t *testing.T) {
	check := NewEtcdCheck(&rest.Config{})
	if check.Name() != "etcd" {
		t.Errorf("Name() = %q, want %q", check.Name(), "etcd")
	}
	if check.DefaultSeverity() != "critical" {
		t.Errorf("DefaultSeverity() = %q, want %q", check.DefaultSeverity(), "critical")
	}
	if check.DefaultCategory() != "control-plane" {
		t.Errorf("DefaultCategory() = %q, want %q", check.DefaultCategory(), "control-plane")
	}
}

func TestEtcdCheck_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz/etcd" {
			t.Errorf("unexpected path: %s, want /healthz/etcd", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	check := NewEtcdCheck(&rest.Config{Host: srv.URL})
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected Ready=true, got false: %s", result.Message)
	}
}

func TestEtcdCheck_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("etcd unhealthy"))
	}))
	defer srv.Close()

	check := NewEtcdCheck(&rest.Config{Host: srv.URL})
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Errorf("expected Ready=false, got true")
	}
}

func TestEtcdCheck_InvalidConfig(t *testing.T) {
	check := NewEtcdCheck(&rest.Config{})
	_, err := check.Run(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON config, got nil")
	}
}

// ---------------------------------------------------------------------------
// Scheduler Check Tests
// ---------------------------------------------------------------------------

func newFakeScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = coordinationv1.AddToScheme(s)
	return s
}

func TestSchedulerCheck_Metadata(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).Build()
	check := NewSchedulerCheck(c)
	if check.Name() != "kube-scheduler" {
		t.Errorf("Name() = %q, want %q", check.Name(), "kube-scheduler")
	}
	if check.DefaultSeverity() != "critical" {
		t.Errorf("DefaultSeverity() = %q, want %q", check.DefaultSeverity(), "critical")
	}
	if check.DefaultCategory() != "control-plane" {
		t.Errorf("DefaultCategory() = %q, want %q", check.DefaultCategory(), "control-plane")
	}
}

func TestSchedulerCheck_HealthyLease(t *testing.T) {
	renewTime := metav1.NewMicroTime(time.Now().Add(-5 * time.Second))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-scheduler",
			Namespace: "kube-system",
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).WithObjects(lease).Build()
	check := NewSchedulerCheck(c)
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected Ready=true, got false: %s", result.Message)
	}
}

func TestSchedulerCheck_StaleLease(t *testing.T) {
	renewTime := metav1.NewMicroTime(time.Now().Add(-5 * time.Minute))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-scheduler",
			Namespace: "kube-system",
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).WithObjects(lease).Build()
	check := NewSchedulerCheck(c)
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Errorf("expected Ready=false for stale lease, got true")
	}
}

func TestSchedulerCheck_LeaseNotFound(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).Build()
	check := NewSchedulerCheck(c)
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Errorf("expected Ready=false when lease is missing, got true")
	}
}

func TestSchedulerCheck_NilRenewTime(t *testing.T) {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-scheduler",
			Namespace: "kube-system",
		},
		Spec: coordinationv1.LeaseSpec{},
	}

	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).WithObjects(lease).Build()
	check := NewSchedulerCheck(c)
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Errorf("expected Ready=false when renewTime is nil, got true")
	}
}

// ---------------------------------------------------------------------------
// Controller Manager Check Tests
// ---------------------------------------------------------------------------

func TestControllerManagerCheck_Metadata(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).Build()
	check := NewControllerManagerCheck(c)
	if check.Name() != "kube-controller-manager" {
		t.Errorf("Name() = %q, want %q", check.Name(), "kube-controller-manager")
	}
	if check.DefaultSeverity() != "critical" {
		t.Errorf("DefaultSeverity() = %q, want %q", check.DefaultSeverity(), "critical")
	}
	if check.DefaultCategory() != "control-plane" {
		t.Errorf("DefaultCategory() = %q, want %q", check.DefaultCategory(), "control-plane")
	}
}

func TestControllerManagerCheck_HealthyLease(t *testing.T) {
	renewTime := metav1.NewMicroTime(time.Now().Add(-5 * time.Second))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-controller-manager",
			Namespace: "kube-system",
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).WithObjects(lease).Build()
	check := NewControllerManagerCheck(c)
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected Ready=true, got false: %s", result.Message)
	}
}

// ---------------------------------------------------------------------------
// Cloud Controller Manager Check Tests
// ---------------------------------------------------------------------------

func TestCloudControllerManagerCheck_Metadata(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).Build()
	check := NewCloudControllerManagerCheck(c)
	if check.Name() != "cloud-controller-manager" {
		t.Errorf("Name() = %q, want %q", check.Name(), "cloud-controller-manager")
	}
	if check.DefaultSeverity() != "critical" {
		t.Errorf("DefaultSeverity() = %q, want %q", check.DefaultSeverity(), "critical")
	}
	if check.DefaultCategory() != "control-plane" {
		t.Errorf("DefaultCategory() = %q, want %q", check.DefaultCategory(), "control-plane")
	}
}

func TestCloudControllerManagerCheck_LeaseNotFound(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).Build()
	check := NewCloudControllerManagerCheck(c)
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Errorf("expected Ready=false when lease is missing, got true")
	}
}

// ---------------------------------------------------------------------------
// Shared Lease Helper Tests
// ---------------------------------------------------------------------------

func TestCheckLease_InvalidConfig(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).Build()
	_, err := checkLease(context.Background(), c, json.RawMessage(`{bad`), "test-lease", "test-check")
	if err == nil {
		t.Fatal("expected error for invalid JSON config, got nil")
	}
}

func TestCheckLease_CustomConfig(t *testing.T) {
	renewTime := metav1.NewMicroTime(time.Now().Add(-5 * time.Second))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-custom-lease",
			Namespace: "custom-ns",
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).WithObjects(lease).Build()
	cfg := json.RawMessage(`{"namespace":"custom-ns","leaseName":"my-custom-lease","stalenessThresholdSeconds":120}`)
	result, err := checkLease(context.Background(), c, cfg, "default-name", "test-check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected Ready=true with custom config, got false: %s", result.Message)
	}
	if result.Details["namespace"] != "custom-ns" {
		t.Errorf("expected namespace=custom-ns, got %s", result.Details["namespace"])
	}
	if result.Details["leaseName"] != "my-custom-lease" {
		t.Errorf("expected leaseName=my-custom-lease, got %s", result.Details["leaseName"])
	}
}

func TestCheckLease_CustomThresholdStaleness(t *testing.T) {
	// Lease renewed 90 seconds ago; with a 60s threshold it should be stale,
	// but with a 120s threshold it should be healthy.
	renewTime := metav1.NewMicroTime(time.Now().Add(-90 * time.Second))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-scheduler",
			Namespace: "kube-system",
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	c := fake.NewClientBuilder().WithScheme(newFakeScheme()).WithObjects(lease).Build()

	// With default threshold (60s) — should be stale
	check := NewSchedulerCheck(c)
	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Errorf("expected Ready=false with default 60s threshold for 90s-old lease")
	}

	// With 120s threshold — should be healthy
	cfg := json.RawMessage(`{"stalenessThresholdSeconds":120}`)
	result, err = check.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected Ready=true with 120s threshold for 90s-old lease: %s", result.Message)
	}
}
