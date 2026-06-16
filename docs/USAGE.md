# Keycloak Operator: Usage Guide

## Overview

The Keycloak Operator enables a **declarative, Kubernetes-native approach** to managing Keycloak configuration. Instead of manual changes via the Keycloak Admin Console, teams define their requirements as Kubernetes Custom Resources (CRDs) and commit them to Git, enabling a full **GitOps workflow**.

The operator covers the full Keycloak resource hierarchy:

```text
KeycloakInstance (via KRO)
├── Realm
│   ├── Client              (realmRef required)
│   ├── ClientScope         (realmRef required)
│   ├── Group               (realmRef required)
│   ├── User                (realmRef required)
│   ├── IdentityProvider    (realmRef required)
│   └── AuthFlow            (realmRef required)

CNPG-native Day-2 resources (database scope)
├── Backup
└── ScheduledBackup
```

All resources are **namespace-scoped** and must be applied to the namespace of their target Keycloak instance (e.g. `keycloak-dev`). A `Realm` CR is the reconciliation anchor: child controllers validate their own object, create any needed Kubernetes-side Secret, annotate the referenced `Realm`, and the Realm controller builds one combined `realm.json` payload for `keycloak-config-cli`.

`realmRef` is mandatory for every child CR. The operator intentionally does not fall back to the privileged `master` realm when `realmRef` is missing.

---

## GitOps Workflow

The typical flow for an application team consuming a Keycloak client:

```mermaid
sequenceDiagram
    participant Dev as App Developer
    participant Git as Git Repo (App)
    participant CD as CI/CD / GitOps (ArgoCD)
    participant K8s as Kubernetes API
    participant Op as Keycloak Operator
    participant Job as config-cli Job
    participant KC as Keycloak Server
    participant App as Application Pod

    Note over Dev, Git: CONSUMER TEAM (App)
    Dev->>Git: Push Client CR & App Code
    Git->>CD: Trigger Deployment
    CD->>K8s: Apply Client CR

    Note over K8s, KC: PLATFORM TEAM (Keycloak)
    Op->>K8s: Watch all Keycloak CRs
    Op->>K8s: Ensure client Secret for confidential clients
    Op->>K8s: Create/Update config Secret (realm.json)
    Op->>K8s: Spawn config-cli Job
    Job->>KC: REST API Import (realm.json)
    Job->>K8s: Update Job Status (Complete/Failed)
    Op->>K8s: Update Realm status from Job result

    Note over Op, KC: Operator does NOT call Keycloak REST directly;\nthe config-cli Job performs the import.

    Note over App: App Deployment
    K8s->>App: Mount Secret (envFrom: secretRef)
    App->>KC: Authenticate (Client ID + Secret)
```

---

## Realm

Realms are the top-level container for all other resources.

> [!IMPORTANT]
> **Federated Anchor:** You MUST create a `Realm` before applying any clients, scopes, groups, or users that reference it. Child resources without a valid `realmRef` (or missing the referenced Realm object) will fail to synchronize.

```yaml
apiVersion: keycloak.opendefense.cloud/v1alpha1
kind: Realm
metadata:
  name: tenant-a
  namespace: keycloak-dev
spec:
  realmName: tenant-a          # Keycloak realm ID — immutable after creation
  displayName: "Tenant A"
  enabled: true
  registrationAllowed: false
  resetPasswordAllowed: true
  bruteForceProtected: true
  accessTokenLifespan: 300     # seconds
  sslRequired: external
  accountTheme: keycloak
  adminTheme: keycloak
  emailTheme: keycloak
  internationalizationEnabled: true
  supportedLocales:
    - en
    - de
  defaultLocale: en
```

```bash
kubectl apply -f realm.yaml
kubectl get Realms -n keycloak-dev
# NAME       REALMNAME   ENABLED   READY   AGE
# tenant-a   tenant-a    true      true    30s
```

> `realmName` is the Keycloak realm ID used as the identifier in all API calls. Do not change it after creation — the operator will attempt to create a second realm rather than rename the existing one.

### Spec Fields (Realm)

| Field | Type | Description |
|---|---|---|
| `realmName` | string | Unique identifier for the realm. |
| `displayName` | string | Human-readable name. |
| `enabled` | boolean | Whether the realm is active. |
| `sslRequired` | string | SSL requirement level: `all`, `external`, `none`. |
| `registrationAllowed` | boolean | If users can register themselves. |
| `resetPasswordAllowed`| boolean | If users can use the "Forgot Password" flow. |
| `bruteForceProtected` | boolean | Enables brute-force detection logic. |
| `loginTheme` | string | Theme used for login pages. |
| `accountTheme` | string | Theme used for the user account console. |
| `adminTheme` | string | Theme used for the admin console. |
| `emailTheme` | string | Theme used for system emails. |
| `internationalizationEnabled` | boolean | Enables multi-language support. |
| `supportedLocales` | string[] | List of enabled language codes (e.g. `["en", "de"]`). |
| `defaultLocale` | string | Default language for the realm. |
| `accessTokenLifespan` | integer | Default lifespan for access tokens in seconds. |

> [!TIP]
> **Drift Healing:** All spec fields are strictly enforced. If a user changes the theme or SSL settings via the Keycloak UI, the operator will automatically revert those changes to the state defined in your CRD during the next reconciliation cycle (default every 5 minutes).


> Deleting a `Realm` CR does **not** delete the realm from Keycloak — the realm is intentionally preserved to protect existing users and sessions. Remove it manually via the Keycloak Admin Console if needed.

> Deleting any other CR type (`Client`, `Group`, `User`, `ClientScope`, `AuthFlow`, `IdentityProvider`) **does** remove the corresponding resource from Keycloak. The operator uses Kubernetes finalizers to propagate the deletion through a successful Realm sync before the CR is garbage-collected.

---

## ClientScope

```yaml
apiVersion: keycloak.opendefense.cloud/v1alpha1
kind: ClientScope
metadata:
  name: tenant-a-profile
  namespace: keycloak-dev
spec:
  realmRef: tenant-a
  name: profile
  protocol: openid-connect
  description: "Standard profile scope exposing name and email claims"
  attributes:
    include.in.token.scope: "true"
    display.on.consent.screen: "true"
    consent.screen.text: "Access your profile information"
```

```bash
kubectl get ClientScopes -n keycloak-dev
# NAME                REALM      SCOPENAME   PROTOCOL        READY
# tenant-a-profile    tenant-a   profile     openid-connect  true
```

### Spec Fields (ClientScope)

| Field | Type | Required | Description |
|---|---|---|---|
| `realmRef` | string | Yes | Name of the target `Realm` CR in the same namespace. |
| `name` | string | Yes | Client scope name in Keycloak. |
| `protocol` | string | No | Protocol name, typically `openid-connect`. |
| `description` | string | No | Human-readable purpose for the scope. |
| `attributes` | map[string]string | No | Keycloak client-scope attributes passed through to `keycloak-config-cli`. |

---

## Group

```yaml
apiVersion: keycloak.opendefense.cloud/v1alpha1
kind: Group
metadata:
  name: tenant-a-developers
  namespace: keycloak-dev
spec:
  realmRef: tenant-a
  name: developers
  attributes:
    department:
      - engineering
  realmRoles:
    - developer
```

```bash
kubectl get Groups -n keycloak-dev
# NAME                   REALM      GROUPNAME    READY
# tenant-a-developers    tenant-a   developers   true
```

### Spec Fields (Group)

| Field | Type | Required | Description |
|---|---|---|---|
| `realmRef` | string | Yes | Name of the target `Realm` CR in the same namespace. |
| `name` | string | Yes | Group name in Keycloak. |
| `path` | string | No | Optional Keycloak group path for nested group structures. |
| `attributes` | map[string]string[] | No | Multi-valued group attributes. |
| `realmRoles` | string[] | No | Realm roles assigned to the group. |

---

## Client

Clients are the most frequently changing resource — every application deployment may add or update one. The operator creates a Kubernetes Secret with the client credentials, ready to be mounted into the application pod.

```yaml
apiVersion: keycloak.opendefense.cloud/v1alpha1
kind: Client
metadata:
  name: odc-showcase-client
  namespace: keycloak-dev
spec:
  realmRef: tenant-a           # required; no fallback to master
  clientId: odc-showcase-client
  enabled: true
  redirectUris:
    - "https://showcase.example.com/*"
  webOrigins:
    - "https://showcase.example.com"
```

The operator automatically creates a secret named `<spec.clientId>-secret` (e.g. `odc-showcase-client-secret`) containing `CLIENT_ID` and `CLIENT_SECRET`. Mount it in your application:

```yaml
env:
  - name: OIDC_CLIENT_ID
    valueFrom:
      secretKeyRef:
        name: odc-showcase-client-secret
        key: CLIENT_ID
  - name: OIDC_CLIENT_SECRET
    valueFrom:
      secretKeyRef:
        name: odc-showcase-client-secret
        key: CLIENT_SECRET
```

```bash
kubectl get Clients -n keycloak-dev
# NAME                  REALM      CLIENTID               PROTOCOL        READY   AGE
# odc-showcase-client   tenant-a   odc-showcase-client    openid-connect  true    2m
```

### Spec Fields (Client)

| Field | Type | Required | Description |
|---|---|---|---|
| `realmRef` | string | Yes | Name of the target `Realm` CR in the same namespace. |
| `clientId` | string | Yes | Keycloak client identifier. |
| `name` | string | No | Display name. |
| `description` | string | No | Human-readable client description. |
| `enabled` | boolean | No | Whether the client is enabled. |
| `protocol` | string | No | Client protocol, typically `openid-connect`. |
| `publicClient` | boolean | No | If `true`, no client secret is generated or imported. |
| `standardFlowEnabled` | boolean | No | Enables authorization-code flow. |
| `implicitFlowEnabled` | boolean | No | Enables implicit flow. |
| `directAccessGrantsEnabled` | boolean | No | Enables direct access grants. |
| `serviceAccountsEnabled` | boolean | No | Enables service account support. |
| `redirectUris` | string[] | No | Allowed redirect URI patterns. |
| `webOrigins` | string[] | No | Allowed web origins. |

---

## User

```yaml
apiVersion: keycloak.opendefense.cloud/v1alpha1
kind: User
metadata:
  name: jane-doe
  namespace: keycloak-dev
spec:
  realmRef: tenant-a
  username: jane.doe
  email: jane.doe@example.com
  firstName: Jane
  lastName: Doe
  enabled: true
  emailVerified: false
  groups:
    - developers            # resolved to group ID at sync time
  attributes:
    department: ["engineering"]
  realmRoles:
    - offline_access
  clientRoles:
    account: ["view-profile", "manage-account"]
  initialPassword:
    secretName: jane-doe-initial-password
    secretKey: password     # default key name
```

### Spec Fields (User)

| Field | Type | Description |
|---|---|---|
| `realmRef` | string | Reference to the management realm. |
| `username` | string | Unique login name. |
| `email` | string | User email address. |
| `firstName` | string | User's first name. |
| `lastName` | string | User's last name. |
| `enabled` | boolean | Whether the user account is active. |
| `emailVerified` | boolean | Marks email as verified without requiring the flow. |
| `groups` | string[] | List of group names to join. |
| `attributes` | map[string]string[] | Custom metadata for the user profile. |
| `realmRoles` | string[] | List of realm-level roles to assign. |
| `clientRoles` | map[string]string[] | Map of client-id to roles to assign. |
| `initialPassword.secretName` | string | Name of the secret containing the password. |
| `initialPassword.secretKey` | string | Key within the secret. |

> [!TIP]
> **Declarative Roles & Attributes:** Unlike groups, **roles** and **attributes** defined in the CRD are strictly reconciled. Manual additions or deletions in the Keycloak UI will be overwritten by the operator.


Create the initial password Secret before applying the user CR:

```bash
kubectl create secret generic jane-doe-initial-password \
  --namespace keycloak-dev \
  --from-literal=password=ChangeMeNow!
```

```bash
kubectl get Users -n keycloak-dev
# NAME       REALM      USERNAME    EMAIL                    READY
# jane-doe   tenant-a   jane.doe    jane.doe@example.com     true
```

> `initialPassword` is written **only on user creation**. Changing the secret or spec after creation does not reset the Keycloak password — use the Admin Console or API for subsequent changes.

> Group membership is synced every reconciliation cycle. Adding a group to `spec.groups` adds the user to it on the next cycle.

---

## AuthFlow

Auth flows define custom login behavior in Keycloak (for example browser flow with MFA requirement). In this operator, the model is intentionally opinionated and toggle-based to reduce misconfiguration risk.

```yaml
apiVersion: keycloak.opendefense.cloud/v1alpha1
kind: AuthFlow
metadata:
  name: hardened-browser-flow
  namespace: keycloak-dev
spec:
  realmRef: tenant-a
  alias: "Hardened Browser Flow"
  description: "Browser flow requiring MFA"
  topLevel: true
  
  # Defense Profile Toggles
  requireMFA: true        # Enforces OTP (Google Authenticator / FreeOTP)
```

```bash
kubectl get AuthFlows -n keycloak-dev
# NAME                    REALM      ALIAS                  READY
# hardened-browser-flow   tenant-a   Hardened Browser Flow  true
```

### Spec Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `realmRef` | string | Yes | Target realm reference. |
| `alias` | string | Yes | Flow alias in Keycloak (must be unique in the realm). |
| `description` | string | No | Human-readable flow purpose. |
| `topLevel` | boolean | No | Marks this as a top-level flow. |
| `requireMFA` | boolean | No | If `true`, enforces OTP in the generated flow. |

If `realmRef` is omitted, the API server rejects the resource. This is intentional to prevent accidental writes into the privileged `master` realm.

`requireMFA: true` adds OTP as required execution in the generated flow. With `false`, the flow stays username/password only.

## IdentityProvider

Identity providers connect an external IdP (OIDC/SAML) to a realm. The operator creates, updates, and deletes the IdP definition in Keycloak.

```yaml
apiVersion: keycloak.opendefense.cloud/v1alpha1
kind: IdentityProvider
metadata:
  name: tenant-a-oidc-provider
  namespace: keycloak-dev
spec:
  realmRef: tenant-a
  alias: tenant-a-oidc
  type: oidc
  enabled: true
  displayName: "Tenant A Corporate OIDC"
  trustEmail: true
  config:
    authorizationUrl: "https://idp.corp.example.com/auth"
    tokenUrl: "https://idp.corp.example.com/token"
    clientId: "tenant-a-keycloak-client"
    defaultScope: "openid profile email"
```

### Spec Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `realmRef` | string | Yes | Target realm reference. |
| `alias` | string | Yes | Unique IdP alias inside the realm. |
| `type` | string | Yes | Protocol type: `oidc` or `saml`. |
| `enabled` | boolean | No | Whether the IdP is active. |
| `displayName` | string | No | Human-readable label shown on login pages. |
| `storeToken` | boolean | No | Stores brokered tokens in Keycloak. |
| `addReadTokenRoleOnCreate` | boolean | No | Adds read-token role on IdP creation. |
| `trustEmail` | boolean | No | Trusts email from upstream IdP without re-verification. |
| `linkOnly` | boolean | No | Allows linking only (no login button usage). |
| `firstBrokerLoginFlowAlias` | string | No | First-login broker flow alias. |
| `postBrokerLoginFlowAlias` | string | No | Post-login broker flow alias. |
| `config` | map[string]string | No | Provider-specific settings (for example endpoints and clientId). |
| `clientSecretRef.name` | string | Conditional | Secret name for OIDC client secret. |
| `clientSecretRef.key` | string | Conditional | Secret key for OIDC client secret value. |
| `signingCertificateRef.name` | string | Conditional | Secret name for SAML signing certificate. |
| `signingCertificateRef.key` | string | Conditional | Secret key for SAML signing certificate PEM. |

If `realmRef` is omitted, the API server rejects the resource. This is intentional to prevent accidental writes into the privileged `master` realm.

For OIDC, keep endpoint and client metadata in `spec.config` (for example `authorizationUrl`, `tokenUrl`, `clientId`, `defaultScope`) and place the sensitive `clientSecret` in `clientSecretRef`.

For SAML, keep protocol options in `spec.config` and provide certificate material via `signingCertificateRef`.

```bash
kubectl get IdentityProviders -n keycloak-dev
# NAME                     REALM      ALIAS            READY
# tenant-a-oidc-provider   tenant-a   tenant-a-oidc    true
```

---

## Backup and Restore (CNPG-native)

Database backup and restore are handled directly via CloudNativePG resources.
This keeps the Keycloak operator lightweight and avoids custom backup and restore controllers.

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Backup
metadata:
  name: pre-upgrade-backup
  namespace: keycloak-dev
spec:
  cluster:
    name: keycloak-db
  method: plugin
  pluginConfiguration:
    name: barman-cloud.cloudnative-pg.io
```

```bash
kubectl get backups.postgresql.cnpg.io -n keycloak-dev
# NAME                PHASE       READY   AGE
# pre-upgrade-backup  completed   true    5m
```

Restore is performed by applying a recovery CNPG `Cluster` manifest and then
cutting Keycloak over to the restored cluster host. See [UPGRADE.md](UPGRADE.md)
and [examples/restore-cluster-example.yaml](../examples/restore-cluster-example.yaml).

---

## CR Status and Conditions

Every resource managed by the operator exposes a `status` subresource. The `Realm` status is the authoritative result of the `keycloak-config-cli` Job because the Realm controller builds and applies the combined `realm.json` payload. Child resources (`Client`, `Group`, `User`, `ClientScope`, `AuthFlow`, `IdentityProvider`) currently report whether delegation to the Realm sync succeeded; they do not independently confirm Keycloak-side success after the Realm Job completes.

| Field | Type | Description |
|---|---|---|
| `status.ready` | boolean | For `Realm`, `true` when the config-cli Job completed successfully. For child resources, currently `false` with `JobRunning` after successful delegation to the Realm sync. |
| `status.keycloakId` | string | For `Realm`, set to the realm name on successful sync. Child resources currently do not populate Keycloak-internal UUIDs. |
| `status.message` | string | Human-readable summary of the last operation or error |
| `status.lastSyncTime` | string (RFC3339) | Timestamp of the last reconciliation attempt |
| `status.conditions` | array | Kubernetes-standard condition entries |

The `conditions` array contains a single `Ready` condition. For a `Realm`, the condition changes from `False/JobRunning` while the import Job is running to `True/Synced` after success, or `False/SyncFailed` after failure:

```bash
kubectl get Realms -n keycloak-dev -o wide
# NAME       REALMNAME   ENABLED   READY   OBSGEN   AGE
# tenant-a   tenant-a    true      true    3        2m

kubectl describe Realm tenant-a -n keycloak-dev
# Status:
#   Conditions:
#     Last Transition Time:  2026-03-04T10:00:00Z
#     Message:               Synced successfully
#     Reason:                Synced
#     Status:                True
#     Type:                  Ready
#   Keycloak Id:             tenant-a
#   Last Sync Time:          2026-03-04T10:00:00Z
#   Message:                 Synced successfully
#   Observed Generation:     3
#   Ready:                   true
```

After a child resource delegates successfully, expect a pending status similar to:

```bash
kubectl describe Client my-app -n keycloak-dev
# Status:
#   Conditions:
#     Message:  Delegated to Realm Sync - awaiting Job completion
#     Reason:   JobRunning
#     Status:   False
#     Type:     Ready
#   Ready:      false
```

If Keycloak is unreachable, the `Realm` Job fails and the Realm condition is set to `Ready=False` with the Job failure message. Once connectivity is restored, the next successful Realm reconciliation sets the Realm back to `Ready=True`.

---

## Troubleshooting

### Check resource status

```bash
kubectl get Realms,Clients,Groups,Users,ClientScopes,AuthFlows,IdentityProviders -n keycloak-dev
kubectl describe Realm tenant-a -n keycloak-dev
```

### Operator logs

```bash
kubectl logs -l app=keycloak-operator -n keycloak-dev --follow
```

### Common issues

| Symptom | Likely cause |
|---|---|
| `status.ready: false`, message contains `target Realm` | Realm in `realmRef` does not exist yet - apply the `Realm` CR first |
| Child CR remains `JobRunning` | Check the referenced `Realm` status and latest `keycloak-config-cli` Job for the authoritative import result |
| Client secret not created | Check operator logs; the client may be `publicClient: true` or secret creation failed |

---

## Related Documents

| Topic | Document |
|---|---|
| Architecture overview | [ARCHITECTURE.md](ARCHITECTURE.md) |
| Operator strategy & ADR | [CLIENT.md](CLIENT.md) |
| PostgreSQL with CloudNativePG | [DATABASE.md](DATABASE.md) |
| Deployment guide | [DEPLOYMENT.md](DEPLOYMENT.md) |
| Observability (OTEL, Prometheus, alerts) | [OBSERVABILITY.md](OBSERVABILITY.md) |
| Upgrade runbook & backup/restore | [UPGRADE.md](UPGRADE.md) |
| Security hardening & CIS controls | [HARDENING.md](HARDENING.md) |
