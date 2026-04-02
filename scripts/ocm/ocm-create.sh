#!/bin/bash
# ==============================================================================
# ocm-create.sh - Create OCM component archive for air-gapped deployment
# ==============================================================================
#
# PURPOSE:
#   Creates an Open Component Model (OCM) component archive that bundles
#   all container images and manifests needed for air-gapped deployment.
#   The archive can be transferred to disconnected environments.
#
# USAGE:
#   ./scripts/ocm/ocm-create.sh [output-directory]
#
# ARGUMENTS:
#   output-directory   Optional. Where to create the archive
#                      (default: ./ocm-output)
#
# EXAMPLES:
#   ./scripts/ocm/ocm-create.sh                    # Create in ./ocm-output
#   ./scripts/ocm/ocm-create.sh /tmp/ocm-bundle    # Custom location
#
# PREREQUISITES:
#   - OCM CLI must be installed (https://ocm.software)
#   - Network access to pull container images
#
# BUNDLES:
#   See component-constructor.yaml for list of bundled images and resources.
#
# TRANSFER TO AIR-GAPPED:
#   Use scripts/ocm/ocm-transfer.sh
#
# SEE ALSO:
#   component-constructor.yaml - Component metadata
#   https://ocm.software          - OCM documentation
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

PROJECT_ROOT="$(cd "$(dirname "$(dirname "$SCRIPT_DIR")")" && pwd)"
OUTPUT_DIR="${1:-$PROJECT_ROOT/ocm-output}"
SBOM_FILE="$PROJECT_ROOT/ocm-sbom.cdx.json"
# Component name and version are defined in component-constructor.yaml

cleanup_temp_files() {
    rm -f "$PROJECT_ROOT/manifests.tar"
}

trap cleanup_temp_files EXIT

# Check for ocm CLI
if ! command -v ocm &>/dev/null; then
    fail "OCM CLI not found. Install from: https://ocm.software" 1
fi

info "Creating OCM component archive..."
info "Output: $OUTPUT_DIR"

mkdir -p "$OUTPUT_DIR"

cd "$PROJECT_ROOT"

# WORKAROUND: Create manual tarball to avoid OCM CLI file locking issues on Windows
# The OCM CLI fails to cleanup temp files when internal compression is used on Windows.
info "Creating manifests.tar (workaround for Windows)..."
tar -cf manifests.tar -C manifests . || fail "Failed to create manifests.tar" 8

info "Generating CycloneDX SBOM..."
if command -v syft &>/dev/null; then
    syft "dir:$PROJECT_ROOT" -o "cyclonedx-json=$SBOM_FILE" || fail "Failed to generate SBOM with syft" 10
elif command -v trivy &>/dev/null; then
    trivy fs --quiet --format cyclonedx --output "$SBOM_FILE" "$PROJECT_ROOT" || fail "Failed to generate SBOM with trivy" 10
else
    fail "SBOM generator not found. Install syft or trivy before running ocm-create.sh" 10
fi

# CLEANUP: Remove existing archive to prevent appending to old components
if [[ -d "$OUTPUT_DIR/component-archive" ]]; then
    info "Removing old component archive..."
    rm -rf "$OUTPUT_DIR/component-archive"
fi

# Create component archive using declarative constructor
info "Adding component version from constructor..."
ocm add componentversions --create --file "$OUTPUT_DIR/component-archive" "component-constructor.yaml" \
    || fail "Failed to create component archive from constructor" 2

info "Creating CTF tarball..."
tar -czf "$OUTPUT_DIR/keycloak-bundle-ctf.tar.gz" -C "$OUTPUT_DIR/component-archive" . \
    || fail "Failed to create CTF tarball" 9

info "=== OCM component archive created ==="
info "Location: $OUTPUT_DIR/component-archive"
info "CTF TGZ:  $OUTPUT_DIR/keycloak-bundle-ctf.tar.gz"
info ""
info "To transfer to air-gapped registry:"
info "  ./scripts/ocm/ocm-transfer.sh $OUTPUT_DIR/keycloak-bundle-ctf.tar.gz"
