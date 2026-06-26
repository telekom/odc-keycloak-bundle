# Hardening Reference

This document records the security controls applied to the keycloak-bundle deployment and
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
| 5.1.6 Ensure service account tokens are not auto-mounted unnecessarily | ⚠️ Partial | `keycloak-config-cli` disables token automounting. Keycloak and operator pods still use service account tokens. The Keycloak ServiceAccount/RBAC is prepared for KUBE_PING, but the shipped Deployment does not currently enable `KC_CACHE_STACK=kubernetes`. |

**Keycloak RBAC scope:** The `keycloak-pod-discovery` Role grants `get/list/watch` on
`pods` within the instance namespace only. This is the permission set needed when
`KC_CACHE_STACK=kubernetes` (Infinispan KUBE_PING) is enabled. The current manifests
prepare this RBAC but do not set `KC_CACHE_STACK=kubernetes` by default. No ClusterRole
is used for Keycloak pod discovery.

**Operator RBAC scope:** The operator is granted namespace-scoped permissions to manage
Keycloak CRDs and their dependent resources. No cluster-wide write access is granted.
Database backup and restore use CNPG-native resources directly (Backup, ScheduledBackup,
ObjectStore, recovery Cluster) without a custom Keycloak backup controller.

---

## Network Hardening

| Control | Status | Notes |
|---------|--------|-------|
| CIS K8s 5.3 — Network Policies | ✅ Enforced | `keycloak-networkpolicy.yaml` restricts **ingress** to the Keycloak HTTP/HTTPS ports (8080/8443) from same-namespace and operator-namespace pods, the management port (9000) from Prometheus pods, and JGroups clustering (7800) between Keycloak peers. **Egress** is restricted to DNS (kube-system:53), the PostgreSQL database (`cnpg.io/cluster=keycloak-db`:5432), JGroups clustering peers (7800), the **Kubernetes API Server (443/6443)** for Infinispan `KUBE_PING` pod discovery, and OTLP tracing (observability:4317). Requires a CNI that enforces NetworkPolicy. |

---

## Secret Management

### CIS K8s 5.4 — Secrets

| Control | Status | Implementation |
|---------|--------|----------------|
| 5.4.1 Prefer using Secrets as files over environment variables | ⚠️ Partial | Database credentials are injected via `secretKeyRef` environment variables. Keycloak's external secret API requires env-var injection. Admin credentials use the same pattern. |
| 5.4.2 Consider external secret management | ⚠️ Deviation | No external secret store (Vault, ESO) is integrated. Kubernetes Secrets provide the baseline. This is accepted for the current scope; external secret management is recommended for production. |
| No plaintext credentials in code | ✅ Applied | Gitleaks runs on every PR and blocks detected secrets. The KRO schema references the bootstrap admin Secret by name/key and does not accept `spec.adminPassword` as a plaintext CR field. |

---

## Image Supply Chain

| Control | Status | Implementation |
|---------|--------|----------------|
| Use specific image tags and digests | ✅ Applied | OCM image resources are digest-pinned for Keycloak `26.6.3`, BusyBox `1.37`, PostgreSQL `18.4`, CNPG `1.29.1`, Prometheus Operator `0.91.0`, and `keycloak-config-cli`; the operator image is recorded as `keycloak-operator-image` using the immutable `OPERATOR_IMAGE_REF` supplied by CI or release tooling. |
| CVE scanning | ✅ Applied | `.github/workflows/security.yml` runs Trivy and blocks HIGH/CRITICAL image findings for the same image versions recorded in the component descriptor. |
| Image provenance | ✅ Applied | OCM component archive is signed (`ocm-sign.sh`) and signature is validated before deployment (`ocm-validate.sh`/`ocm-verify.sh`) |
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
| TLS termination | ⚠️ Deviation | `KC_HTTP_ENABLED=true`, `KC_PROXY_HEADERS=xforwarded`, and `KC_HOSTNAME_STRICT=false` are set for CI and dev cluster compatibility. In production, use a trusted reverse proxy configuration, restrict exposed paths, do not expose the management port externally, and prefer HTTPS/TLS passthrough or a validated edge-termination pattern. |
| Admin credential rotation | ⚠️ Deviation | Standalone scripts create `keycloak-admin` dynamically when missing for development. KRO references an existing admin Secret and does not create it from CR values. For production, provision bootstrap credentials through an approved secret workflow and rotate/remove the bootstrap admin after setup. |
| Structured logging | ✅ Applied | `KC_LOG_FORMAT=json` enables structured log output compatible with log aggregation pipelines. |
| Metrics and health endpoints | ✅ Applied | `KC_HEALTH_ENABLED=true` and `KC_METRICS_ENABLED=true`; management port (9000) is not exposed externally via the Service. |
| Session clustering | ⚠️ Prepared, not enabled | ServiceAccount/RBAC and NetworkPolicy ports are prepared, but the shipped Keycloak Deployment and KRO RGD do not set `KC_CACHE_STACK=kubernetes`. Do not claim cross-pod session failover until this is enabled and tested. |

---

## Accepted Deviations Summary

| # | Control | Deviation | Justification |
|---|---------|-----------|---------------|
| 1 | CIS K8s 5.2.6 | `readOnlyRootFilesystem` not set on Keycloak | Upstream image requires writable fs for Quarkus augmentation cache |
| 2 | CIS K8s 5.1.6 | `automountServiceAccountToken` not disabled on non-operator pods | Keycloak requires service account token for KUBE_PING pod list |
| 3 | CIS K8s 5.3 | ~~No NetworkPolicy~~ **Resolved** | `keycloak-networkpolicy.yaml` now enforces ingress **and** egress (default-deny egress with explicit allows for DNS, database, clustering, and tracing). Requires a NetworkPolicy-capable CNI. |
| 4 | CIS K8s 5.4.2 | No external secret store | Dev/CI scope; Kubernetes Secrets are baseline |
| 5 | Keycloak TLS | HTTP enabled, strict hostname disabled | Dev/CI cluster compatibility; production requires TLS at ingress |
| 6 | Seccomp on Keycloak server | `seccompProfile` not set | Upstream image compatibility; `RuntimeDefault` recommended for production |
| 7 | CIS K8s 5.2.6 | `readOnlyRootFilesystem: false` on config-cli Job | keycloak-config-cli writes temporary import state to local disk during realm sync |
