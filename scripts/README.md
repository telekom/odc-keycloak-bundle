# Keycloak Deployment Scripts

This directory contains shell scripts for building, deploying, and verifying the Keycloak OCM component.
Scripts are designed to be portable and are used across manual CLI workflows and GitHub Actions.
The full observability stack (Prometheus Operator, Jaeger) is installed automatically by `deploy-all.sh`
and verified end-to-end by `tests/test-observability.sh`.

## Directory Structure

| Directory | Purpose |
| :--- | :--- |
| `ocm/` | **Packaging**: Create, sign, and transfer OCM components. |
| `deploy/` | **Deployment**: Deploy Keycloak to Kubernetes. |
| `utils/` | **Utilities**: Status checks, logs, port-forwarding, and shared libraries. |
| `tests/` | **Verification**: Scripts for testing CRDs and deployments. |

## Prerequisites

Before running these scripts, ensure you have:

*   **Tools**:
    *   `kubectl` (latest version recommended)
    *   `ocm` CLI (for packaging/signing)
    *   `curl` (for health/metrics checks in test scripts)
    *   `jq` (for JSON parsing in test scripts)
  *   `syft` or `trivy` (for CycloneDX SBOM generation)
*   **Access**:
    *   A Kubernetes cluster (Kubeconfig configured)
    *   **Cluster-Admin** permissions (required for installing CNPG Operator & CRDs)
    *   Registry credentials (if pushing/pulling OCM components)

## Quick Start

```bash
# 1. Deploy a complete Keycloak instance
./scripts/deploy/deploy-all.sh

# 2. Check status
./scripts/utils/status.sh dev-<random-id>

# 3. Port-forward (localhost:8080)
./scripts/utils/portforward.sh dev-<random-id>
```

## Script Compatibility Matrix

| Script | CLI (Manual) | GitHub Actions | Description |
| :--- | :---: | :---: | :--- |
| **OCM Packaging** | | | |
| `ocm/ocm-create.sh` | ✅ | ✅ | Creates the component archive (CTF tarball). |
| `ocm/ocm-sign.sh` | ✅ | ✅ | Signs the archive (CI expects secret-provided keys). |
| `ocm/ocm-validate.sh` | ✅ | ✅ | Validates structure and optionally verifies signatures. |
| `ocm/ocm-verify.sh` | ✅ | ✅ | Verifies CTF signature with a public key (air-gapped gate). |
| `ocm/ocm-transfer.sh` | ✅ | ✅ | Transfers archive to OCI registry. |
| **Deployment** | | | |
| `deploy/deploy-all.sh` | ✅ | ✅ | **Main Entry Point**: Deploys full stack (CNPG + Prometheus Operator + Jaeger + Keycloak + Operator + monitoring manifests). Accepts `--namespace NS` (administrator-controlled target namespace), `--clean` (delete and recreate namespace), and `--no-monitoring` (skip observability components). |
| `deploy/cleanup.sh` | ✅ | ✅ | Removes a specific instance/namespace. |
| `deploy/install-cnpg.sh` | ✅ | ✅ | Installs CloudNativePG operator (idempotent; checks first). |
| `deploy/install-prometheus-operator.sh` | ✅ | ✅ | Installs Prometheus Operator v0.80.1 via upstream bundle (idempotent; called by `deploy-all.sh`). |
| `deploy/install-jaeger.sh` | ✅ | ✅ | Deploys Jaeger all-in-one (in-memory) into the `observability` namespace for OTEL tracing (idempotent; called by `deploy-all.sh`). |
| `utils/status.sh` | ✅ | ✅ | **Smoke Test**: Checks Pods, Services, and HTTP Health. |
| `tests/test-crd-smoke.sh` | ✅ | ✅ | **Verification (Fast)**: Realm foundation, CRD create/read checks, coexistence checks. |
| `tests/test-crd-lifecycle.sh` | ✅ | ✅ | **Verification (Lifecycle)**: Deletion propagation checks for IdentityProvider/AuthFlow. |
| `tests/test-crd-suite.sh` | ✅ | ✅ | **Verification (Full)**: Runs smoke and lifecycle suites in sequence. |
| `tests/test-crd.sh` | ✅ | ✅ | Backward-compatible wrapper for `tests/test-crd-suite.sh full`. |
| `tests/test-observability.sh` | ✅ | ✅ | Verifies ServiceMonitor, PodMonitor, OTEL tracing (Jaeger spans), and PrometheusRule alert definitions. |
| `tests/test-backup-restore.sh` | ✅ | ✅ | CNPG-native backup + restore checks. CI deploy stage executes the live smoke flow when backup secrets are configured. |
| `tests/setup-backup-provider.sh` | ✅ | ✅ | CI helper for backup provider setup (`external-s3` or `incluster-minio`), credential secret provisioning, and secret-read RBAC for the CNPG service account. |

For `incluster-minio`, the helper reuses existing MinIO root credentials by default and only restarts MinIO if credentials actually change.
`incluster-minio` is CI-focused by default; non-CI usage requires an explicit override flag in the helper script.
| **Helpers** | | | |
| `utils/logs.sh` | ✅ | ❌ | Streams logs (interactive/debug only). |
| `utils/portforward.sh` | ✅ | ❌ | Opens localhost tunnel (interactive only). |
| `utils/common.sh` | 🔒 | 🔒 | Library sourced by other scripts. |

## CI/CD Reference Implementation

### GitHub Actions (Primary)
The `.github/workflows/ci.yml` is the primary pipeline for this project. It runs on every commit/PR and performs:
1. Linting (YAML, ShellCheck, Gitleaks)
2. Build & Sign (OCM)
3. Transfer (to OCI Registry)
4. Deploy & Verify (Smoke Tests + CRD Tests)
5. Backup & Restore Verify (`setup-backup-provider.sh` + `test-backup-restore.sh --live`, supports `external-s3` and `incluster-minio`)
6. Observability Verify (`test-observability.sh` — ServiceMonitor, PodMonitor, OTEL tracing, alert rules)

## Usage Examples

### 1. Build & Transfer (OCM)
```bash
# Create and Sign
./scripts/ocm/ocm-create.sh
./scripts/ocm/ocm-sign.sh ocm-output/component-archive ./ocm-key.priv ./ocm-key.pub

# Mandatory before deployment in restricted/air-gapped environments
./scripts/ocm/ocm-verify.sh ocm-output/keycloak-bundle-ctf.tar.gz ./security/ocm-signing-public-key.pub

# Transfer to Registry
./scripts/ocm/ocm-transfer.sh --user <user> --password <pass>

# Optional: immutable mode (fails if version already exists)
OCM_TRANSFER_IMMUTABLE=true ./scripts/ocm/ocm-transfer.sh --user <user> --password <pass>

# Optional: explicit overwrite mode (default)
./scripts/ocm/ocm-transfer.sh --overwrite --user <user> --password <pass>
```

### 2. Deploy to Cluster
```bash
# Deploy a new instance named 'dev-1'
./scripts/deploy/deploy-all.sh dev-1
```

### 3. Verify
```bash
# Check Status
./scripts/utils/status.sh dev-1

# Run fast CRD smoke checks
./scripts/tests/test-crd-smoke.sh keycloak-dev-1

# Run lifecycle deletion checks
./scripts/tests/test-crd-lifecycle.sh keycloak-dev-1

# Run full CRD verification suite
./scripts/tests/test-crd-suite.sh keycloak-dev-1 full

# Optional: write JUnit report files for CI systems
CRD_TEST_REPORT_FILE=reports/crd-smoke.xml ./scripts/tests/test-crd-smoke.sh keycloak-dev-1

# Run Observability Tests
./scripts/tests/test-observability.sh keycloak-dev-1

# Run CNPG-native backup/restore checks (static)
./scripts/tests/test-backup-restore.sh

# Optional live backup + restore smoke
./scripts/tests/test-backup-restore.sh --live \
  --namespace keycloak-dev-1 \
  --cluster-name keycloak-db \
  --restore-cluster-name keycloak-db-restore \
  --destination-path s3://my-bucket/keycloak \
  --credentials-secret keycloak-backup-s3 \
  --endpoint-url https://s3.example.com

# Note: when overriding --restore-cluster-name, keep it <= 47 characters.
# CNPG appends '-1-full-recovery' for an internal bootstrap job and Kubernetes
# object names are limited to 63 characters.
```

### 3a. Deploy without monitoring stack (e.g. quick local test)
```bash
./scripts/deploy/deploy-all.sh dev-1 --no-monitoring
```

### 3b. Install observability components manually
```bash
# Install Prometheus Operator (cluster-wide, once per cluster)
./scripts/deploy/install-prometheus-operator.sh

# Install Jaeger all-in-one (in-memory, once per cluster)
./scripts/deploy/install-jaeger.sh

# Apply monitoring manifests into an existing instance namespace
kubectl apply -n keycloak-dev-1 -f manifests/monitoring/
```

### 3a. Test Fixtures vs Examples

- `scripts/tests/fixtures/`: deterministic CI fixtures for automated integration tests.
- `examples/`: user-facing reference manifests for manual usage and documentation.
- Keep both sets in sync on schema changes, but do not couple CI stability to doc-example changes.

### 4. Cleanup
```bash
./scripts/deploy/cleanup.sh dev-1
```

## Architecture

```text
Run: ./scripts/deploy/deploy-all.sh [instance] [--namespace NS] [--clean] [--no-monitoring]
  ├── [1] Checks/Installs CloudNativePG Operator        (install-cnpg.sh)
  ├── [2] Checks/Installs Prometheus Operator           (install-prometheus-operator.sh)  [skipped with --no-monitoring]
  ├── [3] Checks/Installs Jaeger all-in-one             (install-jaeger.sh)               [skipped with --no-monitoring]
  ├── [4] Deploys PostgreSQL Cluster                    (deploy-postgres.sh)
  ├── [5] Deploys Keycloak                              (deploy-keycloak.sh)
  ├── [6] Deploys Operator                              (deploy-operator.sh)
  └── [7] Applies monitoring manifests into namespace   (manifests/monitoring/)            [skipped with --no-monitoring]
        ├── keycloak-service-monitor.yaml
        ├── cnpg-pod-monitor.yaml
        └── keycloak-prometheus-rules.yaml

Run: ./scripts/tests/test-observability.sh <namespace>
  ├── [1] ServiceMonitor presence + Keycloak /metrics endpoint (port 9000)
  ├── [2] PodMonitor presence + CNPG /metrics endpoint (port 9187)
  ├── [3] OTEL tracing: enable KC_TRACING_ENABLED, login via kcadm.sh, assert spans in Jaeger
  └── [4] PrometheusRule: assert all 9 expected alert names are defined
```
