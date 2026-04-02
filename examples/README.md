# Examples

This directory contains user-facing reference manifests.

## Scope

- These files are meant for documentation and manual usage.
- They should be generally understandable and reusable in real environments.
- They must not rely on CI-only naming conventions.

## Air-gapped usage guidance

- Replace all external host placeholders (for example `*.example.com`) with internal endpoints.
- Never commit plaintext secrets; use Kubernetes Secrets and secret references.
- Keep `namespace` and `realmRef` aligned with your deployed Keycloak instance.

For deterministic CI integration test inputs, see `scripts/tests/fixtures/`.
