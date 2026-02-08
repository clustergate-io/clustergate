package controlplane

import (
	"context"
	"encoding/json"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/camcast3/platform-preflight/internal/checks"
)

const SchedulerCheckName = "kube-scheduler"

// SchedulerCheck verifies kube-scheduler health by inspecting its leader-election Lease.
type SchedulerCheck struct {
	client client.Client
}

// NewSchedulerCheck creates a new SchedulerCheck.
func NewSchedulerCheck(c client.Client) *SchedulerCheck {
	return &SchedulerCheck{client: c}
}

func (s *SchedulerCheck) Name() string            { return SchedulerCheckName }
func (s *SchedulerCheck) DefaultSeverity() string { return "critical" }
func (s *SchedulerCheck) DefaultCategory() string { return "control-plane" }

func (s *SchedulerCheck) Run(ctx context.Context, rawConfig json.RawMessage) (checks.Result, error) {
	return checkLease(ctx, s.client, rawConfig, "kube-scheduler", SchedulerCheckName)
}
