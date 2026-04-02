#!/bin/bash
# ==============================================================================
# ocm-transfer.sh - Transfer OCM component archive to target registry
# ==============================================================================
#
# PURPOSE:
#   Uploads the locally created OCM component archive (CTF) to a remote OCI registry.
#   This is the "Import" step in an Air-Gap scenario.
#
# USAGE:
#   ./scripts/ocm/ocm-transfer.sh [archive-path] [target-registry]
#
# ARGUMENTS:
#   archive-path   Optional. Path to the component archive (CTF tarball)
#                  (default: ocm-output/keycloak-bundle-ctf.tar.gz)
#   target-registry Optional. Target OCI registry to push the archive to.
#                   (default: value of OCM_REGISTRY env var)
#
# OPTIONS:
#   --user <user>       Registry username (or set OCM_USER env var)
#   --password <pass>   Registry password (or set OCM_PASSWORD env var)
#   --immutable         Fail if component version already exists in target registry
#   --overwrite         Overwrite existing component version (default)
#
# ENV:
#   OCM_TRANSFER_IMMUTABLE=true|false
#     - true:  immutable transfer (no overwrite)
#     - false: allow overwrite (default)
#
# EXAMPLES:
#   ./scripts/ocm/ocm-transfer.sh
#   ./scripts/ocm/ocm-transfer.sh ./my-archive
#   ./scripts/ocm/ocm-transfer.sh ocm-output/keycloak-bundle-ctf.tar.gz my-registry.com/repo --user admin
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

PROJECT_ROOT="$(cd "$(dirname "$(dirname "$SCRIPT_DIR")")" && pwd)"
DEFAULT_ARCHIVE="$PROJECT_ROOT/ocm-output/keycloak-bundle-ctf.tar.gz"

# Defaults
ARCHIVE_PATH=""
TARGET_REGISTRY="$OCM_REGISTRY"
REGISTRY_USER="${OCM_USER:-}"
REGISTRY_PASSWORD="${OCM_PASSWORD:-}"
IMMUTABLE_TRANSFER="${OCM_TRANSFER_IMMUTABLE:-false}"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --user)
            REGISTRY_USER="$2"
            shift 2
            ;;
        --password)
            REGISTRY_PASSWORD="$2"
            shift 2
            ;;
        --immutable)
            IMMUTABLE_TRANSFER="true"
            shift
            ;;
        --overwrite)
            IMMUTABLE_TRANSFER="false"
            shift
            ;;
        *)
            if [[ -z "$ARCHIVE_PATH" ]]; then
                ARCHIVE_PATH="$1"
            elif [[ "$TARGET_REGISTRY" == "$OCM_REGISTRY" ]]; then # Only override if not already set or default
                TARGET_REGISTRY="$1"
            fi
            shift
            ;;
    esac
done

# Set default path if not provided
if [[ -z "$ARCHIVE_PATH" ]]; then
    ARCHIVE_PATH="$DEFAULT_ARCHIVE"
fi

IMMUTABLE_TRANSFER="$(echo "$IMMUTABLE_TRANSFER" | tr '[:upper:]' '[:lower:]')"
if [[ "$IMMUTABLE_TRANSFER" != "true" && "$IMMUTABLE_TRANSFER" != "false" ]]; then
    fail "Invalid OCM_TRANSFER_IMMUTABLE value '$IMMUTABLE_TRANSFER' (expected true|false)." 1
fi

info "=== OCM Component Transfer ==="
info "Archive:  $ARCHIVE_PATH"
info "Target:   $TARGET_REGISTRY"

# Validate Archive
if [[ ! -d "$ARCHIVE_PATH" && ! -f "$ARCHIVE_PATH" ]]; then
    fail "Archive path not found: $ARCHIVE_PATH" 1
fi

# Credentials Prompt
if [[ -z "$REGISTRY_USER" ]]; then
    read -p "Registry Username: " REGISTRY_USER
fi

if [[ -z "$REGISTRY_PASSWORD" ]]; then
    read -s -p "Registry Password: " REGISTRY_PASSWORD
    echo ""
fi

if [[ -z "$REGISTRY_USER" || -z "$REGISTRY_PASSWORD" ]]; then
    fail "Credentials required." 2
fi

info "Transferring archive (CTF)..."
REGISTRY_HOST=$(echo "$TARGET_REGISTRY" | cut -d'/' -f1)

TRANSFER_FLAGS=()
if [[ "$IMMUTABLE_TRANSFER" == "true" ]]; then
    info "Transfer mode: immutable (existing versions are not overwritten)."
else
    warn "Transfer mode: overwrite enabled (existing versions may be replaced)."
    TRANSFER_FLAGS+=(--overwrite)
fi

if ! ocm --cred :type=OCIRegistry \
    --cred :hostname="$REGISTRY_HOST" \
    --cred username="$REGISTRY_USER" \
    --cred password="$REGISTRY_PASSWORD" \
    transfer ctf "$ARCHIVE_PATH" "$TARGET_REGISTRY" \
    "${TRANSFER_FLAGS[@]}"; then
    if [[ "$IMMUTABLE_TRANSFER" == "true" ]]; then
        fail "Transfer failed in immutable mode. If the version already exists, bump component version or rerun with --overwrite." 3
    fi
    fail "Transfer failed" 3
fi

info "Transfer successful!"
