# Deployment

This document describes how to deploy and remove the Keycloak OCM component on a Kubernetes cluster, starting from the component in an OCI registry.

## Overview

The Keycloak solution is distributed as a signed OCM component archive via an OCI registry. The component bundles the declared runtime dependency images, including the Keycloak Operator image reference, plus Kubernetes manifests, CRDs, RGD, chart, and SBOM required by the current deployment model. No external network access should be needed at deploy time after all required images and manifests have been mirrored internally.

Deployment happens in three phases:

```text
1. Retrieve           2. Cluster Setup (once)       3. Per Instance
────────────────      ──────────────────────        ──────────────────
Download component    CloudNativePG Operator   -->  Namespace
Verify signature      Prometheus Operator            PostgreSQL Cluster
Extract resources     (optional monitoring)          Keycloak Application
                                                    Keycloak Operator
```

## Prerequisites

- Kubernetes cluster 1.28+
- `kubectl` with cluster-admin permissions
- Helm 3+ (for the Keycloak Operator)
- OCM CLI installed ([ocm.software](https://ocm.software))
- Access credentials for the OCI registry hosting the component

## Retrieve the Component

The component is published to an OCI registry by the CI/CD pipeline (see [CICD.md](CICD.md)). On the target environment, download and inspect it using the OCM CLI.

> **Air-Gapped Deployment:** If deploying into a disconnected/air-gapped environment, use `scripts/ocm/ocm-transfer.sh` or `ocm transfer component --copy-resources` to mirror the entire bundle—including all referenced OCI images and Helm charts—to your internal registry.
> ```bash
> ocm transfer component --copy-resources \
>   ghcr.io/opendefensecloud//opendefense.cloud/keycloak-bundle:0.3.0 \
>   your-internal-registry.local/mirror
> ```
> Afterward, target your internal registry for the subsequent steps.

### Inspect the Component

```bash
COMPONENT=opendefense.cloud/keycloak-bundle
VERSION=0.3.0
REGISTRY=ghcr.io/opendefensecloud

ocm get componentversions "$REGISTRY//$COMPONENT:$VERSION"
ocm get resources "$REGISTRY//$COMPONENT:$VERSION"
```

The component contains:

| Resource | Type | Description |
|----------|------|-------------|
| `keycloak-image` | ociImage | Keycloak server |
| `postgres-image` | ociImage | PostgreSQL database |
| `cnpg-operator-image` | ociImage | CloudNativePG operator |
| `prometheus-operator-image` | ociImage | Prometheus Operator for optional monitoring |
| `keycloak-config-cli-image` | ociImage | Declarative Keycloak import engine used by the operator Jobs |
| `keycloak-operator-image` | ociImage | Keycloak reconciliation operator image reference injected by CI or release tooling |
| `keycloak-operator` | helmChart | Helm chart for the Keycloak operator (includes all seven CRDs) |
| `keycloak-instance-rgd` | blueprint | KRO ResourceGraphDefinition for one-CR stack instantiation |
| `keycloak-bundle-sbom` | sbom | CycloneDX SBOM for offline vulnerability and license analysis |
| `manifests` | directory | Kubernetes manifests (PostgreSQL cluster, Keycloak deployment, monitoring) |

The Keycloak Operator image is represented as the `keycloak-operator-image` OCM resource.
CI sets `OPERATOR_IMAGE_REF` to the immutable `sha-<commit-sha>` image produced by the same
run. For manually assembled releases, set `OPERATOR_IMAGE_REF` to the mirrored digest-pinned
operator image before running `scripts/ocm/ocm-create.sh`.

### Verify Signature

Signature verification is mandatory before any installation step.

Use the repository-published public key:

- `security/ocm-signing-public-key.pub`

Compute its SHA256 locally:

```bash
sha256sum security/ocm-signing-public-key.pub
```

```bash
ocm verify componentversions \
  --signature keycloak-bundle-sig \
  --public-key security/ocm-signing-public-key.pub \
  "$REGISTRY//$COMPONENT:$VERSION"
```

If you deploy from a downloaded CTF archive, verify it first:

```bash
./scripts/ocm/ocm-verify.sh \
  ./keycloak-bundle-ctf.tar.gz \
  ./security/ocm-signing-public-key.pub \
  keycloak-bundle-sig
```

### Download Resources

Extract the resources from the component to a local directory:

```bash
# Kubernetes manifests (Keycloak, PostgreSQL, monitoring)
ocm download resources "$REGISTRY//$COMPONENT:$VERSION" \
  manifests -O manifests.tar
tar xf manifests.tar -C manifests/

# KRO ResourceGraphDefinition (for KRO-based one-CR instantiation)
ocm download resources "$REGISTRY//$COMPONENT:$VERSION" \
  keycloak-instance-rgd -O keycloak-instance-rgd.yaml

# Operator Helm chart (includes all seven CRDs)
ocm download resources "$REGISTRY//$COMPONENT:$VERSION" \
  keycloak-operator -O operator-chart.tgz
```

In air-gapped environments, transfer the container images from the component to the cluster-local registry:

```bash
ocm transfer componentversions \
  "$REGISTRY//$COMPONENT:$VERSION" \
  <cluster-local-registry>
```

## Cluster-Wide Setup

### Install CloudNativePG Operator

The CloudNativePG operator manages PostgreSQL clusters declaratively. It is installed once and shared by all Keycloak instances. The operator image is bundled in the component (`cnpg-operator-image`).

For connected development clusters, the helper script installs the upstream release manifest:

```bash
./scripts/deploy/install-cnpg.sh 1.29.1

kubectl wait --for=condition=available --timeout=120s \
  deployment/cnpg-controller-manager -n cnpg-system
```

This creates the `cnpg-system` namespace, the CNPG CRDs, and the controller deployment.

For production air-gapped clusters, do not fetch the upstream URL from the target environment. Transfer the signed component to the internal registry, mirror the `cnpg-operator-image`, and apply an internally approved CNPG manifest that references the mirrored image.

### Install Prometheus Operator (optional — required for monitoring)

The Prometheus Operator is required to activate the `ServiceMonitor`, `PodMonitor`, and
`PrometheusRule` resources bundled with each instance. Skip this step if Prometheus
Operator is already installed or if monitoring is not needed.

For connected development clusters, the helper script installs the upstream bundle manifest. The operator image is bundled in the component (`prometheus-operator-image`).

```bash
./scripts/deploy/install-prometheus-operator.sh 0.91.0

kubectl wait --for=condition=available --timeout=120s \
  deployment/prometheus-operator -n default
```

This registers the `monitoring.coreos.com/v1` CRDs (`ServiceMonitor`, `PodMonitor`,
`PrometheusRule`, etc.) and starts the operator deployment in the `default` namespace.

> **Production note:** For production clusters, deploy Prometheus Operator from an internally approved bundle/image mirror. Consider deploying the full
> [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)
> Helm chart, which adds Prometheus, Alertmanager, and Grafana alongside the operator.

Once the operator is running, deploy the monitoring manifests into each instance namespace:

```bash
NAMESPACE=identity-<name>

kubectl -n "$NAMESPACE" apply -f manifests/monitoring/keycloak-service-monitor.yaml
kubectl -n "$NAMESPACE" apply -f manifests/monitoring/cnpg-pod-monitor.yaml
kubectl -n "$NAMESPACE" apply -f manifests/monitoring/keycloak-prometheus-rules.yaml
```

**Why per-namespace?** The monitoring resources are namespace-scoped and belong to the
instance they observe. This is consistent with the namespace-per-instance isolation
model (see [ARCHITECTURE.md](ARCHITECTURE.md)): each `ServiceMonitor` selects only pods
within its own namespace; there are no cluster-wide selectors. Prometheus must therefore
be configured to discover `ServiceMonitor` and `PodMonitor` resources across all
instance namespaces selected by your namespace selector — see [OBSERVABILITY.md](OBSERVABILITY.md) for details.

See [OBSERVABILITY.md](OBSERVABILITY.md) for scraping verification and alert tuning.

## Instance Deployment

### KRO-Based Deployment (recommended)

With KRO installed on the cluster, a single custom resource creates the entire stack — namespace, PostgreSQL cluster, Keycloak server, and operator — in the correct dependency order. Install the KRO RGD once per cluster:

```bash
kubectl apply -f kro/rgd/keycloak-instance-rgd.yaml
```

Provision the bootstrap admin Secret in the target namespace through your approved
secret-management workflow before the Keycloak pods start. For a local non-production
test, create a namespace and Secret manually:

```bash
kubectl create namespace identity-dev
kubectl create secret generic keycloak-admin \
  -n identity-dev \
  --from-literal=KEYCLOAK_ADMIN=admin \
  --from-literal=KEYCLOAK_ADMIN_PASSWORD='<rendered-secret-value>'
```

Then create an instance by applying a `KeycloakInstance` CR that references the Secret
name and keys, but never contains the password value:

```yaml
apiVersion: kro.run/v1alpha1
kind: KeycloakInstance
metadata:
  name: dev
spec:
  namespace: identity-dev  # administrator chooses the exact namespace name
  replicas: 1          # Keycloak replica count (set ≥2 for HA)
  dbInstances: 1       # PostgreSQL instances (set 3 for HA)
  dbStorageSize: 5Gi
  adminSecretName: keycloak-admin
  adminUserSecretKey: KEYCLOAK_ADMIN
  adminPasswordSecretKey: KEYCLOAK_ADMIN_PASSWORD
```

> **Credential warning:** the KRO schema does not accept an admin password value. It references an existing Secret through `adminSecretName`, `adminUserSecretKey`, and `adminPasswordSecretKey`. Do not commit rendered production Secrets in Git. For defense or other high-assurance environments, provision this Secret via an external secret manager or protected rendering workflow and rotate/remove the bootstrap admin after setup.

```bash
kubectl apply -f keycloak-instance.yaml

# Watch progress
kubectl get keycloakinstance dev
kubectl get pods -n identity-dev
```

Deleting the `KeycloakInstance` CR removes the resources managed by KRO, including the namespace, PostgreSQL cluster, Keycloak Deployment, and operator resources. Externally provisioned Secrets may be controlled by your secret-management workflow and should be handled according to that ownership model. The operator itself (CRDs) must be removed separately.

> **Note:** KRO must be installed on the cluster before the RGD is applied. See [KRO installation](https://kro.run/docs/getting-started/installation).

---

### Manual Deployment

Each instance lives in its own administrator-controlled namespace, providing isolation for data, configuration, network, and RBAC. The namespace name is chosen by the administrator — no prefix is enforced. Repeat the following steps for each instance.

### Step 1: Create Namespace

```bash
NAMESPACE="identity-poc"   # administrator chooses the exact namespace name

kubectl create namespace "$NAMESPACE"
```

### Step 2: Deploy PostgreSQL

```bash
kubectl apply -n "$NAMESPACE" -f manifests/postgres/
```

Wait for the primary pod:

```bash
kubectl wait pod -n "$NAMESPACE" \
  -l cnpg.io/cluster=keycloak-db,cnpg.io/instanceRole=primary \
  --for=condition=Ready --timeout=600s
```

CNPG creates a `keycloak-db-app` secret with auto-generated database credentials and a `keycloak-db-rw` service pointing to the primary.

### Step 3: Deploy Keycloak

```bash
kubectl apply -n "$NAMESPACE" -f manifests/keycloak/
```

This deploys the Keycloak server and its ClusterIP service on port 8080. The deployment expects a `keycloak-admin` secret for bootstrap credentials; the helper script `scripts/deploy/deploy-keycloak.sh` creates it automatically when missing (using `KEYCLOAK_ADMIN_USERNAME`/`KEYCLOAK_ADMIN_PASSWORD` or a generated random password).

> **Note:** In production, provision `keycloak-admin` out-of-band via your secret management flow and rotate credentials regularly.

Wait for Keycloak:

```bash
kubectl wait -n "$NAMESPACE" --for=condition=ready pod \
  -l app=keycloak --timeout=300s
```

If the helper script generated a random password, you can retrieve it from the Kubernetes secret via:

```bash
kubectl get secret keycloak-admin -n "$NAMESPACE" -o jsonpath='{.data.KEYCLOAK_ADMIN_PASSWORD}' | base64 -d
```

### Step 4: Access the Instance

```bash
kubectl port-forward -n "$NAMESPACE" svc/keycloak 8080:8080
```

Open http://localhost:8080 and log in with the admin credentials.

## Keycloak Operator

The Keycloak Operator enables declarative management for all seven Keycloak CRDs. It must run in the same namespace as the Keycloak instance it manages. The chart sets `WATCH_NAMESPACE` to the operator pod namespace, so installing one central release in `keycloak-operator` will only watch that namespace and will not reconcile CRs in instance namespaces.

### Install CRDs and Operator

The Helm chart bundles all seven CRDs (`Realm`, `AuthFlow`, `ClientScope`, `Group`, `Client`, `User`, `IdentityProvider`) and installs them automatically. The helper script copies the `keycloak-admin` bootstrap Secret into the chart's default `keycloak-admin-creds` shape and deploys the operator into the instance namespace:

```bash
./scripts/deploy/deploy-operator.sh "$NAMESPACE"
```

When installing the chart manually, first create the operator admin Secret in the instance namespace. The default chart expects `keycloak-admin-creds` with `username` and `password` keys:

```bash
ADMIN_USER="$(kubectl get secret keycloak-admin -n "$NAMESPACE" -o jsonpath='{.data.KEYCLOAK_ADMIN}' | base64 -d)"
ADMIN_PASSWORD="$(kubectl get secret keycloak-admin -n "$NAMESPACE" -o jsonpath='{.data.KEYCLOAK_ADMIN_PASSWORD}' | base64 -d)"

kubectl create secret generic keycloak-admin-creds \
  --namespace "$NAMESPACE" \
  --from-literal=username="$ADMIN_USER" \
  --from-literal=password="$ADMIN_PASSWORD"
```

Then install the chart into the same namespace:

```bash
helm upgrade --install keycloak-operator \
  charts/keycloak-operator \
  --namespace "$NAMESPACE" \
  --wait --timeout 120s
```

When deploying from the OCM component, extract the chart first:

```bash
ocm download resources "$REGISTRY//$COMPONENT:$VERSION" \
  keycloak-operator -O operator-chart.tgz

helm upgrade --install keycloak-operator operator-chart.tgz \
  --namespace "$NAMESPACE" \
  --wait --timeout 120s
```

### Create Resources

Apply any Keycloak CR into the instance namespace. The operator reconciles all seven resource types (realms, auth flows, client scopes, groups, clients, users, identity providers) and syncs state back to Keycloak. See [USAGE.md](USAGE.md) for full examples and field reference.

## Instance Removal

### Remove a Single Instance

Deleting the namespace removes all instance resources (PostgreSQL, Keycloak, secrets, PVCs):

```bash
kubectl delete namespace <namespace>
```

When the instance was created with KRO, delete the `KeycloakInstance` CR instead — KRO will remove all resources including the namespace:

```bash
kubectl delete keycloakinstance <instance-name>
```

List existing instances:

```bash
kubectl get keycloakinstance
kubectl get ns
```

### Remove Cluster-Wide Components

Remove these only after all instances have been deleted:

```bash
# Keycloak Operator release for an instance namespace
helm uninstall keycloak-operator -n <namespace>

# Keycloak configuration CRDs (cluster-wide)
kubectl delete crd \
  realms.keycloak.opendefense.cloud \
  clients.keycloak.opendefense.cloud \
  clientscopes.keycloak.opendefense.cloud \
  groups.keycloak.opendefense.cloud \
  users.keycloak.opendefense.cloud \
  authflows.keycloak.opendefense.cloud \
  identityproviders.keycloak.opendefense.cloud

# CloudNativePG
kubectl delete -f "<internal-cnpg-1.29.1-manifest.yaml>"
```

## Troubleshooting

**Keycloak stuck in init container** -- PostgreSQL is not ready. Check the CNPG cluster status:

```bash
kubectl get cluster keycloak-db -n "$NAMESPACE"
kubectl logs -n "$NAMESPACE" -l app=keycloak -c wait-for-db
```

**Keycloak pod restarting** -- Check for database connectivity or configuration errors:

```bash
kubectl logs -n "$NAMESPACE" -l app=keycloak -c keycloak
```

**Keycloak Operator not reconciling** -- Verify the operator pod is running in the same namespace as the target Keycloak instance and can reach Keycloak:

```bash
kubectl logs -n "$NAMESPACE" -l app=keycloak-operator
```

**Namespace stuck in Terminating** -- Finalizers or running pods can block deletion:

```bash
kubectl get all -n "$NAMESPACE"
```

## Related Documents

| Topic | Document |
|-------|----------|
| Architecture and multi-instance model | [ARCHITECTURE.md](ARCHITECTURE.md) |
| PostgreSQL and CloudNativePG decision | [DATABASE.md](DATABASE.md) |
| Client operator and CRD strategy | [CLIENT.md](CLIENT.md) |
| CI/CD pipeline and automated deployment | [CICD.md](CICD.md) |
| CRD usage and GitOps workflow | [USAGE.md](USAGE.md) |
| Metrics, alerts, and tracing | [OBSERVABILITY.md](OBSERVABILITY.md) |
