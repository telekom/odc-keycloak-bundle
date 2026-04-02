#!/bin/bash
# ==============================================================================
# test-crd-smoke.sh - Fast CRD smoke and coexistence checks
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/crd-test-lib.sh" "$@"

export CRD_TEST_SUITE_NAME="crd-smoke"

start_test_run "Keycloak Operator — CRD Smoke Test"
check_prerequisites

info ""
info "── 1/4 Realm Foundation ───────────────────────────────────"
apply_fixture "realm-master.yaml"
info "Realm '${REALM_NAME}' applied."
wait_for_reconcile "${REALM_NAME}" || true
sleep 3

if wait_for_realm_exists 180; then
    info "Realm '${REALM_NAME}' verified in Keycloak ✅"
    record_result "ATC-04-1 Realm CRD lifecycle" "PASS"
else
    warn "Realm '${REALM_NAME}' NOT found in Keycloak ❌"
    record_result "ATC-04-1 Realm CRD lifecycle" "FAIL"
fi

info ""
info "── 2/4 Independent CRDs (parallel apply) ─────────────────"
if [[ "${CRD_TEST_PARALLEL_APPLY:-true}" == "true" ]]; then
    run_parallel_steps \
        "Apply ClientScope" "kubectl apply -n '${NAMESPACE}' -f '${FIXTURES_DIR}/clientscope-ci-test-scope.yaml'" \
        "Apply Group" "kubectl apply -n '${NAMESPACE}' -f '${FIXTURES_DIR}/group-ci-test-group.yaml'" \
        "Apply Client" "kubectl apply -n '${NAMESPACE}' -f '${FIXTURES_DIR}/client-odc-showcase.yaml'" \
        "Apply IdentityProvider" "kubectl apply -n '${NAMESPACE}' -f '${FIXTURES_DIR}/identityprovider-ci-test-oidc.yaml'" \
        "Apply AuthFlow" "kubectl apply -n '${NAMESPACE}' -f '${FIXTURES_DIR}/authflow-ci-test-browser-mfa.yaml'"
else
    apply_fixture "clientscope-ci-test-scope.yaml"
    apply_fixture "group-ci-test-group.yaml"
    apply_fixture "client-odc-showcase.yaml"
    apply_fixture "identityprovider-ci-test-oidc.yaml"
    apply_fixture "authflow-ci-test-browser-mfa.yaml"
fi

wait_for_reconcile "${REALM_NAME}" || true
sleep 5

if wait_for_clientscope_exists 180; then
    info "ClientScope '${CLIENTSCOPE_NAME}' verified in Keycloak ✅"
    record_result "ATC-04-3 ClientScope CRD lifecycle" "PASS"
else
    warn "ClientScope '${CLIENTSCOPE_NAME}' NOT found in Keycloak ❌"
    record_result "ATC-04-3 ClientScope CRD lifecycle" "FAIL"
fi

if wait_for_group_exists 180; then
    info "Group '${GROUP_NAME}' verified in Keycloak ✅"
    record_result "ATC-04-4 Group CRD lifecycle" "PASS"
else
    warn "Group '${GROUP_NAME}' NOT found in Keycloak ❌"
    record_result "ATC-04-4 Group CRD lifecycle" "FAIL"
fi

if wait_for_client_exists 180; then
    info "Client '${CLIENT_ID}' verified in Keycloak ✅"
    record_result "ATC-04-2 Client CRD lifecycle" "PASS"

    if wait_for_secret_exists "${CLIENT_ID}-secret" 120; then
        info "Client Secret '${CLIENT_ID}-secret' created ✅"
    else
        warn "Client Secret '${CLIENT_ID}-secret' NOT found"
    fi
else
    warn "Client '${CLIENT_ID}' NOT found in Keycloak ❌"
    record_result "ATC-04-2 Client CRD lifecycle" "FAIL"
fi

if wait_for_identityprovider_exists 180; then
    info "IdentityProvider '${IDP_ALIAS}' verified in Keycloak ✅"
    record_result "ATC-05-1 IdentityProvider CRD lifecycle" "PASS"
else
    warn "IdentityProvider '${IDP_ALIAS}' NOT found in Keycloak ❌"
    record_result "ATC-05-1 IdentityProvider CRD lifecycle" "FAIL"
fi

if wait_for_authflow_exists 180; then
    info "AuthFlow '${AUTHFLOW_ALIAS}' verified in Keycloak ✅"
    record_result "ATC-05-2 AuthFlow CRD lifecycle" "PASS"
else
    warn "AuthFlow '${AUTHFLOW_ALIAS}' NOT found in Keycloak ❌"
    record_result "ATC-05-2 AuthFlow CRD lifecycle" "FAIL"
fi

info ""
info "── 3/4 User CRD (depends on Group) ───────────────────────"
apply_fixture "secret-ci-test-user-password.yaml"
apply_fixture "user-ci-test-user.yaml"
info "User '${USERNAME}' applied with group '${GROUP_NAME}'."
wait_for_reconcile "${REALM_NAME}" || true
sleep 5

if wait_for_user_exists_and_membership 240; then
    info "User '${USERNAME}' and group membership verified ✅"
    record_result "ATC-04-5 User CRD lifecycle (with group membership)" "PASS"
else
    warn "User '${USERNAME}' or group membership verification failed ❌"
    record_result "ATC-04-5 User CRD lifecycle (with group membership)" "FAIL"
fi

info ""
info "── 4/4 Coexistence ───────────────────────────────────────"
if verify_coexistence; then
    info "All CRD types coexist without conflicts ✅"
    record_result "ATC-04-6 Coexistence of all CRD types" "PASS"
else
    record_result "ATC-04-6 Coexistence of all CRD types" "FAIL"
fi

finish_with_summary
