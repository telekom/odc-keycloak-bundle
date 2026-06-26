#!/usr/bin/env bash
# Validates that all CRD CEL admission rules reject invalid resources and accept valid ones.
# Requires: kubectl configured against a cluster with the Keycloak CRDs installed.
set -euo pipefail

NAMESPACE="${NAMESPACE:-cel-validation-test}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURES_DIR="${SCRIPT_DIR}/../../tests/cel"
PASS=0
FAIL=0
RESULT_NAMES=()
RESULT_STATUSES=()

# ── helpers ──────────────────────────────────────────────────────────────────

pass() {
    PASS=$((PASS + 1))
    echo "  PASS: $1"
    RESULT_NAMES+=("$1")
    RESULT_STATUSES+=("pass")
}

fail() {
    FAIL=$((FAIL + 1))
    echo "  FAIL: $1"
    RESULT_NAMES+=("$1")
    RESULT_STATUSES+=("fail")
}

assert_rejected() {
    local name="$1"
    local file="$2"
    local expected_msg="$3"

    actual=$(kubectl apply -n "${NAMESPACE}" -f "${file}" 2>&1 || true)
    if echo "${actual}" | grep -qF "${expected_msg}"; then
        pass "${name}"
    else
        fail "${name} — expected rejection containing '${expected_msg}', got: ${actual}"
    fi
}

assert_accepted() {
    local name="$1"
    local file="$2"

    if kubectl apply -n "${NAMESPACE}" -f "${file}" 2>&1; then
        pass "${name}"
    else
        fail "${name} — expected acceptance but got an error"
    fi
}

# ── setup ────────────────────────────────────────────────────────────────────

echo "Setting up namespace ${NAMESPACE}..."
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

echo ""
echo "Waiting for CRDs to be established..."
for crd in \
    realms.keycloak.opendefense.cloud \
    clients.keycloak.opendefense.cloud \
    identityproviders.keycloak.opendefense.cloud \
    authflows.keycloak.opendefense.cloud \
    users.keycloak.opendefense.cloud; do
    kubectl wait --for=condition=Established crd/"${crd}" --timeout=30s
done

# ── invalid fixtures (must be rejected) ─────────────────────────────────────

echo ""
echo "=== Invalid fixtures (must be rejected) ==="

assert_rejected \
    "IdentityProvider OIDC without config" \
    "${FIXTURES_DIR}/invalid/identity-provider-oidc-no-config.yaml" \
    "issuerUrl or authorizationUrl"

assert_rejected \
    "IdentityProvider OIDC with empty issuerUrl" \
    "${FIXTURES_DIR}/invalid/identity-provider-oidc-empty-issuer.yaml" \
    "non-empty issuerUrl or authorizationUrl"

assert_rejected \
    "IdentityProvider SAML without singleSignOnServiceUrl" \
    "${FIXTURES_DIR}/invalid/identity-provider-saml-no-sso.yaml" \
    "singleSignOnServiceUrl is required"

assert_rejected \
    "Client: publicClient and serviceAccountsEnabled both true" \
    "${FIXTURES_DIR}/invalid/client-public-and-sa.yaml" \
    "mutually exclusive"

assert_rejected \
    "Client: standardFlowEnabled without redirectUris" \
    "${FIXTURES_DIR}/invalid/client-standard-flow-no-redirect.yaml" \
    "redirectUris must not be empty"

assert_rejected \
    "Realm: internationalizationEnabled without supportedLocales" \
    "${FIXTURES_DIR}/invalid/realm-i18n-no-locales.yaml" \
    "supportedLocales must not be empty"

assert_rejected \
    "Realm: internationalizationEnabled without defaultLocale" \
    "${FIXTURES_DIR}/invalid/realm-i18n-no-default-locale.yaml" \
    "defaultLocale must be set"

assert_rejected \
    "User: initialPassword with empty secretName" \
    "${FIXTURES_DIR}/invalid/user-initial-password-empty-secret-name.yaml" \
    "initialPassword.secretName"

# ── alias immutability (requires create-then-update) ─────────────────────────

echo ""
echo "=== Alias immutability test ==="

kubectl apply -n "${NAMESPACE}" -f "${FIXTURES_DIR}/valid/authflow.yaml" >/dev/null 2>&1

actual=$(kubectl apply -n "${NAMESPACE}" -f "${FIXTURES_DIR}/invalid/authflow-alias-change.yaml" 2>&1 || true)
if echo "${actual}" | grep -qF "alias is immutable"; then
    pass "AuthFlow: alias change rejected on update"
else
    fail "AuthFlow: alias change should have been rejected, got: ${actual}"
fi

# ── valid fixtures (must be accepted) ────────────────────────────────────────

echo ""
echo "=== Valid fixtures (must be accepted) ==="

assert_accepted \
    "IdentityProvider OIDC with issuerUrl" \
    "${FIXTURES_DIR}/valid/identity-provider-oidc.yaml"

assert_accepted \
    "IdentityProvider SAML with singleSignOnServiceUrl" \
    "${FIXTURES_DIR}/valid/identity-provider-saml.yaml"

assert_accepted \
    "Client: publicClient without serviceAccountsEnabled" \
    "${FIXTURES_DIR}/valid/client-public.yaml"

assert_accepted \
    "Client: serviceAccountsEnabled without publicClient" \
    "${FIXTURES_DIR}/valid/client-service-account.yaml"

assert_accepted \
    "Realm: internationalizationEnabled with both locales fields" \
    "${FIXTURES_DIR}/valid/realm-i18n.yaml"

assert_accepted \
    "User: initialPassword with explicit secretKey" \
    "${FIXTURES_DIR}/valid/user-initial-password.yaml"

# ── cleanup ──────────────────────────────────────────────────────────────────

kubectl delete namespace "${NAMESPACE}" --ignore-not-found >/dev/null 2>&1

# ── summary ──────────────────────────────────────────────────────────────────

echo ""
echo "=== Results: ${PASS} passed, ${FAIL} failed ==="

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    {
        echo "### CEL Validation Tests"
        echo ""
        echo "| Test | Kind | Result |"
        echo "| :--- | :--- | :---: |"
        for i in "${!RESULT_NAMES[@]}"; do
            name="${RESULT_NAMES[$i]}"
            status="${RESULT_STATUSES[$i]}"
            case "${name}" in
                *"rejected"* | *"without"* | *"empty"* | *"both true"* | *"change"*)
                    kind="reject"
                    ;;
                *)
                    kind="accept"
                    ;;
            esac
            if [ "${status}" = "pass" ]; then
                echo "| ${name} | ${kind} | pass |"
            else
                echo "| ${name} | ${kind} | **FAIL** |"
            fi
        done
        echo ""
        if [ "${FAIL}" -eq 0 ]; then
            echo "**${PASS} / $((PASS + FAIL)) passed** — all admission rules behave as expected."
        else
            echo "**${FAIL} of $((PASS + FAIL)) failed** — check step output for details."
        fi
    } >>"${GITHUB_STEP_SUMMARY}"
fi

if [ "${FAIL}" -gt 0 ]; then
    exit 1
fi
