# ClusterGate — Roadmap

Prioritized list of improvements to bring the project to production-grade quality. Items are grouped by priority: **Critical** (must-have before production), **High** (should-have for operational confidence), and **Medium** (nice-to-have for adoption and maintainability).

Items marked with ✅ have been completed.

---

## Critical

### ✅ 1. CRD Scope — Make All Resources Cluster-Scoped

GateCheck and GateProfile were namespaced, but ClusterReadiness is cluster-scoped and references them by name only (no namespace qualifier). All three resources should be cluster-scoped since they represent cluster-wide platform health, not tenant workloads.

**Status:** Completed — `scope=Cluster` markers added to GateCheck and GateProfile.

### ✅ 2. Generate CRD Manifests from Markers

The CRD YAML files in `config/crd/bases/` were hand-written. They are now generated from Go type markers using `controller-gen crd`.

**Status:** Completed — `make manifests` generates CRDs with `allowDangerousTypes=true` for PromQL float thresholds.

### ✅ 3. RBAC — Non-Resource URL Access for Health Checks

Built-in checks (etcd, apiserver) probe `/healthz/*` endpoints, which require explicit non-resource URL RBAC rules in the ClusterRole.

**Status:** Completed — RBAC markers added for `/healthz/*`, `/livez/*`, `/readyz/*`.

### 4. Tighten RBAC — Remove Wildcard Rules

The current ClusterRole includes `apiGroups: ["*"], resources: ["*"]` for the ResourceCheck executor. This grants read access to every resource in the cluster, including Secrets. Replace the wildcard with explicit rules for the resource types users actually check (Deployments, StatefulSets, DaemonSets, etc.), or document the security implications and provide a restrictive alternative.

**Files:** `config/rbac/role.yaml`

### 5. Namespace-Scoped ScriptCheck Isolation

ScriptCheck creates Jobs in the operator's own namespace by default. If a GateCheck CR specifies a malicious image or mounts sensitive volumes, the Job runs with the operator's service account context. Add:
- A dedicated ServiceAccount for ScriptCheck Jobs (not the operator's own SA)
- A `SecurityContext` with `runAsNonRoot`, `readOnlyRootFilesystem`, `drop ALL capabilities` on all ScriptCheck Jobs by default
- Network policy to restrict Job pod egress if possible

**Files:** `internal/checks/dynamic/script_check.go`, new RBAC manifests

### 6. Input Validation and Admission Controls

Add webhook-based validation for GateCheck CRs to catch invalid specs at admission time rather than during reconciliation:
- Reject unknown check types
- Validate URL formats in HTTPCheck
- Validate PromQL syntax in PromQLCheck
- Enforce image allow-lists for ScriptCheck (configurable)

**Files:** new webhook handler, `api/v1alpha1/`, `config/webhook/`

---

## High

### 7. ProfileRef ExcludeChecks Support

When a ClusterReadiness references a GateProfile, there is no way to exclude individual checks from that profile without forking the entire profile. The `ProfileRef` type now has an `excludeChecks` field but the resolver does not yet honor it. Implement filtering in `ResolveChecks()` so excluded checks are skipped.

**Files:** `internal/controller/resolver.go`

### 8. Richer `/readyz` Endpoint Response

The `/readyz` endpoint currently returns only `{"ready":false}` with no detail about which checks are failing, their severity, or category breakdowns. Platform engineers integrating with CI gates or load balancers need actionable information without running `kubectl describe`. Add:
- Per-check status in the response body
- Summary counts (passing/failing/total)
- Category breakdowns
- Support for `?verbose=true` for full detail vs. minimal for health probes

**Files:** `internal/server/health.go`

### 9. CLI Subcommand Structure

The CLI uses raw flags (`./bin/clustergate --checks dns,etcd`) instead of the `check` subcommand documented in the README (`./bin/clustergate check --checks dns,etcd`). Adopt a proper subcommand structure (e.g., using cobra) so the CLI can grow to support future commands like `clustergate status` (query the CR) or `clustergate profile list`.

**Files:** `cmd/clustergate/main.go`

### 10. CLI DNS Check Fails Outside Cluster

Running the CLI from outside the cluster fails the DNS check because it tries to resolve `kubernetes.default.svc.cluster.local` against the host's DNS resolver, not the cluster's CoreDNS. The CLI should either:
- Skip DNS resolution when running out-of-cluster and only check CoreDNS pod readiness
- Support a `--dns-server` flag to target the cluster's DNS directly
- Clearly indicate in the output that DNS resolution is expected to fail outside the cluster

**Files:** `internal/checks/dns/dns.go`, `cmd/clustergate/main.go`

### 11. `kubectl get` Output Improvements

- GateProfile should show the number of checks (currently shows nothing because the JSONPath pointed at an array, not a count)
- ClusterReadiness should show a human-friendly status message (e.g., "5/6 passing") instead of separate columns
- Consider adding a `Reason` column showing `CriticalChecksFailing` or `AllChecksPassing`

**Files:** `api/v1alpha1/*_types.go`

### 12. Dockerfile Multi-Stage Build

The Dockerfile copies pre-built binaries and requires `CGO_ENABLED=0` to be set externally. A multi-stage Dockerfile that builds from source would be more portable, ensure static linking, and work correctly in CI without relying on the host build environment.

**Files:** `Dockerfile`

### 13. Resource Usage Measurement and Capacity Planning

Profile the operator under realistic workloads to establish baseline resource consumption and right-size requests/limits. Current defaults (100m/64Mi request, 500m/128Mi limit) are estimates with no load-testing data behind them.

**Files:** `config/manager/manager.yaml`, README

### 14. Structured Logging

Replace `setupLog` and ad-hoc log calls with structured logging throughout all reconcilers and check executors. Use consistent keys (`check`, `namespace`, `duration`, `error`) so logs are grep-friendly and indexable.

**Files:** `internal/controller/*.go`, `internal/checks/**/*.go`

### 15. Graceful Shutdown and Signal Handling

The `/readyz` HTTP server runs in a plain goroutine with no shutdown path. If the manager receives SIGTERM, the HTTP server may drop in-flight requests. Use `mgr.Add(manager.RunnableFunc(...))` to register the HTTP server with the controller-runtime lifecycle.

**Files:** `cmd/manager/main.go`

### 16. Retry and Back-off for Transient Failures

Dynamic checks (HTTP, PromQL) currently fail immediately on transient errors (network blips, brief Prometheus unavailability). Add configurable retry with exponential back-off to reduce false negatives.

**Files:** `internal/checks/dynamic/http_check.go`, `internal/checks/dynamic/promql_check.go`

### 17. Rate Limiting and Concurrency Controls

When many ClusterReadiness CRs or GateCheck CRs exist, reconciliation can generate a burst of simultaneous HTTP and API calls. Add:
- `MaxConcurrentReconciles` option on the controller
- Semaphore or worker pool for concurrent dynamic check execution within a single reconciliation

**Files:** `internal/controller/clusterreadiness_controller.go`, `cmd/manager/main.go`

### 18. End-to-End Tests

Add E2E tests that deploy the operator to a real (or kind) cluster and verify:
- CRD creation and validation
- ClusterReadiness status updates
- Built-in check execution
- ScriptCheck Job lifecycle
- `/readyz` endpoint responses
- Prometheus metric emission

**Files:** new `test/e2e/` directory

### 19. CI/CD Pipeline

Set up GitHub Actions (or equivalent) with:
- `go vet`, `golangci-lint`, `go test ./...` on every PR
- Integration tests with envtest
- Docker image build verification
- Optional: E2E tests with kind

**Files:** `.github/workflows/ci.yaml`

### 20. Helm Chart

Package the operator as a Helm chart for standard Kubernetes distribution. Include configurable values for: image, replicas, resource requests/limits, RBAC scope, leader election, namespace, and check intervals.

**Files:** new `charts/clustergate/` directory

### 21. Status Observability Improvements

- Add `observedGeneration` to all CRD status types so clients can detect stale status
- Add a `lastTransitionTime` or `duration` field to per-check status entries
- Emit Kubernetes Events for check state transitions (passing -> failing, failing -> passing)

**Files:** `api/v1alpha1/*_types.go`, `internal/controller/clusterreadiness_controller.go`

---

## Medium

### 22. Makefile — `kind-load` Target

Local development with kind requires manually specifying the cluster name (`kind load docker-image --name <cluster>`). Add a `make kind-load` target that auto-detects the kind cluster and loads the image.

**Files:** `Makefile`

### 23. Sample YAMLs — Separate Base and Extended Examples

The sample CRs reference services that may not exist (Istio, Vault), causing immediate failures that confuse new users. Provide:
- `config/samples/basic/` — minimal examples that pass on any cluster (control plane + DNS only)
- `config/samples/extended/` — full examples showing dynamic checks, profiles with Istio/Vault references

**Files:** `config/samples/`

### 24. Finalizers for ClusterReadiness Cleanup

When a ClusterReadiness CR is deleted, the readiness state and metrics for that CR should be cleaned up. Add a finalizer to ensure `ReadinessState.Remove()` and metric label cleanup happen before the CR is removed from the API server.

**Files:** `internal/controller/clusterreadiness_controller.go`

### 25. Check Result Caching and Deduplication

If multiple ClusterReadiness CRs reference the same GateCheck, the check is executed independently for each CR. Add a shared result cache with TTL so the same check is not executed more frequently than its interval, regardless of how many CRs reference it.

**Files:** `internal/checks/dynamic/executor.go`, new cache package

### 26. Configuration Documentation

Document all built-in check configuration options (JSON schema for each check's `config` field) either in the README or in a dedicated `docs/` directory. Include examples for each check type.

**Files:** README or new `docs/checks.md`

### 27. Controller Restart Resilience

On startup, the operator has no historical check state — all checks show as unknown until the first reconciliation cycle completes. Consider persisting the last-known status in the CR status and using it as the initial state on restart, with a `stale` flag until checks re-run.

**Files:** `internal/controller/clusterreadiness_controller.go`

### 28. Multi-Cluster Support

The architecture supports multiple ClusterReadiness CRs per cluster, but there is no mechanism for a single operator instance to monitor remote clusters. Consider adding a `kubeconfig` or `context` field to ClusterReadiness for multi-cluster setups.

**Files:** `api/v1alpha1/clusterreadiness_types.go`, `internal/controller/`

### 29. Alert Integration

Add optional alert routing configuration (webhook URL, Slack, PagerDuty) so check failures can trigger notifications without requiring a separate alerting stack. This is a convenience feature — most production environments will use Prometheus alerting rules on the exported metrics instead.

**Files:** new `internal/notifier/` package, `api/v1alpha1/clusterreadiness_types.go`

### 30. Documentation Site

Publish a documentation site (e.g., via MkDocs or Docusaurus) with:
- Getting started guide
- Architecture deep-dive
- Check type reference
- API reference (generated from CRD OpenAPI schema)
- Troubleshooting guide

**Files:** new `docs/` directory
