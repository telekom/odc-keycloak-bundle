#!/bin/bash
# ==============================================================================
# cleanup.sh - Remove a single Keycloak instance
# ==============================================================================
#
# PURPOSE:
#   Deletes a Keycloak instance by removing its entire namespace.
#   This removes PostgreSQL, Keycloak, secrets, and all related resources.
#
# USAGE:
#   ./scripts/deploy/cleanup.sh <instance-name> [namespace]
#
# ARGUMENTS:
#   instance-name   REQUIRED. Name of instance to remove.
#   namespace       Optional. Explicit target namespace.
#                   Defaults to "keycloak-<instance-name>" when not provided.
#
# NAMING CONVENTION:
#   - poc           : Proof of concept
#   - alpha, beta   : Pre-release stages
#   - final         : Production release
#
# EXAMPLES:
#   ./scripts/deploy/cleanup.sh poc          # Remove keycloak-poc
#
# NOTES:
#   - Uses --wait=false for faster return (deletion continues in background)
#   - Safe to run if namespace doesn't exist
#   - Does NOT remove CloudNativePG operator or CRDs
#
# SEE ALSO:
#   deploy-all.sh  - Deploy a new instance
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

# Require instance name
if [[ -z "$1" ]]; then
    fail "Usage: $0 <instance-name>\n\nExample: $0 poc\n\nTo list instances: kubectl get ns | grep keycloak" 1
fi

INSTANCE_NAME="$1"
NAMESPACE="${2:-keycloak-$INSTANCE_NAME}"

API_RESOURCES="$(kubectl api-resources --verbs=list --namespaced -o name 2>/dev/null || true)"

resource_available() {
    local resource="$1"
    grep -Fxq "$resource" <<<"$API_RESOURCES"
}

delete_all_for_resource() {
    local resource="$1"
    local timeout="${2:-60s}"

    if resource_available "$resource"; then
        kubectl delete "$resource" --all -n "$NAMESPACE" --ignore-not-found=true --timeout="$timeout" || true
    fi
}

collect_remaining_objects() {
    local objs=()
    local resource

    for resource in "${TARGET_RESOURCES[@]}"; do
        if ! resource_available "$resource"; then
            continue
        fi

        while IFS= read -r obj; do
            if [[ -n "$obj" ]]; then
                objs+=("$obj")
            fi
        done < <(kubectl get "$resource" -n "$NAMESPACE" -o name 2>/dev/null || true)
    done

    if [ "${#objs[@]}" -gt 0 ]; then
        printf '%s\n' "${objs[@]}"
    fi
}

NEW_CR_RESOURCES=(
    "clients.keycloak.opendefense.cloud"
    "users.keycloak.opendefense.cloud"
    "groups.keycloak.opendefense.cloud"
    "clientscopes.keycloak.opendefense.cloud"
    "identityproviders.keycloak.opendefense.cloud"
    "authflows.keycloak.opendefense.cloud"
    "realms.keycloak.opendefense.cloud"
)
TARGET_RESOURCES=("${NEW_CR_RESOURCES[@]}")

info "=== Cleanup: $INSTANCE_NAME ==="

# Remove per-instance Prometheus RBAC (cluster-scoped, not deleted with the namespace)
info "Removing Prometheus RBAC for namespace: $NAMESPACE"
kubectl delete clusterrolebinding "prometheus-${NAMESPACE}" --ignore-not-found=true || true
kubectl delete clusterrole "prometheus-${NAMESPACE}" --ignore-not-found=true || true

if kubectl get namespace "$NAMESPACE" &>/dev/null; then
    info "Cleaning up Keycloak Custom Resources before namespace deletion"
    # Delete CRs before deleting namespace so the operator can process finalizers.
    for resource in "${TARGET_RESOURCES[@]}"; do
        delete_all_for_resource "$resource"
    done
    
    delete_all_for_resource "clusters.postgresql.cnpg.io" "120s"

    # CNPG resources are not part of TARGET_RESOURCES safety abort, but warn if they still exist.
    if resource_available "clusters.postgresql.cnpg.io"; then
        mapfile -t remaining_cnpg_clusters < <(kubectl get clusters.postgresql.cnpg.io -n "$NAMESPACE" -o name 2>/dev/null || true)
        if [ "${#remaining_cnpg_clusters[@]}" -gt 0 ]; then
            warn "${#remaining_cnpg_clusters[@]} CNPG cluster resource(s) still present after delete request; namespace termination may wait for CNPG finalizers."
        fi
    fi

    mapfile -t remaining_objects < <(collect_remaining_objects)

    if [ "${#remaining_objects[@]}" -gt 0 ]; then
        warn "${#remaining_objects[@]} Keycloak CRs still present; attempting finalizer cleanup."
        for obj in "${remaining_objects[@]}"; do
            kubectl patch "$obj" -n "$NAMESPACE" --type=merge -p '{"metadata":{"finalizers":[]}}' 2>/dev/null || true
            kubectl delete "$obj" -n "$NAMESPACE" --ignore-not-found=true --timeout=30s || true
        done

        mapfile -t remaining_objects < <(collect_remaining_objects)

        if [ "${#remaining_objects[@]}" -gt 0 ]; then
            fail "Safety Abort: ${#remaining_objects[@]} Keycloak Custom Resources could not be deleted (Operator timeout or error). The Keycloak pod must remain running to process finalizers. Fix the operator or manual purge required." 1
        fi
    fi

    # Delete Prometheus Operator CRs so their finalizers are processed before the namespace goes away.
    for monitoring_resource in \
        "prometheuses.monitoring.coreos.com" \
        "servicemonitors.monitoring.coreos.com" \
        "podmonitors.monitoring.coreos.com" \
        "prometheusrules.monitoring.coreos.com" \
        "alertmanagers.monitoring.coreos.com"; do
        delete_all_for_resource "$monitoring_resource" "60s"
    done

    # Force-delete any stuck pods so CNPG PVC pvc-protection finalizers are released.
    if kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -q .; then
        info "Force-deleting remaining pods in $NAMESPACE"
        kubectl delete pods --all -n "$NAMESPACE" --grace-period=0 --force \
            --ignore-not-found=true 2>/dev/null || true
    fi

    # Clear pvc-protection (and any other) finalizers from PVCs so the namespace can terminate.
    while IFS= read -r pvc; do
        [[ -z "$pvc" ]] && continue
        kubectl patch "$pvc" -n "$NAMESPACE" --type=merge \
            -p '{"metadata":{"finalizers":[]}}' 2>/dev/null || true
    done < <(kubectl get pvc -n "$NAMESPACE" -o name 2>/dev/null || true)

    info "Deleting namespace: $NAMESPACE"
    kubectl delete namespace "$NAMESPACE" --wait=false || warn "Failed to delete namespace"
else
    info "Namespace $NAMESPACE does not exist."
fi

info "Cleanup initiated."
