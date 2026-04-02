#!/bin/bash
# ==============================================================================
# common.sh - Shared library for all Keycloak deployment scripts
# ==============================================================================
#
# PURPOSE:
#   Provides common logging, error handling, and utility functions used by all
#   other scripts in this directory. This is a pure library file - it should be
#   sourced, not executed directly.
#
# USAGE:
#   source "$SCRIPT_DIR/common.sh"
#
# FUNCTIONS:
#   info "message"        - Print informational message ([INFO] prefix)
#   warn "message"        - Print warning message ([WARN] prefix)
#   fail "message" [code] - Print error message and exit with code (default: 1)
#   generate_suffix       - Generate a random 5-character suffix (like K8s)
#
# ==============================================================================
#
# OCM Registry (override via environment variable for local testing)
# Example: export OCM_REGISTRY="my-registry.example.com/keycloak-bundle"
OCM_REGISTRY="${OCM_REGISTRY:-ghcr.io/opendefensecloud/keycloak-bundle}"

# Logging functions
info() {
    echo "[INFO] $*"
}

warn() {
    echo "[WARN] $*"
}

fail() {
    local code="${2:-1}"
    echo "[FAIL] $*" >&2
    exit "$code"
}

# Generate a random 5-character suffix (lowercase alphanumeric, like Kubernetes)
# Usage: SUFFIX=$(generate_suffix)
generate_suffix() {
    cat /dev/urandom | tr -dc 'a-z0-9' | fold -w 5 | head -n 1
}
