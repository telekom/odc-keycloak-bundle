# OCM Signing Key Rotation Runbook

This document describes how to rotate the OCM component signing key without
interrupting ongoing deployments.

---

## When to rotate

- Suspected or confirmed key compromise.
- Scheduled rotation policy (e.g. annually).
- Team member with key access offboards.
- Secret scanning alert fires on the private key.

---

## Rotation procedure

### 1. Generate a new keypair

Using the OCM CLI:
```bash
ocm create rsakeypair ocm-key-new.priv ocm-key-new.pub
```

Or using `openssl`:
```bash
openssl genrsa -out ocm-key-new.priv 4096
openssl rsa -in ocm-key-new.priv -pubout -out ocm-key-new.pub
```

### 2. Stage the new public key

Copy the new public key into the repository alongside the existing one so that
both keys are valid during the transition window:
```bash
cp security/ocm-signing-public-key.pub security/ocm-signing-public-key-old.pub
cp ocm-key-new.pub security/ocm-signing-public-key.pub
```

Commit and merge this change so CI uses the new public key for verification.

### 3. Update the GitHub Actions secret

In the repository settings, update the `OCM_SIGNING_PRIVATE_KEY` secret to the
content of `ocm-key-new.priv`:
```bash
gh secret set OCM_SIGNING_PRIVATE_KEY --repo <org>/<repo> < ocm-key-new.priv
```

### 4. Re-sign the latest component archive

Trigger a new CI run or re-sign locally:
```bash
./scripts/ocm/ocm-sign.sh ocm-output/component-archive ocm-key-new.priv security/ocm-signing-public-key.pub
```

Verify the new signature:
```bash
./scripts/ocm/ocm-verify.sh ocm-output/keycloak-bundle-ctf.tar.gz security/ocm-signing-public-key.pub
```

### 5. Remove the old public key

Once all artifacts in the registry have been re-signed and verified, remove the
old public key:
```bash
git rm security/ocm-signing-public-key-old.pub
```

Commit and merge. From this point, signatures from the old key will no longer
be accepted.

### 6. Destroy the old private key

Securely delete any local copies of the old private key:
```bash
shred -u ocm-key-old.priv 2>/dev/null || rm -f ocm-key-old.priv
```

---

## Verification

```bash
# Verify a signed archive against the current public key
./scripts/ocm/ocm-verify.sh <path-to-ctf.tar.gz> security/ocm-signing-public-key.pub
```

A successful verification prints:
```
Successfully verified signature for component "opendefense.cloud/keycloak-bundle"
```

---

## Notes

- The private key must **never** be committed to the repository. It lives only in
  the `OCM_SIGNING_PRIVATE_KEY` GitHub Actions secret and is materialised into a
  temporary file during CI (automatically shredded after use).
- All signing operations in CI run with `if: github.event_name != 'pull_request'`
  so fork PRs cannot access the key.
