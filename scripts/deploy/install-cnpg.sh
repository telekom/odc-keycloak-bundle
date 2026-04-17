#!/bin/bash
# ==============================================================================
# install-cnpg.sh - Install CloudNativePG operator (cluster-wide)
# ==============================================================================
#
# PURPOSE:
#   Installs the CloudNativePG operator which provides PostgreSQL database
#   clusters for Kubernetes. This is a one-time, cluster-wide installation.
#
# USAGE:
#   ./scripts/deploy/install-cnpg.sh [version]
#
# ARGUMENTS:
#   version   Optional. CloudNativePG version (default: "1.28.1")
#
# EXAMPLES:
#   ./scripts/deploy/install-cnpg.sh              # Install v1.28.1
#   ./scripts/deploy/install-cnpg.sh 1.29.0       # Install specific version
#
# CREATES:
#   - Namespace "cnpg-system"
#   - CRDs for CloudNativePG (clusters.postgresql.cnpg.io, etc.)
#   - Operator deployment in cnpg-system
#
# NOTES:
#   - Requires cluster-admin permissions
#   - Only needs to be run once per cluster
#   - deploy-all.sh calls this automatically if needed
#
# SEE ALSO:
#   https://cloudnative-pg.io/documentation/
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

CNPG_VERSION="${1:-1.29.0}"
CNPG_MINOR="${CNPG_VERSION%.*}"   # e.g. 1.28.1 -> 1.28

info "Installing CloudNativePG operator v${CNPG_VERSION}..."

kubectl apply --validate=false -f "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-${CNPG_MINOR}/releases/cnpg-${CNPG_VERSION}.yaml" || fail "Failed to apply CloudNativePG manifests" 1

info "Waiting for CloudNativePG operator to be ready..."
kubectl wait --for=condition=available --timeout=120s deployment/cnpg-controller-manager -n cnpg-system || fail "CloudNativePG operator did not become ready" 2

info "CloudNativePG operator installed."
kubectl get pods -n cnpg-system
