package builtin

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clustergate/clustergate/internal/checks"
	"github.com/clustergate/clustergate/internal/checks/controlplane"
	"github.com/clustergate/clustergate/internal/checks/dns"
)

// RegisterAll registers all built-in readiness checks into the global registry.
func RegisterAll(c client.Client, cfg *rest.Config, enableCloudControllerManager bool) {
	RegisterControlPlane(c, cfg, enableCloudControllerManager)
	checks.Register(dns.New(c))
}

// RegisterControlPlane registers only the control plane checks.
// This is the default set for the CLI tool.
func RegisterControlPlane(c client.Client, cfg *rest.Config, enableCloudControllerManager bool) {
	checks.Register(controlplane.NewAPIServerCheck(cfg))
	checks.Register(controlplane.NewEtcdCheck(cfg))
	checks.Register(controlplane.NewSchedulerCheck(c))
	checks.Register(controlplane.NewControllerManagerCheck(c))
	if enableCloudControllerManager {
		checks.Register(controlplane.NewCloudControllerManagerCheck(c))
	}
}
