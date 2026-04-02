# Hardening Reference

This document records the security controls applied to the keycloak-ocm deployment and
the rationale for any accepted deviations. Controls are referenced against the
**CIS Kubernetes Benchmark v1.9** and the **CIS Keycloak Benchmark** where applicable.

---

## Container Runtime Hardening

### CIS K8s 5.2 — Pod Security Standards

| Control | Status | Implementation |
|---------|--------|----------------|
| 5.2.1 Ensure privileged containers are not used | ✅ Applied | No container sets `privileged: true` |
| 5.2.2 Minimize root containers (`runAsNonRoot`) | ✅ Applied | All containers set `runAsNonRoot: true` or a non-zero `runAsUser` (Keycloak: 1000; operator: 1000) |
| 5.2.3 Minimize containers with `allowPrivilegeEscalation` | ✅ Applied | All containers set `allowPrivilegeEscalation: false` |
| 5.2.4 Minimize containers with `NET_RAW` capability | ✅ Applied | `capabilities.drop: [ALL]` removes all capabilities including `NET_RAW` |
| 5.2.7 Minimize containers with added capabilities | ✅ Applied | No container adds capabilities |
| 5.2.8 Minimize containers with host path volumes | ✅ Applied | No `hostPath` volumes are used |
| 5.2.9 Minimize containers with host network | ✅ Applied | No container uses `hostNetwork: true` |
| 5.2.6 Minimize containers with root file system write access | ⚠️ Deviation | Keycloak requires a writable filesystem for Quarkus augmentation cache. `readOnlyRootFilesystem: true` is not set. Accepted deviation — see note below. |

**Deviation note (5.2.6):** The upstream `quay.io/keycloak/keycloak` image writes to the
filesystem during the Quarkus build step on startup. Enabling `readOnlyRootFilesystem`
would require a pre-built optimised image (i.e. a custom `Dockerfile` that calls
`kc.sh build`). This is accepted for the current delivery scope; a pre-built image
would eliminate this deviation in a production hardening pass.

### Seccomp Profiles

| Component | Profile | Notes |
|-----------|---------|-------|
| Keycloak operator | `RuntimeDefault` | Set via `seccompProfile.type: RuntimeDefault` in container securityContext |
| Keycloak server | Not set | Upstream image compatibility; `RuntimeDefault` is recommended for production |
| CNPG PostgreSQL | Managed by CNPG operator | CNPG 1.28+ sets `RuntimeDefault` by default on supported clusters |

---

## RBAC Hardening

### CIS K8s 5.1 — RBAC and Service Accounts

| Control | Status | Implementation |
|---------|--------|----------------|
| 5.1.1 Ensure cluster-admin role is not used unnecessarily | ✅ Applied | No workload binds `cluster-admin` |
| 5.1.3 Minimize wildcard use in Roles and ClusterRoles | ✅ Applied | All verbs and resources are explicitly listed |
| 5.1.5 Ensure default service accounts are not bound to active roles | ✅ Applied | All workloads use dedicated ServiceAccounts |
| 5.1.6 Ensure service account tokens are not auto-mounted unnecessarily | ⚠️ Deviation | `automountServiceAccountToken` is not explicitly disabled on non-operator pods. The Keycloak pod requires pod-list access for KUBE_PING (see below). |

**Keycloak RBAC scope:** The `keycloak-pod-discovery` Role grants `get/list/watch` on
`pods` within the instance namespace only. This is the minimum required by
`KC_CACHE_STACK=kubernetes` (Infinispan KUBE_PING). No ClusterRole is used.

**Operator RBAC scope:** The operator is granted namespace-scoped permissions to manage
Keycloak CRDs and their dependent resources. No cluster-wide write access is granted.
Database backup and restore use CNPG-native resources directly (Backup, ScheduledBackup,
ObjectStore, recovery Cluster) without a custom Keycloak backup controller.

---

## Network Hardening

| Control | Status | Notes |
|---------|--------|-------|
| CIS K8s 5.3 — Network Policies | ⚠️ Deviation | No `NetworkPolicy` objects are deployed. In a production environment, ingress should be restricted to the Keycloak port (8080/8443) and Prometheus scrape port (9000). Egress should be limited to the database service. This deviation is accepted for the current delivery scope targeting dev/CI clusters without a CNI enforcing NetworkPolicy. |

---

## Secret Management

### CIS K8s 5.4 — Secrets

| Control | Status | Implementation |
|---------|--------|----------------|
| 5.4.1 Prefer using Secrets as files over environment variables | ⚠️ Partial | Database credentials are injected via `secretKeyRef` environment variables. Keycloak's external secret API requires env-var injection. Admin credentials use the same pattern. |
| 5.4.2 Consider external secret management | ⚠️ Deviation | No external secret store (Vault, ESO) is integrated. Kubernetes Secrets provide the baseline. This is accepted for the current scope; external secret management is recommended for production. |
| No plaintext credentials in code | ✅ Applied | Gitleaks runs on every PR and blocks merges on detected secrets. |

---

## Image Supply Chain

| Control | Status | Implementation |
|---------|--------|----------------|
| Use specific image tags (not `latest`) | ✅ Applied | All images are pinned to a digest-equivalent SHA or semantic version tag (Keycloak: `26.5.5`, CNPG: `1.28.1`, busybox: `1.37`) |
| CVE scanning | ✅ Applied | Trivy scans all images for HIGH/CRITICAL CVEs on every PR and weekly. Merges are blocked on unfixed critical CVEs. |
| Image provenance | ✅ Applied | OCM component archive is signed (`ocm-sign.sh`) and signature is validated before deployment (`ocm-validate.sh`) |
| Operator image non-root | ✅ Applied | Operator `Dockerfile` uses a non-root `distroless` base; runtime `securityContext` enforces non-root execution |

---

## Resource Limits

All containers define both `requests` and `limits` to prevent runaway resource consumption
and to enable the Kubernetes scheduler to make informed placement decisions.

| Container | CPU request | CPU limit | Memory request | Memory limit |
|-----------|-------------|-----------|----------------|--------------|
| Keycloak | 50m | 1000m | 512Mi | 1Gi |
| wait-for-db (init) | 10m | 100m | 64Mi | 128Mi |
| Keycloak operator | 10m | 500m | 64Mi | 128Mi |

---

## Keycloak Application Hardening

The following controls align with the CIS Keycloak Benchmark and general hardening
guidance for Keycloak deployments.

| Control | Status | Notes |
|---------|--------|-------|
| TLS termination | ⚠️ Deviation | `KC_HTTP_ENABLED=true` and `KC_HOSTNAME_STRICT=false` are set for CI and dev cluster compatibility. In production, TLS should terminate at the ingress with HTTP disabled inside the cluster, or `KC_HTTPS_*` configured with a certificate. |
| Admin credential rotation | ⚠️ Deviation | The bootstrap admin secret is static in `keycloak-secret.yaml`. Production deployments should rotate via external secret management or use a one-time bootstrap flow. |
| Structured logging | ✅ Applied | `KC_LOG_FORMAT=json` enables structured log output compatible with log aggregation pipelines. |
| Metrics and health endpoints | ✅ Applied | `KC_HEALTH_ENABLED=true` and `KC_METRICS_ENABLED=true`; management port (9000) is not exposed externally via the Service. |
| Session clustering | ✅ Applied | `KC_CACHE_STACK=kubernetes` enables Infinispan KUBE_PING for HA session sharing across replicas. |

---

## Accepted Deviations Summary

| # | Control | Deviation | Justification |
|---|---------|-----------|---------------|
| 1 | CIS K8s 5.2.6 | `readOnlyRootFilesystem` not set on Keycloak | Upstream image requires writable fs for Quarkus augmentation cache |
| 2 | CIS K8s 5.1.6 | `automountServiceAccountToken` not disabled on non-operator pods | Keycloak requires service account token for KUBE_PING pod list |
| 3 | CIS K8s 5.3 | No NetworkPolicy | Dev/CI cluster scope; CNI enforcement not available |
| 4 | CIS K8s 5.4.2 | No external secret store | Dev/CI scope; Kubernetes Secrets are baseline |
| 5 | Keycloak TLS | HTTP enabled, strict hostname disabled | Dev/CI cluster compatibility; production requires TLS at ingress |
| 6 | Seccomp on Keycloak server | `seccompProfile` not set | Upstream image compatibility; `RuntimeDefault` recommended for production |
