# Keycloak Bundle Solution for Kubernetes

A software-defined Keycloak solution packaged as an Open Component Model (OCM) component for air-gapped and cloud-native Kubernetes deployments. It features Kubernetes-native configuration via Custom Resources (CRDs) and a robust CI/CD pipeline.

## Table of Contents

- [Intention](#intention)
- [Features](#features)
- [Prerequisites](#prerequisites)
- [Project Structure](#project-structure)
- [Documentation](#documentation)
- [License](#license)

## Intention

This repository contains a **standalone OCM component** for Keycloak, developed as a building block for the [opendefensecloud/ocm-components](https://github.com/opendefensecloud/ocm-components) project. Until integration, it operates independently with its own deployment scripts and CI/CD pipeline.

The goal is to provide a fully reproducible, air-gap-capable Keycloak deployment that can be versioned, signed, and transferred as an OCM component archive. The solution includes a PostgreSQL database (via CloudNativePG), a namespace-scoped Keycloak Operator for declarative realm configuration, and multi-instance namespace isolation.

> [!NOTE]
> **Integration into opendefensecloud/ocm-components**
>
> In `ocm-components` this solution is intended to be published as **keycloak-bundle**. Supporting
> software like PostgreSQL and CloudNativePG are separate OCM components in the same
> repository. The KRO ResourceGraphDefinition (RGD) references these companion components
> rather than bundling them, so each dependency is versioned, signed, and transferable
> independently.
>
> For integration the keycloak component archive -- containing the Keycloak container image,
> Kubernetes manifests, the Keycloak configuration CRDs, and the RGD -- will be transferred into the
> shared OCI registry of `ocm-components`. Deployment then works through KRO: a
> `KeycloakInstance` custom resource triggers the RGD which creates an isolated namespace
> (`keycloak-<instance>`) and orchestrates the full stack -- referencing the PostgreSQL OCM
> component for the database, deploying Keycloak, and starting the Keycloak operator -- in the
> correct startup order. Consumer teams never interact with this repository directly; they
> declare `Client` CRs in their application repositories and the operator reconciles
> them against the running Keycloak instance, syncing credentials back as Kubernetes Secrets.
>
> Until that integration is complete, this repository operates standalone: it bundles all
> dependencies (including PostgreSQL images) in its own component archive and provides its
> own CI/CD pipeline and helper scripts to build, sign, transfer, and deploy independently.

## Features

- **OCM Packaging** -- Keycloak and supporting dependency images, including the Keycloak Operator image reference, CRDs, the operator Helm chart, the KRO RGD, a CycloneDX SBOM, and Kubernetes manifests are bundled into a signed OCM component archive for transfer into air-gapped environments. CI injects the immutable operator image reference through `OPERATOR_IMAGE_REF` when building the component.
- **Automated CI/CD Pipeline** -- GitHub Actions workflow covering linting, ShellCheck, Gitleaks scanning, OCM build, sign, transfer, deployment, and smoke testing
- **Multi-Instance Isolation** -- Each Keycloak instance runs in a dedicated, administrator-selected namespace with its own PostgreSQL database, secrets, and RBAC boundaries
- **Declarative Kubernetes Configuration** -- Seven namespace-scoped Kubernetes CRDs (`Realm`, `Client`, `ClientScope`, `Group`, `User`, `AuthFlow`, `IdentityProvider`) with a reconciling operator that aggregates desired state into `realm.json`, applies it through `keycloak-config-cli`, and writes confidential-client credentials as Kubernetes Secrets
- **KRO-Based Instantiation** -- A single `KeycloakInstance` CR triggers KRO to create the namespace, deploy PostgreSQL, and start Keycloak and its operator in the correct dependency order; deleting the CR removes everything cleanly
- **High Availability Primitives** -- Configurable replica count for Keycloak and PostgreSQL via KRO/CNPG, rolling-update settings, readiness/liveness probes, operator leader election, and a PodDisruptionBudget. Session-clustering configuration is not currently enabled in the shipped Keycloak manifests and must be completed before claiming production session failover.
- **Resilient Startup Sequence** -- Init containers wait for database availability, readiness and liveness probes monitor Keycloak health, and CNPG manages PostgreSQL primary pod election
- **Observability** -- Structured JSON logging, OpenTelemetry tracing (opt-in via `KC_TRACING_ENABLED`), Prometheus metrics on management port 9000, and pre-built `ServiceMonitor`, `PodMonitor`, and `PrometheusRule` resources for Prometheus Operator
- **Automated Dependency Updates** -- Renovate Bot configuration tracks upstream releases of Keycloak, CloudNativePG, and PostgreSQL images with digest pinning and automated PRs
- **Security Hardened** -- Non-root containers with dropped capabilities, namespace-scoped operator RBAC, ingress NetworkPolicy, Gitleaks secret scanning, ShellCheck for scripts, YAML linting, Trivy scanning, OCM signing, and generated CycloneDX SBOM evidence. Production deployments must still provide an external secret-management flow and a hardened TLS/hostname configuration.
- **Reproducible Deployments** -- OCM component resources are versioned and image resources are digest-pinned; helper scripts support `--clean` for fresh CI environments and deterministic component archives

## Prerequisites

- Kubernetes cluster (1.28+)
- `kubectl` configured for the target cluster
- `ocm` CLI (for OCM packaging and transfer)

## Signature Verification

All final delivery artifacts are signed and must be verified before deployment.

- Repository public key: `security/ocm-signing-public-key.pub`
- Public key SHA256 (compute locally):

```bash
sha256sum security/ocm-signing-public-key.pub
```

Verify from registry:

```bash
ocm verify componentversions \
	--signature keycloak-bundle-sig \
	--public-key security/ocm-signing-public-key.pub \
	"<registry>//opendefense.cloud/keycloak-bundle:<version>"
```

Verify a downloaded CTF archive before installation:

```bash
./scripts/ocm/ocm-verify.sh \
	./keycloak-bundle-ctf.tar.gz \
	./security/ocm-signing-public-key.pub \
	keycloak-bundle-sig
```

## Project Structure

```text
keycloak/
├── .github/workflows/          # CI/CD pipeline (ci.yml)
├── operator/                   # Go Operator Source Code (Sync Engine)
├── Makefile                    # Root Makefile for developer workflows
├── manifests/                  # Kubernetes manifests
│   ├── keycloak/               #   Keycloak deployment + init container
│   ├── postgres/               #   CloudNativePG cluster
│   └── monitoring/             #   ServiceMonitor, PodMonitor, PrometheusRules
├── charts/                     # Helm charts
│   └── keycloak-operator/
│       └── crds/               #   All seven Keycloak CRDs
├── scripts/                    # Deployment & OCM scripts
├── component-constructor.yaml  # OCM component constructor (definition)
├── kro/                        # KRO Resource Group Definitions
├── examples/                   # Example resources
├── docs/                       # Documentation
└── README.md
```

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/ARCHITECTURE.md) | OCM/KRO architecture, multi-instance model, namespace isolation, HA |
| [Database](docs/DATABASE.md) | PostgreSQL with CloudNativePG decision and deployment model |
| [Client Configuration](docs/CLIENT.md) | Declarative configuration approach comparison and keycloak-operator |
| [CI/CD Pipeline](docs/CICD.md) | GitHub Actions pipeline, secrets, deployment strategy, troubleshooting |
| [Deployment](docs/DEPLOYMENT.md) | Deploying and removing the Keycloak OCM component on a cluster |
| [Usage Guide](docs/USAGE.md) | All CRD field references, GitOps workflow, CR status conditions, worked examples |
| [Observability](docs/OBSERVABILITY.md) | OpenTelemetry tracing, Prometheus metrics, alerting rule configuration |
| [Upgrade Runbook](docs/UPGRADE.md) | Safe upgrade procedures for Keycloak, PostgreSQL, CloudNativePG, and backup/restore |
| [Hardening Reference](docs/HARDENING.md) | CIS Benchmark controls applied, accepted deviations with justifications |

Additionally the documentation of the helper scripts for the CI/CD pipeline and for local development can be found at [scripts/README.md](scripts/README.md).

## License

Apache 2.0
