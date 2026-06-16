# Operator Migration Guide

This document describes how to upgrade the Keycloak Operator across minor versions,
including CRD migration steps and field-level changes to watch for.

---

## General procedure

1. **Backup CRs** before upgrading:
   ```bash
   kubectl get realm,client,user,group,clientscope,authflow,identityprovider \
     -A -o yaml > keycloak-crds-backup.yaml
   ```
2. **Apply updated CRDs** — CRDs must be upgraded before the operator:
   ```bash
   kubectl apply -f charts/keycloak-operator/crds/
   ```
3. **Upgrade the operator** via Helm:
   ```bash
   helm upgrade keycloak-operator charts/keycloak-operator \
     --namespace <keycloak-instance-namespace> \
     --reuse-values
   ```
   The operator watches only its own namespace (`WATCH_NAMESPACE` is set from the pod
   namespace). Upgrade one release per Keycloak instance namespace.
4. **Verify** existing resources are still healthy:
   ```bash
   kubectl get realm -A
   kubectl get realm -A -o yaml | grep observedGeneration
   ```

---

## v0.1.x → v0.2.x

### New CRD fields

| Resource | Field | Description |
|----------|-------|-------------|
| `Realm` | `status.observedGeneration` | Set to `metadata.generation` after each reconcile. Used by GitOps tooling to detect pending spec changes. |
| `Realm` | `status.activeJobName` | Name of the most recently spawned `config-cli` Job. Useful for debugging stuck syncs. |
| All CRs | `status.observedGeneration` | Added to `CommonStatus` — present on all managed resource types. |

### Removed/changed RBAC

The operator Role was tightened in v0.2.x:
- `delete` and `patch` removed from the `secrets` rule.
- `update` and `patch` removed from the `jobs` rule.

If you manage the Role manually (i.e., `serviceAccount.create: false`), update your
custom Role to remove those verbs before upgrading.

### New Helm values

| Key | Default | Description |
|-----|---------|-------------|
| `configCLI.serviceAccountName` | `keycloak-config-cli` | ServiceAccount assigned to config-cli Job pods. |

The chart now creates a dedicated `keycloak-config-cli` ServiceAccount. If your
namespace policies restrict ServiceAccount creation, pre-create this account before
upgrading:
```bash
kubectl create serviceaccount keycloak-config-cli -n <keycloak-instance-namespace>
```

### Validation markers on CRDs

`realmRef`, `realmName`, `clientId`, `alias`, and `name` fields now carry
`minLength`, `maxLength`, and pattern validation. Existing resources that already
violate these constraints will not be rejected at upgrade time (Kubernetes only
validates on create/update), but any subsequent update to such a resource will fail
admission. Check for empty or non-conforming values before upgrading:

```bash
kubectl get realm -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}: {.spec.realmName}{"\n"}{end}'
```

---

## Post-upgrade checklist

- [ ] All Realm CRs show `status.ready: true`.
- [ ] `status.observedGeneration` equals `metadata.generation` on each CR.
- [ ] CI pipeline passes (`make check-versions` for OCM/chart version parity).
- [ ] `kubectl get realm -A -o yaml` shows no deprecated field warnings in events.
