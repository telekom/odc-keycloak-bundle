# Requirements

## Software-defined Keycloak Solution — OCM Package

| Attribute | Value |
|-----------|-------|
| **Document ID** | SRS-KEYCLOAK-001 |
| **Standard** | IREB / IEEE 29148:2018 |
| **Project** | Contract Item 2.1 — Software-defined Keycloak solution |
| **Repository** | https://github.com/opendefensecloud/keycloak-ocm |

This document defines the requirements for the software-defined Keycloak solution
distributed as an Open Component Model (OCM) component package. It is the authoritative
reference for what the system must do, what properties it must exhibit, and what
constraints it must satisfy.

Each requirement carries a unique **REQ-XX** identifier and belongs to one of three
delivery phases. The **Depends on** field lists direct prerequisite requirements by ID.
Acceptance tests for each requirement are defined in [ATS.md](ATS.md).

---

## Requirement Types

| Symbol | Meaning |
|--------|---------|
| **F** | Functional — describes observable system behaviour |
| **Q** | Quality / Non-Functional — describes a system property |
| **C** | Constraint — mandates a delivery format or technology choice |

## Delivery Phases

| Phase | Description |
|-------|-------------|
| **PoC** | Foundational packaging, first Keycloak instantiation, namespace isolation |
| **Alpha** | Complete core CRD set, multi-instance operation, KRO-based deployment |
| **Final** | Day-2 operations, security hardening, full documentation coverage |

---

## Overview

| ID | Title | Type | Phase | Depends on |
|----|-------|------|-------|------------|
| REQ-01 | OCM Component Package | C | PoC | — |
| REQ-02 | Multi-Instance Namespace Isolation | F | PoC | — |
| REQ-03 | Database Integration | F | PoC | REQ-02 |
| REQ-04 | Core Declarative CRDs | F | Alpha | REQ-02, REQ-03 |
| REQ-05 | Extended CRDs — Identity Providers & Auth Flows | F | Final | REQ-04 |
| REQ-06 | CR Status Reporting | F | Alpha | REQ-04 |
| REQ-07 | Secure Secrets Management | Q | PoC | REQ-02 |
| REQ-08 | HA & Scalability | Q | Alpha | REQ-02, REQ-03 |
| REQ-09 | KRO-Based Instantiation | F | Alpha | REQ-01, REQ-02, REQ-03, REQ-08 |
| REQ-10 | Observability | Q | Final | REQ-02, REQ-03 |
| REQ-11 | Backup & Restore | F | Final | REQ-03, REQ-06 |
| REQ-12 | Zero-Downtime Rolling Updates | Q | Final | REQ-08, REQ-11 |
| REQ-13 | Documentation | Q | PoC–Final | REQ-04, REQ-05, REQ-09, REQ-10, REQ-11, REQ-12 |
| REQ-14 | Quality Assurance & Hardening | Q | Final | REQ-01 |

---

## REQ-01 · OCM Component Package

| | |
|---|---|
| **Type** | Constraint |
| **Phase** | PoC |
| **Depends on** | — |

The solution shall be packaged as a single, versioned Open Component Model (OCM) component
archive. This archive is the exclusive unit of distribution: it must be self-contained,
signable, storable in any OCI-compatible registry, and transferable into air-gapped
environments without external network access at deployment time.

The archive shall include all container images with pinned versions and digests, all
Kubernetes CRD YAML files, the KRO ResourceGraphDefinition, the operator Helm chart, and
all supporting Kubernetes manifests.

### Acceptance Criteria

1. A single `ocm` component archive can be built, signed, and pushed to an OCI-compatible
   registry using the provided CI scripts.
2. The archive contains the Keycloak image, the PostgreSQL image, the CNPG operator image,
   all CRD YAML files, the KRO RGD, and all Kubernetes manifests.
3. All container image references use pinned digests or immutable tags.
4. The archive can be transferred to a disconnected private registry and a full deployment
   completed without any external network calls.
5. The component version is reflected consistently across all bundled resources.
6. All included software is open-source; the component descriptor records the upstream
   source reference for each resource.

Acceptance tests: [ATS.md § REQ-01](ATS.md#req-01--ocm-component-package--poc)

---

## REQ-02 · Multi-Instance Namespace Isolation

| | |
|---|---|
| **Type** | Functional |
| **Phase** | PoC |
| **Depends on** | — |

The solution shall support simultaneous operation of multiple independent Keycloak instances
on a single Kubernetes cluster. Each instance shall be fully self-contained with its own
namespace, database, Secrets, ServiceAccount, and RBAC policies. No instance shall be able
to read, modify, or interfere with the resources of any other instance through the
Kubernetes API.

The operator shall be deployed once per instance and shall restrict its watch scope to its
own namespace. All RBAC policies shall be namespace-scoped Roles and RoleBindings; no
ClusterRoles shall be used for instance-specific permissions.

### Acceptance Criteria

1. Two instances deployed in parallel operate independently; creating, updating, or deleting
   resources in one has no observable effect on the other.
2. Each instance resides in a dedicated, administrator-selected namespace; no resources of one
   instance appear in another's namespace.
3. The operator's RBAC Role is namespace-scoped and grants no permissions outside its own
   instance namespace.
4. Deleting one instance's `KeycloakInstance` CR does not affect any other running instance.
5. The operator sets `WATCH_NAMESPACE` to its own pod namespace and does not process CRs
   from foreign namespaces.

Acceptance tests: [ATS.md § REQ-02](ATS.md#req-02--multi-instance-namespace-isolation--poc)

---

## REQ-03 · Database Integration

| | |
|---|---|
| **Type** | Functional |
| **Phase** | PoC |
| **Depends on** | REQ-02 |

Each Keycloak instance shall use a dedicated, production-grade PostgreSQL database. The
OCM package shall provision this database automatically as part of instance creation. An
externally managed PostgreSQL cluster may alternatively be used by providing connection
parameters through the instance CRD.

The database shall be deployed via CloudNativePG (CNPG). Keycloak shall not start until
database connectivity is confirmed. All database credentials shall be managed as Kubernetes
Secrets (see REQ-07).

### Acceptance Criteria

1. A fresh deployment automatically provisions a CloudNativePG PostgreSQL cluster alongside
   Keycloak with no manual database setup.
2. Keycloak does not start until the init container confirms the database port is reachable.
3. Database credentials are injected exclusively via `secretKeyRef`; no plaintext password
   appears in any Deployment spec or log.
4. The `KeycloakInstance` CR exposes fields to configure database replica count and storage
   size; changes are reflected in the CNPG Cluster CR.
5. An external PostgreSQL instance can be used by providing connection details via the
   instance CR, without the OCM package deploying a CNPG cluster.
6. The CNPG cluster provides automatic failover; the Keycloak service remains available
   after a simulated primary pod failure.

Acceptance tests: [ATS.md § REQ-03](ATS.md#req-03--database-integration--poc)

---

## REQ-04 · Core Declarative CRDs

| | |
|---|---|
| **Type** | Functional |
| **Phase** | Alpha |
| **Depends on** | REQ-02, REQ-03 |

Administrators shall be able to configure the core Keycloak building blocks declaratively
using Kubernetes custom resources, enabling GitOps workflows where desired state is
committed to Git and automatically reconciled by the operator.

The five core resource types are: **realms**, **clients**, **client scopes**, **groups**,
and **users**. The operator shall handle create, update, and delete for each type. All CRDs
shall be namespace-scoped. Realm deletion shall intentionally preserve the realm in
Keycloak; all other resource types shall be removed upon CR deletion.

### Acceptance Criteria

1. A `KeycloakRealm` CR creates or updates realm-level settings; deleting the CR does not
   remove the realm from Keycloak.
2. A `KeycloakClient` CR creates, updates, and removes the corresponding OIDC or SAML
   client when applied or deleted.
3. A `KeycloakClientScope` CR manages scope definitions; deletion removes the scope.
4. A `KeycloakGroup` CR manages group definitions and realm-role assignments; deletion
   removes the group.
5. A `KeycloakUser` CR manages user accounts including group membership; deletion removes
   the user.
6. All five resource types coexist in the same namespace and reconcile without mutual
   interference.
7. Each CRD OpenAPI schema includes `description` annotations on all `spec` properties,
   enabling `kubectl explain` to surface inline documentation.
8. The operator reconciles CRs within one reconciliation cycle (≤ 30 s) of a CR being
   created, updated, or deleted.

Acceptance tests: [ATS.md § REQ-04](ATS.md#req-04--core-declarative-crds--alpha)

---

## REQ-05 · Extended CRDs — Identity Providers & Authentication Flows

| | |
|---|---|
| **Type** | Functional |
| **Phase** | Final |
| **Depends on** | REQ-04 |

The solution shall provide declarative CRDs for two additional Keycloak building blocks:
**identity providers** (external IdP connections such as OIDC or SAML) and
**authentication flows** (customisable step-by-step login sequences including MFA policies
and conditional execution).

Identity provider secrets shall be referenced via Kubernetes Secret references and shall
never be embedded as plaintext in a CR spec (see REQ-07). Authentication flow CRDs shall
support the full Keycloak execution model: required, alternative, and conditional steps.
Both CRD types shall be included in the OCM archive (REQ-01) and documented in
[USAGE.md](USAGE.md) (REQ-13).

### Acceptance Criteria

1. A `KeycloakIdentityProvider` CR configures an external OIDC or SAML IdP in Keycloak
   and the configuration is updated when the CR is modified.
2. A `KeycloakAuthFlow` CR defines a custom authentication flow and overrides the default
   browser or direct-grant flow when assigned to a realm.
3. Deleting either CR removes the corresponding configuration from Keycloak.
4. All secrets required by identity provider CRs are referenced via `secretKeyRef` and
   never stored as plaintext in any CR spec field.
5. Both CRD types are included in the OCM component archive.
6. Both CRD types are documented with field-level reference tables and examples in
   [USAGE.md](USAGE.md).

Acceptance tests: [ATS.md § REQ-05](ATS.md#req-05--extended-crds--final)

---

## REQ-06 · CR Status Reporting

| | |
|---|---|
| **Type** | Functional |
| **Phase** | Alpha |
| **Depends on** | REQ-04 |

Every custom resource managed by the operator shall expose its current health and
synchronisation state through a `status` subresource in the Kubernetes API. Operations
staff and automated tooling shall be able to determine whether a resource is synchronised,
pending, or in error — without inspecting operator logs.

Every CRD shall define a `status` subresource containing: a `ready` boolean, a
`keycloakId` string, a `lastSyncTime` timestamp, a human-readable `message`, and a
`conditions` array with at least a `Ready` condition following the standard Kubernetes
pattern. The operator shall update the status on every reconciliation cycle, on both
success and failure paths.

### Acceptance Criteria

1. `kubectl get keycloakclient -o wide` displays a `READY` column reflecting the actual
   synchronisation state.
2. When Keycloak is unreachable, all affected CRs transition to `Ready=False` with a
   descriptive error message within one reconciliation cycle.
3. When connectivity is restored, the condition transitions back to `Ready=True`
   automatically without manual intervention.
4. `status.lastSyncTime` is updated on every reconciliation pass.
5. `status.keycloakId` is populated with the Keycloak-internal identifier after the first
   successful synchronisation.
6. A failed creation or update includes the HTTP status code or error message in the
   `status.conditions[Ready].message` field.

Acceptance tests: [ATS.md § REQ-06](ATS.md#req-06--cr-status-reporting--alpha)

---

## REQ-07 · Secure Secrets Management

| | |
|---|---|
| **Type** | Quality (Security) |
| **Phase** | PoC |
| **Depends on** | REQ-02 |

All sensitive data — database passwords, Keycloak admin credentials, TLS certificates,
and external IdP client secrets — shall be stored and managed exclusively as Kubernetes
Secrets. No credential shall appear in plaintext in any manifest, Helm values file, CRD
spec field, operator source code, or log line.

CRD fields that require credentials shall use a `secretKeyRef` pattern. Operator-generated
credentials shall be stored as Secrets in the target namespace. Automated secret scanning
in CI (see REQ-14) shall enforce this constraint on every pull request.

### Acceptance Criteria

1. A static credential scan (Gitleaks) over the full repository history finds no
   credentials embedded in any manifest or source file.
2. The Keycloak admin password is accessible only via a Kubernetes Secret.
3. Database credentials are injected exclusively via `secretKeyRef` references.
4. Client credential Secrets created for confidential clients are placed in the
   `KeycloakClient` CR namespace and contain exactly the keys `CLIENT_ID` and
   `CLIENT_SECRET`.
5. The `KeycloakUser` CRD accepts initial passwords only as a Secret reference; no
   plaintext password field is present in the CRD spec.
6. All CRD fields that would contain credentials use a `secretKeyRef` sub-object instead
   of a string value field.

Acceptance tests: [ATS.md § REQ-07](ATS.md#req-07--secure-secrets-management--poc)

---

## REQ-08 · HA & Scalability

| | |
|---|---|
| **Type** | Quality (Availability) |
| **Phase** | Alpha |
| **Depends on** | REQ-02, REQ-03 |

The Keycloak deployment shall support multi-replica operation so that the loss of a single
pod does not cause a service outage. Session data and distributed caches shall be shared
across all replicas. The operator shall also support running as multiple replicas, with
exactly one replica active as the reconciliation leader at any time.

Keycloak shall use Kubernetes-native cluster discovery (`KC_CACHE_STACK=kubernetes`) for
Infinispan session sharing. A `PodDisruptionBudget` shall ensure at least one replica
remains available during voluntary disruptions. Replica counts for both Keycloak and the
database shall be configurable via the `KeycloakInstance` CR.

### Acceptance Criteria

1. A `KeycloakInstance` CR with `spec.replicas: 3` results in three Keycloak pods reaching
   `Ready` state and forming a single Infinispan cluster.
2. A session established on one pod remains valid after that pod is forcibly deleted.
3. A `PodDisruptionBudget` with `minAvailable: 1` prevents all Keycloak pods from being
   simultaneously evicted.
4. Only one operator replica holds the `Lease` resource at any given time.
5. If the lease-holder pod is deleted, another replica acquires the lease within one
   lease-duration interval and resumes reconciliation.
6. Replica counts for Keycloak and PostgreSQL are configurable through the
   `KeycloakInstance` CR without modifying any manifest.

Acceptance tests: [ATS.md § REQ-08](ATS.md#req-08--ha--scalability--alpha)

---

## REQ-09 · KRO-Based Instantiation

| | |
|---|---|
| **Type** | Functional |
| **Phase** | Alpha |
| **Depends on** | REQ-01, REQ-02, REQ-03, REQ-08 |

The entire Keycloak stack — namespace, database, operator, and Keycloak itself — shall be
deployable by applying a single Kubernetes custom resource. The Kubernetes Resource
Orchestrator (KRO) shall model the dependency graph between components so that they are
created in the correct order without manual sequencing. Deleting the single CR shall
cleanly remove all created resources.

A `ResourceGraphDefinition` (RGD) shall encode the full dependency graph and be included
in the OCM component archive (REQ-01). The `KeycloakInstance` CR schema shall expose only
the parameters an operator genuinely needs to configure.

### Acceptance Criteria

1. Applying a single `KeycloakInstance` CR triggers KRO to create the namespace, deploy
   the CNPG cluster, deploy the operator, and start Keycloak in dependency order, with no
   manual sequencing required.
2. Keycloak pods reach `Ready` state without the operator being deployed separately.
3. Deleting the `KeycloakInstance` CR removes all KRO-managed child resources cleanly.
4. The deployment works end-to-end from an OCM archive in a private registry, without
   internet access during deployment.
5. The RGD is included in the OCM component archive as a `blueprint` resource.
6. Two separate `KeycloakInstance` CRs can be applied simultaneously and result in two
   fully independent instances.

Acceptance tests: [ATS.md § REQ-09](ATS.md#req-09--kro-based-instantiation--alpha)

---

## REQ-10 · Observability

| | |
|---|---|
| **Type** | Quality (Operability) |
| **Phase** | Final |
| **Depends on** | REQ-02, REQ-03 |

Operations teams shall have full visibility into each Keycloak instance through three
observability pillars: structured log output, distributed tracing, and Prometheus metrics.

Logs shall be emitted in structured JSON format to stdout. Tracing shall use the
OpenTelemetry protocol (OTLP/gRPC); the collector endpoint shall be configurable via
environment variable and disabled by default. Metrics shall be exposed on the Keycloak
management port at `/metrics`. Prometheus scraping shall be configured via `ServiceMonitor`
and `PodMonitor` CRs for the Prometheus Operator. Alerting rules shall cover at minimum:
pod availability, high login failure rate, database connection exhaustion, and pod restart
frequency. Observability configuration shall be documented in [OBSERVABILITY.md](OBSERVABILITY.md).

### Acceptance Criteria

1. Keycloak pods emit logs in structured JSON format to stdout.
2. Setting `KC_TRACING_ENABLED=true` and providing an OTLP/gRPC endpoint causes Keycloak
   to produce spans visible in a compatible tracing backend.
3. A port-forward to management port 9000 returns Prometheus-format metrics at `/metrics`.
4. A Prometheus Operator deployment automatically picks up the `ServiceMonitor` and begins
   scraping without additional configuration.
5. The liveness probe (`/health/live`) and readiness probe (`/health/ready`) on port 9000
   correctly gate pod traffic and lifecycle transitions.
6. The defined `PrometheusRule` alerts fire correctly under simulated fault conditions.
7. Observability configuration is documented in [OBSERVABILITY.md](OBSERVABILITY.md).

Acceptance tests: [ATS.md § REQ-10](ATS.md#req-10--observability--final)

---

## REQ-11 · Backup & Restore

| | |
|---|---|
| **Type** | Functional |
| **Phase** | Final |
| **Depends on** | REQ-03, REQ-06 |

Operations staff shall be able to trigger an on-demand backup of the Keycloak database and
restore from a known-good backup. The mechanism shall be triggerable via Kubernetes custom
resources so that it can be automated from CI/CD pipelines without direct database access.

Backup covers the PostgreSQL data managed by CloudNativePG. The backup storage location
shall be configurable (S3-compatible object store). Backup status shall be reported via the
CR `status` subresource (REQ-06). The complete restore procedure shall be documented in
[UPGRADE.md](UPGRADE.md).

### Acceptance Criteria

1. A backup is triggered via CNPG-native Kubernetes resources (`Backup` or
   `ScheduledBackup`) and completes successfully.
2. The backup storage location (S3-compatible endpoint, bucket, credentials) is
   configurable without modifying operator code or manifests.
3. A restore procedure applied from a known-good backup returns Keycloak to the backed-up
   state; users and clients created before the backup are present after the restore.
4. The full backup and restore cycle is documented with exact commands in [UPGRADE.md](UPGRADE.md).
5. Backup completion and errors are reported in the CNPG backup resource `status`.

Acceptance tests: [ATS.md § REQ-11](ATS.md#req-11--backup--restore--final)

---

## REQ-12 · Zero-Downtime Rolling Updates

| | |
|---|---|
| **Type** | Quality (Availability) |
| **Phase** | Final |
| **Depends on** | REQ-08, REQ-11 |

Updates to the Keycloak version, operator version, or runtime configuration shall not cause
a service outage. Rolling update strategies shall ensure that new pods are fully
initialised and healthy before old pods are terminated. This applies to both minor version
updates (backward-compatible) and major version upgrades (which may require database schema
migrations).

The Keycloak Deployment shall use `RollingUpdate` strategy with `maxUnavailable: 0`.
Readiness probes (REQ-10) shall gate traffic so requests are only routed to fully
initialised pods. Operator updates shall not break existing CRs; a migration path or
compatibility guarantee shall be documented for CRD schema changes. The major version
upgrade runbook shall be documented in [UPGRADE.md](UPGRADE.md).

### Acceptance Criteria

1. A Keycloak minor version bump applied via the `KeycloakInstance` CR causes a rolling
   restart with zero dropped requests, verified by a continuous request probe.
2. A minor PostgreSQL version upgrade via CNPG rolling restart completes without Keycloak
   losing database connectivity or dropping any in-flight requests.
3. A Keycloak major version upgrade following the documented runbook completes with at most
   the explicitly stated maintenance window for schema migration.
4. [UPGRADE.md](UPGRADE.md) covers the full major version upgrade procedure, including the
   single-instance migration mode and scale-back procedure.
5. The operator's CRD compatibility policy is documented: which fields are immutable, what
   happens when new optional fields are added, and what migration steps apply to breaking
   schema changes.

Acceptance tests: [ATS.md § REQ-12](ATS.md#req-12--zero-downtime-rolling-updates--final)

---

## REQ-13 · Documentation

| | |
|---|---|
| **Type** | Quality (Usability) |
| **Phase** | PoC–Final (ongoing) |
| **Depends on** | REQ-04, REQ-05, REQ-09, REQ-10, REQ-11, REQ-12 |

The solution shall be accompanied by complete written documentation covering installation,
configuration, and operation, maintained in the same repository as the code and updated
with every delivery.

Documentation shall cover: installation from the OCM archive, all CRD fields and their
effects, multi-instance setup, upgrade procedures including major version and rollback,
observability configuration, and backup and restore. Every CRD shall carry field-level
`description` annotations so that `kubectl explain` surfaces inline documentation.

### Acceptance Criteria

1. `README.md` links accurately to all existing documentation files and is updated with
   every delivery.
2. A new operator following only the written documentation can deploy a working Keycloak
   instance end-to-end without assistance.
3. Every CRD `spec` property carries a `description` annotation; `kubectl explain` returns
   a non-empty description for every field.
4. [USAGE.md](USAGE.md) contains field-level reference tables and worked examples for all
   CRD types delivered to date.
5. [UPGRADE.md](UPGRADE.md) covers all component upgrade scenarios including rollback.
6. A final documentation review confirms that every acceptance criterion has a
   corresponding section in the documentation.
7. [OBSERVABILITY.md](OBSERVABILITY.md) documents OTEL endpoint configuration, Prometheus
   scraping setup, and alerting rule tuning.

Acceptance tests: [ATS.md § REQ-13](ATS.md#req-13--documentation--pocfinal)

---

## REQ-14 · Quality Assurance & Hardening

| | |
|---|---|
| **Type** | Quality (Security / Maintainability) |
| **Phase** | Final |
| **Depends on** | REQ-01 |

The codebase and deployment artefacts shall meet a defined quality bar covering automated
testing, security scanning, and security hardening. Contribution guidelines shall ensure
that all future changes maintain these standards. Hardening shall follow a recognised guide
(DISA STIGs or CIS Benchmark) and deviations shall be documented with justification.

CI pipelines shall run on every pull request and shall block merges on: YAML linting
failures, ShellCheck violations, detected secrets, and critical CVEs in container images.
All container images in the OCM archive shall run as non-root with dropped Linux
capabilities and no privilege escalation.

### Acceptance Criteria

1. A pull request introducing a shell script that fails ShellCheck is blocked before merge.
2. A pull request introducing a plaintext credential detectable by Gitleaks is blocked
   before merge.
3. A pull request introducing a container image with a critical CVE is blocked before merge.
4. A pull request introducing syntactically invalid YAML is blocked by the linting step.
5. `CONTRIBUTING.md` exists in the repository root and covers: branching strategy, commit
   message conventions, pull request review process, and CI quality gates.
6. All container images in the OCM archive run under a non-root UID, with
   `allowPrivilegeEscalation: false` and `capabilities.drop: [ALL]`.
7. A documented hardening checklist cross-references the applied STIG or CIS Benchmark
   controls and records accepted deviations with justification.

Acceptance tests: [ATS.md § REQ-14](ATS.md#req-14--quality-assurance--hardening--final)
