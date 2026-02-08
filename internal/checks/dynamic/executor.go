package dynamic

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
	"github.com/camcast3/platform-preflight/internal/checks"
)

// Executor evaluates PreflightCheck specs at runtime.
type Executor struct {
	client     client.Client
	httpClient *http.Client
	clientset  kubernetes.Interface
	namespace  string
}

// NewExecutor creates a new dynamic check executor.
// The rest.Config is used to build a kubernetes.Clientset for Job-based checks.
// namespace is the namespace where script check Jobs will be created.
func NewExecutor(c client.Client, cfg *rest.Config, namespace string) (*Executor, error) {
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset for script checks: %w", err)
	}
	return &Executor{
		client: c,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		clientset: cs,
		namespace: namespace,
	}, nil
}

// Execute runs the appropriate check type from a PreflightCheckSpec.
// The checkName is used to label Jobs for ScriptCheck-based checks.
func (e *Executor) Execute(ctx context.Context, checkName string, spec preflightv1alpha1.PreflightCheckSpec) (checks.Result, error) {
	switch {
	case spec.PodCheck != nil:
		return e.executePodCheck(ctx, spec.PodCheck)
	case spec.HTTPCheck != nil:
		return e.executeHTTPCheck(ctx, spec.HTTPCheck)
	case spec.ResourceCheck != nil:
		return e.executeResourceCheck(ctx, spec.ResourceCheck)
	case spec.PromQLCheck != nil:
		return e.executePromQLCheck(ctx, spec.PromQLCheck)
	case spec.ScriptCheck != nil:
		return executeScriptCheck(ctx, e.clientset, e.namespace, checkName, spec.ScriptCheck)
	default:
		return checks.Result{}, fmt.Errorf("no check type specified in PreflightCheck")
	}
}

// httpClientForSpec returns an HTTP client configured for the check spec.
func httpClientForSpec(insecureSkipTLS bool, timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecureSkipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}
