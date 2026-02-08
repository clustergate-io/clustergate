package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
)

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	scheme    = k8sruntime.NewScheme()
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestMain(m *testing.M) {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(preflightv1alpha1.AddToScheme(scheme))

	ctx, cancel = context.WithCancel(context.Background())

	// KUBEBUILDER_ASSETS is set by the Makefile test targets via setup-envtest.
	// If not set, auto-detect from the project's bin/k8s/ directory.
	binDir := os.Getenv("KUBEBUILDER_ASSETS")
	if binDir == "" {
		// Walk up from test/integration/ to project root, then into bin/k8s/.
		root := filepath.Join("..", "..")
		entries, _ := os.ReadDir(filepath.Join(root, "bin", "k8s"))
		// Pick the last entry (sorted alphabetically = highest version).
		for _, e := range entries {
			if e.IsDir() {
				binDir = filepath.Join(root, "bin", "k8s", e.Name())
			}
		}
	}
	if binDir == "" {
		panic("KUBEBUILDER_ASSETS not set and no binaries found in bin/k8s/. Run: setup-envtest use 1.33.0 --bin-dir bin")
	}

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: binDir,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic("failed to start envtest: " + err.Error())
	}
	if cfg == nil {
		panic("envtest config is nil")
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic("failed to create client: " + err.Error())
	}

	// Give the API server a moment to settle.
	time.Sleep(500 * time.Millisecond)

	exitCode := m.Run()

	cancel()
	if err := testEnv.Stop(); err != nil {
		// On Windows, envtest cannot send Unix signals to stop processes.
		// This is a known limitation — the processes will be cleaned up on exit.
		if runtime.GOOS == "windows" {
			fmt.Fprintf(os.Stderr, "warning: envtest stop: %v\n", err)
		} else {
			panic("failed to stop envtest: " + err.Error())
		}
	}

	os.Exit(exitCode)
}
