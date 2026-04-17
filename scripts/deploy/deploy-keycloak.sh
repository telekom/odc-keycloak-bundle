#!/bin/bash
# ==============================================================================
# deploy-keycloak.sh - Deploy Keycloak application server
# ==============================================================================
#
# PURPOSE:
#   Deploys Keycloak to an existing namespace with PostgreSQL already running.
#   Applies all manifests from manifests/keycloak/ and waits for readiness.
#
# USAGE:
#   ./scripts/deploy/deploy-keycloak.sh <namespace>
#
# ARGUMENTS:
#   namespace   REQUIRED. Target namespace (e.g., "keycloak-poc")
#
# EXAMPLES:
#   ./scripts/deploy/deploy-keycloak.sh keycloak-poc         # Proof of concept
#
# PREREQUISITES:
#   - PostgreSQL must be running in the namespace (deploy-postgres.sh)
#   - Secret "keycloak-db-app" must exist (created by CloudNativePG)
#
# CREATES:
#   - Deployment "keycloak"
#   - Service "keycloak" (ports 8080, 8443)
#   - Secret "keycloak-admin" (admin credentials, if missing)
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

info "Deploying Keycloak to namespace: $NAMESPACE"

# Check if PostgreSQL secret exists (created by CloudNativePG)
if ! kubectl get secret keycloak-db-app -n "$NAMESPACE" &>/dev/null; then
    fail "PostgreSQL not ready. Secret 'keycloak-db-app' not found. Run: ./scripts/deploy-postgres.sh $NAMESPACE" 1
fi

# Ensure Keycloak admin credentials secret exists
if ! kubectl get secret keycloak-admin -n "$NAMESPACE" &>/dev/null; then
    ADMIN_USER="${KEYCLOAK_ADMIN_USERNAME:-admin}"
    ADMIN_PASS="${KEYCLOAK_ADMIN_PASSWORD:-$(tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 24)}"

    info "Creating secret 'keycloak-admin' in namespace '$NAMESPACE'..."
    kubectl create secret generic keycloak-admin -n "$NAMESPACE" \
        --from-literal=KEYCLOAK_ADMIN="$ADMIN_USER" \
        --from-literal=KEYCLOAK_ADMIN_PASSWORD="$ADMIN_PASS" || fail "Failed to create admin secret" 2

    if [[ -z "${KEYCLOAK_ADMIN_PASSWORD:-}" ]]; then
        warn "Generated random admin password (not printed). Retrieve it with:"
        warn "kubectl get secret keycloak-admin -n $NAMESPACE -o jsonpath='{.data.KEYCLOAK_ADMIN_PASSWORD}' | base64 -d"
    fi
fi

# Apply Keycloak manifests
kubectl apply -n "$NAMESPACE" -f "$PROJECT_ROOT/manifests/keycloak/" || fail "Failed to apply Keycloak manifests" 2

# Wait for Keycloak to be ready
info "Waiting for Keycloak to be ready..."
info "(This may take a few minutes for first startup)"

if ! kubectl wait -n "$NAMESPACE" --for=condition=ready pod -l app=keycloak --timeout=300s; then
    warn "Keycloak did not become ready within timeout."
    kubectl get pods -n "$NAMESPACE" -l app=keycloak
fi

info "Keycloak deployed."
info "Access: kubectl port-forward -n $NAMESPACE svc/keycloak 8080:8080"
info "URL: http://localhost:8080"
info "User: admin"
info "Password: kubectl get secret keycloak-admin -n $NAMESPACE -o jsonpath='{.data.KEYCLOAK_ADMIN_PASSWORD}' | base64 -d"
