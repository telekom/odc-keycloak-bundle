#!/bin/bash
# ==============================================================================
# ocm-validate.sh - Validate an OCM component archive
# ==============================================================================
#
# PURPOSE:
#   Inspects and validates an OCM component archive by listing component
#   versions and resources. Optionally runs a full describe (requires
#   registry/OCI access).
#
# USAGE:
#   ./scripts/ocm/ocm-validate.sh [archive-path] [--full] [--public-key <path>]
#
# ARGUMENTS:
#   archive-path   Optional. Path to the component archive (default: ocm-output/component-archive)
#   --full         Optional. Also run 'ocm describe' (requires OCI registry access)
#   --public-key   Optional. Verify signature using this public key
#
# EXAMPLES:
#   ./scripts/ocm/ocm-validate.sh                          # Basic validation
#   ./scripts/ocm/ocm-validate.sh --full                   # With full describe
#   ./scripts/ocm/ocm-validate.sh gen/ctf                  # Custom archive path
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

ARCHIVE_PATH="ocm-output/component-archive"
FULL_DESCRIBE=false
PUBLIC_KEY_PATH=""
SIGNATURE_NAME="${OCM_SIGNATURE_NAME:-keycloak-bundle-sig}"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --full)
            FULL_DESCRIBE=true
            shift
            ;;
        --public-key)
            PUBLIC_KEY_PATH="$2"
            shift 2
            ;;
        *)
            ARCHIVE_PATH="$1"
            shift
            ;;
    esac
done

# Validate archive exists
if [[ ! -d "$ARCHIVE_PATH" ]]; then
    fail "Component archive not found: $ARCHIVE_PATH (run ocm-create.sh first)"
fi

info "Validating component archive: $ARCHIVE_PATH"

echo ""
echo "=== Component Versions ==="
ocm get componentversions "$ARCHIVE_PATH"

echo ""
echo "=== Resources ==="
ocm get resources "$ARCHIVE_PATH"

if [[ -n "$PUBLIC_KEY_PATH" ]]; then
    echo ""
    echo "=== Signature Verification ==="
    ocm verify componentversions \
        --signature "$SIGNATURE_NAME" \
        --public-key "$PUBLIC_KEY_PATH" \
        "$ARCHIVE_PATH"
fi

if [[ "$FULL_DESCRIBE" == "true" ]]; then
    echo ""
    echo "=== Full Component Descriptor ==="
    ocm describe component "$ARCHIVE_PATH"
fi

info "Validation complete."
