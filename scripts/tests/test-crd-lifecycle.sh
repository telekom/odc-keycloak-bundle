#!/bin/bash
# ==============================================================================
# test-crd-lifecycle.sh - Deletion lifecycle checks for extended CRDs
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/crd-test-lib.sh" "$@"

export CRD_TEST_SUITE_NAME="crd-lifecycle"

start_test_run "Keycloak Operator — CRD Deletion Lifecycle Test"
check_prerequisites

info ""
info "── Prepare deletion targets (IdP + AuthFlow) ─────────────"
apply_fixture "identityprovider-ci-test-oidc.yaml"
apply_fixture "authflow-ci-test-browser-mfa.yaml"
wait_for_reconcile "${REALM_NAME}" || true
sleep 3

if ! wait_for_identityprovider_exists 180; then
    warn "Precondition failed: IdentityProvider '${IDP_ALIAS}' missing before deletion test"
fi
if ! wait_for_authflow_exists 180; then
    warn "Precondition failed: AuthFlow '${AUTHFLOW_ALIAS}' missing before deletion test"
fi

info ""
info "── Execute deletion workflow (parallel delete) ───────────"
if [[ "${CRD_TEST_PARALLEL_DELETE:-true}" == "true" ]]; then
    run_parallel_steps \
        "Delete IdentityProvider CR" "kubectl delete identityprovider ci-test-oidc-provider -n '${NAMESPACE}' --timeout=60s 2>/dev/null || true" \
        "Delete AuthFlow CR" "kubectl delete authflow ci-test-browser-mfa -n '${NAMESPACE}' --timeout=60s 2>/dev/null || true"
else
    kubectl delete identityprovider ci-test-oidc-provider -n "${NAMESPACE}" --timeout=60s 2>/dev/null || true
    kubectl delete authflow ci-test-browser-mfa -n "${NAMESPACE}" --timeout=60s 2>/dev/null || true
fi

kubectl wait --for=delete identityprovider/ci-test-oidc-provider -n "${NAMESPACE}" --timeout=120s >/dev/null 2>&1 || true
kubectl wait --for=delete authflow/ci-test-browser-mfa -n "${NAMESPACE}" --timeout=120s >/dev/null 2>&1 || true
wait_for_reconcile "${REALM_NAME}" || true

IDP_DELETE_OK=false
AUTHFLOW_DELETE_OK=false

if wait_for_idp_absent 240; then
    info "IdentityProvider '${IDP_ALIAS}' removed from Keycloak after CR deletion ✅"
    IDP_DELETE_OK=true
else
    warn "IdentityProvider '${IDP_ALIAS}' still exists after CR deletion ❌"
    run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get identity-provider/instances -r ${REALM_NAME} --config /tmp/kcadm.config" 2>/dev/null || true
fi

if wait_for_authflow_absent 240; then
    info "AuthFlow '${AUTHFLOW_ALIAS}' removed from Keycloak after CR deletion ✅"
    AUTHFLOW_DELETE_OK=true
else
    warn "AuthFlow '${AUTHFLOW_ALIAS}' still exists after CR deletion ❌"
    run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get authentication/flows -r ${REALM_NAME} --config /tmp/kcadm.config" 2>/dev/null || true
fi

if [[ "${IDP_DELETE_OK}" == true ]] && [[ "${AUTHFLOW_DELETE_OK}" == true ]]; then
    record_result "ATC-05-3 CR deletion removes IdP and AuthFlow" "PASS"
else
    record_result "ATC-05-3 CR deletion removes IdP and AuthFlow" "FAIL"
fi

finish_with_summary
