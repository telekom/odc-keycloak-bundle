#!/bin/bash
# ==============================================================================
# logs.sh - View logs for Keycloak or PostgreSQL
# ==============================================================================
#
# PURPOSE:
#   Streams logs from Keycloak or PostgreSQL pods for debugging and
#   monitoring during development.
#
# USAGE:
#   ./scripts/utils/logs.sh <instance-name> [component]
#
# ARGUMENTS:
#   instance-name   REQUIRED. Instance name (e.g., "poc", "alpha")
#   component       Optional. "keycloak" or "postgres" (default: "keycloak")
#
# EXAMPLES:
#   ./scripts/utils/logs.sh poc                    # Keycloak logs
#   ./scripts/utils/logs.sh poc keycloak           # Keycloak logs (explicit)
#   ./scripts/utils/logs.sh poc postgres           # PostgreSQL logs
#   ./scripts/utils/logs.sh poc db                 # PostgreSQL logs (alias "db")
#
# NOTES:
#   - Streams last 100 lines and follows new output
#   - Press Ctrl+C to stop
#
# SEE ALSO:
#   status.sh      - Show instance status
#   portforward.sh - Access Keycloak locally
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Require instance name
if [[ -z "$1" ]]; then
    fail "Usage: $0 <instance-name> [keycloak|postgres]\n\nExample: $0 poc\n         $0 poc postgres\n\nTo list instances: kubectl get ns | grep keycloak" 1
fi

INSTANCE_NAME="$1"
COMPONENT="${2:-keycloak}"
NAMESPACE="${3:-keycloak-$INSTANCE_NAME}"

case "$COMPONENT" in
    keycloak)
        SELECTOR="app=keycloak"
        ;;
    postgres|db)
        SELECTOR="cnpg.io/cluster=keycloak-db"
        ;;
    *)
        fail "Unknown component: $COMPONENT\nValid options: keycloak, postgres (or db)" 1
        ;;
esac

info "Showing logs for $COMPONENT in $NAMESPACE"
info "Press Ctrl+C to stop"

kubectl logs -n "$NAMESPACE" -l "$SELECTOR" -f --tail=100 || fail "Failed to get logs" 2
