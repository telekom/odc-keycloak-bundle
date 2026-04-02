# Upgrade Runbook

This document describes how to safely upgrade the components of the Keycloak OCM bundle.

---

## General Principles

- Always upgrade one component at a time and verify health before proceeding.
- Take a database backup before any upgrade that touches Keycloak or PostgreSQL.
- All version changes must go through the `component-constructor.yaml` and a new OCM component release.

---

## 1. Keycloak Minor / Patch Version Upgrade

Keycloak handles database schema migrations automatically on startup. Minor and patch upgrades are rolling and safe.

**Steps:**

1. Take a CloudNativePG backup (see section 3).
2. Update the image tag in `manifests/keycloak/keycloak-deployment.yaml` and `component-constructor.yaml`.
3. Commit, push, and let the CI pipeline build and publish the new OCM component version.
4. Apply the updated manifests to the target cluster:
   ```sh
   kubectl apply -f manifests/keycloak/keycloak-deployment.yaml
   ```
5. Verify the rolling update completes:
   ```sh
   kubectl rollout status deployment/keycloak
   ```
6. Confirm health:
   ```sh
   kubectl exec -it deploy/keycloak -- curl -s http://localhost:9000/health/ready
   ```

---

## 2. Keycloak Major Version Upgrade

Major Keycloak upgrades may include breaking database schema changes. Keycloak applies migrations
automatically on startup, but a concurrent write from the old version during migration causes
undefined behaviour. The procedure below serialises the upgrade by scaling to zero before
switching the image.

**Pre-upgrade checklist:**

- Read the upstream [Keycloak migration guide](https://www.keycloak.org/docs/latest/upgrading/) for
  the target version and note any fields that have been removed or renamed in the Admin REST API —
  these may require updates to operator CRD reconcilers.
- Confirm at least one backup is available and recent (see section 8).
- Ensure `PodDisruptionBudget` is in place (it is, by default). If you have auto-scaling policies
  that might start new pods, disable them temporarily.

**Steps:**

1. Take a database backup before proceeding:
   ```sh
  kubectl apply -f examples/backup-example.yaml
  kubectl wait backups.postgresql.cnpg.io/<name> \
     --for=jsonpath='{.status.phase}'=completed \
     --timeout=300s -n <namespace>
   ```
2. Scale Keycloak to 0 to prevent any writes to the database during migration:
   ```sh
   kubectl scale deployment keycloak --replicas=0 -n <namespace>
   kubectl rollout status deployment/keycloak -n <namespace>
   ```
3. Update the image tag in `manifests/keycloak/keycloak-deployment.yaml` and
   `component-constructor.yaml`.
4. Apply the updated manifest:
   ```sh
   kubectl apply -f manifests/keycloak/keycloak-deployment.yaml -n <namespace>
   ```
5. Scale back to 1 replica so Keycloak performs the DB migration with a single writer:
   ```sh
   kubectl scale deployment keycloak --replicas=1 -n <namespace>
   ```
6. Monitor startup logs — Keycloak logs each migration step and exits non-zero if migration fails:
   ```sh
   kubectl logs -f deploy/keycloak -n <namespace>
   ```
   Wait until the log line `Keycloak X.Y.Z on JVM started` appears before scaling further.
7. Verify health:
   ```sh
   kubectl exec -n <namespace> deploy/keycloak -- \
     curl -s http://localhost:9000/health/ready
   ```
8. Perform a smoke test: open the admin console, issue a token, confirm login works.
9. Scale to the production replica count:
   ```sh
   kubectl scale deployment keycloak --replicas=<desired> -n <namespace>
   ```

**Rollback:**

If step 6 shows migration errors:

1. Scale Keycloak back to 0:
   ```sh
   kubectl scale deployment keycloak --replicas=0 -n <namespace>
   ```
2. Restore the database from the pre-upgrade backup (see section 8.4).
3. Revert the image tag in `manifests/keycloak/keycloak-deployment.yaml` and re-apply.
4. Scale back up to 1, confirm health, then scale to the desired replica count.

---

## 3. PostgreSQL Minor Version Upgrade (CloudNativePG)

CloudNativePG handles minor PostgreSQL upgrades as in-place rolling restarts.

**Steps:**

1. Update the `imageName` in `manifests/postgres/cluster.yaml` to the new minor version tag.
2. Apply:
   ```sh
   kubectl apply -f manifests/postgres/cluster.yaml
   ```
3. CloudNativePG will perform a rolling restart of the cluster instances.
4. Verify the cluster is healthy:
   ```sh
   kubectl get cluster keycloak-db
   kubectl describe cluster keycloak-db
   ```

---

## 4. PostgreSQL Major Version Upgrade (CloudNativePG)

Major PostgreSQL upgrades (e.g., 17 → 18) require a cluster clone + switchover procedure via CloudNativePG's `pg_upgrade` support.

**Steps:**

1. Ensure a recent backup exists:
   ```sh
  kubectl get backups.postgresql.cnpg.io -l cnpg.io/cluster=keycloak-db
   ```
2. Create a new cluster manifest (`manifests/postgres/cluster-new.yaml`) targeting the new major version with `bootstrap.pg_upgrade` pointing to the existing cluster.
3. Apply the new cluster — CloudNativePG will handle `pg_upgrade` in-place:
   ```sh
   kubectl apply -f manifests/postgres/cluster-new.yaml
   ```
4. Monitor the upgrade job:
   ```sh
   kubectl logs -l cnpg.io/cluster=keycloak-db-new -f
   ```
5. Once complete and healthy, update Keycloak's `KC_DB_URL_HOST` to point to the new cluster's service and restart Keycloak.
6. Decommission the old cluster only after verifying full functionality.

Reference: [CloudNativePG Major Upgrades](https://cloudnative-pg.io/documentation/current/postgresql_upgrade/)

---

## 5. Taking a Manual Database Backup

CloudNativePG supports on-demand backups. To trigger one:

```sh
kubectl apply -f - <<EOF
apiVersion: postgresql.cnpg.io/v1
kind: Backup
metadata:
  name: keycloak-db-manual-$(date +%Y%m%d%H%M)
spec:
  cluster:
    name: keycloak-db
  method: plugin
  pluginConfiguration:
    name: barman-cloud.cloudnative-pg.io
EOF
```

> Note: Requires the Barman Cloud Plugin with an `ObjectStore` and `spec.plugins` enabled on the CNPG `Cluster`. For pre-upgrade safety without an object store, use a volume snapshot if your storage class supports it.

---

## 6. CloudNativePG Operator Upgrade

1. Update the operator image tag in `component-constructor.yaml`.
2. Re-apply the Helm chart or operator manifests per the CloudNativePG upgrade guide.
3. The operator upgrade does not restart database clusters unless a manifest change triggers it.

Reference: [CloudNativePG Operator Upgrade](https://cloudnative-pg.io/documentation/current/installation_upgrade/)

---

## 7. Verifying Observability After Upgrades

After any upgrade, confirm the full observability stack is operational:

```sh
# Check Keycloak metrics endpoint
kubectl exec -it deploy/keycloak -- curl -s http://localhost:9000/metrics | head -20

# Check ServiceMonitor is being scraped (requires Prometheus access)
kubectl get servicemonitor keycloak

# Check PodMonitor for database
kubectl get podmonitor keycloak-db-metrics

# Check PrometheusRules loaded
kubectl get prometheusrule keycloak
```

If OTEL tracing is enabled (`KC_TRACING_ENABLED=true`), verify trace data reaches the collector:

```sh
kubectl logs -l app=opentelemetry-collector -n observability | grep keycloak
```

---

## 8. Backup and Restore (CNPG-native)

Backup and restore are executed with CloudNativePG native resources. This keeps
the bundle lightweight and avoids a custom backup/restore orchestration layer.

S3 credentials must be pre-created as a Kubernetes Secret:

```sh
kubectl create secret generic keycloak-backup-s3 -n <namespace> \
  --from-literal=ACCESS_KEY_ID=<id> \
  --from-literal=SECRET_ACCESS_KEY=<secret>
```

Define an `ObjectStore` and enable plugin WAL archiving on the source cluster:

```yaml
apiVersion: barmancloud.cnpg.io/v1
kind: ObjectStore
metadata:
  name: keycloak-backup-store
  namespace: <namespace>
spec:
  configuration:
    destinationPath: s3://<bucket>/<prefix>
    endpointURL: https://<s3-host>
    s3Credentials:
      accessKeyId:
        name: keycloak-backup-s3
        key: ACCESS_KEY_ID
      secretAccessKey:
        name: keycloak-backup-s3
        key: SECRET_ACCESS_KEY
    wal:
      compression: gzip
    data:
      compression: gzip
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: keycloak-db
  namespace: <namespace>
spec:
  plugins:
  - name: barman-cloud.cloudnative-pg.io
    isWALArchiver: true
    parameters:
      barmanObjectName: keycloak-backup-store
```

### 8.1 On-demand backup

Apply a CNPG `Backup` to trigger an immediate backup:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Backup
metadata:
  name: keycloak-db-manual
  namespace: <namespace>
spec:
  cluster:
    name: keycloak-db
  method: plugin
  pluginConfiguration:
    name: barman-cloud.cloudnative-pg.io
```

```sh
kubectl apply -f <above>.yaml
kubectl wait backups.postgresql.cnpg.io/keycloak-db-manual \
  --for=jsonpath='{.status.phase}'=completed \
  --timeout=300s -n <namespace>
kubectl get backups.postgresql.cnpg.io keycloak-db-manual -n <namespace>
```

### 8.2 Scheduled backup

Apply a CNPG `ScheduledBackup` with cron syntax:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: keycloak-db-nightly
  namespace: <namespace>
spec:
  cluster:
    name: keycloak-db
  schedule: "0 2 * * *"    # daily at 02:00 UTC
  method: plugin
  pluginConfiguration:
    name: barman-cloud.cloudnative-pg.io
```

### 8.3 Checking backup status

```sh
# List all CNPG backup resources and phase
kubectl get backups.postgresql.cnpg.io -n <namespace>

# Describe a specific backup
kubectl describe backups.postgresql.cnpg.io <name> -n <namespace>
```

Expected output when complete (example):

```
NAME                  PHASE       READY   AGE
keycloak-db-manual    completed   true    4m
```

### 8.4 Restoring from a backup

Restore is performed by creating a new CNPG recovery cluster using `bootstrap.recovery`
and then cutting Keycloak over to the restored cluster.

**Procedure:**

1. Apply a recovery Cluster manifest pointing at the backup location:
   ```yaml
   apiVersion: postgresql.cnpg.io/v1
   kind: Cluster
   metadata:
     name: keycloak-db-restore
     namespace: <namespace>
   spec:
     instances: 1
     bootstrap:
       recovery:
         source: keycloak-backup
     externalClusters:
       - name: keycloak-backup
         plugin:
           name: barman-cloud.cloudnative-pg.io
           parameters:
             barmanObjectName: keycloak-backup-store
             serverName: keycloak-db
     storage:
       size: 5Gi
   ```
   ```sh
  kubectl apply -f examples/restore-cluster-example.yaml
   ```
2. Wait for the recovery cluster to become ready:
   ```sh
   kubectl wait pod -n <namespace> \
    -l cnpg.io/cluster=keycloak-db-restore,cnpg.io/instanceRole=primary \
     --for=condition=Ready --timeout=600s
   ```
3. Cut Keycloak over to the restored cluster and restart:
   ```sh
  kubectl set env deployment/keycloak -n <namespace> KC_DB_URL_HOST=keycloak-db-restore-rw
  kubectl rollout restart deployment/keycloak -n <namespace>
  kubectl rollout status deployment/keycloak -n <namespace>
   ```
4. Verify data integrity: log in to the Keycloak admin console and confirm realms,
   clients, and users that existed before the backup are present.

Quick validation command:

```sh
./scripts/tests/test-backup-restore.sh --live \
  --namespace <namespace> \
  --cluster-name keycloak-db \
  --restore-cluster-name keycloak-db-restore \
  --destination-path s3://<bucket>/<prefix> \
  --credentials-secret keycloak-backup-s3 \
  --endpoint-url https://<s3-host>
```

---

## 9. CRD Compatibility and Field Immutability

### 9.1 Immutable fields

The following fields serve as Keycloak-side identifiers. Changing them after the resource is
created would cause the operator to attempt an operation that Keycloak does not support (e.g.
renaming a realm, changing a client's protocol type). The API server does not enforce
immutability via `x-kubernetes-validations`; enforcement is handled in the reconciler, which
logs an error and sets the resource to `Ready=false` if a change is detected.

| Resource | Immutable field(s) | Reason |
|---|---|---|
| `Realm` | `spec.realmName` | Realm ID in Keycloak; renaming requires full delete/recreate |
| `Client` | `spec.clientId`, `spec.realmRef` | Client ID is the Keycloak identifier; `realmRef` change would orphan the object |
| `ClientScope` | `spec.name`, `spec.realmRef` | Scope name is the Keycloak identifier |
| `Group` | `spec.name`, `spec.realmRef` | Group name is the Keycloak identifier |
| `User` | `spec.username`, `spec.realmRef` | Username is the Keycloak identifier |
| `IdentityProvider` | `spec.alias`, `spec.type`, `spec.realmRef` | Alias is the IdP identifier; `type` determines the protocol and cannot be switched in place |
| `AuthFlow` | `spec.alias`, `spec.realmRef` | Flow alias is the Keycloak identifier |

**To change an immutable field:** delete the CR and recreate it with the new value. The
operator will delete the corresponding resource in Keycloak (via the finalizer) and recreate
it. Take note of any dependent resources (e.g. clients that reference a realm by `realmRef`)
— they will also require updating.

### 9.2 New optional fields

New optional spec fields are introduced without a CRD version bump provided they carry a
default value or are omitted from the Keycloak API call when absent. Existing CRs continue
to reconcile without modification. No migration steps are required for consumers when an
optional field is added.

### 9.3 Breaking changes and CRD versioning

A change is considered breaking if it:
- removes or renames an existing required field, or
- changes the type or validation constraints of an existing field in a way that would
  cause currently-valid CRs to fail schema validation.

Breaking changes require a new CRD version (e.g. `v1alpha2`). Both the old and new versions
shall be served simultaneously with a conversion webhook or documented manual migration path.
The old version shall remain `storage: true` until all instances have been migrated.

The current version across all CRDs is `v1alpha1`. No breaking changes have been made since
initial delivery.
