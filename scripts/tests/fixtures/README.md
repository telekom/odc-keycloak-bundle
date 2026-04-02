# CRD Test Fixtures

This directory contains deterministic fixtures used only by the CRD integration suites.

## Why separate fixtures instead of reusing examples/

- `examples/` are user-facing reference manifests and may evolve for documentation clarity.
- `scripts/tests/fixtures/` are test-facing manifests and must stay stable for CI reproducibility.
- Decoupling avoids accidental CI regressions when documentation examples are simplified or adapted.

## Naming convention

- Files are named `<kind>-<purpose>.yaml`.
- `ci-test-*` values in object names are intentional to avoid collision with user-managed resources in shared CI namespaces.

## Scope

- `realm-master.yaml` - foundation realm for test orchestration
- `clientscope-ci-test-scope.yaml` - smoke scope
- `group-ci-test-group.yaml` - smoke group
- `client-odc-showcase.yaml` - smoke client
- `secret-ci-test-user-password.yaml` - test secret for user bootstrap
- `user-ci-test-user.yaml` - smoke user and group membership check
- `identityprovider-ci-test-oidc.yaml` - lifecycle test target
- `authflow-ci-test-browser-mfa.yaml` - lifecycle test target

If fields in CRDs change, update fixtures and examples independently.
