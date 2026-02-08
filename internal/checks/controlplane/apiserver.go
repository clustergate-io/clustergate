package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/clustergate/clustergate/internal/checks"
)

const (
	APIServerCheckName     = "kube-apiserver"
	defaultHealthzEndpoint = "/healthz"
)

// APIServerConfig configures the kube-apiserver health check.
type APIServerConfig struct {
	Endpoint string `json:"endpoint,omitempty"`
}

// APIServerCheck verifies the API server is healthy via its /healthz endpoint.
type APIServerCheck struct {
	restConfig *rest.Config
}

// NewAPIServerCheck creates a new APIServerCheck using the provided rest.Config.
func NewAPIServerCheck(cfg *rest.Config) *APIServerCheck {
	return &APIServerCheck{restConfig: cfg}
}

func (a *APIServerCheck) Name() string            { return APIServerCheckName }
func (a *APIServerCheck) DefaultSeverity() string { return "critical" }
func (a *APIServerCheck) DefaultCategory() string { return "control-plane" }

func (a *APIServerCheck) Run(ctx context.Context, rawConfig json.RawMessage) (checks.Result, error) {
	cfg := APIServerConfig{Endpoint: defaultHealthzEndpoint}
	if len(rawConfig) > 0 {
		if err := json.Unmarshal(rawConfig, &cfg); err != nil {
			return checks.Result{}, fmt.Errorf("parsing apiserver check config: %w", err)
		}
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultHealthzEndpoint
	}

	return doHealthzRequest(ctx, a.restConfig, cfg.Endpoint, APIServerCheckName)
}

// doHealthzRequest performs an authenticated HTTP GET against the API server's
// health endpoint and returns a checks.Result.
func doHealthzRequest(ctx context.Context, restCfg *rest.Config, path, checkName string) (checks.Result, error) {
	details := map[string]string{
		"endpoint": path,
	}

	transportCfg, err := restCfg.TransportConfig()
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("%s: failed to build transport config: %v", checkName, err),
			Details: details,
		}, nil
	}

	rt, err := transport.New(transportCfg)
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("%s: failed to create transport: %v", checkName, err),
			Details: details,
		}, nil
	}

	httpClient := &http.Client{
		Transport: rt,
		Timeout:   10 * time.Second,
	}

	url := restCfg.Host + path
	details["url"] = url

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("%s: failed to create request: %v", checkName, err),
			Details: details,
		}, nil
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("%s: health request failed: %v", checkName, err),
			Details: details,
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	details["statusCode"] = fmt.Sprintf("%d", resp.StatusCode)
	details["body"] = string(body)

	if resp.StatusCode != http.StatusOK {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("%s: unhealthy (status %d): %s", checkName, resp.StatusCode, string(body)),
			Details: details,
		}, nil
	}

	return checks.Result{
		Ready:   true,
		Message: fmt.Sprintf("%s: healthy (status 200)", checkName),
		Details: details,
	}, nil
}
