package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clustergate/clustergate/internal/checks"
)

const (
	defaultLeaseNamespace         = "kube-system"
	defaultStalenessThresholdSecs = 60
)

// LeaseConfig configures a lease-based health check.
type LeaseConfig struct {
	Namespace                 string `json:"namespace,omitempty"`
	LeaseName                 string `json:"leaseName,omitempty"`
	StalenessThresholdSeconds int    `json:"stalenessThresholdSeconds,omitempty"`
}

// checkLease fetches a coordination.k8s.io/v1 Lease and verifies that its
// renewTime is within the staleness threshold. It is used by the scheduler,
// controller-manager, and cloud-controller-manager checks.
func checkLease(ctx context.Context, c client.Client, rawConfig json.RawMessage, defaultLeaseName, checkName string) (checks.Result, error) {
	cfg := LeaseConfig{
		Namespace:                 defaultLeaseNamespace,
		LeaseName:                 defaultLeaseName,
		StalenessThresholdSeconds: defaultStalenessThresholdSecs,
	}
	if len(rawConfig) > 0 {
		if err := json.Unmarshal(rawConfig, &cfg); err != nil {
			return checks.Result{}, fmt.Errorf("parsing %s check config: %w", checkName, err)
		}
	}
	if cfg.Namespace == "" {
		cfg.Namespace = defaultLeaseNamespace
	}
	if cfg.LeaseName == "" {
		cfg.LeaseName = defaultLeaseName
	}
	if cfg.StalenessThresholdSeconds <= 0 {
		cfg.StalenessThresholdSeconds = defaultStalenessThresholdSecs
	}

	details := map[string]string{
		"namespace": cfg.Namespace,
		"leaseName": cfg.LeaseName,
	}

	var lease coordinationv1.Lease
	key := types.NamespacedName{
		Namespace: cfg.Namespace,
		Name:      cfg.LeaseName,
	}
	if err := c.Get(ctx, key, &lease); err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("%s: lease not found: %v", checkName, err),
			Details: details,
		}, nil
	}

	if lease.Spec.RenewTime == nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("%s: lease has no renewTime", checkName),
			Details: details,
		}, nil
	}

	threshold := time.Duration(cfg.StalenessThresholdSeconds) * time.Second
	age := time.Since(lease.Spec.RenewTime.Time)
	details["renewTime"] = lease.Spec.RenewTime.Time.Format(time.RFC3339)
	details["age"] = age.Truncate(time.Second).String()
	details["threshold"] = threshold.String()

	if age > threshold {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("%s: lease is stale (renewed %s ago, threshold %s)", checkName, age.Truncate(time.Second), threshold),
			Details: details,
		}, nil
	}

	return checks.Result{
		Ready:   true,
		Message: fmt.Sprintf("%s: healthy (lease renewed %s ago)", checkName, age.Truncate(time.Second)),
		Details: details,
	}, nil
}
