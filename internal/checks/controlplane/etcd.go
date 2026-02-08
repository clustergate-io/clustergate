package controlplane

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/camcast3/platform-preflight/internal/checks"
)

const (
	EtcdCheckName          = "etcd"
	defaultEtcdHealthzPath = "/healthz/etcd"
)

// EtcdConfig configures the etcd health check.
type EtcdConfig struct {
	Endpoint string `json:"endpoint,omitempty"`
}

// EtcdCheck verifies etcd health via the API server's proxied /healthz/etcd endpoint.
type EtcdCheck struct {
	restConfig *rest.Config
}

// NewEtcdCheck creates a new EtcdCheck using the provided rest.Config.
func NewEtcdCheck(cfg *rest.Config) *EtcdCheck {
	return &EtcdCheck{restConfig: cfg}
}

func (e *EtcdCheck) Name() string            { return EtcdCheckName }
func (e *EtcdCheck) DefaultSeverity() string { return "critical" }
func (e *EtcdCheck) DefaultCategory() string { return "control-plane" }

func (e *EtcdCheck) Run(ctx context.Context, rawConfig json.RawMessage) (checks.Result, error) {
	cfg := EtcdConfig{Endpoint: defaultEtcdHealthzPath}
	if len(rawConfig) > 0 {
		if err := json.Unmarshal(rawConfig, &cfg); err != nil {
			return checks.Result{}, fmt.Errorf("parsing etcd check config: %w", err)
		}
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultEtcdHealthzPath
	}

	return doHealthzRequest(ctx, e.restConfig, cfg.Endpoint, EtcdCheckName)
}
