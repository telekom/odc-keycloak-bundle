# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [0.2.1-final]

### Security

- **Hardcoded default admin credentials removed** — `keycloak-secret.yaml` with static `admin/admin` credentials deleted; the deploy script now creates the `keycloak-admin` secret dynamically, using `KEYCLOAK_ADMIN_PASSWORD` if set or a generated random password otherwise.
- **Operator RBAC scoped to namespace** — the operator's `ClusterRole` replaced by a namespace-scoped `Role`, reducing blast radius to the deployment namespace.
- **NetworkPolicy added** — new `keycloak-networkpolicy.yaml` restricts ingress to Keycloak pods: HTTP/HTTPS limited to same-namespace pods and the operator namespace, the management port limited to Prometheus pods, and Infinispan clustering limited to peer Keycloak pods.
- **All OCI image references SHA-pinned** — Keycloak, PostgreSQL, CloudNativePG, BusyBox, and the operator base images are now pinned by digest in manifests, `component-constructor.yaml`, KRO RGD, and the operator `Dockerfile`.
- **GitHub Actions SHA-pinned** — all third-party actions pinned by commit SHA to prevent supply-chain substitution.

### Changed

- **Image versions updated** — Keycloak 26.5.5 → 26.6.1, CloudNativePG operator 1.28.1 → 1.29.0, Prometheus Operator v0.80.1 → v0.90.1.
- **Go and Kubernetes dependencies updated** — Go 1.23 → 1.26, `k8s.io/*` 0.31 → 0.35, `controller-runtime` 0.19 → 0.23.
- **golangci-lint configuration upgraded** to v2 format with additional linters (`gosec`, `bodyclose`, `errname`, `misspell`, `unconvert`, `unparam`) and `gofmt` formatting enforcement.
- **CI workflows consolidated** — standalone `golangci-lint.yml` and `operator-tests.yml` removed; lint and unit tests integrated into the main `ci.yml` pipeline.
- **Renovate configured for digest pinning** — `pinDigests: true` enabled globally; regex managers extended with `autoReplaceStringTemplate` to maintain digest pins on automated updates.
- **Secret update logic corrected** — the operator now mutates the existing config secret in-place (preserving `ResourceVersion`) rather than constructing a replacement object, avoiding conflict errors under concurrent reconciliation.
- **Deployment documentation updated** — air-gapped transfer instructions added; admin credential handling revised to reflect dynamic secret creation.

---

## [0.2.0-final]

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
