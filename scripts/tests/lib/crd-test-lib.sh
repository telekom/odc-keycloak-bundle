#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
source "${PROJECT_ROOT}/scripts/utils/common.sh"

NAMESPACE="${1:-}"
if [[ -z "${NAMESPACE}" ]]; then
    fail "Usage: <script> <namespace>" 1
fi

export MSYS_NO_PATHCONV=1

REALM_NAME="${REALM_NAME:-master}"
CLIENTSCOPE_NAME="${CLIENTSCOPE_NAME:-ci-test-scope}"
GROUP_NAME="${GROUP_NAME:-ci-test-group}"
CLIENT_ID="${CLIENT_ID:-odc-showcase-client}"
USERNAME="${USERNAME:-ci-test-user}"
IDP_ALIAS="${IDP_ALIAS:-ci-test-oidc}"
AUTHFLOW_ALIAS="${AUTHFLOW_ALIAS:-ci-test-browser-mfa}"

FIXTURES_DIR="${PROJECT_ROOT}/scripts/tests/fixtures"
KEYCLOAK_POD=""
ADMIN_USER=""
ADMIN_PASS=""

PASSED=0
FAILED=0
SKIPPED=0
RESULTS=()
RESULT_DURATIONS=()
TEST_RUN_START_TS=0
LAST_RESULT_TS=0

format_seconds() {
    local seconds="$1"
    if [[ -z "${seconds}" || "${seconds}" -lt 0 ]]; then
        seconds=0
    fi
    printf '%.3f' "${seconds}"
}

xml_escape() {
    local s="$1"
    s="${s//&/&amp;}"
    s="${s//</&lt;}"
    s="${s//>/&gt;}"
    s="${s//\"/&quot;}"
    s="${s//\'/&apos;}"
    printf '%s' "${s}"
}

write_junit_report() {
    local report_file="${CRD_TEST_REPORT_FILE:-}"
    local suite_name="${CRD_TEST_SUITE_NAME:-crd-suite}"
    local now_ts=0
    local suite_duration=0

    if [[ -z "${report_file}" ]]; then
        return 0
    fi

    now_ts=$(date +%s)
    if [[ "${TEST_RUN_START_TS}" -gt 0 ]]; then
        suite_duration=$((now_ts - TEST_RUN_START_TS))
    fi

    mkdir -p "$(dirname "${report_file}")"

    {
        echo "<?xml version=\"1.0\" encoding=\"UTF-8\"?>"
        echo "<testsuite name=\"$(xml_escape "${suite_name}")\" tests=\"$((PASSED + FAILED + SKIPPED))\" failures=\"${FAILED}\" skipped=\"${SKIPPED}\" time=\"$(format_seconds "${suite_duration}")\">"

        local i entry status name duration
        for i in "${!RESULTS[@]}"; do
            entry="${RESULTS[$i]}"
            status="${entry%% *}"
            name="${entry#* }"
            duration="${RESULT_DURATIONS[$i]:-0}"
            echo "  <testcase classname=\"crd.integration\" name=\"$(xml_escape "${name}")\" time=\"$(format_seconds "${duration}")\">"
            if [[ "${status}" == "FAIL" ]]; then
                echo "    <failure message=\"failed\">$(xml_escape "${name} failed")</failure>"
            elif [[ "${status}" != "PASS" ]]; then
                echo "    <skipped message=\"skipped\"/>"
            fi
            echo "  </testcase>"
        done

        echo "</testsuite>"
    } > "${report_file}"

    info "JUnit report written: ${report_file}"
}

record_result() {
    local test_name="$1"
    local status="$2"
    local duration_seconds="${3:-}"
    local now_ts

    if [[ -z "${duration_seconds}" ]]; then
        now_ts=$(date +%s)
        if [[ "${LAST_RESULT_TS}" -gt 0 ]]; then
            duration_seconds=$((now_ts - LAST_RESULT_TS))
        else
            duration_seconds=0
        fi
        LAST_RESULT_TS="${now_ts}"
    fi

    RESULTS+=("${status} ${test_name}")
    RESULT_DURATIONS+=("${duration_seconds}")
    if [[ "${status}" == "PASS" ]]; then
        PASSED=$((PASSED + 1))
    elif [[ "${status}" == "FAIL" ]]; then
        FAILED=$((FAILED + 1))
    else
        SKIPPED=$((SKIPPED + 1))
    fi
}

print_summary() {
    info ""
    info "============================================================"
    info "  Test Results Summary"
    info "============================================================"

    local entry status name
    for entry in "${RESULTS[@]}"; do
        status="${entry%% *}"
        name="${entry#* }"
        if [[ "${status}" == "PASS" ]]; then
            info "  ✅ ${name}"
        elif [[ "${status}" == "FAIL" ]]; then
            info "  ❌ ${name}"
        else
            info "  ⏭  ${name}"
        fi
    done

    info ""
    info "  Total: $((PASSED + FAILED + SKIPPED)) | Passed: ${PASSED} | Failed: ${FAILED} | Skipped: ${SKIPPED}"
    info "============================================================"
}

finish_with_summary() {
    write_junit_report
    print_summary
    if [[ "${FAILED}" -gt 0 ]]; then
        fail "Integration test failed: ${FAILED} test(s) did not pass." 1
    fi
    info "All integration tests passed ✅"
}

start_test_run() {
    local title="$1"
    TEST_RUN_START_TS=$(date +%s)
    LAST_RESULT_TS="${TEST_RUN_START_TS}"

    info "============================================================"
    info "  ${title}"
    info "  Namespace: ${NAMESPACE}"
    info "============================================================"
}

check_prerequisites() {
    info "Checking prerequisites..."
    kubectl wait --for=condition=ready pod -l app=keycloak -n "${NAMESPACE}" --timeout=60s || fail "Keycloak not ready" 1
    kubectl wait --for=condition=ready pod -l app=keycloak-operator -n "${NAMESPACE}" --timeout=60s || fail "Operator not ready" 1

    KEYCLOAK_POD=$(kubectl get pod -n "${NAMESPACE}" -l app=keycloak -o jsonpath='{.items[0].metadata.name}')
    info "Keycloak pod: ${KEYCLOAK_POD}"
}

wait_for_reconcile() {
    local realm_name="${1:-${REALM_NAME}}"
    local job_name="kc-config-job-${realm_name}"

    local wait_count=0
    while ! kubectl get job "${job_name}" -n "${NAMESPACE}" &>/dev/null; do
        wait_count=$((wait_count + 1))
        if [ "${wait_count}" -ge 24 ]; then
            info "Job '${job_name}' not created after 120s; assuming direct API reconciliation."
            sleep 10
            return 0
        fi
        sleep 5
    done

    if kubectl wait --for=condition=complete "job/${job_name}" -n "${NAMESPACE}" --timeout=180s; then
        info "Config-CLI Job '${job_name}' completed ✅"
    else
        warn "Job '${job_name}' did not complete within timeout."
        kubectl logs "job/${job_name}" -n "${NAMESPACE}" --tail=60 || true
        return 1
    fi
}

load_admin_credentials() {
    if [[ -n "${ADMIN_USER}" && -n "${ADMIN_PASS}" ]]; then
        return 0
    fi

    ADMIN_USER=$(kubectl get secret keycloak-admin -n "${NAMESPACE}" -o jsonpath='{.data.KEYCLOAK_ADMIN}' | base64 -d)
    ADMIN_PASS=$(kubectl get secret keycloak-admin -n "${NAMESPACE}" -o jsonpath='{.data.KEYCLOAK_ADMIN_PASSWORD}' | base64 -d)
}

run_kcadm() {
    local keycloak_pod="$1"
    shift
    local cmd="$*"

    load_admin_credentials

    kubectl exec -n "${NAMESPACE}" "${keycloak_pod}" -- sh -c \
        "/opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080 --realm master --user ${ADMIN_USER} --password ${ADMIN_PASS} --config /tmp/kcadm.config >/dev/null && ${cmd}"
}

apply_fixture() {
    local fixture_name="$1"
    kubectl apply -n "${NAMESPACE}" -f "${FIXTURES_DIR}/${fixture_name}"
}

run_parallel_steps() {
    local pids=()
    local labels=()

    while [[ "$#" -gt 0 ]]; do
        local label="$1"
        local cmd="$2"
        labels+=("${label}")
        bash -lc "${cmd}" &
        pids+=("$!")
        shift 2
    done

    local i
    for i in "${!pids[@]}"; do
        if wait "${pids[$i]}"; then
            info "${labels[$i]} completed ✅"
        else
            fail "${labels[$i]} failed" 1
        fi
    done
}

verify_realm_exists() {
    local realm_result
    realm_result=$(run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get realms/${REALM_NAME} --config /tmp/kcadm.config" 2>/dev/null || echo "")
    echo "${realm_result}" | grep -Eq '"realm"[[:space:]]*:[[:space:]]*"'"${REALM_NAME}"'"'
}

verify_clientscope_exists() {
    local scope_result
    scope_result=$(run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get client-scopes -r ${REALM_NAME} --config /tmp/kcadm.config" 2>/dev/null || echo "")
    [[ "${scope_result}" == *"\"name\" : \"${CLIENTSCOPE_NAME}\""* ]]
}

verify_group_exists() {
    local group_result
    group_result=$(run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get groups -r ${REALM_NAME} --config /tmp/kcadm.config" 2>/dev/null || echo "")
    [[ "${group_result}" == *"\"name\" : \"${GROUP_NAME}\""* ]]
}

verify_client_exists() {
    local client_result
    client_result=$(run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get clients -r ${REALM_NAME} -q clientId=${CLIENT_ID} --config /tmp/kcadm.config" 2>/dev/null || echo "")
    [[ "${client_result}" == *"\"clientId\" : \"${CLIENT_ID}\""* ]]
}

verify_user_exists_and_membership() {
    local user_result user_id group_membership

    user_result=$(run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get users -r ${REALM_NAME} -q username=${USERNAME} --config /tmp/kcadm.config" 2>/dev/null || echo "")
    if [[ "${user_result}" != *"\"username\" : \"${USERNAME}\""* ]]; then
        return 1
    fi

    user_id=$(echo "${user_result}" | grep -o '"id" : "[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -z "${user_id}" ]]; then
        return 1
    fi

    group_membership=$(run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get users/${user_id}/groups -r ${REALM_NAME} --config /tmp/kcadm.config" 2>/dev/null || echo "")
    [[ "${group_membership}" == *"\"name\" : \"${GROUP_NAME}\""* ]]
}

verify_identityprovider_exists() {
    local idp_result
    idp_result=$(run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get identity-provider/instances -r ${REALM_NAME} --config /tmp/kcadm.config" 2>/dev/null || echo "")
    [[ "${idp_result}" == *"\"alias\" : \"${IDP_ALIAS}\""* ]]
}

verify_authflow_exists() {
    local authflow_result
    authflow_result=$(run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get authentication/flows -r ${REALM_NAME} --config /tmp/kcadm.config" 2>/dev/null || echo "")
    [[ "${authflow_result}" == *"\"alias\" : \"${AUTHFLOW_ALIAS}\""* ]]
}

wait_for_check() {
    local check_fn="$1"
    local timeout_seconds="${2:-180}"
    local trigger_reconcile="${3:-false}"
    local deadline=$((SECONDS + timeout_seconds))

    while [ "${SECONDS}" -lt "${deadline}" ]; do
        if "${check_fn}"; then
            return 0
        fi

        if [[ "${trigger_reconcile}" == "true" ]]; then
            wait_for_reconcile "${REALM_NAME}" || true
        fi
        sleep 5
    done

    return 1
}

wait_for_realm_exists() {
    wait_for_check verify_realm_exists "${1:-180}" true
}

wait_for_clientscope_exists() {
    wait_for_check verify_clientscope_exists "${1:-180}" true
}

wait_for_group_exists() {
    wait_for_check verify_group_exists "${1:-180}" true
}

wait_for_client_exists() {
    wait_for_check verify_client_exists "${1:-180}" true
}

wait_for_identityprovider_exists() {
    wait_for_check verify_identityprovider_exists "${1:-180}" true
}

wait_for_user_exists_and_membership() {
    wait_for_check verify_user_exists_and_membership "${1:-240}" true
}

wait_for_authflow_exists() {
    wait_for_check verify_authflow_exists "${1:-180}" true
}

wait_for_secret_exists() {
    local secret_name="$1"
    local timeout_seconds="${2:-120}"
    local deadline=$((SECONDS + timeout_seconds))

    while [ "${SECONDS}" -lt "${deadline}" ]; do
        if kubectl get secret "${secret_name}" -n "${NAMESPACE}" > /dev/null 2>&1; then
            return 0
        fi
        sleep 3
    done

    return 1
}

idp_absent_in_keycloak() {
    local idp_check
    idp_check=$(run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get identity-provider/instances/${IDP_ALIAS} -r ${REALM_NAME} --config /tmp/kcadm.config" 2>&1 || true)
    echo "${idp_check}" | tr '[:upper:]' '[:lower:]' | grep -Eq 'not[_ -]?found|resource not found|404'
}

wait_for_idp_absent() {
    local timeout_seconds="${1:-240}"
    local deadline=$((SECONDS + timeout_seconds))
    while [ "${SECONDS}" -lt "${deadline}" ]; do
        if idp_absent_in_keycloak; then
            return 0
        fi
        sleep 5
    done
    return 1
}

authflow_absent_in_keycloak() {
    local authflow_list
    authflow_list=$(run_kcadm "${KEYCLOAK_POD}" "/opt/keycloak/bin/kcadm.sh get authentication/flows -r ${REALM_NAME} --config /tmp/kcadm.config" 2>/dev/null || echo "")
    [[ "${authflow_list}" != *"\"alias\" : \"${AUTHFLOW_ALIAS}\""* ]]
}

wait_for_authflow_absent() {
    local timeout_seconds="${1:-240}"
    local deadline=$((SECONDS + timeout_seconds))
    while [ "${SECONDS}" -lt "${deadline}" ]; do
        if authflow_absent_in_keycloak; then
            return 0
        fi
        sleep 5
    done
    return 1
}

verify_coexistence() {
    local all_ok=true

    kubectl get realm "${REALM_NAME}" -n "${NAMESPACE}" -o jsonpath='{.metadata.name}' &>/dev/null || { warn "Realm missing"; all_ok=false; }
    kubectl get clientscope "${CLIENTSCOPE_NAME}" -n "${NAMESPACE}" -o jsonpath='{.metadata.name}' &>/dev/null || { warn "ClientScope missing"; all_ok=false; }
    kubectl get group "${GROUP_NAME}" -n "${NAMESPACE}" -o jsonpath='{.metadata.name}' &>/dev/null || { warn "Group missing"; all_ok=false; }
    kubectl get user "${USERNAME}" -n "${NAMESPACE}" -o jsonpath='{.metadata.name}' &>/dev/null || { warn "User missing"; all_ok=false; }

    [[ "${all_ok}" == true ]]
}
