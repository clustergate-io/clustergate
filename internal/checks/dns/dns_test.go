package dns

import (
	"context"
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
)

func dnsTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(clustergatev1alpha1.AddToScheme(s))
	return s
}

func TestDNSCheck_Name(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(dnsTestScheme()).Build()
	check := New(c)
	if got := check.Name(); got != "dns" {
		t.Errorf("Name() = %q, want %q", got, "dns")
	}
}

func TestDNSCheck_DefaultSeverity(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(dnsTestScheme()).Build()
	check := New(c)
	if got := check.DefaultSeverity(); got != "critical" {
		t.Errorf("DefaultSeverity() = %q, want %q", got, "critical")
	}
}

func TestDNSCheck_DefaultCategory(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(dnsTestScheme()).Build()
	check := New(c)
	if got := check.DefaultCategory(); got != "networking" {
		t.Errorf("DefaultCategory() = %q, want %q", got, "networking")
	}
}

func TestDNSCheck_InvalidConfig(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(dnsTestScheme()).Build()
	check := New(c)

	_, err := check.Run(context.Background(), json.RawMessage(`{invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON config")
	}
}

func TestDNSCheck_NoDNSPods(t *testing.T) {
	// No pods in the fake client
	c := fake.NewClientBuilder().WithScheme(dnsTestScheme()).Build()
	check := New(c)

	result, err := check.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false when no DNS pods exist")
	}
}
