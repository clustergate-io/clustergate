# ClusterGate — Roadmap

Prioritized list of improvements to bring the project to production-grade quality. Items are grouped by priority: **Critical** (must-have before production), **High** (should-have for operational confidence), and **Medium** (nice-to-have for adoption and maintainability).

---

## Critical

### 1. Tighten RBAC — Remove Wildcard Rules

The current ClusterRole includes `apiGroups: ["*"], resources: ["*"]` for the ResourceCheck executor. This grants read access to every resource in the cluster, including Secrets. Replace the wildcard with explicit rules for the resource types users actually check (Deployments, StatefulSets, DaemonSets, etc.), or document the security implications and provide a restrictive alternative.

**Files:** `config/rbac/role.yaml`

### 2. Namespace-Scoped ScriptCheck Isolation

ScriptCheck creates Jobs in the operator's own namespace by default. If a GateCheck CR specifies a malicious image or mounts sensitive volumes, the Job runs with the operator's service account context. Add:
- A dedicated ServiceAccount for ScriptCheck Jobs (not the operator's own SA)
- A `SecurityContext` with `runAsNonRoot`, `readOnlyRootFilesystem`, `drop ALL capabilities` on all ScriptCheck Jobs by default
- Network policy to restrict Job pod egress if possible

**Files:** `internal/checks/dynamic/script_check.go`, new RBAC manifests

### 3. Input Validation and Admission Controls

Add webhook-based validation for GateCheck CRs to catch invalid specs at admission time rather than during reconciliation:
- Reject unknown check types
- Validate URL formats in HTTPCheck
- Validate PromQL syntax in PromQLCheck
- Enforce image allow-lists for ScriptCheck (configurable)

**Files:** new webhook handler, `api/v1alpha1/`, `config/webhook/`

### 4. Generate CRD Manifests from Markers

The CRD YAML files in `config/crd/bases/` were hand-written. They should be generated from the Go type markers using `controller-gen crd`. This ensures the OpenAPI schema, default values, enum validation, and required fields stay in sync with the Go types. Add `//+kubebuilder:validation:` markers to all spec fields.

**Files:** `api/v1alpha1/*.go`, `Makefile`, `config/crd/bases/`

---

## High

### 5. Resource Usage Measurement and Capacity Planning

Profile the operator under realistic workloads to establish baseline resource consumption and right-size requests/limits. Current defaults (100m/64Mi request, 500m/128Mi limit) are estimates with no load-testing data behind them. This work should include:
- Benchmark memory and CPU usage with varying numbers of ClusterReadiness CRs, GateCheck CRs, and check intervals
- Measure memory growth over time to identify leaks (especially from Prometheus metric cardinality and ScriptCheck Job log retrieval)
- Establish per-check overhead (goroutine count, API call volume, network I/O)
- Document recommended resource requests/limits for small (< 20 checks), medium (20-100), and large (100+) deployments
- Add resource usage guidance to the README and Helm chart values

**Files:** `config/manager/manager.yaml`, README, Helm chart values (when created)

### 6. Structured Logging

Replace `setupLog` and ad-hoc log calls with structured logging throughout all reconcilers and check executors. Use consistent keys (`check`, `namespace`, `duration`, `error`) so logs are grep-friendly and indexable.

**Files:** `internal/controller/*.go`, `internal/checks/**/*.go`

### 7. Graceful Shutdown and Signal Handling

The `/readyz` HTTP server runs in a plain goroutine with no shutdown path. If the manager receives SIGTERM, the HTTP server may drop in-flight requests. Use `mgr.Add(manager.RunnableFunc(...))` to register the HTTP server with the controller-runtime lifecycle so it participates in graceful shutdown.

**Files:** `cmd/manager/main.go`

### 8. Retry and Back-off for Transient Failures

Dynamic checks (HTTP, PromQL) currently fail immediately on transient errors (network blips, brief Prometheus unavailability). Add configurable retry with exponential back-off (1-2 retries, short delay) to reduce false negatives.

**Files:** `internal/checks/dynamic/http_check.go`, `internal/checks/dynamic/promql_check.go`

### 9. Rate Limiting and Concurrency Controls

When many ClusterReadiness CRs or GateCheck CRs exist, reconciliation can generate a burst of simultaneous HTTP and API calls. Add:
- `MaxConcurrentReconciles` option on the controller
- Semaphore or worker pool for concurrent dynamic check execution within a single reconciliation

**Files:** `internal/controller/clusterreadiness_controller.go`, `cmd/manager/main.go`

### 10. End-to-End Tests

Add E2E tests that deploy the operator to a real (or kind) cluster and verify:
- CRD creation and validation
- ClusterReadiness status updates
- Built-in check execution
- ScriptCheck Job lifecycle
- `/readyz` endpoint responses
- Prometheus metric emission

**Files:** new `test/e2e/` directory

### 11. CI/CD Pipeline

Set up GitHub Actions (or equivalent) with:
- `go vet`, `golangci-lint`, `go test ./...` on every PR
- Integration tests with envtest
- Docker image build verification
- Optional: E2E tests with kind

**Files:** `.github/workflows/ci.yaml`

### 12. Helm Chart

Package the operator as a Helm chart for standard Kubernetes distribution. Include configurable values for: image, replicas, resource requests/limits, RBAC scope, leader election, namespace, and check intervals.

**Files:** new `charts/clustergate/` directory

### 13. Status Observability Improvements

- Add `observedGeneration` to all CRD status types so clients can detect stale status
- Add a `lastTransitionTime` or `duration` field to per-check status entries
- Emit Kubernetes Events for check state transitions (passing -> failing, failing -> passing)

**Files:** `api/v1alpha1/*_types.go`, `internal/controller/clusterreadiness_controller.go`

---

## Medium

### 14. Finalizers for ClusterReadiness Cleanup

When a ClusterReadiness CR is deleted, the readiness state and metrics for that CR should be cleaned up. Add a finalizer to ensure `ReadinessState.Remove()` and metric label cleanup happen before the CR is removed from the API server.

**Files:** `internal/controller/clusterreadiness_controller.go`

### 15. Check Result Caching and Deduplication

If multiple ClusterReadiness CRs reference the same GateCheck, the check is executed independently for each CR. Add a shared result cache with TTL so the same check is not executed more frequently than its interval, regardless of how many CRs reference it.

**Files:** `internal/checks/dynamic/executor.go`, new cache package

### 16. Configuration Documentation

Document all built-in check configuration options (JSON schema for each check's `config` field) either in the README or in a dedicated `docs/` directory. Include examples for each check type.

**Files:** README or new `docs/checks.md`

### 17. Makefile Improvements

- Add `make lint` as a dependency of `make test`
- Add `make release` target for tagged image builds
- Add `make kind-load` for local development with kind clusters

**Files:** `Makefile`

### 18. Controller Restart Resilience

On startup, the operator has no historical check state — all checks show as unknown until the first reconciliation cycle completes. Consider persisting the last-known status in the CR status and using it as the initial state on restart, with a `stale` flag until checks re-run.

**Files:** `internal/controller/clusterreadiness_controller.go`

### 19. Multi-Cluster Support

The architecture supports multiple ClusterReadiness CRs per cluster, but there is no mechanism for a single operator instance to monitor remote clusters. Consider adding a `kubeconfig` or `context` field to ClusterReadiness for multi-cluster setups.

**Files:** `api/v1alpha1/clusterreadiness_types.go`, `internal/controller/`

### 20. Alert Integration

Add optional alert routing configuration (webhook URL, Slack, PagerDuty) so check failures can trigger notifications without requiring a separate alerting stack. This is a convenience feature — most production environments will use Prometheus alerting rules on the exported metrics instead.

**Files:** new `internal/notifier/` package, `api/v1alpha1/clusterreadiness_types.go`

### 21. Documentation Site

Publish a documentation site (e.g., via MkDocs or Docusaurus) with:
- Getting started guide
- Architecture deep-dive
- Check type reference
- API reference (generated from CRD OpenAPI schema)
- Troubleshooting guide

**Files:** new `docs/` directory
