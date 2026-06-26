# CEL Validation Rules

[Common Expression Language (CEL)](https://kubernetes.io/docs/reference/using-api/cel/) rules are embedded directly in the CRD schemas and evaluated by `kube-apiserver` at admission time — before any controller reconciles a resource. They enforce cross-field constraints that cannot be expressed with `type`, `pattern`, or `enum` constraints alone.

All rules produce a human-readable rejection message. No webhook is required.

---

## Operator CRDs

Rules are defined as `+kubebuilder:validation:XValidation` markers in `operator/api/v1alpha1/` and compiled into `x-kubernetes-validations` blocks in the generated CRD YAML by `make manifests`.

### Realm (`realm_types.go`)

| Rule | Message |
|------|---------|
| `internationalizationEnabled: true` requires `supportedLocales` to be non-empty | `supportedLocales must not be empty when internationalizationEnabled is true` |
| `internationalizationEnabled: true` requires `defaultLocale` to be set | `defaultLocale must be set when internationalizationEnabled is true` |

### Client (`client_types.go`)

| Rule | Message |
|------|---------|
| `publicClient` and `serviceAccountsEnabled` cannot both be `true` | `publicClient and serviceAccountsEnabled are mutually exclusive` |
| `standardFlowEnabled: true` requires at least one entry in `redirectUris` | `redirectUris must not be empty when standardFlowEnabled is true` |

### IdentityProvider (`identityprovider_types.go`)

| Rule | Message |
|------|---------|
| `type: oidc` requires `config` to contain a non-empty `issuerUrl` or `authorizationUrl` | `config must contain non-empty issuerUrl or authorizationUrl for OIDC identity providers` |
| `type: saml` requires `config.singleSignOnServiceUrl` to be non-empty | `config.singleSignOnServiceUrl is required for SAML identity providers` |

### AuthFlow (`authflow_types.go`)

| Rule | Message |
|------|---------|
| `alias` is immutable after creation (field-level `oldSelf` rule, k8s ≥ 1.28) | `alias is immutable after creation` |

Renaming an auth flow after creation would orphan all realm references that bind flows by alias. The immutability guard prevents this at admission rather than at reconcile time.

### User / InitialPasswordRef (`user_types.go`)

| Rule | Message |
|------|---------|
| `initialPassword.secretName` must be non-empty (schema `minLength: 1` + CEL) | `initialPassword.secretName must not be empty` |

---

## KRO ResourceGraphDefinition

Rules are defined directly in `kro/rgd/keycloak-instance-rgd.yaml` under `spec.schema.spec.x-kubernetes-validations`. KRO passes these through to the generated `KeycloakInstance` CRD.

| Rule | Message |
|------|---------|
| `dbInstances` must be 1 or an odd number | `dbInstances must be 1 or odd (3, 5, ...) for CNPG quorum` |
| `hostname`, when non-empty, must be a valid lowercase DNS hostname | `hostname must be a valid lowercase DNS hostname when set` |
| `dbStorageSize` must be a valid Kubernetes Quantity | `dbStorageSize must be a valid Kubernetes Quantity (e.g. 5Gi)` |

The `dbInstances` rule prevents even values (2, 4, …) that would create a CNPG cluster without a Raft quorum majority, leading to split-brain on failure.

---

## Regenerating CRDs

After editing any `+kubebuilder:validation:XValidation` marker in `operator/api/v1alpha1/`, run:

```bash
make manifests
```

Commit both the Go source change and the updated files under `charts/keycloak-operator/crds/` in the same PR. The CI "Verify CRD Manifests" step fails if they diverge.

---

## Testing

### CI job: "2a. CEL Validation Tests"

A dedicated CI job runs on every push and PR (parallel to the operator build). It:

1. Spins up a `kind` cluster
2. Applies all CRDs from `charts/keycloak-operator/crds/`
3. Runs `scripts/tests/test-cel-validation.sh`

The script applies fixtures from `tests/cel/` and asserts:

- **`tests/cel/invalid/`** — each file must be **rejected** by kube-apiserver with the expected error substring
- **`tests/cel/valid/`** — each file must be **accepted**
- **Alias immutability** — creates the valid AuthFlow then re-applies the changed-alias variant; expects rejection

### Running locally

```bash
# against any cluster with the CRDs installed:
scripts/tests/test-cel-validation.sh

# against a fresh local kind cluster:
kind create cluster
kubectl apply -f charts/keycloak-operator/crds/
scripts/tests/test-cel-validation.sh
kind delete cluster
```

### Adding a new rule

1. Add the `+kubebuilder:validation:XValidation` marker to the relevant struct or field in `operator/api/v1alpha1/`.
2. Run `make manifests`.
3. Add an `invalid/` fixture that violates the rule and a `valid/` fixture that satisfies it.
4. Add the corresponding `assert_rejected` / `assert_accepted` calls in `scripts/tests/test-cel-validation.sh`.
