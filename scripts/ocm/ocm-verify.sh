#!/bin/bash
# ==============================================================================
# ocm-verify.sh - Verify signed OCM component archive with public key
# ==============================================================================
#
# USAGE:
#   ./scripts/ocm/ocm-verify.sh [ctf-path] [public-key-path] [signature-name]
#
# EXAMPLES:
#   ./scripts/ocm/ocm-verify.sh
#   ./scripts/ocm/ocm-verify.sh ocm-output/keycloak-bundle-ctf.tar.gz ocm-key.pub
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

CTF_PATH="${1:-ocm-output/keycloak-bundle-ctf.tar.gz}"
PUBLIC_KEY_PATH="${2:-${OCM_PUBLIC_KEY_PATH:-ocm-key.pub}}"
SIGNATURE_NAME="${3:-${OCM_SIGNATURE_NAME:-keycloak-bundle-sig}}"

if ! command -v ocm &>/dev/null; then
    fail "OCM CLI not found. Install from: https://ocm.software" 1
fi

if [[ ! -f "$PUBLIC_KEY_PATH" ]]; then
    fail "Public key not found: $PUBLIC_KEY_PATH" 2
fi

if [[ ! -f "$CTF_PATH" && ! -d "$CTF_PATH" ]]; then
    fail "CTF path not found: $CTF_PATH" 3
fi

TARGET_PATH="$CTF_PATH"
TEMP_DIR=""

cleanup() {
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        rm -rf "$TEMP_DIR"
    fi
}
trap cleanup EXIT

if [[ -f "$CTF_PATH" ]]; then
    TEMP_DIR="$(mktemp -d)"
    case "$CTF_PATH" in
        *.tar.gz|*.tgz)
            tar -xzf "$CTF_PATH" -C "$TEMP_DIR" || fail "Failed to extract CTF tar.gz: $CTF_PATH" 4
            ;;
        *.tar)
            tar -xf "$CTF_PATH" -C "$TEMP_DIR" || fail "Failed to extract CTF tar: $CTF_PATH" 4
            ;;
        *)
            fail "Unsupported CTF file format: $CTF_PATH (expected .tar/.tar.gz/.tgz or extracted directory)" 4
            ;;
    esac
    TARGET_PATH="$TEMP_DIR"
fi

info "Verifying OCM component signature..."
info "  CTF:       $CTF_PATH"
info "  Signature: $SIGNATURE_NAME"
info "  PublicKey: $PUBLIC_KEY_PATH"

ocm verify componentversions \
    --signature "$SIGNATURE_NAME" \
    --public-key "$PUBLIC_KEY_PATH" \
    "$TARGET_PATH" || fail "Signature verification failed" 5

info "Signature verification successful."
