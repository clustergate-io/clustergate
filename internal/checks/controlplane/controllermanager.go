package controlplane

import (
	"context"
	"encoding/json"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/camcast3/platform-preflight/internal/checks"
)

const ControllerManagerCheckName = "kube-controller-manager"

// ControllerManagerCheck verifies kube-controller-manager health by inspecting its leader-election Lease.
type ControllerManagerCheck struct {
	client client.Client
}

// NewControllerManagerCheck creates a new ControllerManagerCheck.
func NewControllerManagerCheck(c client.Client) *ControllerManagerCheck {
	return &ControllerManagerCheck{client: c}
}

func (cm *ControllerManagerCheck) Name() string            { return ControllerManagerCheckName }
func (cm *ControllerManagerCheck) DefaultSeverity() string { return "critical" }
func (cm *ControllerManagerCheck) DefaultCategory() string { return "control-plane" }

func (cm *ControllerManagerCheck) Run(ctx context.Context, rawConfig json.RawMessage) (checks.Result, error) {
	return checkLease(ctx, cm.client, rawConfig, "kube-controller-manager", ControllerManagerCheckName)
}
