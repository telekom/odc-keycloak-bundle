# Declarative Keycloak Configuration

This document records the architectural decision for declarative Keycloak configuration in the Open Defense Cloud project and compares the approaches that were evaluated.

## Problem

A deployed Keycloak instance is an empty IAM server. Configuration (Realms, Clients, Users) must be:

- **Declarative**: Defined as K8s Custom Resources (CRDs), version-controlled in Git.
- **Continuously Reconciled**: Drift from the desired state must be detected and corrected.
- **Air-gap Compatible**: All artifacts must fit into a single OCM component.
- **Open Source**: Permissive license (Apache 2.0).

## Approaches Evaluated

### 1. Thin Custom Operator

Build a minimal Operator (Bash/Helm or Go) that directly watches specific CRDs and reconciles them against the Keycloak Admin API.

*   **Pros**: Lightest footprint (~20MB), full control, zero external dependencies.
*   **Cons**: Maintenance burden (we own the integration).

### 2. keycloak-config-cli (Wrapped)

Use [keycloak-config-cli](https://github.com/adorsys/keycloak-config-cli) as the engine, triggered by a thin K8s controller.

*   **Pros**: 100% Feature coverage, low maintenance.
*   **Cons**: "Run-to-completion" (Job) instead of continuous watch; potential temporary drift.

### 3. Crossplane Provider

Use `crossplane-contrib/provider-keycloak`.

*   **Pros**: Standard "Infrastructure as Data" model.
*   **Cons**: **Heavy footprint** (>500MB runtime, 100+ CRDs). Viable only if Crossplane is already present.

### 4. Official Keycloak Operator (RealmImport)

Use the official `RealmImport` CR.

*   **Pros**: Official supported.
*   **Cons**: **Create-only**. No updates, no drift correction. Disqualified for Day-2 operations.

### 5. Hybrid Operator (Custom + Config-CLI)

Combine **Option 1 (Custom Operator)** for high-frequency resources (Clients) and **Option 2 (Config-CLI Wrapper)** for complex, stable resources (Realms/Users).

*   **Pros**: Best of both worlds: Speed for dev-facing resources, stability for admin-facing resources.
*   **Cons**: Dual maintenance path (two controllers or logic branches).

## Comparison Matrix

| Feature | Custom Operator | config-cli Wrapper | Crossplane | Hybrid |
| :--- | :---: | :---: | :---: | :---: |
| **K8s CRDs** | ✅ Custom | ✅ Custom | ✅ Native | ✅ Custom |
| **Reconciliation** | ✅ Continuous | 🟡 Triggered | ✅ Continuous | ✅ Mixed |
| **Air-gap Fit** | ✅ Excellent | ✅ Good | ⚠️ Heavy | ✅ Good |
| **Footprint** | 🟢 Low | 🟢 Low | 🔴 High | 🟢 Low |

## Architecture

The CRD hierarchy follows the Keycloak domain model, scoped to Namespaces:

```text
KeycloakInstance (via KRO)
└── Realm
    ├── Client
    ├── User
    ├── Group
    └── ClientScope
```

For usage details and examples, see [USAGE.md](USAGE.md).

## Decision Record

### CRD Model for Keycloak Configuration (2026-01-31)

*Decision: Implement namespace-scoped CRDs for Keycloak resources.*

Declarative configuration is a core requirement. Namespace-scoped CRDs align with the multi-instance isolation model (see [ARCHITECTURE.md](ARCHITECTURE.md)) and enable GitOps workflows.

### Declarative Configuration Strategy — Initial (2026-02-11)

*Decision: Start with Client (Custom Operator) while evaluating the full Hybrid approach.*

Five approaches were evaluated. `keycloak-config-cli` lacks continuous reconciliation for high-frequency changes. A pure Custom Operator is too expensive to maintain for the full scope.

The Hybrid approach (Option 5) was selected as the working assumption, starting with `Client` as a proof-of-concept. The decision was kept open pending operational experience.

### Declarative Configuration Strategy — Final (2026-02-24)

*Decision: Extend the Custom Operator to cover all resource types. The Hybrid approach and Config-CLI are dropped.*

POC experience with `Client` confirmed that the Bash-based operator pattern is straightforward to extend. The Keycloak Admin API for Realm, User, Group, and ClientScope is not significantly more complex than for Client. Extending the existing operator avoids introducing a second tool (`keycloak-config-cli`) into the OCM component, keeps the OCM footprint minimal (one operator image), eliminates dual maintenance paths, and provides continuous reconciliation for all resource types — not just clients.

From `v0.2.0`, the single Custom Operator manages the full CRD hierarchy.

### Declarative Configuration Strategy — Revised for Enterprise OSS (2026-03-19)

*Decision: Pivot to the "Config-CLI Controller" (Hybrid Wrapper) architecture. A Go-based Kubernetes Operator provides Continuous Reconciliation, delegating the execution payload to `keycloak-config-cli`.*

The previous decision (Custom Operator with Bash/REST API) proved unmaintainable and highly error-prone (e.g., missing API logic for AuthFlow execution priorities). Hand-rolling an HTTP client for the Keycloak Admin API is bad practice for a security-critical Open Defense Cloud deployment and creates an unsustainable maintenance burden for the open-source community as APIs evolve.

The revised architecture leverages a lightweight **Go Operator** (using standard `controller-runtime`) to watch K8s Native CRDs automatically. It aggregates the desired state into Keycloak JSON format and triggers the **`keycloak-config-cli`** engine to safely execute the synchronization. This guarantees 100% API feature coverage, native Kubernetes UX, and continuous drift correction without maintaining any custom REST logic.

### Federated vs. Monolithic Reconciler (2026-03-26)

*Decision: Retain 6 distinct controllers (Federated Pattern) instead of a monolithic sync controller.*

A proposal to consolidate the 6 child controllers (`Client`, `User`, etc.) into a single generic "Universal Reconciler" was rejected. While a monolithic approach reduces Go boilerplate, it compromises the core security and operational requirements of the project. 

Separate controllers remain the finalized architectural standard to ensure:
- **Strict Finalizer Management** (Audit-proof deletion propagation).
- **CRD-level Status Reporting** (Immediate feedback for developers).
- **Multi-Instance Isolation** (Clean separation of concerns across namespaces).

### Shift-Left Data Validation (2026-03-31)

*Decision: Rely exclusively on Kubernetes API Server (`+kubebuilder:validation`) for schema enforcement, removing manual guard checks from the operator's execution logic.*

Previously, the operator contained defensive Go code to filter out invalid Custom Resources (e.g. missing Client IDs) before triggering keycloak-config-cli. This was replaced by strict `kubebuilder` markers (`Required`, `Enum`). This shifts validation left: invalid configurations are rejected directly during PR checks (`kubectl apply` or ArgoCD dry-runs) rather than failing silently or causing reconciliation deadlocks in production.

## Related Documents

| Topic | Document |
|---|---|
| Architecture Overview | [ARCHITECTURE.md](ARCHITECTURE.md) |
| Technical Usage Guide | [USAGE.md](USAGE.md) |
