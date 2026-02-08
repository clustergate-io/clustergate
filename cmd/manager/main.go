package main

import (
	"flag"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
	"github.com/camcast3/platform-preflight/internal/checks"
	"github.com/camcast3/platform-preflight/internal/checks/controlplane"
	"github.com/camcast3/platform-preflight/internal/checks/dns"
	"github.com/camcast3/platform-preflight/internal/checks/dynamic"
	"github.com/camcast3/platform-preflight/internal/controller"
	_ "github.com/camcast3/platform-preflight/internal/metrics" // register prometheus collectors
	"github.com/camcast3/platform-preflight/internal/server"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(preflightv1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr                  string
		probeAddr                    string
		readyzAddr                   string
		leaderElect                  bool
		enableCloudControllerManager bool
		namespace                    string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metrics endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&readyzAddr, "readyz-bind-address", ":8082", "The address the readyz endpoint binds to.")
	flag.BoolVar(&leaderElect, "leader-elect", false,
		"Enable leader election for controller manager. Ensures only one active controller instance.")
	flag.BoolVar(&enableCloudControllerManager, "enable-cloud-controller-manager", false,
		"Enable the cloud-controller-manager health check. Set to true for cloud-provider Kubernetes clusters.")
	flag.StringVar(&namespace, "namespace", "platform-preflight-system",
		"The namespace where the operator runs. Used for creating script check Jobs.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         leaderElect,
		LeaderElectionID:       "platform-preflight.preflight.platform.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	// Register built-in checks now that we have a client.
	registerChecks(mgr.GetClient(), mgr.GetConfig(), enableCloudControllerManager)
	setupLog.Info("registered checks", "available", checks.List())

	// Shared readiness state between controller and HTTP server.
	readinessState := server.NewReadinessState()

	// Create the dynamic executor for PreflightCheck CRs.
	dynamicExecutor, err := dynamic.NewExecutor(mgr.GetClient(), mgr.GetConfig(), namespace)
	if err != nil {
		setupLog.Error(err, "unable to create dynamic executor")
		os.Exit(1)
	}

	// Set up the ClusterReadiness reconciler.
	if err := (&controller.ClusterReadinessReconciler{
		Client:          mgr.GetClient(),
		ReadinessState:  readinessState,
		DynamicExecutor: dynamicExecutor,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterReadiness")
		os.Exit(1)
	}

	// Set up the PreflightCheck validation reconciler.
	if err := (&controller.PreflightCheckReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PreflightCheck")
		os.Exit(1)
	}

	// Set up the PreflightProfile validation reconciler.
	if err := (&controller.PreflightProfileReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PreflightProfile")
		os.Exit(1)
	}

	// Standard liveness/readiness probes for the operator pod itself.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Start the cluster readyz HTTP server for external consumers.
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/readyz", server.ReadyzHandler(readinessState))
		setupLog.Info("starting cluster readyz server", "addr", readyzAddr)
		if err := http.ListenAndServe(readyzAddr, mux); err != nil {
			setupLog.Error(err, "cluster readyz server failed")
		}
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// registerChecks registers all built-in readiness checks.
// New checks should be added here.
func registerChecks(c client.Client, cfg *rest.Config, enableCCM bool) {
	checks.Register(dns.New(c))
	checks.Register(controlplane.NewAPIServerCheck(cfg))
	checks.Register(controlplane.NewEtcdCheck(cfg))
	checks.Register(controlplane.NewSchedulerCheck(c))
	checks.Register(controlplane.NewControllerManagerCheck(c))
	if enableCCM {
		checks.Register(controlplane.NewCloudControllerManagerCheck(c))
	}
}
