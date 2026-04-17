# Deployment

This document describes how to deploy and remove the Keycloak OCM component on a Kubernetes cluster, starting from the component in an OCI registry.

## Overview

The Keycloak solution is distributed as a signed OCM component archive via an OCI registry. The component bundles all container images, Kubernetes manifests, and CRDs required for a complete air-gapped deployment. No external network access is needed at deploy time.

Deployment happens in three phases:

```text
1. Retrieve           2. Cluster Setup (once)       3. Per Instance
────────────────      ──────────────────────        ──────────────────
Download component    CloudNativePG Operator   -->  Namespace
Extract resources                                   PostgreSQL Cluster
                                                    Keycloak Application
                                                    Client Operator (optional)
```

## Prerequisites

- Kubernetes cluster 1.28+
- `kubectl` with cluster-admin permissions
- Helm 3+ (for the Client Operator)
- OCM CLI installed ([ocm.software](https://ocm.software))
- Access credentials for the OCI registry hosting the component

## Retrieve the Component

The component is published to an OCI registry by the CI/CD pipeline (see [CICD.md](CICD.md)). On the target environment, download and inspect it using the OCM CLI.

> **Air-Gapped Deployment:** If deploying into a disconnected/air-gapped environment, use the `ocm transfer component` command to mirror the entire bundle—including all nested OCI images and Helm charts—to your internal registry. 
> ```bash
> ocm transfer component --copy-resources \
>   ghcr.io/opendefensecloud/keycloak-bundle//opendefense.cloud/keycloak-bundle:0.2.0 \
>   your-internal-registry.local/mirror/keycloak-bundle
> ```
> Afterward, target your internal registry for the subsequent steps.

### Inspect the Component

```bash
COMPONENT=opendefense.cloud/keycloak-bundle
VERSION=0.2.0
REGISTRY=ghcr.io/opendefensecloud/keycloak-bundle

ocm get componentversions "$REGISTRY//$COMPONENT:$VERSION"
ocm get resources "$REGISTRY//$COMPONENT:$VERSION"
```

The component contains:

| Resource | Type | Description |
|----------|------|-------------|
| `keycloak-image` | ociImage | Keycloak server |
| `postgres-image` | ociImage | PostgreSQL database |
| `cnpg-operator-image` | ociImage | CloudNativePG operator |
| `operator-chart` | helmChart | Helm chart for the Keycloak operator (includes all seven CRDs) |
| `keycloak-instance-rgd` | blueprint | KRO ResourceGraphDefinition for one-CR stack instantiation |
| `manifests` | directory | Kubernetes manifests (PostgreSQL cluster, Keycloak deployment, monitoring) |

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

```bash
kubectl apply -f "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.28/releases/cnpg-1.28.1.yaml"

kubectl wait --for=condition=available --timeout=120s \
  deployment/cnpg-controller-manager -n cnpg-system
```

This creates the `cnpg-system` namespace, the CNPG CRDs, and the controller deployment.

### Install Prometheus Operator (optional — required for monitoring)

The Prometheus Operator is required to activate the `ServiceMonitor`, `PodMonitor`, and
`PrometheusRule` resources bundled with each instance. Skip this step if Prometheus
Operator is already installed or if monitoring is not needed.

The operator is installed via the upstream bundle manifest. The operator image is bundled
in the component (`prometheus-operator-image`).

```bash
kubectl apply --server-side -f \
  "https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.80.1/bundle.yaml"

kubectl wait --for=condition=available --timeout=120s \
  deployment/prometheus-operator -n default
```

This registers the `monitoring.coreos.com/v1` CRDs (`ServiceMonitor`, `PodMonitor`,
`PrometheusRule`, etc.) and starts the operator deployment in the `default` namespace.

> **Production note:** For production clusters consider deploying the full
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

Then create an instance by applying a `KeycloakInstance` CR:

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
  adminUser: admin
  adminPassword: ChangeMe!   # Change before production use
```

```bash
kubectl apply -f keycloak-instance.yaml

# Watch progress
kubectl get keycloakinstance dev
kubectl get pods -n identity-dev
```

Deleting the `KeycloakInstance` CR removes all created resources including the namespace, PostgreSQL cluster, and all secrets. The operator itself (CRDs) must be removed separately.

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

## Client Operator

The Client Operator enables declarative client management via `Client` custom resources. It is optional and runs cluster-wide, independent of individual instances.

### Install CRDs and Operator

The Helm chart bundles all seven CRDs (`Realm`, `AuthFlow`, `ClientScope`, `Group`, `Client`, `User`, `IdentityProvider`) and installs them automatically:

```bash
helm upgrade --install keycloak-operator \
  charts/keycloak-operator \
  --namespace keycloak-operator \
  --create-namespace \
  --wait --timeout 120s
```

When deploying from the OCM component, extract the chart first:

```bash
ocm download resources "$REGISTRY//$COMPONENT:$VERSION" \
  operator-chart -O operator-chart.tgz

helm upgrade --install keycloak-operator operator-chart.tgz \
  --namespace keycloak-operator \
  --create-namespace \
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
# Client Operator
helm uninstall keycloak-operator -n keycloak-operator
kubectl delete namespace keycloak-operator
kubectl delete crd Clients.keycloak.opendefense.cloud

# CloudNativePG
kubectl delete -f "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.28/releases/cnpg-1.28.1.yaml"
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

**Client Operator not reconciling** -- Verify the operator pod and its connectivity to Keycloak:

```bash
kubectl logs -n keycloak-operator -l app=keycloak-operator
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
