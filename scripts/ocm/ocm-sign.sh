#!/bin/bash
# ==============================================================================
# ocm-sign.sh - Sign and verify an OCM component archive
# ==============================================================================
#
# PURPOSE:
#   Signs an OCM component archive with an RSA keypair and verifies the
#   signature. In CI, keys must be supplied securely via secrets.
#
# USAGE:
#   ./scripts/ocm/ocm-sign.sh [archive-path] [private-key-path] [public-key-path]
#
# ARGUMENTS:
#   archive-path       Optional. Path to the component archive (default: ocm-output/component-archive)
#   private-key-path   Optional. Path to private key (default: $OCM_PRIVATE_KEY_PATH or ocm-key.priv)
#   public-key-path    Optional. Path to public key  (default: $OCM_PUBLIC_KEY_PATH or ocm-key.pub)
#
# EXAMPLES:
#   ./scripts/ocm/ocm-sign.sh
#   ./scripts/ocm/ocm-sign.sh ocm-output/component-archive
#   ./scripts/ocm/ocm-sign.sh ocm-output/component-archive ./ocm-key.priv ./ocm-key.pub
#
# OUTPUT:
#   - <key-name>.priv  : RSA private key (generated if missing)
#   - <key-name>.pub   : RSA public key  (generated if missing)
#   - Signed component archive with signature "keycloak-bundle-sig"
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

ARCHIVE_PATH="${1:-ocm-output/component-archive}"
PRIVATE_KEY_PATH="${2:-${OCM_PRIVATE_KEY_PATH:-ocm-key.priv}}"
PUBLIC_KEY_PATH="${3:-${OCM_PUBLIC_KEY_PATH:-ocm-key.pub}}"
SIGNATURE_NAME="${OCM_SIGNATURE_NAME:-keycloak-bundle-sig}"
REQUIRE_EXISTING_KEY="${REQUIRE_EXISTING_KEY:-false}"

if [[ "${CI:-}" == "true" ]]; then
    REQUIRE_EXISTING_KEY=true
fi

if ! command -v ocm &>/dev/null; then
    fail "OCM CLI not found. Install from: https://ocm.software" 1
fi

# Validate archive exists
if [[ ! -d "$ARCHIVE_PATH" ]]; then
    fail "Component archive not found: $ARCHIVE_PATH (run ocm-create.sh first)"
fi

# Generate keypair only for local workflows. CI must provide keys via secrets.
if [[ ! -f "$PRIVATE_KEY_PATH" || ! -f "$PUBLIC_KEY_PATH" ]]; then
    if [[ "$REQUIRE_EXISTING_KEY" == "true" ]]; then
        fail "Signing keys missing. Provide PRIVATE='$PRIVATE_KEY_PATH' and PUBLIC='$PUBLIC_KEY_PATH' via CI secrets." 4
    fi

    info "Generating RSA keypair for local use: $PRIVATE_KEY_PATH / $PUBLIC_KEY_PATH"
    ocm create rsakeypair "$PRIVATE_KEY_PATH"

    GENERATED_PUBLIC="${PRIVATE_KEY_PATH%.priv}.pub"
    if [[ "$GENERATED_PUBLIC" != "$PUBLIC_KEY_PATH" && -f "$GENERATED_PUBLIC" ]]; then
        cp "$GENERATED_PUBLIC" "$PUBLIC_KEY_PATH"
    fi
fi

chmod 600 "$PRIVATE_KEY_PATH" 2>/dev/null || true

# Sign
info "Signing component archive..."
ocm sign componentversions \
    --signature "$SIGNATURE_NAME" \
    --private-key "$PRIVATE_KEY_PATH" \
    "$ARCHIVE_PATH"

# Verify
info "Verifying signature..."
ocm verify componentversions \
    --signature "$SIGNATURE_NAME" \
    --public-key "$PUBLIC_KEY_PATH" \
    "$ARCHIVE_PATH"

# Repackage as CTF tarball if likely in standard output structure
PARENT_DIR="$(dirname "$ARCHIVE_PATH")"
if [[ -f "$PARENT_DIR/keycloak-bundle-ctf.tar.gz" ]]; then
    info "Updating CTF tarball with signature..."
    tar -czf "$PARENT_DIR/keycloak-bundle-ctf.tar.gz" -C "$ARCHIVE_PATH" .
fi

info "Component signed and verified successfully."
info "  Signature : $SIGNATURE_NAME"
info "  Public key: $PUBLIC_KEY_PATH"
