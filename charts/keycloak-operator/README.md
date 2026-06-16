# keycloak-operator Helm Chart

Kubernetes Operator for managing Keycloak resources (Realm, Client, Group, User,
ClientScope, AuthFlow, IdentityProvider) via CRDs.

The operator watches custom resources in its own namespace, assembles a Keycloak
realm export, and applies it by running a short-lived `keycloak-config-cli` Job.
See [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md) for the full reconcile flow.

---

## Prerequisites

- Kubernetes 1.26+
- Helm 3.10+
- A running Keycloak instance reachable from the operator pod
- A Kubernetes Secret containing Keycloak admin credentials in the release namespace
  (default name `keycloak-admin-creds`, keys `username` and `password`)
- (Optional) Prometheus Operator — required if `ServiceMonitor` resources are enabled

---

## Installation

```bash
helm install keycloak-operator charts/keycloak-operator \
  --namespace <keycloak-instance-namespace> \
  --create-namespace
```

Install one operator release per Keycloak instance namespace. The chart sets
`WATCH_NAMESPACE` from the operator pod namespace and does not reconcile CRs in other
namespaces.

Current chart limitation: the operator Deployment still reads `KEYCLOAK_USER` and
`KEYCLOAK_PASSWORD` from a Secret named `keycloak-admin-creds` with keys `username` and
`password`. Keep that Secret name for now, or patch the Deployment template together with
any `operator.adminSecret.*` override.

---

## Values reference

| Key | Default | Description |
|-----|---------|-------------|
| `replicaCount` | `2` | Number of operator replicas. |
| `image.repository` | `ghcr.io/opendefensecloud/keycloak-operator` | Operator container image repository. |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy. |
| `image.tag` | `"0.3.0"` | Image tag. Defaults to chart `appVersion` if omitted. |
| `imagePullSecrets` | `[]` | List of image pull secret names to inject into the operator pod. These are also propagated to config-cli Job pods automatically. |
| `nameOverride` | `""` | Override the chart name portion of generated resource names. |
| `fullnameOverride` | `""` | Override the full name of generated resources. |
| `labels.component` | `keycloak-operator` | Extra label `app.kubernetes.io/component`. |
| `labels.partOf` | `keycloak-bundle` | Extra label `app.kubernetes.io/part-of`. |
| `serviceAccount.create` | `true` | Create the operator ServiceAccount. |
| `serviceAccount.name` | `""` | Override the ServiceAccount name. Auto-generated when empty. |
| `resources.limits.cpu` | `500m` | CPU limit for the operator pod. |
| `resources.limits.memory` | `128Mi` | Memory limit for the operator pod. |
| `resources.requests.cpu` | `10m` | CPU request for the operator pod. |
| `resources.requests.memory` | `64Mi` | Memory request for the operator pod. |
| `nodeSelector` | `{}` | Node selector constraints for the operator pod. |
| `tolerations` | `[]` | Tolerations for the operator pod. |
| `affinity` | `{}` | Affinity rules for the operator pod. |
| `operator.logLevel` | `info` | Operator log verbosity (`debug`, `info`, `warn`, `error`). |
| `operator.configCliImage.repository` | `quay.io/adorsys/keycloak-config-cli` | Container image for the config-cli Job. |
| `operator.configCliImage.tag` | `latest-26@sha256:1b22dfaa9ae0c71f74b0342f9221a6510f272da5def683dbba26a98e6b1b1411` | Digest-pinned config-cli image tag used by reconciliation Jobs. |
| `operator.adminSecret.name` | `keycloak-admin-creds` | Name passed to the controller for config-cli Job password lookup. The operator Deployment currently also expects a Secret named `keycloak-admin-creds`. |
| `operator.adminSecret.key` | `password` | Password key passed to the controller for config-cli Jobs. The operator Deployment currently reads `password` directly. The username is read from the `username` key. |
| `metrics.enabled` | `true` | Expose the operator's Prometheus metrics endpoint. |
| `metrics.port` | `8080` | Port the metrics server listens on inside the container. |
| `metrics.path` | `/metrics` | HTTP path that Prometheus scrapes. |
| `metrics.serviceMonitor.enabled` | `false` | Create a Prometheus Operator `ServiceMonitor` for the operator. Requires `metrics.enabled: true` and the Prometheus Operator CRDs to be installed. |
| `metrics.serviceMonitor.interval` | `"30s"` | Scrape interval for the `ServiceMonitor`. |
| `metrics.serviceMonitor.labels` | `{}` | Extra labels added to the `ServiceMonitor` (e.g. `release: prometheus`). |
| `configCLI.serviceAccountName` | `keycloak-config-cli` | ServiceAccount assigned to config-cli Job pods. The chart creates this account with `automountServiceAccountToken: false`; config-cli only calls Keycloak's HTTP API and requires no K8s API access. |

---

## Upgrading

See [docs/MIGRATION.md](../../docs/MIGRATION.md) for version-specific migration steps
and CRD upgrade instructions.
