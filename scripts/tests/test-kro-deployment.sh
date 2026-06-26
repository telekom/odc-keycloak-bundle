#!/usr/bin/env bash
# Verifies the KRO deployment path by applying the RGD, creating a KeycloakInstance,
# and asserting that KRO materializes the expected backing resources.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

INSTANCE_NAME="${INSTANCE_NAME:-kro-ci}"
NAMESPACE="${NAMESPACE:-keycloak-kro-ci}"
RGD_FILE="${RGD_FILE:-${PROJECT_ROOT}/kro/rgd/keycloak-instance-rgd.yaml}"
REPLICAS="${REPLICAS:-1}"
DB_INSTANCES="${DB_INSTANCES:-1}"
DB_STORAGE_SIZE="${DB_STORAGE_SIZE:-1Gi}"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-ghcr.io/opendefensecloud/keycloak-operator:0.3.1}"
CONFIG_CLI_IMAGE="${CONFIG_CLI_IMAGE:-quay.io/adorsys/keycloak-config-cli:latest-26@sha256:1b22dfaa9ae0c71f74b0342f9221a6510f272da5def683dbba26a98e6b1b1411}"
ADMIN_SECRET_NAME="${ADMIN_SECRET_NAME:-keycloak-admin}"
ADMIN_USER_SECRET_KEY="${ADMIN_USER_SECRET_KEY:-KEYCLOAK_ADMIN}"
ADMIN_PASSWORD_SECRET_KEY="${ADMIN_PASSWORD_SECRET_KEY:-KEYCLOAK_ADMIN_PASSWORD}"
TIMEOUT_SECONDS="${KRO_TEST_TIMEOUT_SECONDS:-180}"
KEEP_RESOURCES="${KEEP_KRO_TEST_RESOURCES:-false}"

TMP_DIR="$(mktemp -d)"

info() {
    echo "[INFO] $*"
}

fail() {
    echo "[ERROR] $*" >&2
    exit 1
}

cleanup() {
    rm -rf "${TMP_DIR}"

    if [[ "${KEEP_RESOURCES}" == "true" ]]; then
        info "Keeping KRO test resources because KEEP_KRO_TEST_RESOURCES=true"
        return
    fi

    kubectl delete keycloakinstance "${INSTANCE_NAME}" --ignore-not-found --timeout=60s >/dev/null 2>&1 || true
    kubectl delete namespace "${NAMESPACE}" --ignore-not-found --timeout=120s >/dev/null 2>&1 || true
}

trap cleanup EXIT

wait_for_resource() {
    local resource="$1"
    local namespace="${2:-}"
    local started
    started="$(date +%s)"

    while true; do
        if [[ -n "${namespace}" ]]; then
            if kubectl get "${resource}" -n "${namespace}" >/dev/null 2>&1; then
                return 0
            fi
        else
            if kubectl get "${resource}" >/dev/null 2>&1; then
                return 0
            fi
        fi

        if (($(date +%s) - started >= TIMEOUT_SECONDS)); then
            return 1
        fi
        sleep 2
    done
}

jsonpath() {
    local resource="$1"
    local namespace="$2"
    local path="$3"

    if [[ -n "${namespace}" ]]; then
        kubectl get "${resource}" -n "${namespace}" -o "jsonpath=${path}"
    else
        kubectl get "${resource}" -o "jsonpath=${path}"
    fi
}

assert_jsonpath_equals() {
    local name="$1"
    local resource="$2"
    local namespace="$3"
    local path="$4"
    local expected="$5"
    local actual

    actual="$(jsonpath "${resource}" "${namespace}" "${path}")"
    if [[ "${actual}" != "${expected}" ]]; then
        fail "${name}: expected '${expected}', got '${actual}'"
    fi
    info "${name}: ${actual}"
}

require_crd() {
    local crd="$1"
    local component="$2"

    if ! kubectl get "crd/${crd}" >/dev/null 2>&1; then
        fail "Missing ${component} CRD '${crd}'. Install ${component} before running this test."
    fi
}

command -v kubectl >/dev/null 2>&1 || fail "kubectl is required"
[[ -f "${RGD_FILE}" ]] || fail "RGD file not found: ${RGD_FILE}"

if grep -q '{{' "${RGD_FILE}"; then
    fail "RGD still contains Go/Helm-style template delimiters"
fi

require_crd "resourcegraphdefinitions.kro.run" "KRO"
require_crd "clusters.postgresql.cnpg.io" "CloudNativePG"

info "Applying KRO RGD: ${RGD_FILE}"
kubectl apply -f "${RGD_FILE}"

info "Waiting for generated KeycloakInstance CRD"
kubectl wait --for=condition=Established crd/keycloakinstances.kro.run --timeout=60s

cat >"${TMP_DIR}/keycloak-instance.yaml" <<EOF
apiVersion: kro.run/v1alpha1
kind: KeycloakInstance
metadata:
  name: ${INSTANCE_NAME}
spec:
  namespace: ${NAMESPACE}
  replicas: ${REPLICAS}
  dbInstances: ${DB_INSTANCES}
  dbStorageSize: ${DB_STORAGE_SIZE}
  operatorImage: ${OPERATOR_IMAGE}
  configCliImage: ${CONFIG_CLI_IMAGE}
  adminSecretName: ${ADMIN_SECRET_NAME}
  adminUserSecretKey: ${ADMIN_USER_SECRET_KEY}
  adminPasswordSecretKey: ${ADMIN_PASSWORD_SECRET_KEY}
EOF

info "Creating KeycloakInstance/${INSTANCE_NAME}"
kubectl apply -f "${TMP_DIR}/keycloak-instance.yaml"

info "Waiting for target namespace ${NAMESPACE}"
wait_for_resource "namespace/${NAMESPACE}" || fail "Namespace ${NAMESPACE} was not created by KRO"

info "Creating bootstrap admin Secret in ${NAMESPACE}"
kubectl create secret generic "${ADMIN_SECRET_NAME}" \
    -n "${NAMESPACE}" \
    --from-literal="${ADMIN_USER_SECRET_KEY}=admin" \
    --from-literal="${ADMIN_PASSWORD_SECRET_KEY}=change-me" \
    --dry-run=client -o yaml | kubectl apply -f -

info "Waiting for KRO-managed resources"
wait_for_resource "clusters.postgresql.cnpg.io/keycloak-db" "${NAMESPACE}" || fail "CNPG Cluster was not created"
wait_for_resource "deployment/keycloak" "${NAMESPACE}" || fail "Keycloak Deployment was not created"
wait_for_resource "service/keycloak" "${NAMESPACE}" || fail "Keycloak Service was not created"
wait_for_resource "poddisruptionbudget/keycloak" "${NAMESPACE}" || fail "Keycloak PDB was not created"
wait_for_resource "serviceaccount/keycloak-operator" "${NAMESPACE}" || fail "Operator ServiceAccount was not created"
wait_for_resource "serviceaccount/keycloak-config-cli" "${NAMESPACE}" || fail "Config CLI ServiceAccount was not created"
wait_for_resource "role/keycloak-operator" "${NAMESPACE}" || fail "Operator Role was not created"
wait_for_resource "rolebinding/keycloak-operator" "${NAMESPACE}" || fail "Operator RoleBinding was not created"
wait_for_resource "deployment/keycloak-operator" "${NAMESPACE}" || fail "Operator Deployment was not created"

info "Validating rendered values"
assert_jsonpath_equals "Namespace name" "namespace/${NAMESPACE}" "" "{.metadata.name}" "${NAMESPACE}"
assert_jsonpath_equals "CNPG instances" "clusters.postgresql.cnpg.io/keycloak-db" "${NAMESPACE}" "{.spec.instances}" "${DB_INSTANCES}"
assert_jsonpath_equals "CNPG storage size" "clusters.postgresql.cnpg.io/keycloak-db" "${NAMESPACE}" "{.spec.storage.size}" "${DB_STORAGE_SIZE}"
assert_jsonpath_equals "Keycloak replicas" "deployment/keycloak" "${NAMESPACE}" "{.spec.replicas}" "${REPLICAS}"
assert_jsonpath_equals "Operator image" "deployment/keycloak-operator" "${NAMESPACE}" "{.spec.template.spec.containers[0].image}" "${OPERATOR_IMAGE}"
assert_jsonpath_equals "Config CLI image env" "deployment/keycloak-operator" "${NAMESPACE}" "{.spec.template.spec.containers[0].env[?(@.name=='CONFIG_CLI_IMAGE')].value}" "${CONFIG_CLI_IMAGE}"
assert_jsonpath_equals "Admin secret name env" "deployment/keycloak-operator" "${NAMESPACE}" "{.spec.template.spec.containers[0].env[?(@.name=='KEYCLOAK_ADMIN_SECRET_NAME')].value}" "${ADMIN_SECRET_NAME}"

RENDERED="${TMP_DIR}/rendered.yaml"
{
    kubectl get namespace "${NAMESPACE}" -o yaml
    kubectl get clusters.postgresql.cnpg.io/keycloak-db -n "${NAMESPACE}" -o yaml
    kubectl get deployment/keycloak -n "${NAMESPACE}" -o yaml
    kubectl get service/keycloak -n "${NAMESPACE}" -o yaml
    kubectl get poddisruptionbudget/keycloak -n "${NAMESPACE}" -o yaml
    kubectl get serviceaccount/keycloak-operator -n "${NAMESPACE}" -o yaml
    kubectl get serviceaccount/keycloak-config-cli -n "${NAMESPACE}" -o yaml
    kubectl get role/keycloak-operator -n "${NAMESPACE}" -o yaml
    kubectl get rolebinding/keycloak-operator -n "${NAMESPACE}" -o yaml
    kubectl get deployment/keycloak-operator -n "${NAMESPACE}" -o yaml
} >"${RENDERED}"

if grep -q '{{' "${RENDERED}"; then
    fail "Rendered KRO resources still contain Go/Helm-style template delimiters"
fi

info "KRO deployment path materialization verified"
