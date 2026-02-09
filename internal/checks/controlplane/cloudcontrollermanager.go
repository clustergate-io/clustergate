package controlplane

import (
	"context"
	"encoding/json"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clustergate/clustergate/internal/checks"
)

const CloudControllerManagerCheckName = "cloud-controller-manager"

// CloudControllerManagerCheck verifies cloud-controller-manager health by inspecting its leader-election Lease.
// This check is only registered when the --enable-cloud-controller-manager flag is set.
type CloudControllerManagerCheck struct {
	client client.Client
}

// NewCloudControllerManagerCheck creates a new CloudControllerManagerCheck.
func NewCloudControllerManagerCheck(c client.Client) *CloudControllerManagerCheck {
	return &CloudControllerManagerCheck{client: c}
}

func (c *CloudControllerManagerCheck) Name() string            { return CloudControllerManagerCheckName }
func (c *CloudControllerManagerCheck) DefaultSeverity() string { return "critical" }
func (c *CloudControllerManagerCheck) DefaultCategory() string { return "control-plane" }

func (c *CloudControllerManagerCheck) Run(ctx context.Context, rawConfig json.RawMessage) (checks.Result, error) {
	return checkLease(ctx, c.client, rawConfig, "cloud-controller-manager", CloudControllerManagerCheckName)
}
