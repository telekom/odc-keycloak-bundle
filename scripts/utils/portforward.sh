#!/bin/bash
# ==============================================================================
# portforward.sh - Port-forward Keycloak for local access
# ==============================================================================
#
# PURPOSE:
#   Creates a port-forward from your local machine to the Keycloak service
#   running in Kubernetes. Allows accessing Keycloak UI at localhost.
#
# USAGE:
#   ./scripts/utils/portforward.sh <instance-name> [local-port] [namespace]
#   ./scripts/utils/portforward.sh <instance-name> [namespace] [local-port]
#
# ARGUMENTS:
#   instance-name   REQUIRED. Instance name (e.g., "poc", "alpha")
#   local-port      Optional. Local port to use (default: 8080)
#   namespace       Optional. Explicit namespace override
#                   (default: keycloak-<instance-name>)
#
# EXAMPLES:
#   ./scripts/utils/portforward.sh poc                         # localhost:8080, namespace keycloak-poc
#   ./scripts/utils/portforward.sh poc 9090                    # localhost:9090, namespace keycloak-poc
#   ./scripts/utils/portforward.sh poc identity-poc            # localhost:8080, explicit namespace
#   ./scripts/utils/portforward.sh poc identity-poc 9090       # explicit namespace + custom port
#   ./scripts/utils/portforward.sh poc 9090 identity-poc       # custom port + explicit namespace
#
# ACCESS:
#   After running, open: http://localhost:<port>
#   Default credentials: admin / admin
#
# NOTES:
#   - Press Ctrl+C to stop the port-forward
#   - Only one port-forward can use a port at a time
#
# SEE ALSO:
#   logs.sh   - View logs
#   status.sh - Check instance status
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Require instance name
if [[ -z "$1" ]]; then
    fail "Usage: $0 <instance-name> [local-port] [namespace]\n       $0 <instance-name> [namespace] [local-port]\n\nExamples:\n  $0 poc\n  $0 poc 9090\n  $0 poc identity-poc\n  $0 poc identity-poc 9090\n\nTo list namespaces: kubectl get ns" 1
fi

INSTANCE_NAME="$1"
NAMESPACE="keycloak-$INSTANCE_NAME"
LOCAL_PORT="8080"

is_valid_port() {
    local value="$1"
    [[ "$value" =~ ^[0-9]+$ ]] && (( value >= 1 && value <= 65535 ))
}

if [[ -n "${2:-}" ]]; then
    if is_valid_port "$2"; then
        LOCAL_PORT="$2"
        if [[ -n "${3:-}" ]]; then
            NAMESPACE="$3"
        fi
    else
        NAMESPACE="$2"
        if [[ -n "${3:-}" ]]; then
            if is_valid_port "$3"; then
                LOCAL_PORT="$3"
            else
                fail "Invalid local port: $3 (must be 1-65535)" 1
            fi
        fi
    fi
fi

info "Port-forwarding Keycloak from $NAMESPACE to localhost:$LOCAL_PORT"
info "Press Ctrl+C to stop"
info ""
info "Open: http://localhost:$LOCAL_PORT"

kubectl port-forward -n "$NAMESPACE" svc/keycloak "$LOCAL_PORT:8080" || fail "Port-forward failed" 1
