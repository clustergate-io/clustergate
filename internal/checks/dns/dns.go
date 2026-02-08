package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clustergate/clustergate/internal/checks"
)

const CheckName = "dns"

// Config holds DNS check-specific configuration.
type Config struct {
	// TestDomain is the domain to resolve for validation.
	// Defaults to "kubernetes.default.svc.cluster.local".
	TestDomain string `json:"testDomain,omitempty"`
}

// DNSCheck verifies that cluster DNS is operational.
type DNSCheck struct {
	client client.Client
}

// New creates a new DNSCheck with the given Kubernetes client.
func New(c client.Client) *DNSCheck {
	return &DNSCheck{client: c}
}

func (d *DNSCheck) Name() string {
	return CheckName
}

func (d *DNSCheck) DefaultSeverity() string {
	return "critical"
}

func (d *DNSCheck) DefaultCategory() string {
	return "networking"
}

func (d *DNSCheck) Run(ctx context.Context, rawConfig json.RawMessage) (checks.Result, error) {
	cfg := Config{
		TestDomain: "kubernetes.default.svc.cluster.local",
	}
	if len(rawConfig) > 0 {
		if err := json.Unmarshal(rawConfig, &cfg); err != nil {
			return checks.Result{}, fmt.Errorf("parsing dns check config: %w", err)
		}
	}

	details := make(map[string]string)

	// Step 1: Verify CoreDNS pods are running.
	podList := &corev1.PodList{}
	dnsSelector := labels.SelectorFromSet(labels.Set{"k8s-app": "kube-dns"})
	if err := d.client.List(ctx, podList,
		client.InNamespace("kube-system"),
		client.MatchingLabelsSelector{Selector: dnsSelector},
	); err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("failed to list DNS pods: %v", err),
		}, nil
	}

	runningCount := 0
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningCount++
		}
	}
	details["dnsPodsRunning"] = fmt.Sprintf("%d", runningCount)

	if runningCount == 0 {
		return checks.Result{
			Ready:   false,
			Message: "no DNS pods found in Running state in kube-system",
			Details: details,
		}, nil
	}

	// Step 2: Attempt DNS resolution.
	resolver := &net.Resolver{}
	addrs, err := resolver.LookupHost(ctx, cfg.TestDomain)
	if err != nil {
		details["resolveError"] = err.Error()
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("DNS resolution failed for %s: %v", cfg.TestDomain, err),
			Details: details,
		}, nil
	}
	details["resolvedAddresses"] = fmt.Sprintf("%v", addrs)

	return checks.Result{
		Ready:   true,
		Message: fmt.Sprintf("DNS operational: %d pods running, %s resolves to %v", runningCount, cfg.TestDomain, addrs),
		Details: details,
	}, nil
}
