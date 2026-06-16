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
#   OCM_TRANSFER_COPY_RESOURCES=true|false
#     - true:  localize referenced resources during transfer (default, air-gap ready)
#     - false: keep referential resource access unchanged
#
# EXAMPLES:
#   OCM_REGISTRY=ghcr.io/my-org ./scripts/ocm/ocm-transfer.sh
#   OCM_REGISTRY=ghcr.io/my-org ./scripts/ocm/ocm-transfer.sh ./my-archive
#   ./scripts/ocm/ocm-transfer.sh ocm-output/keycloak-bundle-ctf.tar.gz ghcr.io/my-org --user admin
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/../utils/common.sh"

PROJECT_ROOT="$(cd "$(dirname "$(dirname "$SCRIPT_DIR")")" && pwd)"
DEFAULT_ARCHIVE="$PROJECT_ROOT/ocm-output/keycloak-bundle-ctf.tar.gz"

select_default_registry() {
    if [[ -n "${OCM_REGISTRY:-}" ]]; then
        echo "$OCM_REGISTRY"
        return
    fi

    fail "Error: set OCM_REGISTRY or pass target-registry as the second argument." 2
}

# Defaults
ARCHIVE_PATH=""
TARGET_REGISTRY=""
REGISTRY_USER="${OCM_USER:-}"
REGISTRY_PASSWORD="${OCM_PASSWORD:-}"
IMMUTABLE_TRANSFER="${OCM_TRANSFER_IMMUTABLE:-false}"
COPY_RESOURCES="${OCM_TRANSFER_COPY_RESOURCES:-true}"

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
            elif [[ -z "$TARGET_REGISTRY" ]]; then
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
if [[ -z "$TARGET_REGISTRY" ]]; then
    TARGET_REGISTRY="$(select_default_registry)"
fi

IMMUTABLE_TRANSFER="$(echo "$IMMUTABLE_TRANSFER" | tr '[:upper:]' '[:lower:]')"
if [[ "$IMMUTABLE_TRANSFER" != "true" && "$IMMUTABLE_TRANSFER" != "false" ]]; then
    fail "Invalid OCM_TRANSFER_IMMUTABLE value '$IMMUTABLE_TRANSFER' (expected true|false)." 1
fi

COPY_RESOURCES="$(echo "$COPY_RESOURCES" | tr '[:upper:]' '[:lower:]')"
if [[ "$COPY_RESOURCES" != "true" && "$COPY_RESOURCES" != "false" ]]; then
    fail "Invalid OCM_TRANSFER_COPY_RESOURCES value '$COPY_RESOURCES' (expected true|false)." 1
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
    read -r -p "Registry Username: " REGISTRY_USER
fi

if [[ -z "$REGISTRY_PASSWORD" ]]; then
    read -r -s -p "Registry Password: " REGISTRY_PASSWORD
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
if [[ "$COPY_RESOURCES" == "true" ]]; then
    info "Resource transfer: copy-resources enabled (referential resources will be localized)."
    TRANSFER_FLAGS+=(--copy-resources)
else
    warn "Resource transfer: copy-resources disabled (referential resources remain external)."
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
