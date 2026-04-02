#!/bin/bash
# ==============================================================================
# deploy-operator.sh - Deploy Keycloak Client Operator
# ==============================================================================
#
# PURPOSE:
#   Deploys the Keycloak Client Operator which manages Client custom
#   resources. The operator automates client registration in Keycloak.
#
# USAGE:
#   ./scripts/deploy/deploy-operator.sh [namespace]
#
# ARGUMENTS:
#   namespace   Optional. Operator namespace (default: "keycloak-operator")
#
# EXAMPLES:
#   ./scripts/deploy/deploy-operator.sh                        # Deploy to keycloak-operator
#   ./scripts/deploy/deploy-operator.sh keycloak-system        # Custom namespace
#
# CREATES:
#   - Namespace for the operator
#   - CRD: Clients.keycloak.opendefense.cloud
#   - Operator deployment (when templates are complete)
#
# STATUS:
#   Operator is implemented (Bash-based controller).
#
# SEE ALSO:
#   charts/keycloak-operator/ - Helm chart for the operator
#   examples/client-example.yaml     - Example Client resource
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

OPERATOR_NAMESPACE="${1:-keycloak-operator}"
PROJECT_ROOT="$(cd "$(dirname "$(dirname "$SCRIPT_DIR")")" && pwd)"
VERIFY_REQUIRED="${OCM_VERIFY_REQUIRED:-false}"
OCM_CTF_PATH="${OCM_CTF_PATH:-$PROJECT_ROOT/ocm-output/keycloak-bundle-ctf.tar.gz}"
OCM_PUBLIC_KEY_PATH="${OCM_PUBLIC_KEY_PATH:-$PROJECT_ROOT/ocm-key.pub}"
OCM_SIGNATURE_NAME="${OCM_SIGNATURE_NAME:-keycloak-bundle-sig}"

info "Deploying Keycloak Client Operator to: $OPERATOR_NAMESPACE"

# Create operator namespace
kubectl create namespace "$OPERATOR_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - || fail "Failed to create namespace" 1

# Install CRDs
info "Installing CRDs..."
kubectl apply -f "$PROJECT_ROOT/charts/keycloak-operator/crds/" || fail "Failed to install CRDs" 2

# Check for helm
if ! command -v helm &> /dev/null; then
    fail "Helm is not installed. Please install helm first." 1
fi

# Air-gapped hardening gate: verify signed OCM artifact before helm deployment.
if [[ "${VERIFY_REQUIRED,,}" == "true" || "${VERIFY_REQUIRED,,}" == "1" || "${VERIFY_REQUIRED,,}" == "yes" ]]; then
    info "OCM signature verification is enforced before helm install."
    if [[ ! -f "$OCM_CTF_PATH" ]]; then
        fail "OCM CTF archive not found for verification: $OCM_CTF_PATH" 7
    fi
    if [[ ! -f "$OCM_PUBLIC_KEY_PATH" ]]; then
        fail "OCM public key not found for verification: $OCM_PUBLIC_KEY_PATH" 8
    fi
    "$PROJECT_ROOT/scripts/ocm/ocm-verify.sh" "$OCM_CTF_PATH" "$OCM_PUBLIC_KEY_PATH" "$OCM_SIGNATURE_NAME" || fail "OCM signature verification failed" 9
fi

# Deploy operator using Helm
# Allow callers to override the image so CI pipelines can inject the freshly-built tag.
# If OPERATOR_IMAGE_REPO or OPERATOR_IMAGE_TAG are set they are forwarded as --set overrides.
HELM_IMAGE_ARGS=()
if [ -n "${OPERATOR_IMAGE_REPO:-}" ]; then
    HELM_IMAGE_ARGS+=(--set "image.repository=${OPERATOR_IMAGE_REPO}")
fi
if [ -n "${OPERATOR_IMAGE_TAG:-}" ]; then
    HELM_IMAGE_ARGS+=(--set "image.tag=${OPERATOR_IMAGE_TAG}")
fi

# Create a docker-registry pull secret when registry credentials are provided.
# OPERATOR_IMAGE_PULL_SECRET — name for the k8s secret (e.g. "harbor-pull-secret")
# REGISTRY_USERNAME / REGISTRY_PASSWORD — credentials for the registry host
if [ -n "${OPERATOR_IMAGE_PULL_SECRET:-}" ] && \
   [ -n "${REGISTRY_USERNAME:-}" ] && \
   [ -n "${REGISTRY_PASSWORD:-}" ]; then
    REGISTRY_HOST="${OPERATOR_IMAGE_REPO%%/*}"
    info "Creating image pull secret '${OPERATOR_IMAGE_PULL_SECRET}' for ${REGISTRY_HOST}..."
    kubectl create secret docker-registry "${OPERATOR_IMAGE_PULL_SECRET}" \
        --docker-server="${REGISTRY_HOST}" \
        --docker-username="${REGISTRY_USERNAME}" \
        --docker-password="${REGISTRY_PASSWORD}" \
        --namespace "${OPERATOR_NAMESPACE}" \
        --dry-run=client -o yaml | kubectl apply -f - || fail "Failed to create image pull secret" 5
    HELM_IMAGE_ARGS+=(--set "imagePullSecrets[0].name=${OPERATOR_IMAGE_PULL_SECRET}")
fi

# Ensure Keycloak Admin Password Secret exists for the Operator
# In this environment, we fetch it from the 'keycloak-admin' secret created during deploy-keycloak.sh
if kubectl get secret keycloak-admin -n "$OPERATOR_NAMESPACE" &>/dev/null; then
    ADMIN_PASS=$(kubectl get secret keycloak-admin -n "$OPERATOR_NAMESPACE" -o jsonpath='{.data.KEYCLOAK_ADMIN_PASSWORD}' | base64 -d)
    ADMIN_USER=$(kubectl get secret keycloak-admin -n "$OPERATOR_NAMESPACE" -o jsonpath='{.data.KEYCLOAK_ADMIN}' | base64 -d)
    
    info "Provisioning 'keycloak-admin-creds' for Operator..."
    kubectl create secret generic keycloak-admin-creds \
        --from-literal=username="$ADMIN_USER" \
        --from-literal=password="$ADMIN_PASS" \
        --namespace "$OPERATOR_NAMESPACE" \
        --dry-run=client -o yaml | kubectl apply -f - || fail "Failed to create admin secret" 6
fi

info "Deploying operator with Helm..."
[ ${#HELM_IMAGE_ARGS[@]} -gt 0 ] && info "  image override: ${OPERATOR_IMAGE_REPO:-}:${OPERATOR_IMAGE_TAG:-}"

MAX_HELM_ATTEMPTS="${MAX_HELM_ATTEMPTS:-5}"
HELM_RETRY_DELAY_SECONDS="${HELM_RETRY_DELAY_SECONDS:-20}"

helm_upgrade_install() {
    helm upgrade --install keycloak-operator "$PROJECT_ROOT/charts/keycloak-operator" \
        --namespace "$OPERATOR_NAMESPACE" \
        --create-namespace \
        --wait \
        --timeout 120s \
        "${HELM_IMAGE_ARGS[@]}"
}

attempt=1
while [ "$attempt" -le "$MAX_HELM_ATTEMPTS" ]; do
    HELM_LOG="$(mktemp)"

    set +e
    helm_upgrade_install >"$HELM_LOG" 2>&1
    rc=$?
    set -e

    cat "$HELM_LOG"

    if [ "$rc" -eq 0 ]; then
        rm -f "$HELM_LOG"
        break
    fi

    if grep -Eqi "another operation .* in progress" "$HELM_LOG"; then
        if [ "$attempt" -lt "$MAX_HELM_ATTEMPTS" ]; then
            warn "Helm release is locked (attempt ${attempt}/${MAX_HELM_ATTEMPTS}). Waiting ${HELM_RETRY_DELAY_SECONDS}s before retry..."
            helm status keycloak-operator --namespace "$OPERATOR_NAMESPACE" 2>/dev/null || true
            rm -f "$HELM_LOG"
            sleep "$HELM_RETRY_DELAY_SECONDS"
            attempt=$((attempt + 1))
            continue
        fi
    fi

    rm -f "$HELM_LOG"
    fail "Failed to deploy operator chart" 3
done

if [ "$attempt" -gt "$MAX_HELM_ATTEMPTS" ]; then
    fail "Failed to deploy operator chart after ${MAX_HELM_ATTEMPTS} attempts" 3
fi

info "Operator deployed and ready."
info "Operator and CRDs installed successfully."
