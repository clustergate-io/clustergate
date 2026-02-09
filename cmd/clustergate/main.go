package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
	"github.com/clustergate/clustergate/internal/checks"
	"github.com/clustergate/clustergate/internal/checks/builtin"
	"github.com/clustergate/clustergate/internal/cli"
)

func main() {
	var (
		kubeconfig string
		outputFmt  string
		checkNames string
		enableCCM  bool
	)

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (uses in-cluster config if empty)")
	flag.StringVar(&outputFmt, "output", "text", "Output format: text or json")
	flag.StringVar(&checkNames, "checks", "", "Comma-separated list of checks to run (default: all)")
	flag.BoolVar(&enableCCM, "enable-ccm", false, "Enable cloud-controller-manager check")
	flag.Parse()

	cfg, err := loadConfig(kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading kubeconfig: %v\n", err)
		os.Exit(1)
	}

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clustergatev1alpha1.AddToScheme(scheme)

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	builtin.RegisterAll(c, cfg, enableCCM)

	filter := make(map[string]bool)
	if checkNames != "" {
		for _, name := range strings.Split(checkNames, ",") {
			filter[strings.TrimSpace(name)] = true
		}
	}

	ctx := context.Background()
	report := cli.RunChecks(ctx, checks.All(), filter)

	switch outputFmt {
	case "json":
		if err := cli.FormatJSON(os.Stdout, report); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
			os.Exit(1)
		}
	default:
		cli.FormatText(os.Stdout, report)
	}

	if !report.Ready {
		os.Exit(1)
	}
}

func loadConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	// Try in-cluster first, then fall back to default kubeconfig loading rules.
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil).ClientConfig()
}
