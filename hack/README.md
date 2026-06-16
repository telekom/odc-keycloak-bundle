# hack/

Scripts in this directory are for local development only. They are not called by
CI.

## Contents

| Script | Purpose |
|--------|---------|
| `generate-test-secrets.sh` | Generate throwaway Kubernetes Secret manifests for local testing. |

## Dev Registry Setup

Set `OCM_REGISTRY` in your shell or a local `.env` file. Use the registry
namespace only, for example:

```bash
export OCM_REGISTRY=ghcr.io/telekom
```

Use `make docker-push-dev` only as a local convenience helper. It pushes a local
operator image to the development package, `${OCM_REGISTRY}/keycloak-operator-dev:dev`.
Release images and OCM release transfers are produced by GitHub Actions, not by local
developer machines.

```bash
make docker-push-dev
```

Local OCM transfers use the same `OCM_REGISTRY` value. Use this only for intentional
development testing; release transfer is handled by GitHub Actions and is restricted
to non-dev repositories on `main`.

```bash
./scripts/ocm/ocm-transfer.sh --user <user> --password <token>
```

## Secret hygiene

- Never commit real passwords, tokens, private keys, or generated Secret
  manifests.
- Use `make generate-test-secrets` to regenerate throwaway test credentials
  locally.
- Run `gitleaks detect --source . --verbose` before pushing.
