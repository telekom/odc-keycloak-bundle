# Acceptance Test Specification
## Software-defined Keycloak Solution — OCM Package

| Attribute | Value |
|-----------|-------|
| **Document ID** | ATS-KEYCLOAK-001 |
| **References** | `docs/REQUIREMENTS.md` |
| **Project** | Contract Item 2.1 — Software-defined Keycloak solution |

---

## Conventions

- Each test case carries a unique **ATC-XX-Y** identifier, where **XX** is the
  requirement number and **Y** is the sequential test number within that requirement.
- Test steps use `kubectl` and `ocm` CLI commands that can be executed verbatim against
  a provisioned test cluster.
- Placeholder values are written in `<angle-brackets>`.

---

## Summary

| Test Case | Title | Requirement | Phase |
|-----------|-------|-------------|-------|
| ATC-01-1 | Archive build, sign, and push | REQ-01 | PoC |
| ATC-01-2 | Archive content completeness | REQ-01 | PoC |
| ATC-01-3 | Image digest pinning | REQ-01 | PoC |
| ATC-01-4 | Air-gapped deployment | REQ-01 | PoC |
| ATC-01-5 | Version consistency | REQ-01 | PoC |
| ATC-01-6 | Open-source provenance | REQ-01 | PoC |
| ATC-02-1 | Parallel instance independence | REQ-02 | PoC |
| ATC-02-2 | Dedicated namespace per instance | REQ-02 | PoC |
| ATC-02-3 | Namespace-scoped RBAC | REQ-02 | PoC |
| ATC-02-4 | Instance deletion isolation | REQ-02 | PoC |
| ATC-02-5 | WATCH_NAMESPACE enforcement | REQ-02 | PoC |
| ATC-03-1 | Automatic database provisioning | REQ-03 | PoC |
| ATC-03-2 | Init container readiness gate | REQ-03 | PoC |
| ATC-03-3 | Credentials via secretKeyRef only | REQ-03 | PoC |
| ATC-03-4 | DB replica and storage configurability | REQ-03 | PoC |
| ATC-03-5 | External database support | REQ-03 | PoC |
| ATC-03-6 | Automatic database failover | REQ-03 | PoC |
| ATC-04-1 | Realm CRD lifecycle | REQ-04 | Alpha |
| ATC-04-2 | Client CRD lifecycle | REQ-04 | Alpha |
| ATC-04-3 | ClientScope CRD lifecycle | REQ-04 | Alpha |
| ATC-04-4 | Group CRD lifecycle | REQ-04 | Alpha |
| ATC-04-5 | User CRD lifecycle with group membership | REQ-04 | Alpha |
| ATC-04-6 | Coexistence of all five CRD types | REQ-04 | Alpha |
| ATC-04-7 | CRD schema field descriptions | REQ-04 | Alpha |
| ATC-04-8 | Reconciliation latency | REQ-04 | Alpha |
| ATC-05-1 | IdentityProvider CRD lifecycle | REQ-05 | Final |
| ATC-05-2 | AuthFlow CRD lifecycle | REQ-05 | Final |
| ATC-05-3 | CR deletion removes IdP and flow from Keycloak | REQ-05 | Final |
| ATC-05-4 | IdP credential references via secretKeyRef | REQ-05 | Final |
| ATC-05-5 | Extended CRDs present in OCM archive | REQ-05 | Final |
| ATC-05-6 | Extended CRDs documented in USAGE.md | REQ-05 | Final |
| ATC-06-1 | READY column in kubectl output | REQ-06 | Alpha |
| ATC-06-2 | Ready=False when Keycloak unreachable | REQ-06 | Alpha |
| ATC-06-3 | Automatic recovery to Ready=True | REQ-06 | Alpha |
| ATC-06-4 | lastSyncTime updated every cycle | REQ-06 | Alpha |
| ATC-06-5 | keycloakId populated after first sync | REQ-06 | Alpha |
| ATC-06-6 | Error message contains HTTP detail | REQ-06 | Alpha |
| ATC-07-1 | No credentials in repository (Gitleaks) | REQ-07 | PoC |
| ATC-07-2 | Admin password only accessible via Secret | REQ-07 | PoC |
| ATC-07-3 | Database credentials injected via secretKeyRef | REQ-07 | PoC |
| ATC-07-4 | Client credential Secrets in target namespace | REQ-07 | PoC |
| ATC-07-5 | User password accepted only as Secret reference | REQ-07 | PoC |
| ATC-07-6 | No plaintext credential fields in CRD schemas | REQ-07 | PoC |
| ATC-08-1 | Three-replica Keycloak cluster formation | REQ-08 | Alpha |
| ATC-08-2 | Session sharing across replicas | REQ-08 | Alpha |
| ATC-08-3 | PodDisruptionBudget prevents full eviction | REQ-08 | Alpha |
| ATC-08-4 | Single operator leader at any time | REQ-08 | Alpha |
| ATC-08-5 | Leader lease re-acquisition after pod loss | REQ-08 | Alpha |
| ATC-08-6 | Replica count configurable via CR | REQ-08 | Alpha |
| ATC-09-1 | Single CR triggers full deployment | REQ-09 | Alpha |
| ATC-09-2 | Pods ready without manual steps | REQ-09 | Alpha |
| ATC-09-3 | CR deletion removes all resources | REQ-09 | Alpha |
| ATC-09-4 | Air-gapped end-to-end deployment | REQ-09 | Alpha |
| ATC-09-5 | RGD present in OCM archive | REQ-09 | Alpha |
| ATC-09-6 | Two simultaneous instances via KRO | REQ-09 | Alpha |
| ATC-10-1 | Structured JSON log output | REQ-10 | Final |
| ATC-10-2 | OTEL tracing spans visible in backend | REQ-10 | Final |
| ATC-10-3 | Prometheus metrics at /metrics | REQ-10 | Final |
| ATC-10-4 | ServiceMonitor picked up automatically | REQ-10 | Final |
| ATC-10-5 | Liveness and readiness probes gate traffic | REQ-10 | Final |
| ATC-10-6 | PrometheusRule alerts fire under fault conditions | REQ-10 | Final |
| ATC-10-7 | OBSERVABILITY.md exists and is complete | REQ-10 | Final |
| ATC-11-1 | Backup triggered via Kubernetes resource | REQ-11 | Final |
| ATC-11-2 | Backup storage location is configurable | REQ-11 | Final |
| ATC-11-3 | Restore returns Keycloak to backed-up state | REQ-11 | Final |
| ATC-11-4 | Backup procedure documented in UPGRADE.md | REQ-11 | Final |
| ATC-11-5 | Backup status reported in CR status | REQ-11 | Final |
| ATC-12-1 | Zero dropped requests during minor version update | REQ-12 | Final |
| ATC-12-2 | CNPG minor upgrade without connectivity loss | REQ-12 | Final |
| ATC-12-3 | Major version upgrade runbook is executable | REQ-12 | Final |
| ATC-12-4 | Major version upgrade runbook in UPGRADE.md | REQ-12 | Final |
| ATC-12-5 | CRD compatibility guarantees documented | REQ-12 | Final |
| ATC-13-1 | README links are complete and accurate | REQ-13 | PoC–Final |
| ATC-13-2 | Deploy from documentation alone | REQ-13 | PoC–Final |
| ATC-13-3 | kubectl explain returns descriptions | REQ-13 | PoC–Final |
| ATC-13-4 | USAGE.md covers all delivered CRD types | REQ-13 | PoC–Final |
| ATC-13-5 | UPGRADE.md covers all upgrade scenarios | REQ-13 | PoC–Final |
| ATC-13-6 | Final documentation review | REQ-13 | PoC–Final |
| ATC-13-7 | OBSERVABILITY.md exists | REQ-13 | PoC–Final |
| ATC-14-1 | ShellCheck violation blocks PR | REQ-14 | Final |
| ATC-14-2 | Gitleaks violation blocks PR | REQ-14 | Final |
| ATC-14-3 | CVE scan blocks PR on critical finding | REQ-14 | Final |
| ATC-14-4 | YAML linting failure blocks PR | REQ-14 | Final |
| ATC-14-5 | CONTRIBUTING.md exists and is complete | REQ-14 | Final |
| ATC-14-6 | Container images run as non-root | REQ-14 | Final |
| ATC-14-7 | Hardening checklist exists | REQ-14 | Final |

---

## REQ-01 · OCM Component Package — PoC

### ATC-01-1 · Archive build, sign, and push

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-01, AC 1 |
| **Phase** | PoC |

**Preconditions:**
- OCM CLI installed and on `$PATH`
- CI pipeline secrets (signing key, registry credentials) are configured
- Repository is at a clean, tagged commit

**Steps:**
1. Trigger the CI pipeline on the target branch (or run locally):
   ```
   ./scripts/ocm/ocm-create.sh
   ./scripts/ocm/ocm-sign.sh
   ./scripts/ocm/ocm-validate.sh
   ```
2. Observe CI stage output for errors.
3. Verify the artifact `keycloak-bundle-ctf.tar.gz` is produced.
4. Push the archive to a test OCI registry:
   ```
   ocm transfer ctf keycloak-bundle-ctf.tar.gz <registry>/keycloak-bundle
   ```

**Expected Result:** All three scripts complete without error. The archive is present as
a CI artifact and is accepted by the OCI registry without errors.

**Pass Criterion:** Exit codes of all three scripts are 0; the archive appears in the
registry under the correct component name and version.

---

### ATC-01-2 · Archive content completeness

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-01, AC 2 |
| **Phase** | PoC |

**Preconditions:** Archive `keycloak-bundle-ctf.tar.gz` has been built (see ATC-01-1).

**Steps:**
1. Inspect the component descriptor:
   ```
    ocm get componentversion --repo ./keycloak-bundle-ctf.tar.gz \
       opendefense.cloud/keycloak-bundle
   ```
2. List all resources:
   ```
    ocm get resources --repo ./keycloak-bundle-ctf.tar.gz \
       opendefense.cloud/keycloak-bundle
   ```
3. Verify the following resources are present in the output:
   - `keycloak-image` (ociImage)
   - `postgres-image` (ociImage)
   - `cnpg-operator-image` (ociImage)
   - `Realm-crd`, `Client-crd`, `User-crd`,
     `Group-crd`, `ClientScope-crd` (kubernetes)
   - `keycloak-instance-rgd` (blueprint)
   - `keycloak-operator` (helmChart)
   - `manifests` (directory)

**Expected Result:** All listed resources appear exactly once in the resource listing with
non-empty access information.

**Pass Criterion:** Every resource listed in step 3 is present; no resource has an empty
or error access descriptor.

---

### ATC-01-3 · Image digest pinning

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-01, AC 3 |
| **Phase** | PoC |

**Preconditions:** Archive has been built (see ATC-01-1).

**Steps:**
1. Inspect the access descriptor of each OCI image resource:
   ```
    ocm get resources --repo ./keycloak-bundle-ctf.tar.gz \
       opendefense.cloud/keycloak-bundle -o yaml | grep -E 'imageReference|digest'
   ```
2. Check `component-constructor.yaml` for digest or immutable tag references.

**Expected Result:** Every image reference in the component descriptor includes either a
`sha256:` digest or an immutable tag (i.e. no `latest` or floating tags).

**Pass Criterion:** No image reference contains `latest` or any tag not tied to a
specific version; at least one of digest or immutable tag is present per image.

---

### ATC-01-4 · Air-gapped deployment

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-01, AC 4 |
| **Phase** | PoC |

**Preconditions:**
- A private OCI registry is available with no outbound internet access.
- The archive `keycloak-bundle-ctf.tar.gz` has been transferred to this registry.
- A Kubernetes cluster is available that can only reach the private registry.

**Steps:**
1. Transfer the archive to the private registry:
   ```
   ocm transfer ctf keycloak-bundle-ctf.tar.gz <private-registry>/keycloak-bundle
   ```
2. Apply the `KeycloakInstance` CR referencing images from the private registry.
3. Monitor pod startup; verify no image pull requests reach the public internet
   (confirm via network policy audit log or DNS query log).
4. Confirm all pods reach `Running` state.

**Expected Result:** All Keycloak and operator pods start successfully using images from
the private registry exclusively. No DNS queries or TCP connections to public image
registries occur.

**Pass Criterion:** All pods reach `Running`; network logs show zero connections to
`quay.io`, `ghcr.io`, or `docker.io` during the deployment.

---

### ATC-01-5 · Version consistency

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-01, AC 5 |
| **Phase** | PoC |

**Preconditions:** Archive has been built from a tagged commit.

**Steps:**
1. Read the component version from the descriptor:
   ```
    ocm get componentversion --repo ./keycloak-bundle-ctf.tar.gz \
       opendefense.cloud/keycloak-bundle -o yaml | grep 'version:'
   ```
2. Compare against the tag in `component-constructor.yaml` and the CI artifact name.
3. Check the `keycloak-image` resource version matches the Keycloak image tag in the
   Keycloak Deployment manifest.

**Expected Result:** The component version, image tag, and CI artifact name all reference
the same version string.

**Pass Criterion:** All three version strings match; no resource reports a different
version than the component-level version.

---

### ATC-01-6 · Open-source provenance

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-01, AC 6 |
| **Phase** | PoC |

**Preconditions:** Component descriptor is available.

**Steps:**
1. List all resources with their `sourceRef` or `labels`:
   ```
    ocm get resources --repo ./keycloak-bundle-ctf.tar.gz \
       opendefense.cloud/keycloak-bundle -o yaml | grep -A3 'labels'
   ```
2. Verify each resource has an upstream source URL recorded.
3. Confirm each upstream URL points to a public, open-source repository.

**Expected Result:** Every bundled resource carries a label or `sourceRef` pointing to a
publicly accessible open-source repository. No proprietary or closed-source component is
included.

**Pass Criterion:** All resources have recorded upstream references; manual check of each
URL confirms an OSS licence.

---

## REQ-02 · Multi-Instance Namespace Isolation — PoC

### ATC-02-1 · Parallel instance independence

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-02, AC 1 |
| **Phase** | PoC |

**Preconditions:** KRO and CNPG operators are installed on the cluster.

**Steps:**
1. Apply two `KeycloakInstance` CRs simultaneously:
   ```
   kubectl apply -f examples/instance-alpha.yaml
   kubectl apply -f examples/instance-beta.yaml
   ```
2. Wait until both instances are fully running:
   ```
   kubectl get pods -n keycloak-alpha
   kubectl get pods -n keycloak-beta
   ```
3. Create a `Client` CR in `keycloak-alpha` and verify it is absent in
   `keycloak-beta`:
   ```
   kubectl apply -n keycloak-alpha -f examples/client-example.yaml
   kubectl get Clients -n keycloak-beta
   ```
4. Delete the `KeycloakInstance` CR for `alpha` and confirm `beta` remains unaffected:
   ```
   kubectl delete keycloakinstance alpha
   kubectl get pods -n keycloak-beta
   ```

**Expected Result:** Resources created in instance `alpha` do not appear in instance
`beta`. Deletion of `alpha` causes no change to `beta`'s pod count or status.

**Pass Criterion:** `kubectl get Clients -n keycloak-beta` returns no resources;
all `keycloak-beta` pods remain in `Running` state after `alpha` is deleted.

---

### ATC-02-2 · Dedicated namespace per instance

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-02, AC 2 |
| **Phase** | PoC |

**Preconditions:** One `KeycloakInstance` CR named `test` has been applied.

**Steps:**
1. Verify the namespace exists with the expected name:
   ```
   kubectl get namespace keycloak-test
   ```
2. Confirm all instance resources are in this namespace:
   ```
   kubectl get all -n keycloak-test
   ```
3. Confirm no Keycloak resources appear in any other namespace:
   ```
   kubectl get Clients --all-namespaces
   ```

**Expected Result:** A namespace `keycloak-test` exists. All pods, services, and
operator resources for this instance are within it.

**Pass Criterion:** `kubectl get namespace keycloak-test` returns the namespace with
`Active` status; no Keycloak workloads appear outside it.

---

### ATC-02-3 · Namespace-scoped RBAC

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-02, AC 3 |
| **Phase** | PoC |

**Preconditions:** Instance `test` is deployed.

**Steps:**
1. Verify no ClusterRole or ClusterRoleBinding exists for the operator:
   ```
   kubectl get clusterrolebinding | grep keycloak-operator
   kubectl get clusterrole | grep keycloak-operator
   ```
2. Inspect the operator Role to confirm it is namespace-scoped:
   ```
   kubectl get role -n keycloak-test
   kubectl describe role keycloak-operator -n keycloak-test
   ```
3. Attempt to list secrets in a foreign namespace using the operator ServiceAccount:
   ```
   kubectl auth can-i list secrets -n default \
     --as=system:serviceaccount:keycloak-test:keycloak-operator
   ```

**Expected Result:** No ClusterRole or ClusterRoleBinding for the operator exists. The
`auth can-i` check returns `no`.

**Pass Criterion:** Both `grep` commands return empty output; `auth can-i` prints `no`.

---

### ATC-02-4 · Instance deletion isolation

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-02, AC 4 |
| **Phase** | PoC |

**Preconditions:** Two instances `alpha` and `beta` are running.

**Steps:**
1. Record the pod list of instance `beta`:
   ```
   kubectl get pods -n keycloak-beta -o name > /tmp/beta-pods-before.txt
   ```
2. Delete instance `alpha`:
   ```
   kubectl delete keycloakinstance alpha
   ```
3. Wait 60 seconds, then record the pod list of `beta` again:
   ```
   kubectl get pods -n keycloak-beta -o name > /tmp/beta-pods-after.txt
   ```
4. Compare the two lists:
   ```
   diff /tmp/beta-pods-before.txt /tmp/beta-pods-after.txt
   ```

**Expected Result:** The diff is empty; instance `beta` is completely unaffected.

**Pass Criterion:** `diff` exits with code 0 (no differences).

---

### ATC-02-5 · WATCH_NAMESPACE enforcement

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-02, AC 5 |
| **Phase** | PoC |

**Preconditions:** Instance `alpha` is running. Instance `beta` also exists.

**Steps:**
1. Read `WATCH_NAMESPACE` from the operator pod of instance `alpha`:
   ```
   kubectl exec -n keycloak-alpha deploy/keycloak-operator \
     -- env | grep WATCH_NAMESPACE
   ```
2. Apply a `Client` CR directly in namespace `keycloak-beta`:
   ```
   kubectl apply -n keycloak-beta -f examples/client-example.yaml
   ```
3. Wait two reconciliation cycles (≥ 20 s), then check the operator `alpha` logs:
   ```
   kubectl logs -n keycloak-alpha deploy/keycloak-operator | grep keycloak-beta
   ```

**Expected Result:** `WATCH_NAMESPACE` is `keycloak-alpha`. The `alpha` operator logs
contain no references to `keycloak-beta` resources.

**Pass Criterion:** `WATCH_NAMESPACE=keycloak-alpha`; log grep returns empty output.

---

## REQ-03 · Database Integration — PoC

### ATC-03-1 · Automatic database provisioning

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-03, AC 1 |
| **Phase** | PoC |

**Preconditions:** CNPG operator is installed cluster-wide. No manual database has been
created.

**Steps:**
1. Apply a `KeycloakInstance` CR with default database settings.
2. Watch the CNPG cluster resource appear:
   ```
   kubectl get cluster -n keycloak-test --watch
   ```
3. Confirm the cluster reaches `Healthy` phase without any manual intervention.
4. Confirm the Secret `keycloak-db-app` is auto-created by CNPG:
   ```
   kubectl get secret keycloak-db-app -n keycloak-test
   ```

**Expected Result:** A `Cluster` resource named `keycloak-db` appears and transitions to
`Healthy` without manual configuration. The credentials Secret exists.

**Pass Criterion:** `kubectl get cluster keycloak-db -n keycloak-test` shows
`STATUS: Cluster in healthy state`; Secret `keycloak-db-app` exists.

---

### ATC-03-2 · Init container readiness gate

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-03, AC 2 |
| **Phase** | PoC |

**Preconditions:** A `KeycloakInstance` is being deployed. The CNPG cluster has not yet
become ready.

**Steps:**
1. Immediately after applying the `KeycloakInstance` CR, check the Keycloak pod:
   ```
   kubectl get pods -n keycloak-test -w
   ```
2. Observe the pod status while the database is initialising.
3. Check the init container logs:
   ```
   kubectl logs -n keycloak-test <keycloak-pod> -c wait-for-db
   ```

**Expected Result:** The Keycloak pod remains in `Init:0/1` status until the CNPG cluster
service `keycloak-db-rw` becomes reachable. The init container logs show repeated
"waiting for db" messages followed by a single exit.

**Pass Criterion:** Pod phase is `Init:0/1` while database is not ready; pod transitions
to `Running` only after `keycloak-db-rw:5432` is reachable.

---

### ATC-03-3 · Credentials via secretKeyRef only

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-03, AC 3 |
| **Phase** | PoC |

**Preconditions:** Instance `test` is deployed.

**Steps:**
1. Inspect the Keycloak Deployment environment variables:
   ```
   kubectl get deployment keycloak -n keycloak-test -o yaml | \
     grep -A5 -E 'KC_DB_USERNAME|KC_DB_PASSWORD'
   ```
2. Confirm both variables use `secretKeyRef` and not `value`.
3. Check that no password literal appears anywhere in the Deployment spec:
   ```
   kubectl get deployment keycloak -n keycloak-test -o yaml | grep -i password | grep -v secretKeyRef
   ```

**Expected Result:** Both database credential environment variables use `secretKeyRef`.
The second grep returns no output.

**Pass Criterion:** Both `KC_DB_USERNAME` and `KC_DB_PASSWORD` reference
`keycloak-db-app`; the plaintext-password grep is empty.

---

### ATC-03-4 · DB replica and storage configurability

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-03, AC 4 |
| **Phase** | PoC |

**Preconditions:** An instance is deployed with `spec.dbInstances: 1`.

**Steps:**
1. Patch the `KeycloakInstance` CR to increase database replicas:
   ```
   kubectl patch keycloakinstance test \
     --type=merge -p '{"spec":{"dbInstances":3,"dbStorageSize":"10Gi"}}'
   ```
2. Observe the CNPG Cluster CR:
   ```
   kubectl get cluster keycloak-db -n keycloak-test -o yaml | grep -E 'instances|storage'
   ```

**Expected Result:** The CNPG `Cluster` CR reflects `instances: 3` and a storage size of
`10Gi` after the patch propagates.

**Pass Criterion:** `instances: 3` and `size: 10Gi` appear in the CNPG Cluster spec
within two reconciliation cycles.

---

### ATC-03-5 · External database support

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-03, AC 5 |
| **Phase** | PoC |

**Preconditions:** An external PostgreSQL instance is accessible from the cluster with
known connection parameters.

**Steps:**
1. Create a `KeycloakInstance` CR with external DB parameters:
   ```yaml
   spec:
     externalDatabase:
       host: <postgres-host>
       port: 5432
       database: keycloak
       credentialsSecret: my-external-db-secret
   ```
2. Apply the CR and observe that no CNPG `Cluster` resource is created:
   ```
   kubectl get cluster -n keycloak-test
   ```
3. Confirm Keycloak connects successfully and reaches `Running`:
   ```
   kubectl get pods -n keycloak-test
   ```

**Expected Result:** No CNPG cluster is provisioned. Keycloak connects to the external
database and reaches `Running` state.

**Pass Criterion:** `kubectl get cluster -n keycloak-test` returns `No resources found`;
Keycloak pod reaches `Running`.

---

### ATC-03-6 · Automatic database failover

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-03, AC 6 |
| **Phase** | PoC |

**Preconditions:** Instance `test` is deployed with `spec.dbInstances: 2`.

**Steps:**
1. Identify the CNPG primary pod:
   ```
   kubectl get pods -n keycloak-test -l cnpg.io/instanceRole=primary
   ```
2. Delete the primary pod:
   ```
   kubectl delete pod <cnpg-primary-pod> -n keycloak-test
   ```
3. Observe CNPG promoting a standby to primary:
   ```
   kubectl get cluster keycloak-db -n keycloak-test --watch
   ```
4. Verify Keycloak logs show at most a brief retry and no fatal error:
   ```
   kubectl logs -n keycloak-test deploy/keycloak --since=2m | grep -i 'error\|fatal'
   ```

**Expected Result:** CNPG elects a new primary within seconds. Keycloak may log a
transient connection retry but recovers without manual intervention and continues
serving requests.

**Pass Criterion:** A new primary pod is labelled `cnpg.io/instanceRole=primary` within
60 s; Keycloak pod remains `Running`.

---

## REQ-04 · Core Declarative CRDs — Alpha

### ATC-04-1 · Realm CRD lifecycle

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-04, AC 1 |
| **Phase** | Alpha |

**Preconditions:** Instance `test` is running with operator active.

**Steps:**
1. Apply a realm CR:
   ```
   kubectl apply -n keycloak-test -f examples/realm-example.yaml
   ```
2. Wait for `status.ready=true`:
   ```
   kubectl wait Realm/example-realm -n keycloak-test \
     --for=jsonpath='{.status.ready}'=true --timeout=60s
   ```
3. Verify the realm exists in Keycloak Admin API:
   ```
   curl -s -H "Authorization: Bearer <admin-token>" \
     http://keycloak.keycloak-test.svc:8080/admin/realms/example-realm | jq .id
   ```
4. Patch the CR to change `displayName` and verify it is updated in Keycloak.
5. Delete the CR and verify the realm **remains** in Keycloak:
   ```
   kubectl delete Realm example-realm -n keycloak-test
   curl -s -H "Authorization: Bearer <admin-token>" \
     http://keycloak.keycloak-test.svc:8080/admin/realms/example-realm | jq .id
   ```

**Expected Result:** The realm is created and updated correctly. After CR deletion the
realm still exists in Keycloak.

**Pass Criterion:** Realm API response contains the realm ID after delete; CR is gone
from the cluster.

---

### ATC-04-2 · Client CRD lifecycle

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-04, AC 2 |
| **Phase** | Alpha |

**Preconditions:** Instance `test` is running; realm `example-realm` exists.

**Steps:**
1. Apply a confidential client CR:
   ```
   kubectl apply -n keycloak-test -f examples/client-example.yaml
   ```
2. Wait for `status.ready=true` and note the `status.keycloakId`.
3. Verify the client exists via the Keycloak Admin API using the keycloakId.
4. Verify the Secret `<clientId>-secret` is created with keys `CLIENT_ID` and
   `CLIENT_SECRET`:
   ```
   kubectl get secret my-app-secret -n keycloak-test -o yaml
   ```
5. Delete the CR and verify the client is removed from Keycloak:
   ```
   kubectl delete Client my-app -n keycloak-test
   curl -s -H "Authorization: Bearer <admin-token>" \
     http://keycloak.keycloak-test.svc:8080/admin/realms/example-realm/clients/<keycloakId>
   ```

**Expected Result:** Client is created with a corresponding credential Secret. After CR
deletion the client returns 404 from the Admin API.

**Pass Criterion:** Admin API returns 404 for the deleted client's UUID.

---

### ATC-04-3 · ClientScope CRD lifecycle

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-04, AC 3 |
| **Phase** | Alpha |

**Preconditions:** Instance `test` is running; realm exists.

**Steps:**
1. Apply a client scope CR:
   ```
   kubectl apply -n keycloak-test -f examples/clientscope-example.yaml
   ```
2. Verify `status.ready=true` and confirm the scope exists in the Admin API.
3. Delete the CR and confirm the scope is removed from the Admin API (404).

**Pass Criterion:** Admin API returns 404 for the scope after CR deletion.

---

### ATC-04-4 · Group CRD lifecycle

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-04, AC 4 |
| **Phase** | Alpha |

**Preconditions:** Instance `test` is running; realm exists.

**Steps:**
1. Apply a group CR:
   ```
   kubectl apply -n keycloak-test -f examples/group-example.yaml
   ```
2. Verify `status.ready=true` and confirm the group exists in the Admin API.
3. Delete the CR and confirm the group is removed from the Admin API (404).

**Pass Criterion:** Admin API returns 404 for the group after CR deletion.

---

### ATC-04-5 · User CRD lifecycle with group membership

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-04, AC 5 |
| **Phase** | Alpha |

**Preconditions:** Instance `test` running; realm and group `developers` exist.

**Steps:**
1. Create a Secret holding the initial password:
   ```
   kubectl create secret generic user-password \
     -n keycloak-test --from-literal=password=InitialPass123!
   ```
2. Apply a user CR referencing the group and the password Secret:
   ```
   kubectl apply -n keycloak-test -f examples/user-example.yaml
   ```
3. Verify `status.ready=true`.
4. Confirm the user belongs to group `developers` via Admin API:
   ```
   curl -s -H "Authorization: Bearer <admin-token>" \
     .../admin/realms/example-realm/users/<userId>/groups | jq '.[].name'
   ```
5. Delete the user CR and confirm the user is removed from Keycloak (404).

**Pass Criterion:** Group membership confirmed; Admin API returns 404 after CR deletion.

---

### ATC-04-6 · Coexistence of all five CRD types

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-04, AC 6 |
| **Phase** | Alpha |

**Preconditions:** Instance `test` is running.

**Steps:**
1. Apply all five resource types simultaneously:
   ```
   kubectl apply -n keycloak-test \
     -f examples/realm-example.yaml \
     -f examples/clientscope-example.yaml \
     -f examples/group-example.yaml \
     -f examples/client-example.yaml \
     -f examples/user-example.yaml
   ```
2. Wait for all to reach `status.ready=true`:
   ```
   kubectl get Realms,Clients,ClientScopes,\
     Groups,Users -n keycloak-test
   ```
3. Verify the `READY` column shows `true` for all resources.

**Pass Criterion:** All five resources show `READY=true` without errors within three
reconciliation cycles.

---

### ATC-04-7 · CRD schema field descriptions

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-04, AC 7 |
| **Phase** | Alpha |

**Preconditions:** CRDs are installed on the cluster.

**Steps:**
1. Run `kubectl explain` on a representative field of each CRD:
   ```
   kubectl explain Realm.spec.realmName
   kubectl explain Client.spec.clientId
   kubectl explain User.spec.username
   kubectl explain Group.spec.name
   kubectl explain ClientScope.spec.protocol
   ```
2. Verify each command returns a non-empty `DESCRIPTION` section.

**Pass Criterion:** All five `kubectl explain` invocations return a `DESCRIPTION` that is
not empty and not the default `<empty>` placeholder.

---

### ATC-04-8 · Reconciliation latency

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-04, AC 8 |
| **Phase** | Alpha |

**Preconditions:** Instance `test` is running with `CHECK_INTERVAL=10`.

**Steps:**
1. Note the current time `T0`.
2. Apply a new `Client` CR.
3. Poll until `status.ready=true`, recording the time `T1`:
   ```
   kubectl wait Client/timing-test -n keycloak-test \
     --for=jsonpath='{.status.ready}'=true --timeout=60s
   ```
4. Compute `T1 - T0`.

**Pass Criterion:** `T1 - T0 ≤ 30 s`.

---

## REQ-05 · Extended CRDs — Final

### ATC-05-1 · IdentityProvider CRD lifecycle

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-05, AC 1 |
| **Phase** | Final |

**Preconditions:** `IdentityProvider` CRD is installed; instance is running.

**Steps:**
1. Apply an identity provider CR for an external OIDC provider.
2. Verify `status.ready=true`.
3. Confirm the IdP exists in Keycloak Admin API.
4. Patch the CR and verify the change is reflected in Keycloak.
5. Delete the CR and verify the IdP is removed (Admin API returns 404).

**Pass Criterion:** Full CRUD lifecycle verified via Admin API.

---

### ATC-05-2 · AuthFlow CRD lifecycle

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-05, AC 2 |
| **Phase** | Final |

**Preconditions:** `AuthFlow` CRD is installed; instance and realm are running.

**Steps:**
1. Apply an auth flow CR defining a browser flow with an OTP required step.
2. Verify `status.ready=true`.
3. Confirm the flow exists and is bound to the realm via Admin API.
4. Delete the CR and verify the flow is removed from Keycloak.

**Pass Criterion:** Full CRUD lifecycle verified via Admin API; realm reverts to default
flow after CR deletion.

---

### ATC-05-3 · CR deletion removes IdP and flow from Keycloak

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-05, AC 3 |
| **Phase** | Final |

**Steps:** Delete both CRs created in ATC-05-1 and ATC-05-2 and confirm Admin API
returns 404 for each.

**Pass Criterion:** Both resources return 404 from the Admin API after their CRs are
deleted.

---

### ATC-05-4 · IdP credential references via secretKeyRef

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-05, AC 4 |
| **Phase** | Final |

**Steps:**
1. Inspect the `IdentityProvider` CRD schema:
   ```
   kubectl explain IdentityProvider.spec --recursive | grep -i secret
   ```
2. Verify no field of type `string` that carries a credential name exists; only
   `secretKeyRef` sub-objects are present.
3. Run `gitleaks detect --source . --redact` and verify it reports no new findings
   after adding an IdP CR with a populated `secretKeyRef`.

**Pass Criterion:** No plaintext credential field in CRD schema; Gitleaks clean.

---

### ATC-05-5 · Extended CRDs present in OCM archive

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-05, AC 5 |
| **Phase** | Final |

**Steps:**
1. List archive resources and verify both new CRDs appear:
   ```
    ocm get resources --repo ./keycloak-bundle-ctf.tar.gz \
       opendefense.cloud/keycloak-bundle | grep -E 'identityprovider|authflow'
   ```

**Pass Criterion:** Both CRD resources appear in the listing.

---

### ATC-05-6 · Extended CRDs documented in USAGE.md

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-05, AC 6 |
| **Phase** | Final |

**Steps:**
1. Open `docs/USAGE.md` and verify a field-reference table exists for
   `IdentityProvider`.
2. Verify a field-reference table exists for `AuthFlow`.
3. Verify at least one worked example is present for each type.

**Pass Criterion:** Both sections exist with tables and examples.

---

## REQ-06 · CR Status Reporting — Alpha

### ATC-06-1 · READY column in kubectl output

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-06, AC 1 |
| **Phase** | Alpha |

**Steps:**
1. List clients:
   ```
   kubectl get Clients -n keycloak-test
   ```
2. Verify a `READY` column is present and shows `true` for synced resources.

**Pass Criterion:** Column `READY` is present; synced resources show `true`.

---

### ATC-06-2 · Ready=False when Keycloak unreachable

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-06, AC 2 |
| **Phase** | Alpha |

**Steps:**
1. Scale Keycloak to zero:
   ```
   kubectl scale deployment keycloak -n keycloak-test --replicas=0
   ```
2. Wait one reconciliation cycle (≥ 10 s), then check status:
   ```
   kubectl get Client my-app -n keycloak-test \
     -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
   ```
3. Inspect the message:
   ```
   kubectl get Client my-app -n keycloak-test \
     -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}'
   ```

**Pass Criterion:** Condition status is `False`; message is non-empty and meaningful.

---

### ATC-06-3 · Automatic recovery to Ready=True

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-06, AC 3 |
| **Phase** | Alpha |

**Steps:**
1. From the state established in ATC-06-2, restore Keycloak:
   ```
   kubectl scale deployment keycloak -n keycloak-test --replicas=1
   kubectl rollout status deployment/keycloak -n keycloak-test --timeout=120s
   ```
2. Wait two reconciliation cycles, then verify:
   ```
   kubectl get Client my-app -n keycloak-test \
     -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
   ```

**Pass Criterion:** Condition status transitions back to `True` without manual
intervention.

---

### ATC-06-4 · lastSyncTime updated every cycle

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-06, AC 4 |
| **Phase** | Alpha |

**Steps:**
1. Record current `lastSyncTime`:
   ```
   kubectl get Client my-app -n keycloak-test \
     -o jsonpath='{.status.lastSyncTime}'
   ```
2. Wait 15 s (more than one reconciliation cycle).
3. Read `lastSyncTime` again and compare.

**Pass Criterion:** The second timestamp is strictly greater than the first.

---

### ATC-06-5 · keycloakId populated after first sync

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-06, AC 5 |
| **Phase** | Alpha |

**Steps:**
1. Apply a new `Client` CR.
2. Wait for `status.ready=true`.
3. Check `status.keycloakId`:
   ```
   kubectl get Client new-client -n keycloak-test \
     -o jsonpath='{.status.keycloakId}'
   ```
4. Verify the UUID exists in the Keycloak Admin API.

**Pass Criterion:** `keycloakId` is a non-empty UUID that is resolvable via the Admin
API.

---

### ATC-06-6 · Error message contains HTTP detail

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-06, AC 6 |
| **Phase** | Alpha |

**Steps:**
1. Apply a `Client` CR referencing a non-existent realm (`badRealm`).
2. Wait one reconciliation cycle.
3. Inspect the condition message:
   ```
   kubectl get Client bad-client -n keycloak-test \
     -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}'
   ```

**Pass Criterion:** The message contains an HTTP status code (e.g. `404`) or a
descriptive error string; it is not empty.

---

## REQ-07 · Secure Secrets Management — PoC

### ATC-07-1 · No credentials in repository (Gitleaks)

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-07, AC 1 |
| **Phase** | PoC |

**Steps:**
1. Run Gitleaks over the full repository history:
   ```
   gitleaks detect --source . --redact -v
   ```
2. Inspect the exit code and output.

**Pass Criterion:** Gitleaks exits with code 0 (no findings).

---

### ATC-07-2 · Admin password only accessible via Secret

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-07, AC 2 |
| **Phase** | PoC |

**Steps:**
1. Search all manifests for the admin password value:
   ```
   grep -r 'KEYCLOAK_ADMIN_PASSWORD' manifests/ charts/ kro/ examples/ | grep -v 'secretKeyRef\|secretName\|valueFrom'
   ```
2. Confirm the Secret `keycloak-admin` exists in the instance namespace:
   ```
   kubectl get secret keycloak-admin -n keycloak-test
   ```
3. Confirm the password does not appear in operator logs:
   ```
   kubectl logs -n keycloak-test deploy/keycloak-operator | grep -i 'admin'
   ```

**Pass Criterion:** Step 1 grep returns empty; Secret exists; log grep contains no
password value.

---

### ATC-07-3 · Database credentials injected via secretKeyRef

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-07, AC 3 |
| **Phase** | PoC |

**Steps:**
1. Inspect DB credential env vars in the Keycloak Deployment (see ATC-03-3).

**Pass Criterion:** Both `KC_DB_USERNAME` and `KC_DB_PASSWORD` use `secretKeyRef`; no
`value:` field is present for credentials.

---

### ATC-07-4 · Client credential Secrets in target namespace

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-07, AC 4 |
| **Phase** | PoC |

**Steps:**
1. After syncing a confidential `Client`, verify the Secret:
   ```
   kubectl get secret my-app-secret -n keycloak-test -o yaml | grep -E 'CLIENT_ID|CLIENT_SECRET'
   ```
2. Confirm both keys are present as base64-encoded values.
3. Confirm no plaintext secret value appears in any CR status field:
   ```
   kubectl get Client my-app -n keycloak-test -o yaml | grep CLIENT_SECRET
   ```

**Pass Criterion:** Secret has both keys; CR status contains no `CLIENT_SECRET` value.

---

### ATC-07-5 · User password accepted only as Secret reference

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-07, AC 5 |
| **Phase** | PoC |

**Steps:**
1. Inspect the `User` CRD schema:
   ```
   kubectl explain User.spec.initialPassword
   ```
2. Attempt to create a user CR with an inline `password: "plaintext"` field and confirm
   schema validation rejects it.

**Pass Criterion:** `kubectl explain` shows `secretName` and `secretKey` sub-fields
only; inline password is rejected by the API server.

---

### ATC-07-6 · No plaintext credential fields in CRD schemas

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-07, AC 6 |
| **Phase** | PoC |

**Steps:**
1. Search all CRD YAML files for field names that suggest plaintext credentials:
   ```
   grep -rn -E 'password:|secret:|clientSecret:|token:' \
     charts/keycloak-operator/crds/ | grep -v 'secretKeyRef\|secretName\|secretKey\|description'
   ```

**Pass Criterion:** Grep returns empty output.

---

## REQ-08 · HA & Scalability — Alpha

### ATC-08-1 · Three-replica Keycloak cluster formation

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-08, AC 1 |
| **Phase** | Alpha |

**Steps:**
1. Apply a `KeycloakInstance` CR with `spec.replicas: 3`.
2. Wait until all three pods are ready:
   ```
   kubectl rollout status deployment/keycloak -n keycloak-test
   kubectl get pods -n keycloak-test -l app=keycloak
   ```
3. Check the Keycloak logs of any pod for cluster member messages:
   ```
   kubectl logs -n keycloak-test <any-keycloak-pod> | grep -i 'member\|cluster\|infinispan'
   ```

**Pass Criterion:** All three pods show `Ready 1/1`; logs contain cluster membership
messages confirming all three nodes joined.

---

### ATC-08-2 · Session sharing across replicas

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-08, AC 2 |
| **Phase** | Alpha |

**Preconditions:** Three-replica cluster is running.

**Steps:**
1. Obtain an access token by logging in via pod A (use `kubectl exec` or port-forward):
   ```
   curl -s -X POST http://<pod-A-ip>:8080/realms/master/protocol/openid-connect/token \
     -d 'grant_type=password&client_id=admin-cli&username=admin&password=<pw>' | jq .access_token
   ```
2. Delete pod A:
   ```
   kubectl delete pod <pod-A> -n keycloak-test
   ```
3. Use the same token to call the Admin API via the service (which routes to pod B or C):
   ```
   curl -s -H "Authorization: Bearer <token>" \
     http://keycloak.keycloak-test.svc:8080/admin/realms | jq '.[].id'
   ```

**Pass Criterion:** The API call using the token obtained from pod A succeeds after pod A
is deleted (session is valid on the remaining replicas).

---

### ATC-08-3 · PodDisruptionBudget prevents full eviction

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-08, AC 3 |
| **Phase** | Alpha |

**Steps:**
1. Verify the PDB exists:
   ```
   kubectl get pdb keycloak -n keycloak-test
   ```
2. Simulate a node drain that would evict all pods:
   ```
   kubectl drain <node> --ignore-daemonsets --delete-emptydir-data --dry-run=client
   ```
3. Alternatively, attempt to delete all Keycloak pods simultaneously:
   ```
   kubectl delete pods -n keycloak-test -l app=keycloak
   ```
4. Observe that at least one pod is kept alive before new ones are scheduled.

**Pass Criterion:** PDB shows `ALLOWED DISRUPTIONS: <replicas-1>`; at least one
Keycloak pod remains `Running` during the eviction sequence.

---

### ATC-08-4 · Single operator leader at any time

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-08, AC 4 |
| **Phase** | Alpha |

**Steps:**
1. Scale the operator to two replicas:
   ```
   kubectl scale deployment keycloak-operator -n keycloak-test --replicas=2
   ```
2. Check the Lease holder:
   ```
   kubectl get lease keycloak-operator-leader -n keycloak-test \
     -o jsonpath='{.spec.holderIdentity}'
   ```
3. Check operator logs to confirm only one pod logs reconciliation cycles:
   ```
   kubectl logs -n keycloak-test -l app=keycloak-operator --prefix | \
     grep 'Reconciliation cycle starting'
   ```

**Pass Criterion:** Exactly one pod name appears in the reconciliation log lines; the
Lease holder matches that pod name.

---

### ATC-08-5 · Leader lease re-acquisition after pod loss

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-08, AC 5 |
| **Phase** | Alpha |

**Preconditions:** Two operator replicas, current leader identified (see ATC-08-4).

**Steps:**
1. Delete the current leader pod:
   ```
   kubectl delete pod <leader-pod> -n keycloak-test
   ```
2. Wait one lease-duration interval (≤ 15 s) plus one renewal (7 s) = ≤ 22 s.
3. Read the Lease holder again:
   ```
   kubectl get lease keycloak-operator-leader -n keycloak-test \
     -o jsonpath='{.spec.holderIdentity}'
   ```
4. Confirm reconciliation log lines resume from the new leader.

**Pass Criterion:** A different pod name holds the Lease within 22 s of the original
leader's deletion; reconciliation cycle log lines resume.

---

### ATC-08-6 · Replica count configurable via CR

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-08, AC 6 |
| **Phase** | Alpha |

**Steps:**
1. Start with `spec.replicas: 1`.
2. Patch to `spec.replicas: 3`:
   ```
   kubectl patch keycloakinstance test --type=merge -p '{"spec":{"replicas":3}}'
   ```
3. Verify the Deployment scales accordingly:
   ```
   kubectl get deployment keycloak -n keycloak-test \
     -o jsonpath='{.spec.replicas}'
   ```

**Pass Criterion:** Deployment `spec.replicas` is `3` within two reconciliation cycles.

---

## REQ-09 · KRO-Based Instantiation — Alpha

### ATC-09-1 · Single CR triggers full deployment

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-09, AC 1 |
| **Phase** | Alpha |

**Preconditions:** KRO and CNPG are installed. No prior Keycloak instance exists.

**Steps:**
1. Apply a single `KeycloakInstance` CR:
   ```
   kubectl apply -f examples/keycloak-instance.yaml
   ```
2. Watch KRO create child resources in order:
   ```
   kubectl get events -n keycloak-test --sort-by='.lastTimestamp' -w
   ```
3. Verify all expected resources are present:
   ```
   kubectl get all,cluster,secret -n keycloak-test
   ```

**Pass Criterion:** Namespace, CNPG cluster, operator, and Keycloak Deployment are all
created automatically; no `kubectl apply` beyond the single CR is needed.

---

### ATC-09-2 · Pods ready without manual steps

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-09, AC 2 |
| **Phase** | Alpha |

**Steps:**
1. From a clean cluster state, apply only the `KeycloakInstance` CR.
2. Wait for all pods to become ready:
   ```
   kubectl wait pods -n keycloak-test --all --for=condition=Ready --timeout=300s
   ```

**Pass Criterion:** `kubectl wait` exits with code 0 within 300 s without any additional
manual steps.

---

### ATC-09-3 · CR deletion removes all resources

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-09, AC 3 |
| **Phase** | Alpha |

**Steps:**
1. Delete the `KeycloakInstance` CR:
   ```
   kubectl delete keycloakinstance test
   ```
2. Wait 60 s, then list all resources in the instance namespace:
   ```
   kubectl get all,cluster,secret -n keycloak-test 2>&1
   ```
3. Confirm the namespace itself is also removed:
   ```
   kubectl get namespace keycloak-test
   ```

**Pass Criterion:** All child resources and the namespace are gone; `kubectl get
namespace keycloak-test` returns `NotFound`.

---

### ATC-09-4 · Air-gapped end-to-end deployment

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-09, AC 4 |
| **Phase** | Alpha |

**Steps:** See ATC-01-4. After transferring the archive to the private registry, apply
the `KeycloakInstance` CR and verify all pods start using the private registry images
only.

**Pass Criterion:** All pods reach `Running`; no public registry is contacted.

---

### ATC-09-5 · RGD present in OCM archive

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-09, AC 5 |
| **Phase** | Alpha |

**Steps:**
1. List resources and filter for the RGD:
   ```
    ocm get resources --repo ./keycloak-bundle-ctf.tar.gz \
       opendefense.cloud/keycloak-bundle | grep keycloak-instance-rgd
   ```

**Pass Criterion:** The resource `keycloak-instance-rgd` of type `blueprint` appears in
the listing.

---

### ATC-09-6 · Two simultaneous instances via KRO

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-09, AC 6 |
| **Phase** | Alpha |

**Steps:**
1. Apply two distinct `KeycloakInstance` CRs with different names.
2. Verify both reach `Running` and operate independently (see ATC-02-1).

**Pass Criterion:** Both instances have all pods in `Running` state; cross-instance
interference test from ATC-02-1 passes.

---

## REQ-10 · Observability — Final

### ATC-10-1 · Structured JSON log output

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-10, AC 1 |
| **Phase** | Final |

**Steps:**
1. Tail the logs of a running Keycloak pod:
   ```
   kubectl logs -n keycloak-test deploy/keycloak --tail=20
   ```
2. Pipe to `jq` to verify JSON structure:
   ```
   kubectl logs -n keycloak-test deploy/keycloak --tail=20 | jq .
   ```

**Pass Criterion:** `jq` parses all lines without error; each line contains at least the
keys `level`, `message`, and `timestamp`.

---

### ATC-10-2 · OTEL tracing spans visible in backend

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-10, AC 2 |
| **Phase** | Final |

**Preconditions:** Jaeger all-in-one is running in namespace `observability`
(deploy via `scripts/deploy/install-jaeger.sh`).

**Steps:**
1. Run the observability verification script:
   ```sh
   ./scripts/tests/test-observability.sh <namespace>
   ```
   Test 3 patches `KC_TRACING_ENABLED=true` and `KC_TRACING_ENDPOINT=http://jaeger.observability.svc:4317`,
   generates a login via `kcadm.sh`, and queries the Jaeger API for service `keycloak`
   and at least one trace.

**Pass Criterion:** `'keycloak'` service present in Jaeger services API; at least one
trace returned by `GET /api/traces?service=keycloak`.  Script auto-reverts the patch.

---

### ATC-10-3 · Prometheus metrics at /metrics

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-10, AC 3 |
| **Phase** | Final |

**Steps:**
1. Port-forward the management port:
   ```
   kubectl port-forward -n keycloak-test svc/keycloak 9000:9000 &
   ```
2. Fetch metrics:
   ```
   curl -s http://localhost:9000/metrics | head -30
   ```

**Pass Criterion:** Output is in Prometheus text exposition format (lines starting with
`# HELP`, `# TYPE`, and metric names); HTTP status is 200.

---

### ATC-10-4 · ServiceMonitor picked up automatically

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-10, AC 4 |
| **Phase** | Final |

**Preconditions:** Prometheus Operator installed via `scripts/deploy/install-prometheus-operator.sh`.

**Steps:**
1. Run the observability verification script (Test 1):
   ```sh
   ./scripts/tests/test-observability.sh <namespace>
   ```
   Test 1 confirms the `ServiceMonitor` resource exists, port-forwards port 9000, and
   asserts that at least one `keycloak_*` metric line is present in the `/metrics` response.
   Test 2 confirms `PodMonitor` exists and `cnpg_collector_up` is present at port 9187.

**Pass Criterion:** `ServiceMonitor keycloak` exists; ≥1 `keycloak_*` metric line at
`/metrics`; `PodMonitor keycloak-db-metrics` exists; `cnpg_collector_up` at port 9187.

---

### ATC-10-5 · Liveness and readiness probes gate traffic

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-10, AC 5 |
| **Phase** | Final |

**Steps:**
1. During pod startup, monitor the ready condition:
   ```
   kubectl get pods -n keycloak-test -w | grep keycloak
   ```
2. Verify the pod transitions through `0/1` (not ready) before reaching `1/1`.
3. Confirm the probes point to port 9000:
   ```
   kubectl get deployment keycloak -n keycloak-test \
     -o jsonpath='{.spec.template.spec.containers[0].livenessProbe}'
   kubectl get deployment keycloak -n keycloak-test \
     -o jsonpath='{.spec.template.spec.containers[0].readinessProbe}'
   ```

**Pass Criterion:** Both probes target `port: 9000` with paths `/health/live` and
`/health/ready` respectively; pod is `0/1` until fully initialised.

---

### ATC-10-6 · PrometheusRule alerts fire under fault conditions

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-10, AC 6 |
| **Phase** | Final |

**Preconditions:**
- Prometheus Operator installed (`scripts/deploy/install-prometheus-operator.sh`).
- Prometheus CR deployed (`manifests/monitoring/prometheus.yaml`), including per-instance
  ClusterRole/ClusterRoleBinding created by `deploy-all.sh`.

**Steps:**
1. Run the observability verification script:
   ```sh
   ./scripts/tests/test-observability.sh <namespace>
   ```
   Test 4 confirms `PrometheusRule keycloak` exists and all 9 expected alert names
   (`KeycloakDown`, `KeycloakNotReady`, `KeycloakHighLoginFailureRate`,
   `KeycloakBruteForceDetected`, `KeycloakHighActiveSessions`,
   `KeycloakDBConnectionPoolExhausted`, `KeycloakPodRestartingFrequently`,
   `KeycloakDBClusterNotReady`, `KeycloakDBReplicationLag`) are defined in the spec.

   Test 5 exercises live rule evaluation:
   - Scales Keycloak to 0 replicas.
   - Polls `GET /api/v1/query?query=up{job="keycloak"}` until the metric becomes 0.
   - Polls `GET /api/v1/alerts` until `KeycloakDown` appears in `pending` or `firing` state
     (within 90 s: 30 s scrape interval + 15 s eval interval + buffer).
   - Restores the original replica count and waits for Keycloak to come back.

**Pass Criterion:** `KeycloakDown` alert appears in `pending` or `firing` state in the
Prometheus alerts API within 90 s of scaling Keycloak to zero; Keycloak is restored
successfully.

---

### ATC-10-7 · OBSERVABILITY.md exists and is complete

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-10, AC 7 |
| **Phase** | Final |

**Steps:**
1. Verify the file exists:
   ```
   ls docs/OBSERVABILITY.md
   ```
2. Confirm it covers: OTEL endpoint configuration, disabling/enabling tracing,
   Prometheus scraping setup, and alerting rule tuning guidance.

**Pass Criterion:** File exists; all four topics are addressed.

---

## REQ-11 · Backup & Restore — Final

### ATC-11-1 · Backup triggered via Kubernetes resource

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-11, AC 1 |
| **Phase** | Final |

**Steps:**
1. Apply a CNPG-native backup resource:
   ```
   kubectl apply -n keycloak-test -f examples/backup-example.yaml
   ```
2. Watch the backup resource status until completion.
3. Verify the backup artefact appears in the configured storage location.

**Pass Criterion:** Backup resource `status.phase` transitions to `Completed`; artefact
is present in S3.

---

### ATC-11-2 · Backup storage location is configurable

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-11, AC 2 |
| **Phase** | Final |

**Steps:**
1. Apply a CNPG `Backup` resource pointing to a second S3-compatible bucket configuration:
   ```sh
   kubectl apply -f - <<EOF
    apiVersion: postgresql.cnpg.io/v1
    kind: Backup
   metadata:
     name: keycloak-db-alt-bucket
     namespace: keycloak-test
   spec:
         cluster:
            name: keycloak-db
         method: plugin
         pluginConfiguration:
            name: barman-cloud.cloudnative-pg.io
   EOF
   ```
2. Wait for `status.phase=completed`.
3. Confirm the artefact is written to the alternative bucket; verify the original
   bucket did not receive a new artefact.

**Pass Criterion:** Artefact appears in the alternative bucket; original bucket unchanged.

---

### ATC-11-3 · Restore returns Keycloak to backed-up state

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-11, AC 3 |
| **Phase** | Final |

**Steps:**
1. Create a realm, client, and user. Trigger a backup.
2. Delete the realm, client, and user from Keycloak directly via the Admin Console.
3. Perform a restore following `docs/UPGRADE.md`.
4. Verify the deleted realm, client, and user are present again.

**Pass Criterion:** All three resources are accessible in Keycloak after the restore.

---

### ATC-11-4 · Backup procedure documented in UPGRADE.md

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-11, AC 4 |
| **Phase** | Final |

**Steps:**
1. Open `docs/UPGRADE.md`.
2. Follow the backup and restore steps as written, without prior knowledge of the
   implementation.

**Pass Criterion:** A tester with no prior knowledge can complete the backup and restore
cycle using only the documented commands.

---

### ATC-11-5 · Backup status reported in CR status

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-11, AC 5 |
| **Phase** | Final |

**Steps:**
1. After triggering a backup, inspect the CNPG `Backup` status:
   ```sh
   kubectl get backups.postgresql.cnpg.io keycloak-db-ondemand -n keycloak-test \
     -o yaml | grep -A15 'status:'
   ```
   Verify `status.phase` and completion timestamps are populated.
2. Simulate a backup failure (e.g. invalid object-store credentials on cluster backup config)
   and verify failure is reflected:
   ```sh
   kubectl get backups.postgresql.cnpg.io bad-creds -n keycloak-test \
     -o jsonpath='{.status.phase}'
   ```

**Pass Criterion:** `status.phase=completed` on success;
backup status indicates failure on credential/configuration errors.

---

## REQ-12 · Zero-Downtime Rolling Updates — Final

### ATC-12-1 · Zero dropped requests during minor version update

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-12, AC 1 |
| **Phase** | Final |

**Steps:**
1. Start a continuous request probe:
   ```
   while true; do
     code=$(curl -s -o /dev/null -w '%{http_code}' http://keycloak.keycloak-test.svc:8080/health)
     [ "$code" != "200" ] && echo "FAILED: $code at $(date)"
     sleep 0.5
   done
   ```
2. Apply a Keycloak patch version update via the `KeycloakInstance` CR.
3. Wait for the rollout to complete.
4. Stop the probe and count failures.

**Pass Criterion:** Zero non-200 responses are logged by the probe during the rollout.

---

### ATC-12-2 · CNPG minor upgrade without connectivity loss

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-12, AC 2 |
| **Phase** | Final |

**Steps:**
1. Run the same continuous request probe from ATC-12-1.
2. Trigger a CNPG minor PostgreSQL version upgrade via the `Cluster` CR.
3. Monitor Keycloak logs for database connection errors during the primary switchover.
4. Stop probe and count failures.

**Pass Criterion:** Zero non-200 probe responses; Keycloak logs show at most a single
transient connection retry.

---

### ATC-12-3 · Major version upgrade runbook is executable

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-12, AC 3 |
| **Phase** | Final |

**Steps:**
1. Open `docs/UPGRADE.md` and locate the major version upgrade section.
2. Follow each step in sequence using only the documented commands.
3. Verify Keycloak starts successfully on the new major version.

**Pass Criterion:** Keycloak reaches `Running` on the new major version; no undocumented
step was required.

---

### ATC-12-4 · Major version upgrade runbook in UPGRADE.md

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-12, AC 4 |
| **Phase** | Final |

**Steps:**
1. Open `docs/UPGRADE.md`.
2. Verify sections exist for: scale-down to single replica, image update, schema
   migration wait, scale-back, and rollback procedure.

**Pass Criterion:** All five sections are present with concrete commands.

---

### ATC-12-5 · CRD compatibility guarantees documented

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-12, AC 5 |
| **Phase** | Final |

**Steps:**
1. Locate the CRD compatibility section in `docs/USAGE.md` or `docs/UPGRADE.md`.
2. Confirm it lists immutable fields (e.g. `spec.realmName`, `spec.clientId`).
3. Confirm it describes the migration path for breaking schema changes.

**Pass Criterion:** Document exists and covers all three points.

---

## REQ-13 · Documentation — PoC–Final

### ATC-13-1 · README links are complete and accurate

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-13, AC 1 |
| **Phase** | PoC–Final |

**Steps:**
1. Open `README.md` and extract all relative documentation links.
2. Verify each linked file exists:
   ```
   grep -oE '\[.*\]\(docs/[^)]+\)' README.md | grep -oE 'docs/[^)]+' | \
     while read f; do [ -f "$f" ] || echo "MISSING: $f"; done
   ```

**Pass Criterion:** No `MISSING` lines are printed.

---

### ATC-13-2 · Deploy from documentation alone

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-13, AC 2 |
| **Phase** | PoC–Final |

**Steps:**
1. A tester with Kubernetes and OCM CLI knowledge but no prior project exposure follows
   `docs/DEPLOYMENT.md` step by step.
2. The tester records any step that requires information not present in the documentation.

**Pass Criterion:** The tester reaches a running Keycloak instance without needing to
consult any source file or ask the development team.

---

### ATC-13-3 · kubectl explain returns descriptions

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-13, AC 3 |
| **Phase** | PoC–Final |

**Steps:**
1. For each of the five CRDs, run `kubectl explain` on three representative fields.
2. Verify the `DESCRIPTION` section is non-empty for each.

**Pass Criterion:** All 15 `kubectl explain` invocations (5 CRDs × 3 fields) return a
non-empty description.

---

### ATC-13-4 · USAGE.md covers all delivered CRD types

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-13, AC 4 |
| **Phase** | PoC–Final |

**Steps:**
1. Open `docs/USAGE.md`.
2. Verify a field-reference table and at least one example exist for each currently
   delivered CRD: `Realm`, `Client`, `ClientScope`,
   `Group`, `User`.

**Pass Criterion:** All five sections are present with tables and examples.

---

### ATC-13-5 · UPGRADE.md covers all upgrade scenarios

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-13, AC 5 |
| **Phase** | PoC–Final |

**Steps:**
1. Open `docs/UPGRADE.md`.
2. Confirm sections for: minor Keycloak version upgrade, major Keycloak version upgrade,
   CNPG minor version upgrade, backup before upgrade, and rollback.

**Pass Criterion:** All five sections present with commands (blocked on REQ-11, REQ-12).

---

### ATC-13-6 · Final documentation review

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-13, AC 6 |
| **Phase** | PoC–Final |

**Steps:**
1. For each acceptance criterion in `docs/REQUIREMENTS.md`, locate the corresponding
   documentation section.
2. Record any criterion that has no matching documentation.

**Pass Criterion:** Zero unmatched criteria (blocked until all Final requirements are
complete).

---

### ATC-13-7 · OBSERVABILITY.md exists

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-13, AC 7 |
| **Phase** | PoC–Final |

**Steps:**
1. Check for the file:
   ```
   ls docs/OBSERVABILITY.md
   ```

**Pass Criterion:** File exists and is non-empty.

---

## REQ-14 · Quality Assurance & Hardening — Final

### ATC-14-1 · ShellCheck violation blocks PR

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-14, AC 1 |
| **Phase** | Final |

**Steps:**
1. Create a test branch and add a shell script with a known ShellCheck warning:
   ```bash
   #!/bin/bash
   ls $UNQUOTED_VAR
   ```
2. Open a pull request.
3. Observe the CI pipeline result.

**Pass Criterion:** The CI pipeline fails on the ShellCheck step; the PR cannot be
merged until the violation is fixed.

---

### ATC-14-2 · Gitleaks violation blocks PR

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-14, AC 2 |
| **Phase** | Final |

**Steps:**
1. Create a test branch and commit a file containing a synthetic secret pattern
   (e.g. `password = "S3cr3tV@lue"`).
2. Open a pull request.
3. Observe the CI pipeline result.

**Pass Criterion:** The Gitleaks step fails; the PR cannot be merged until the secret
is removed.

---

### ATC-14-3 · CVE scan blocks PR on critical finding

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-14, AC 3 |
| **Phase** | Final |

**Preconditions:** `.github/workflows/security.yml` Trivy workflow is active on the
repository (merged to `main`).

**Steps:**
1. Confirm the workflow file is present and the image-scan steps carry `exit-code: 1`
   for `HIGH,CRITICAL` findings:
   ```sh
   grep -A5 'exit-code' .github/workflows/security.yml
   ```
2. Optionally, reference a container image with a known critical CVE in a test PR and
   observe the workflow block.

**Pass Criterion:** Image-scan steps have `exit-code: 1`; a PR introducing a critically
vulnerable image is blocked by the GitHub Actions check.

---

### ATC-14-4 · YAML linting failure blocks PR

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-14, AC 4 |
| **Phase** | Final |

**Steps:**
1. Create a test branch and add a syntactically invalid YAML file to `manifests/`.
2. Open a pull request.
3. Observe the CI pipeline result.

**Pass Criterion:** The YAML lint step fails; the PR cannot be merged until the YAML
error is corrected.

---

### ATC-14-5 · CONTRIBUTING.md exists and is complete

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-14, AC 5 |
| **Phase** | Final |

**Steps:**
1. Check the repository root:
   ```
   ls CONTRIBUTING.md
   ```
2. Verify the file covers: branching strategy, commit message conventions, PR review
   process, and CI quality gates.

**Pass Criterion:** File exists; all four topics are addressed with concrete guidance.

---

### ATC-14-6 · Container images run as non-root

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-14, AC 6 |
| **Phase** | Final |

**Steps:**
1. Check the security context of the Keycloak container:
   ```
   kubectl get deployment keycloak -n keycloak-test \
     -o jsonpath='{.spec.template.spec.containers[0].securityContext}'
   ```
2. Verify `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, and
   `capabilities.drop` contains `ALL`.
3. Repeat for the operator Deployment and the init container.

**Pass Criterion:** All three container specifications show `runAsNonRoot: true`,
`allowPrivilegeEscalation: false`, and `capabilities.drop: [ALL]`.

---

### ATC-14-7 · Hardening checklist exists

| Attribute | Value |
|-----------|-------|
| **Requirement** | REQ-14, AC 7 |
| **Phase** | Final |

**Steps:**
1. Locate the hardening checklist in `docs/`.
2. Verify it cross-references applied STIG or CIS Benchmark controls by ID.
3. Verify accepted deviations are listed with justification.

**Pass Criterion:** Checklist exists; at least one control ID from a recognised guide is
referenced; any deviation has a written justification.
