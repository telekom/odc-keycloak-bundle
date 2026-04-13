#!/bin/bash
# ==============================================================================
# test-security.sh - Static security check: non-root container securityContext
# ==============================================================================
#
# PURPOSE:
#   Verifies that every Deployment manifest shipped in the OCM archive declares
#   the required container-level securityContext fields:
#     - runAsNonRoot: true  OR  runAsUser > 0
#     - allowPrivilegeEscalation: false
#     - capabilities.drop containing ALL
#
#   This is a no-cluster, no-render static check suitable for CI linting.
#
# USAGE:
#   ./scripts/tests/test-security.sh
#
# EXIT CODES:
#   0  All checks passed
#   1  One or more checks failed
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

PASS=0
FAIL=0

check_file() {
    local file="$1"
    local label="$2"
    local failed=0

    if ! grep -q "allowPrivilegeEscalation: false" "$file"; then
        warn "$label: missing 'allowPrivilegeEscalation: false'"
        failed=1
    fi

    if ! grep -qE "runAsNonRoot: true|runAsUser: [1-9][0-9]*" "$file"; then
        warn "$label: missing 'runAsNonRoot: true' or non-zero 'runAsUser'"
        failed=1
    fi

    if ! awk '
        /^[[:space:]]*capabilities:[[:space:]]*$/ {in_cap=1; next}
        in_cap && /^[[:space:]]*drop:[[:space:]]*\[[^]]*ALL[^]]*\][[:space:]]*$/ {found=1; exit}
        in_cap && /^[[:space:]]*drop:[[:space:]]*$/ {in_drop=1; next}
        in_drop && /^[[:space:]]*-[[:space:]]*ALL([[:space:]]|$)/ {found=1; exit}
        in_cap && /^[[:space:]]*[a-zA-Z0-9_-]+:[[:space:]]*/ && $1 !~ /^drop:/ {in_drop=0}
        in_cap && /^[^[:space:]]/ {in_cap=0; in_drop=0}
        END {exit(found ? 0 : 1)}
    ' "$file"; then
        warn "$label: missing 'capabilities.drop: [ALL]'"
        failed=1
    fi

    if [[ "$failed" -eq 0 ]]; then
        info "$label: OK"
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
    fi
}

PROJECT_ROOT="$(cd "$(dirname "$(dirname "$SCRIPT_DIR")")" && pwd)"

info "=== Non-root securityContext check ==="

check_file \
    "$PROJECT_ROOT/manifests/keycloak/keycloak-deployment.yaml" \
    "manifests/keycloak/keycloak-deployment.yaml"

check_file \
    "$PROJECT_ROOT/charts/keycloak-operator/templates/deployment.yaml" \
    "charts/keycloak-operator/templates/deployment.yaml"

info "Results: $PASS passed, $FAIL failed"

if [[ "$FAIL" -gt 0 ]]; then
    fail "Security check failed: $FAIL file(s) missing required securityContext fields" 1
fi

info "Security check passed."
