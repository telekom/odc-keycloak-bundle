#!/bin/bash
# ==============================================================================
# deploy-postgres.sh - Deploy CloudNativePG PostgreSQL cluster
# ==============================================================================
#
# PURPOSE:
#   Deploys a PostgreSQL database cluster using CloudNativePG operator.
#   Creates the namespace if it doesn't exist and waits for the cluster
#   to become healthy before returning.
#
# USAGE:
#   ./scripts/deploy/deploy-postgres.sh <namespace>
#
# ARGUMENTS:
#   namespace   REQUIRED. Target namespace (e.g., "keycloak-poc")
#
# EXAMPLES:
#   ./scripts/deploy/deploy-postgres.sh keycloak-poc         # Proof of concept
#
# PREREQUISITES:
#   - CloudNativePG operator must be installed (see install-cnpg.sh)
#   - kubectl configured with cluster access
#
# CREATES:
#   - Namespace (if not exists)
#   - CloudNativePG Cluster "keycloak-db"
#   - Secret "keycloak-db-app" (auto-generated credentials)
#   - Services: keycloak-db-rw (read-write), keycloak-db-r (read-only)
#
# CALLED BY:
#   deploy-all.sh
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

# Require namespace
if [[ -z "$1" ]]; then
    fail "Usage: $0 <namespace>\n\nExample: $0 keycloak-poc" 1
fi

NAMESPACE="$1"
PROJECT_ROOT="$(cd "$(dirname "$(dirname "$SCRIPT_DIR")")" && pwd)"

info "Deploying CloudNativePG cluster to namespace: $NAMESPACE"

# Check if CNPG operator is installed
if ! kubectl get crd clusters.postgresql.cnpg.io &>/dev/null; then
    fail "CloudNativePG operator not installed. Run: ./scripts/install-cnpg.sh" 1
fi

# Create namespace if not exists
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - || fail "Failed to create namespace" 2

# Apply CloudNativePG cluster
kubectl apply -n "$NAMESPACE" -f "$PROJECT_ROOT/manifests/postgres/" || fail "Failed to apply PostgreSQL manifests" 3

# Wait for cluster to be ready
info "Waiting for PostgreSQL cluster to be ready..."
info "(This may take a few minutes for first startup)"

# Wait for PRIMARY pod to be created and labeled
info "Waiting for PostgreSQL primary pod..."
timeout 300s bash -c "until kubectl get pod -n $NAMESPACE -l cnpg.io/cluster=keycloak-db,cnpg.io/instanceRole=primary 2>/dev/null | grep -q 'Running\|Pending'; do echo 'Waiting for primary pod...'; sleep 5; done"

# Wait for readiness
info "Waiting for PostgreSQL Readiness..."
if ! kubectl wait pod -n "$NAMESPACE" \
  -l cnpg.io/cluster=keycloak-db,cnpg.io/instanceRole=primary \
  --for=condition=Ready --timeout=600s; then
    warn "Timeout waiting for PostgreSQL pod."
fi

info "PostgreSQL deployed."
info "Service: keycloak-db-rw.$NAMESPACE.svc:5432"
info "Credentials: kubectl get secret keycloak-db-app -n $NAMESPACE -o yaml"
