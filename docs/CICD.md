# CI/CD Pipeline

This document describes the CI/CD pipeline for the Keycloak OCM component, implemented as a GitHub Actions workflow.

## Pipeline Overview

The pipeline is defined in `.github/workflows/ci.yml` and follows a staged flow. On `pull_request`, privileged stages are intentionally blocked by policy (see PR Policy section).

```text
Quality Checks  -->  Build Operator Image  -->  Build OCM Package  -->  Transfer  -->  Deploy & Verify
                        Docker build/push        Create / Sign / Validate  Registry       K8s Deploy
                                                                                           Smoke Test
```

Each stage builds on the output of the previous one. The pipeline produces a signed,
validated OCM component archive, builds the Keycloak Operator image, transfers the OCM
archive to an OCI registry, and deploys both artifacts to a Kubernetes cluster for
verification. The operator image is also recorded as the `keycloak-operator-image` OCM
resource; the build step injects `OPERATOR_IMAGE_REF=${OPERATOR_IMAGE}:sha-<commit-sha>`
so the component descriptor points at the immutable image produced by the same run.
`OCM_REGISTRY` is the registry namespace base, for example `ghcr.io/telekom`; CI derives
the operator image package from the repository role. Non-dev repositories publish
`${OCM_REGISTRY}/keycloak-operator` on `main`. Repositories whose name ends in `-dev`
always publish `${OCM_REGISTRY}/keycloak-operator-dev`, even on `main`, so private
development repositories cannot overwrite the release operator package. On release
publishes, the operator image is also tagged as `0.3.0` and `latest` for chart/RGD
defaults and human inspection.
CI also injects `SOURCE_REPO_URL`, `SOURCE_REF`, and `SOURCE_COMMIT`; the source ref stays
a valid Git ref while the commit field records the exact delivered revision.

In the operator build stage, CI also runs `make manifests` and fails if generated CRD/RBAC artifacts are dirty. This keeps checked-in generated files in sync with kubebuilder markers.

## Verification Evidence

For customer acceptance, evidence should exist in both places:

1. **CI run outputs** (primary evidence): job logs, artifacts, and `GITHUB_STEP_SUMMARY` from the exact delivery commit.
2. **Documentation in `/docs`** (auditability): expected behavior, verification flow, and interpretation of CI checks.

### Non-root verification scope

CI enforces non-root hardening via static checks (`scripts/tests/test-security.sh`) and image/security scanning.
This proves the delivered manifests and images are configured for non-root execution.
The main CI workflow scans the freshly built operator image by immutable `sha-<commit>`
tag immediately after pushing it. The separate scheduled/push security workflow scans the
semver release operator image only when that tag already exists; if the tag is not present
yet, it emits a warning instead of failing with a GHCR `MANIFEST_UNKNOWN` error.

If a customer requires explicit live runtime attestation, run an additional cluster check in the target environment (for example, verifying effective container UID/GID on running pods) and attach that output to the delivery evidence package.

## Triggers

| Trigger | Branches | Behavior |
|---------|----------|----------|
| `push` | `main`, `delivery/**`, `feature/**`, `fix/**`, `feat/**` | Runs checks/builds; OCM transfer is restricted to non-dev repositories on `main` |
| `pull_request` | `main` | Runs quality/operator build checks and the privileged verification gate (no OCM archive, transfer, or deploy) |
| `workflow_dispatch` | Any | Manual trigger with selectable stages |

### Manual Trigger

Navigate to **Actions** > **OCM Build & Deploy** > **Run workflow** to trigger a run with individual stage control:

| Input | Default | Description |
|-------|:-------:|-------------|
| `checkout_ref` | empty | Optional ref/SHA for manual runs, e.g. `refs/pull/123/merge` |
| `run_lint` | on | YAML Lint, ShellCheck, Gitleaks |
| `run_build` | on | Build, sign, and validate OCM component |
| `run_transfer` | on | Push OCM archive to registry. Effective only for non-dev repositories on `main` with no `checkout_ref` override. |
| `run_deploy` | on | Deploy to Kubernetes and run smoke tests |
| `run_backup_restore_verify` | on | Run live CNPG backup + restore smoke verification |
| `backup_provider` | `incluster-minio` | Provider for backup verification: `external-s3` or `incluster-minio` |

Stages have dependencies. Disabling an earlier stage while enabling a later one will fail unless a cached artifact from a previous run exists. Enable stages left-to-right.
Manual runs with `checkout_ref` are for privileged verification only; they use the dev
operator package and do not transfer the OCM release component.

## PR Policy and Merge Safety

For security and clarity, publish-dependent jobs are disabled for all `pull_request` events:

1. Build, sign, and validate the OCM archive
2. Transfer to registry
3. Deploy and verify against cluster

The OCM archive records the commit-specific `keycloak-operator-image` digest. PR runs do
not publish that image to GHCR, so the archive is built only from trusted branch pushes or
manual workflow runs.

Release publishing is additionally restricted to non-dev repositories on `main`.
Private development repositories can use the same `OCM_REGISTRY` namespace, but they publish
only the `keycloak-operator-dev` image package and never transfer the release OCM component.

PRs still run quality checks and the operator build/test job. Final privileged verification is done by running the same commit from a branch in the customer org repository (push or workflow dispatch). This gives one clear, fully green delivery signal for the customer.

### Hard merge gate for privileged verification

On `pull_request`, the workflow enforces a gate job that requires label `privileged-verified`.

Maintainer flow:

1. Start `workflow_dispatch` manually with `checkout_ref=refs/pull/<PR_NUMBER>/merge`.
2. Enable privileged stages needed for verification. OCM transfer stays disabled for `checkout_ref` runs by policy.
3. After a green run, add label `privileged-verified` to the PR.

Recommended branch protection: mark the job `0. PR Privileged Verify Gate` as required.

## Secrets and Variables

Configure secrets in GitHub under **Settings > Environments > cicd > Environment secrets**.

Always required secrets:

| Secret | Used By | Description |
|--------|---------|-------------|
| `KUBECONFIG` | Deploy | Base64-encoded kubeconfig for the target cluster |
| `OCM_REGISTRY_USER` | Transfer, Deploy | (Optional) Registry username. Defaults to `github.actor`. |
| `OCM_REGISTRY_PASSWORD` | Transfer, Deploy | (Optional) Registry token. Defaults to `secrets.GITHUB_TOKEN`. |

Provider-specific secrets:

| Secret | Needed When | Description |
|--------|-------------|-------------|
| `BACKUP_S3_ACCESS_KEY_ID` | `backup_provider=external-s3` | Access key ID for CNPG backup object storage smoke test |
| `BACKUP_S3_SECRET_ACCESS_KEY` | `backup_provider=external-s3` | Secret access key for CNPG backup object storage smoke test |
| `BACKUP_S3_DESTINATION_PATH_CI` | `backup_provider=external-s3` | CI-only S3 path for smoke backup writes, e.g. `s3://bucket/ci/keycloak` |
| `BACKUP_S3_ENDPOINT_URL_CI` | `backup_provider=external-s3` (optional) | CI-only S3-compatible endpoint URL for non-AWS targets |
| `MINIO_CI_ACCESS_KEY_ID` | `backup_provider=incluster-minio` (optional) | Fixed access key for in-cluster MinIO mode |
| `MINIO_CI_SECRET_ACCESS_KEY` | `backup_provider=incluster-minio` (optional) | Fixed secret key for in-cluster MinIO mode |

For non-manual GitHub Actions runs, backup verification always uses `incluster-minio`.
`external-s3` is only used when explicitly selected in `workflow_dispatch`.

`OCM_REGISTRY` can be set as a repository variable (not secret) if it does not need to be hidden. The value is the registry namespace base, for example `ghcr.io/telekom`. The pipeline checks `vars.OCM_REGISTRY` first, then `secrets.OCM_REGISTRY`, and falls back to `ghcr.io/<repository-owner>`. The same value is used for the OCM transfer target and as the prefix for the operator image package.

Repository naming controls release safety:

- `*-dev` repositories publish operator images only to `${OCM_REGISTRY}/keycloak-operator-dev`.
- Non-dev repositories on `main` publish `${OCM_REGISTRY}/keycloak-operator`, including `0.3.0` and `latest`.
- OCM transfer runs only from non-dev repositories on `main`, and not from `workflow_dispatch` runs with `checkout_ref`.

Optional repository variable:

| Variable | Default | Description |
|----------|:-------:|-------------|
| `OCM_REGISTRY` | `ghcr.io/<repository-owner>` | Registry namespace base used for OCM transfer and operator image packages. |
| `OCM_TRANSFER_IMMUTABLE` | `false` | `false`: allows overwrite of existing component version (release-safe default). `true`: immutable transfer, fails when the version already exists. |
| `OCM_TRANSFER_COPY_RESOURCES` | `true` | `true`: transfers referential OCM resources by value so the target registry receives localized image/chart blobs. `false`: leaves external references unchanged for debugging only. |

For final delivery with manually managed versions, the default `false` avoids release-day blocking. For long-term OSS operation, switch to `true` and enforce unique component versions.
Keep `OCM_TRANSFER_COPY_RESOURCES=true` for delivery and air-gapped verification.

### Creating the KUBECONFIG Secret

```bash
# From existing kubeconfig
cat ~/.kube/config | base64 -w 0

# From a specific context
kubectl config view --minify --flatten | base64 -w 0
```

## Deployment Strategy

### CI Environment

In CI the pipeline deploys with `--clean`, which deletes the target namespace before each run:

```bash
./scripts/deploy/deploy-all.sh cicd --namespace keycloak-cicd --clean
```

This ensures no state drift from previous runs, reproducible test results, and clean database initialization.

### Production / Development

Without `--clean`, existing resources are updated in-place via Kubernetes rolling updates:

```bash
./scripts/deploy/deploy-all.sh my-instance
```

### Startup Sequence

The deployment script enforces a strict startup order to avoid crash loops:

1. CloudNativePG operator installed (if missing)
2. PostgreSQL cluster created via CNPG CR
3. Wait for primary pod (`cnpg.io/instanceRole=primary`) to reach Ready
4. Keycloak Deployment applied
5. Init container `wait-for-db` confirms database port 5432 is reachable
6. Keycloak main container starts

## Smoke Tests

After deployment the pipeline verifies the instance is healthy:

1. **PostgreSQL** -- primary pod with label `cnpg.io/instanceRole=primary` is Ready
2. **Keycloak** -- `deployment/keycloak` reaches Available condition
3. **Pod status** -- all pods in the namespace are listed
4. **Health check** -- `curl http://localhost:9000/health/ready` through a local port-forward to the Keycloak management port

## Backup and Restore Live Verification

The deploy stage runs an end-to-end CNPG backup and restore smoke check via:

```bash
./scripts/tests/test-backup-restore.sh --live ...
```

Provider modes:

1. `incluster-minio` (default): creates/uses a MinIO service in namespace `backup-ci`, creates a CI bucket and run-specific prefix, and writes backup credentials into `keycloak-backup-s3` in the workload namespace.
2. `external-s3`: uses `BACKUP_S3_*` secrets and writes only to the CI target path.

Decision note: `incluster-minio` is the default to keep OSS onboarding friction low and avoid mandatory external object-store credentials for every contributor run. Use `external-s3` when validating environment parity, compliance controls, or production-like backup routing.
This means default CI runs do not require `BACKUP_S3_*` secrets.

In `incluster-minio` mode, existing MinIO root credentials are reused by default to avoid unnecessary restarts and cross-run flakiness when multiple CI executions target the same cluster.
By default, `incluster-minio` is treated as CI-only test infrastructure.

In both modes, the pipeline creates a namespace-scoped secret `keycloak-backup-s3`,
triggers an on-demand CNPG backup, waits for completion, and then applies a non-destructive
recovery cluster manifest to prove restore readiness.

For auditability, CI keeps both the restore smoke cluster and the smoke backup CR by default,
so post-run inspection in Rancher/Kubernetes remains possible.
Backup data is persisted in object storage (MinIO/S3) and is not deleted by this verification step.

Prerequisite: the target cluster must have the CNPG Barman Cloud Plugin installed
(`objectstores.barmancloud.cnpg.io` CRD present), otherwise the live verification stage fails.

### CI vs. INT separation

Use strictly separate backup targets for CI and integration environments:

1. CI workflow: `.../ci/...` path or dedicated CI bucket/tenant.
2. INT workflow: separate `.../int/...` path or dedicated INT bucket/tenant.

Do not share write credentials between CI and INT. This avoids accidental cleanup,
cross-contamination of test data, and audit ambiguity.

## Troubleshooting

**Pipeline hangs on "Waiting for primary pod..."**

The CNPG label is `cnpg.io/instanceRole=primary` (not `cnpg.io/role=primary`). Verify:

```bash
kubectl get pods -n keycloak-cicd --show-labels
```

**Health check returns "executable file not found"**

Ensure `-c keycloak` is specified in the `kubectl exec` command to target the main container, not the `wait-for-db` init container.

**Namespace stuck in Terminating**

```bash
kubectl delete namespace keycloak-cicd --force --grace-period=0
```

## Decision Record

*Decision: Use GitHub Actions as the CI/CD platform with quality checks, operator-image build, OCM build, transfer, and deploy/verify stages.*

The pipeline is designed around the OCM lifecycle: build a component archive, sign and validate it, transfer it to an OCI registry, and deploy it for verification. GitHub Actions was chosen because the project is hosted on GitHub and the workflow integrates directly with repository secrets and events. The `--clean` strategy for CI ensures reproducible runs by deleting the namespace before each deployment, avoiding state drift. Production deployments use in-place rolling updates instead.

The manual trigger (`workflow_dispatch`) with per-stage toggles allows running individual stages during development and debugging without re-running the full pipeline. Stages are intentionally sequential with left-to-right dependencies to match the OCM build-transfer-deploy lifecycle.

## Related Documents

| Topic | Document |
|-------|----------|
| OCM packaging strategy | [ARCHITECTURE.md](ARCHITECTURE.md) |
