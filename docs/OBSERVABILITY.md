# Observability

This document covers the three observability pillars active in the Keycloak OCM bundle:
structured logging, Prometheus metrics and alerting, and OpenTelemetry tracing.

---

## Structured Logging

Keycloak emits JSON-formatted log lines via `KC_LOG_FORMAT=json`. No additional
configuration is required. Log level is controlled by `KC_LOG_LEVEL` (default `INFO`).

Useful fields in each log event:

| Field | Description |
|-------|-------------|
| `level` | Log level (`INFO`, `WARN`, `ERROR`) |
| `loggerName` | Java logger name (e.g. `org.keycloak.events`) |
| `message` | Human-readable message |
| `realmName` | Realm context where available |
| `userId` / `clientId` | Identity context in event logs |

To increase verbosity for a specific subsystem at runtime, patch the deployment:

```bash
kubectl -n identity-<name> set env deployment/keycloak \
  KC_LOG_LEVEL="INFO,org.keycloak.events:DEBUG"
```

---

## Prometheus Metrics

### Namespace scoping

The monitoring resources (`ServiceMonitor`, `PodMonitor`, `PrometheusRule`) are
**namespace-scoped** and applied into each instance namespace individually. This is
intentional: it follows the namespace-per-instance isolation model so that each
instance's monitoring configuration is fully independent of others.

Prometheus must therefore be configured to discover monitoring resources across all
instance namespaces selected by your namespace selector. When using the kube-prometheus-stack Helm chart, set the
following values:

```yaml
prometheus:
  prometheusSpec:
    serviceMonitorNamespaceSelector: {}   # watch all namespaces
    podMonitorNamespaceSelector: {}
    ruleNamespaceSelector: {}
```

Or restrict to only Keycloak namespaces using a label selector:

```yaml
prometheus:
  prometheusSpec:
    serviceMonitorNamespaceSelector:
      matchExpressions:
        - key: kubernetes.io/metadata.name
          operator: In
          values: []   # populated by automation, or use matchLabels
    serviceMonitorSelectorConfig:
      matchLabels:
        app: keycloak
```

### Prerequisites

A [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator)
installation must be present in the cluster. The operator watches `ServiceMonitor` and
`PodMonitor` resources cluster-wide (or per-namespace depending on its configuration).
See [DEPLOYMENT.md](DEPLOYMENT.md) for installation instructions.

### Deploying the monitoring manifests

Apply the manifests from the `manifests/monitoring/` directory into each instance namespace:

```bash
NAMESPACE=identity-<name>

kubectl -n "$NAMESPACE" apply -f manifests/monitoring/keycloak-service-monitor.yaml
kubectl -n "$NAMESPACE" apply -f manifests/monitoring/cnpg-pod-monitor.yaml
kubectl -n "$NAMESPACE" apply -f manifests/monitoring/keycloak-prometheus-rules.yaml
```

### Keycloak metrics (ServiceMonitor)

`keycloak-service-monitor.yaml` configures scraping of the Keycloak management endpoint:

| Setting | Value |
|---------|-------|
| Port | `management` (9000) |
| Path | `/metrics` |
| Interval | 30 s |
| Scrape timeout | 10 s |

The `management` port is exposed on the `keycloak` Service. Confirm scraping is active
by checking the Prometheus targets page:

```bash
kubectl -n monitoring port-forward svc/prometheus-operated 9090 &
# Open http://localhost:9090/targets and look for job="keycloak"
```

Key Keycloak metric families:

| Metric | Description |
|--------|-------------|
| `keycloak_active_sessions` | Active sessions per realm |
| `keycloak_failed_login_attempts_total` | Failed login counter per realm |
| `keycloak_database_connection_pool_active` | Active DB connections |
| `keycloak_database_connection_pool_max` | Maximum DB connections configured |
| `keycloak_ready` | `1` when Keycloak reports ready, `0` otherwise |

### CloudNativePG metrics (PodMonitor)

`cnpg-pod-monitor.yaml` scrapes the CNPG exporter sidecar on all pods in the
`keycloak-db` cluster (label `cnpg.io/cluster: keycloak-db`, port `metrics`).

Key CNPG metric families:

| Metric | Description |
|--------|-------------|
| `cnpg_collector_up` | `1` when the CNPG exporter is reachable |
| `cnpg_pg_replication_lag` | Replication lag in seconds (HA clusters) |
| `cnpg_pg_stat_activity_count` | Active PostgreSQL connections |
| `pg_up` | `1` when PostgreSQL is accepting connections |

### Alert rules

`keycloak-prometheus-rules.yaml` defines the following alerts:

| Alert | Severity | Condition | For |
|-------|----------|-----------|-----|
| `KeycloakDown` | critical | `up{job="keycloak"} == 0` | 2 m |
| `KeycloakNotReady` | warning | `keycloak_ready == 0` | 5 m |
| `KeycloakHighLoginFailureRate` | warning | `rate(keycloak_failed_login_attempts_total[5m]) > 10` | 5 m |
| `KeycloakBruteForceDetected` | critical | `rate(keycloak_failed_login_attempts_total[1m]) > 30` | 1 m |
| `KeycloakHighActiveSessions` | warning | `keycloak_active_sessions > 10000` | 10 m |
| `KeycloakDBConnectionPoolExhausted` | critical | pool utilisation ≥ 90 % | 5 m |
| `KeycloakPodRestartingFrequently` | warning | > 3 restarts in 1 h | 5 m |
| `KeycloakDBClusterNotReady` | critical | `cnpg_collector_up == 0` | 2 m |
| `KeycloakDBReplicationLag` | warning | replication lag > 30 s | 5 m |

#### Tuning alert thresholds

Edit `manifests/monitoring/keycloak-prometheus-rules.yaml` and re-apply. Common
adjustments:

**Login failure rate** — the default threshold of 10 failures/s may be too sensitive
for environments with automated testing. Raise the threshold or extend the `for` window:

```yaml
expr: rate(keycloak_failed_login_attempts_total[5m]) > 50
for: 10m
```

**Active sessions** — `10000` is a conservative default. Adjust to match expected
peak load:

```yaml
expr: keycloak_active_sessions > 50000
```

**DB connection pool** — the 90 % threshold fires before exhaustion to give time for
intervention. Lower it to 80 % for more headroom:

```yaml
expr: keycloak_database_connection_pool_active / keycloak_database_connection_pool_max >= 0.8
```

---

## OpenTelemetry Tracing

Keycloak's Quarkus runtime includes native OTEL support. Tracing is **disabled by
default** (`KC_TRACING_ENABLED=false`).

### Enabling tracing

Patch the deployment to point at your OTEL Collector's OTLP/gRPC endpoint:

```bash
kubectl -n identity-<name> set env deployment/keycloak \
  KC_TRACING_ENABLED=true \
  KC_TRACING_ENDPOINT=http://opentelemetry-collector.observability.svc:4317
```

Or edit the environment variables directly in the manifest before applying:

```yaml
- name: KC_TRACING_ENABLED
  value: "true"
- name: KC_TRACING_ENDPOINT
  value: "http://opentelemetry-collector.observability.svc:4317"
```

### OTEL environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KC_TRACING_ENABLED` | `false` | Master switch for OTEL tracing |
| `KC_TRACING_ENDPOINT` | `http://opentelemetry-collector.observability.svc:4317` | OTLP/gRPC collector endpoint |
| `KC_TRACING_SAMPLER_TYPE` | `traceidratio` | Sampler: `always_on`, `always_off`, `traceidratio` |
| `KC_TRACING_SAMPLER_RATIO` | `0.1` | Fraction of traces to sample (0.0–1.0) |
| `KC_TRACING_SERVICE_NAME` | `keycloak` | Service name tag on all spans |

### Sampling guidance

| Environment | Recommended ratio | Rationale |
|-------------|------------------|-----------|
| Development | `1.0` | Capture all traces for debugging |
| Staging | `0.1` | 10 % is sufficient for performance analysis |
| Production | `0.01`–`0.05` | Reduce collector load while preserving signal |

### Compatible backends

Any OpenTelemetry-compatible backend works via the OTLP/gRPC protocol. Tested configurations:

- **Jaeger** — deploy with OTLP/gRPC receiver on port 4317; set `KC_TRACING_ENDPOINT`
  to the Jaeger collector service.
- **Grafana Tempo** — set `KC_TRACING_ENDPOINT` to the Tempo distributor gRPC service
  (typically `tempo.observability.svc:4317`).
- **OpenTelemetry Collector** — recommended for production; collector handles
  batching, retry, and fan-out to multiple backends.

### Verifying traces

After enabling tracing, generate a login event and check the backend:

```bash
# Port-forward Jaeger UI (if using Jaeger)
kubectl -n observability port-forward svc/jaeger-query 16686 &
# Open http://localhost:16686 and search for service "keycloak"
```

Expected spans for a login flow: `LoginProtocol`, `AuthenticationProcessor`,
`DBConnectionProvider`, `TokenManager`.