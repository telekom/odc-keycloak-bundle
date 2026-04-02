# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Added

- **Finalizer-based CR deletion** — the operator now sets a `keycloak.opendefense.cloud/cleanup` finalizer on every managed CR (Realm, Client, ClientScope, Group, User) so that deletions are intercepted and the corresponding resource is removed from Keycloak before the CR is released. Realm CRs are intentionally exempt: the finalizer is removed without touching the Keycloak realm, preserving data by design.
- **High Availability support** — Keycloak can now run as multiple replicas with distributed session sharing:
  - `manifests/keycloak/keycloak-sa.yaml` — dedicated `ServiceAccount` (`keycloak`) for Keycloak pods.
  - `manifests/keycloak/keycloak-rbac.yaml` — `Role` and `RoleBinding` granting the ServiceAccount `get`/`list`/`watch` on pods, required by Infinispan KUBE_PING for cluster discovery.
  - `manifests/keycloak/keycloak-pdb.yaml` — `PodDisruptionBudget` (`minAvailable: 1`) preventing scale-to-zero during node maintenance.
- **KRO `ResourceGraphDefinition`** added to the OCM component archive (`component-constructor.yaml`) as a `blueprint`-type resource, enabling air-gapped single-CR instantiation of the full Keycloak stack.
- **HA resources in KRO RGD** — `keycloak-instance-rgd.yaml` now provisions the ServiceAccount, Role, RoleBinding, and PodDisruptionBudget as part of the KRO dependency graph, with the Keycloak Deployment depending on them.
- **OpenTelemetry tracing** support in Keycloak Deployment via Keycloak's native Quarkus OTEL integration (`KC_TRACING_*` env vars). Disabled by default; enable by setting `KC_TRACING_ENABLED=true` and pointing `KC_TRACING_ENDPOINT` at your cluster's OTEL Collector gRPC address.
- **Structured JSON logging** for Keycloak (`KC_LOG_FORMAT=json`, `KC_LOG_LEVEL=INFO`) to enable log aggregation with tools such as Loki or ELK.
- **Management port (9000)** exposed on the Keycloak Service to allow Prometheus scraping of the `/metrics` endpoint.
- `manifests/monitoring/keycloak-service-monitor.yaml` — `ServiceMonitor` resource for Prometheus Operator to scrape Keycloak metrics every 30 s.
- `manifests/monitoring/cnpg-pod-monitor.yaml` — `PodMonitor` resource for Prometheus Operator to scrape CloudNativePG cluster metrics every 30 s.
- `manifests/monitoring/keycloak-prometheus-rules.yaml` — `PrometheusRule` alerting definitions covering:
  - Keycloak availability (pod down, not ready)
  - Authentication anomalies (high login failure rate, brute force detection)
  - Session count thresholds
  - Database connection pool exhaustion
  - Pod restart frequency
  - CloudNativePG cluster unavailability and replication lag
- `renovate.json` — Renovate Bot configuration for automated dependency tracking and PR creation on new upstream releases of Keycloak, CloudNativePG, and PostgreSQL images, with digest pinning enabled.
- `docs/UPGRADE.md` — Upgrade runbook covering:
  - Keycloak minor/patch rolling upgrades
  - Keycloak major version upgrades (with DB migration guidance)
  - PostgreSQL minor version upgrades via CloudNativePG rolling restarts
  - PostgreSQL major version upgrades via CloudNativePG `pg_upgrade` cluster clone procedure
  - Manual database backup procedure
  - CloudNativePG operator upgrades
  - Post-upgrade observability verification checklist

### Changed

- **Keycloak Deployment** — added `RollingUpdate` strategy (`maxUnavailable: 0`, `maxSurge: 1`), `serviceAccountName: keycloak`, and `KC_CACHE_STACK=kubernetes` to activate Infinispan KUBE_PING cluster mode for distributed session replication across replicas.
- **KRO RGD Keycloak Deployment resource** — aligned with standalone manifests: rolling update strategy, `serviceAccountName`, `KC_CACHE_STACK=kubernetes`, and health probes corrected to management port 9000 (was 8080).
- **`docs/ARCHITECTURE.md`** — added High Availability & Scalability section documenting multi-replica setup, PodDisruptionBudget, rolling updates, Infinispan KUBE_PING, and ServiceAccount/RBAC requirements.
- **`docs/USAGE.md`** — extended deletion behaviour documentation to cover all CR types; added CR Status and Conditions reference section.
- **`docs/DEPLOYMENT.md`** — added KRO-based deployment as the primary installation path; updated OCM resource table; fixed broken doc link.
- **`README.md`** — updated feature list and project structure to reflect all five CRDs and HA capabilities.

---

## [0.1.0-poc]

Initial proof-of-concept release.

### Added

- OCM component definition (`component-constructor.yaml`) bundling Keycloak 26.5.3, PostgreSQL 18.1, and CloudNativePG 1.28.1.
- Kubernetes manifests for Keycloak Deployment, Service, and admin Secret.
- CloudNativePG `Cluster` manifest for PostgreSQL with single-instance setup.
- Custom Keycloak Client Operator (Helm chart) with CRD-based declarative client configuration.
- KRO `ResourceGraphDefinition` for multi-instance provisioning.
- GitHub Actions CI/CD pipeline with OCM packaging, signing, and registry transfer.
- Deployment, smoke test, and utility scripts.
- Architecture, database, client, CI/CD, deployment, and usage-concept documentation.
