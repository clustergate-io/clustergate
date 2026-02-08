# ClusterGate

A Kubernetes operator for continuous cluster readiness validation. ClusterGate runs a configurable set of health checks against your cluster and exposes the aggregate result via CRD status, Prometheus metrics, and an HTTP `/readyz` endpoint — giving you a single source of truth for whether a cluster is ready to serve workloads.

## Key Features

- **Built-in control plane checks** — API server, etcd, kube-scheduler, kube-controller-manager, CoreDNS
- **Dynamic check types** — Define checks as Kubernetes CRs without recompiling:
  - **PodCheck** — verify pods matching a label selector are running and ready
  - **HTTPCheck** — probe an HTTP endpoint and validate the response code
  - **ResourceCheck** — assert conditions on any Kubernetes resource
  - **PromQLCheck** — query Prometheus and evaluate the result
  - **ScriptCheck** — run an arbitrary script as a Kubernetes Job
- **Profiles** — bundle checks into reusable `GateProfile` CRs
- **Severity model** — `critical` (blocks readiness), `warning` (reported only), `info` (diagnostic)
- **Prometheus metrics** — per-check readiness gauges, execution duration histograms, cluster/category rollups
- **HTTP readiness endpoint** — `/readyz` with category and severity filtering, designed for load balancer health checks and CI gates
- **High availability** — leader election, PodDisruptionBudget, topology spread across nodes

## Architecture

```
ClusterReadiness CR
       │
       ├── references GateProfile CRs (reusable check bundles)
       └── inline CheckSpecs (built-in or GateCheck references)
                │
                ├── Built-in checks (dns, kube-apiserver, etcd, ...)
                └── GateCheck CRs (podCheck, httpCheck, resourceCheck, promqlCheck, scriptCheck)
```

The `ClusterReadinessReconciler` periodically executes all resolved checks, updates the CR status, publishes Prometheus metrics, and refreshes the `/readyz` HTTP endpoint. Checks run concurrently and respect per-check intervals.

## Custom Resource Definitions

### ClusterReadiness

The primary orchestration resource. Combines built-in and dynamic checks, references profiles, and reports aggregate readiness.

```yaml
apiVersion: clustergate.io/v1alpha1
kind: ClusterReadiness
metadata:
  name: production-readiness
spec:
  interval: 60s
  profiles:
    - name: production-baseline
  checks:
    # Built-in check
    - name: kube-apiserver
      severity: critical
    # Dynamic check reference
    - gateCheckRef: istiod-ready
      severity: critical
```

**Status fields:** `ready`, `summary` (total/passing/failing counts), `categorySummaries`, per-check `checks[]`, `conditions` (Ready, Degraded).

Short names: `cr`

### GateCheck

Defines a single dynamic check. Exactly one check type must be specified.

```yaml
apiVersion: clustergate.io/v1alpha1
kind: GateCheck
metadata:
  name: istiod-ready
spec:
  description: "Verifies istiod control plane pods are running"
  severity: critical
  category: networking
  podCheck:
    namespace: istio-system
    labelSelector:
      matchLabels:
        app: istiod
    minReady: 2
```

Short name: `gchk`

### GateProfile

A reusable bundle of check references with optional overrides.

```yaml
apiVersion: clustergate.io/v1alpha1
kind: GateProfile
metadata:
  name: production-baseline
spec:
  description: "Standard production readiness checks"
  checks:
    - name: kube-apiserver
      severity: critical
    - name: dns
      severity: critical
    - gateCheckRef: istiod-ready
      severity: critical
```

Short name: `gp`

## Check Types

### Built-in Checks

| Check | Category | Description |
|---|---|---|
| `dns` | networking | CoreDNS pods running + DNS resolution working |
| `kube-apiserver` | control-plane | API server `/healthz` endpoint |
| `etcd` | control-plane | etcd health via API server proxy |
| `kube-scheduler` | control-plane | Scheduler leader election lease freshness |
| `kube-controller-manager` | control-plane | Controller manager leader election lease freshness |
| `cloud-controller-manager` | control-plane | Cloud controller manager lease (opt-in via `--enable-cloud-controller-manager`) |

Built-in checks accept optional JSON configuration via the `config` field. For example, overriding the DNS test domain:

```yaml
checks:
  - name: dns
    config:
      testDomain: "my-service.default.svc.cluster.local"
```

### Dynamic Check Types

#### PodCheck

Verify that pods matching a label selector are running and ready.

```yaml
podCheck:
  namespace: istio-system
  labelSelector:
    matchLabels:
      app: istiod
  minReady: 2     # default: 1
```

#### HTTPCheck

Perform an HTTP request and validate the response status code.

```yaml
httpCheck:
  url: "https://vault.vault.svc:8200/v1/sys/health"
  method: GET                    # default: GET
  expectedStatusCodes: [200]     # default: [200]
  timeoutSeconds: 5              # default: 10
  insecureSkipTLSVerify: true    # default: false
  headers:
    Authorization: "Bearer ..."
```

#### ResourceCheck

Assert conditions on any Kubernetes resource, by name or label selector.

```yaml
resourceCheck:
  apiVersion: apps/v1
  kind: Deployment
  namespace: cert-manager
  name: cert-manager              # or use labelSelector
  conditions:
    - type: Available
      status: "True"
```

#### PromQLCheck

Query a Prometheus endpoint and evaluate the result.

```yaml
promqlCheck:
  endpoint: "http://prometheus.monitoring.svc:9090"
  query: 'up{job="etcd"} == 1'
  condition:
    type: resultCount             # or "value"
    operator: gte                 # gte, lte, eq, gt, lt
    threshold: 3
  timeoutSeconds: 10              # default: 10
```

#### ScriptCheck

Run a custom script as a Kubernetes Job. Exit code 0 = ready, non-zero = not ready.

```yaml
scriptCheck:
  image: busybox:latest
  command: ["sh", "-c"]
  args: ["nslookup google.com > /dev/null 2>&1 && echo 'DNS OK' || exit 1"]
  timeoutSeconds: 30              # default: 30
  serviceAccountName: my-sa       # optional
  env:                            # optional
    - name: TARGET_HOST
      value: "10.0.0.5"
  volumes:                        # optional
    - name: nfs-vol
      nfs:
        server: "10.0.0.5"
        path: "/exports/data"
  volumeMounts:                   # optional
    - name: nfs-vol
      mountPath: /mnt
```

## Observability

### Prometheus Metrics

The operator exposes metrics on port 8080 (scraped via `prometheus.io/scrape: "true"` pod annotation).

| Metric | Type | Labels | Description |
|---|---|---|---|
| `clustergate_check_ready` | Gauge | check, cluster_readiness, severity, category | 1 = passing, 0 = failing |
| `clustergate_check_duration_seconds` | Histogram | check, severity, category | Check execution time |
| `clustergate_cluster_ready` | Gauge | cluster_readiness | 1 = all critical checks passing |
| `clustergate_category_ready` | Gauge | category, cluster_readiness | 1 = all critical checks in category passing |

### HTTP Readiness Endpoint

The `/readyz` endpoint on port 8082 returns the cluster readiness status as JSON.

**Response:** `200 OK` when all critical checks pass, `503 Service Unavailable` otherwise.

```bash
# Full readiness status
curl http://localhost:8082/readyz

# Filter by category
curl http://localhost:8082/readyz?category=control-plane

# Filter by severity
curl http://localhost:8082/readyz?severity=critical
```

## Getting Started

### Prerequisites

- Go 1.25+
- A Kubernetes cluster (v1.33+)
- kubectl configured to access the cluster

### Build

```bash
# Build the binary
make build

# Build the Docker image
make docker-build IMG=my-registry/clustergate:v0.1.0

# Push the image
make docker-push IMG=my-registry/clustergate:v0.1.0
```

### Deploy

```bash
# Install CRDs
make install

# Deploy the operator (CRDs + RBAC + controller)
make deploy

# Apply sample checks
make sample
```

### Run Locally (development)

```bash
# Run against the current kubeconfig cluster
make run
```

### Test

```bash
# All tests (unit + integration with envtest)
make test

# Unit tests only
make test-unit

# Integration tests only
make test-integration
```

## CLI Mode

The `clustergate` CLI runs built-in health checks from outside the cluster without requiring the operator to be deployed or CRDs to be installed. This is useful for:

- **Day 0 validation** — verify a new cluster is healthy before deploying workloads
- **Broken cluster diagnosis** — check control plane health when the operator can't be deployed
- **CI gates** — use exit codes to fail pipelines when clusters are unhealthy

### Build

```bash
make build-cli
```

### Usage

```bash
# Run all built-in checks against the current kubeconfig context
./bin/clustergate check

# Specify a kubeconfig file
./bin/clustergate check --kubeconfig /path/to/kubeconfig

# Run specific checks only
./bin/clustergate check --checks dns,kube-apiserver,etcd

# JSON output for scripting
./bin/clustergate check --output json

# Include cloud-controller-manager check
./bin/clustergate check --enable-cloud-controller-manager

# Set a custom timeout
./bin/clustergate check --timeout 60s
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All checks passed |
| 1 | One or more checks failed or encountered errors |

### Example Output

```
CLUSTERGATE CHECK RESULTS
=========================

[PASS] dns (networking/critical)
       DNS operational: 2 pods running, kubernetes.default.svc.cluster.local resolves to [10.96.0.1]

[PASS] kube-apiserver (control-plane/critical)
       kube-apiserver: healthy (status 200)

[PASS] etcd (control-plane/critical)
       etcd: healthy (status 200)

[PASS] kube-scheduler (control-plane/critical)
       kube-scheduler: healthy (lease renewed 3s ago)

[PASS] kube-controller-manager (control-plane/critical)
       kube-controller-manager: healthy (lease renewed 2s ago)

-----------------------
Results: 5/5 passed
Status: PASS
```

## Configuration

### Controller Flags

| Flag | Default | Description |
|---|---|---|
| `--metrics-bind-address` | `:8080` | Metrics endpoint bind address |
| `--health-probe-bind-address` | `:8081` | Health/readiness probe bind address |
| `--readyz-bind-address` | `:8082` | Cluster readiness HTTP endpoint |
| `--leader-elect` | `false` | Enable leader election for HA deployments |
| `--enable-cloud-controller-manager` | `false` | Enable cloud-controller-manager health check |
| `--namespace` | `clustergate-system` | Namespace for ScriptCheck Job creation |

### High Availability

The default deployment runs 2 replicas with:

- **Leader election** via `--leader-elect` — only one replica actively reconciles at a time; the standby takes over within seconds if the leader fails
- **PodDisruptionBudget** — `minAvailable: 1` ensures at least one pod is always available during voluntary disruptions (node drains, upgrades)
- **Topology spread constraints** — pods are spread across nodes to survive single-node failures
- **Pod anti-affinity** — preferred scheduling on different nodes

## Project Structure

```
api/v1alpha1/           CRD type definitions (GateCheck, ClusterReadiness, GateProfile)
cmd/
  manager/              Controller manager entry point
  clustergate/          CLI entry point (run checks without deployment)
config/
  crd/bases/            Generated CRD manifests
  manager/              Deployment, PDB, namespace manifests
  rbac/                 ClusterRole, RoleBindings, leader election RBAC
  samples/              Example CRs
internal/
  checks/               Check interface, registry, and implementations
    builtin/            Shared built-in check registration
    controlplane/       Built-in control plane checks
    dns/                Built-in DNS check
    dynamic/            Dynamic check executor (pod, http, resource, promql, script)
  cli/                  CLI runner and output formatters
  controller/           Reconcilers (ClusterReadiness, GateCheck, GateProfile)
  metrics/              Prometheus metric definitions
  server/               HTTP readiness endpoint
test/integration/       Integration tests with envtest
```

## License

See [LICENSE](LICENSE) for details.
