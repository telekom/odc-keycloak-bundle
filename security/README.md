# OCM Signing Keys

This directory contains the public verification key for OCM component signatures.

- Public key file: `ocm-signing-public-key.pub`
- Private key: never commit to git

## Usage

Use the public key to verify signed component archives:

```bash
./scripts/ocm/ocm-verify.sh ocm-output/keycloak-bundle-ctf.tar.gz security/ocm-signing-public-key.pub
```

## Rotation

Generate a new key pair locally:

```bash
mkdir -p tmp
umask 077
ocm create rsakeypair tmp/ocm-signing-shared.priv
cp tmp/ocm-signing-shared.pub security/ocm-signing-public-key.pub
```

Then update CI secrets and repository key:

1. Update GitHub secret `OCM_SIGNING_PRIVATE_KEY` with `tmp/ocm-signing-shared.priv`.
Optional with GitHub CLI: `gh secret set OCM_SIGNING_PRIVATE_KEY < tmp/ocm-signing-shared.priv`
2. Commit and push `security/ocm-signing-public-key.pub`.
3. Run CI and confirm sign + verify stages succeed.
4. Securely delete temporary private key files.

Example cleanup:

```bash
shred -u tmp/ocm-signing-shared.priv 2>/dev/null || rm -f tmp/ocm-signing-shared.priv
```