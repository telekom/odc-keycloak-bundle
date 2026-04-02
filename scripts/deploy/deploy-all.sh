#!/bin/bash
# ==============================================================================
# deploy-all.sh - Deploy a complete Keycloak instance with all dependencies
# ==============================================================================
#
# PURPOSE:
#   Main entry point for deploying a fully functional Keycloak instance.
#   This script orchestrates the complete deployment including:
#   - CloudNativePG operator installation (if not present)
#   - Prometheus Operator installation (if not present)
#   - Jaeger all-in-one installation (if not present, for OTEL tracing)
#   - PostgreSQL database cluster
#   - Keycloak application server
#   - Monitoring manifests (ServiceMonitor, PodMonitor, PrometheusRule)
#
# USAGE:
#   ./scripts/deploy/deploy-all.sh [instance-name] [options]
#
# ARGUMENTS:
#   instance-name   Optional. Name for the instance.
#                   If not provided, generates "dev-<random>" automatically
#
# OPTIONS:
#   --namespace NS  Target namespace (administrator-controlled). Defaults to
#                   "keycloak-<instance-name>" when not provided.
#   --clean         Delete and recreate the instance namespace before deploying
#   --no-monitoring Skip Prometheus Operator, Jaeger, and monitoring manifests
#
# NAMING CONVENTION:
#   - poc           : Proof of concept
#   - alpha, beta   : Pre-release stages
#   - final         : Production release
#
# EXAMPLES:
#   ./scripts/deploy/deploy-all.sh                                   # Deploy to keycloak-dev-<random>
#   ./scripts/deploy/deploy-all.sh poc --namespace identity-poc      # Deploy to identity-poc
#   ./scripts/deploy/deploy-all.sh mytest --no-monitoring            # Skip monitoring stack
#
# DEPENDENCIES:
#   - kubectl configured with cluster access
#   - Calls: install-cnpg.sh, install-prometheus-operator.sh, install-jaeger.sh,
#            deploy-postgres.sh, deploy-keycloak.sh, deploy-operator.sh
#
# SEE ALSO:
#   utils/status.sh        - Check instance status
#   utils/portforward.sh   - Access Keycloak locally
#   cleanup.sh             - Remove a single instance
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

PROJECT_ROOT="$(cd "$(dirname "$(dirname "$SCRIPT_DIR")")" && pwd)"
info "Project Root: $PROJECT_ROOT"

# Parse arguments
INSTANCE_NAME=""
NAMESPACE_OVERRIDE=""
CLEAN=false
SKIP_MONITORING=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--clean)
            CLEAN=true
            shift
            ;;
        --no-monitoring)
            SKIP_MONITORING=true
            shift
            ;;
        --namespace)
            if [[ -z "$2" || "$2" == -* ]]; then
                fail "--namespace requires a value" 1
            fi
            NAMESPACE_OVERRIDE="$2"
            shift 2
            ;;
        -*)
            fail "Unknown option: $1" 1
            ;;
        *)
            if [[ -z "$INSTANCE_NAME" ]]; then
                INSTANCE_NAME="$1"
            else
                fail "Multiple instance names provided" 1
            fi
            shift
            ;;
    esac
done

# Generate random suffix if no instance name provided
if [[ -z "$INSTANCE_NAME" ]]; then
    INSTANCE_NAME="dev-$(generate_suffix)"
    info "No instance name provided, using: $INSTANCE_NAME"
fi

NAMESPACE="${NAMESPACE_OVERRIDE:-keycloak-$INSTANCE_NAME}"

if [[ "$CLEAN" == "true" ]]; then
    info "Cleanup requested for instance: $INSTANCE_NAME (namespace: $NAMESPACE)"
    if kubectl get namespace "$NAMESPACE" &>/dev/null; then
        "$SCRIPT_DIR/cleanup.sh" "$INSTANCE_NAME" "$NAMESPACE"
        info "Waiting for namespace deletion (timeout 300s)..."
        if ! kubectl wait --for=delete namespace/"$NAMESPACE" --timeout=300s; then
            warn "Namespace deletion did not finish within the initial 300s wait window."
        fi

        # Namespace deletion can complete shortly after the wait timeout.
        # Add a short grace polling window before failing the deployment.
        if kubectl get namespace "$NAMESPACE" &>/dev/null; then
            PHASE=$(kubectl get namespace "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "unknown")
            warn "Namespace $NAMESPACE still exists after initial wait (phase=$PHASE)."

            if [[ "$PHASE" == "Terminating" ]]; then
                    grace_timeout_seconds=300
                    grace_check_interval_seconds=90
                    grace_start=""
                    elapsed=0
                    remaining=0
                    sleep_for=0

                info "Applying additional grace wait for terminating namespace (${grace_timeout_seconds}s, check interval ${grace_check_interval_seconds}s)..."
                grace_start="$(date +%s)"

                while kubectl get namespace "$NAMESPACE" &>/dev/null; do
                    elapsed="$(( $(date +%s) - grace_start ))"
                    if (( elapsed >= grace_timeout_seconds )); then
                        break
                    fi

                    remaining="$(( grace_timeout_seconds - elapsed ))"
                    sleep_for="$grace_check_interval_seconds"
                    if (( remaining < sleep_for )); then
                        sleep_for="$remaining"
                    fi
                    sleep "$sleep_for"
                done

                if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
                    info "Namespace $NAMESPACE deletion completed during grace period."
                fi
            fi
        fi

        # If the namespace is still present (stuck in Terminating), strip all
        # remaining metadata.finalizers so the API server can garbage-collect it.
        if kubectl get namespace "$NAMESPACE" &>/dev/null; then
            PHASE=$(kubectl get namespace "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "unknown")
            warn "Namespace $NAMESPACE still present after 300s wait (phase=$PHASE); attempting finalizer force-clear."

            ALL_RESOURCES="$(kubectl api-resources --verbs=list --namespaced -o name 2>/dev/null || true)"
            while IFS= read -r resource; do
                [[ -z "$resource" ]] && continue
                while IFS= read -r obj; do
                    [[ -z "$obj" ]] && continue
                    kubectl patch "$obj" -n "$NAMESPACE" --type=merge \
                        -p '{"metadata":{"finalizers":[]}}' 2>/dev/null || true
                done < <(kubectl get "$resource" -n "$NAMESPACE" -o name 2>/dev/null || true)
            done <<<"$ALL_RESOURCES"

            # Give the API server a moment to process the cleared finalizers.
            kubectl wait --for=delete namespace/"$NAMESPACE" --timeout=60s || true
        fi

        if kubectl get namespace "$NAMESPACE" &>/dev/null; then
            PHASE=$(kubectl get namespace "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "unknown")
            FINALIZERS=$(kubectl get namespace "$NAMESPACE" -o jsonpath='{.spec.finalizers}' 2>/dev/null || echo "unknown")
            warn "Namespace $NAMESPACE still exists after cleanup (phase=$PHASE, finalizers=$FINALIZERS)."
            warn "Remaining namespaced objects (first 100 lines):"
            kubectl api-resources --verbs=list --namespaced -o name 2>/dev/null \
              | xargs -n 1 kubectl get -n "$NAMESPACE" --ignore-not-found -o name 2>/dev/null \
              | head -n 100 || true
            fail "Cleanup did not fully complete; refusing to deploy into a terminating namespace." 1
        fi
    else
        info "Namespace $NAMESPACE already gone."
    fi
fi

info "=== Deploying Keycloak Instance: $INSTANCE_NAME ==="
info "Namespace: $NAMESPACE"

# Check/install CloudNativePG
if ! kubectl get crd clusters.postgresql.cnpg.io &>/dev/null; then
    info "CloudNativePG not found. Installing..."
    "$SCRIPT_DIR/install-cnpg.sh" || fail "CloudNativePG installation failed" 1
else
    info "CloudNativePG CRD already present. Checking operator readiness..."
fi

# Always verify the CNPG controller and its webhook are ready before proceeding.
# The CRD may exist while the operator pod is restarting (e.g. after a node drain),
# which causes "no endpoints available for service cnpg-webhook-service" on apply.
kubectl wait --for=condition=available --timeout=120s \
    deployment/cnpg-cloudnative-pg -n cnpg-system \
    || fail "CNPG controller-manager did not become available" 1

info "Waiting for CNPG webhook endpoints to be populated..."
timeout 60s bash -c \
    "until kubectl get endpoints cnpg-webhook-service -n cnpg-system 2>/dev/null \
         | grep -v '<none>' | grep -q '[0-9]'; do sleep 2; done" \
    || fail "CNPG webhook endpoints did not become available" 1
info "CNPG webhook ready."

# Check/install Prometheus Operator and Jaeger (unless skipped)
if [[ "$SKIP_MONITORING" == "false" ]]; then
    if ! kubectl get crd servicemonitors.monitoring.coreos.com &>/dev/null; then
        info "Prometheus Operator not found. Installing..."
        "$SCRIPT_DIR/install-prometheus-operator.sh" || fail "Prometheus Operator installation failed" 1
    else
        info "Prometheus Operator already installed. Skipping."
    fi

    if ! kubectl get deployment jaeger -n observability &>/dev/null; then
        info "Jaeger not found. Installing..."
        "$SCRIPT_DIR/install-jaeger.sh" || fail "Jaeger installation failed" 1
    else
        info "Jaeger already installed. Skipping."
    fi

    # Create per-instance Prometheus RBAC (ClusterRole names include the namespace
    # so multiple parallel instances do not collide).
    info "Applying Prometheus RBAC for namespace: $NAMESPACE"
    kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus-${NAMESPACE}
  labels:
    app.kubernetes.io/managed-by: deploy-all
    instance: "${NAMESPACE}"
rules:
  - apiGroups: [""]
    resources: [nodes, nodes/proxy, nodes/metrics, services, endpoints, pods]
    verbs: [get, list, watch]
  - apiGroups: [""]
    resources: [configmaps]
    verbs: [get]
  - nonResourceURLs: [/metrics, /metrics/cadvisor]
    verbs: [get]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus-${NAMESPACE}
  labels:
    app.kubernetes.io/managed-by: deploy-all
    instance: "${NAMESPACE}"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus-${NAMESPACE}
subjects:
  - kind: ServiceAccount
    name: prometheus
    namespace: ${NAMESPACE}
EOF
else
    info "Monitoring stack skipped (--no-monitoring)."
fi

# Deploy PostgreSQL
"$SCRIPT_DIR/deploy-postgres.sh" "$NAMESPACE" || fail "PostgreSQL deployment failed" 2

# Deploy Keycloak
"$SCRIPT_DIR/deploy-keycloak.sh" "$NAMESPACE" || fail "Keycloak deployment failed" 3

# Deploy Client Operator
"$SCRIPT_DIR/deploy-operator.sh" "$NAMESPACE" || fail "Client Operator deployment failed" 4

# Apply monitoring manifests (unless skipped)
if [[ "$SKIP_MONITORING" == "false" ]]; then
    info "Applying monitoring manifests to namespace: $NAMESPACE"
    kubectl apply -n "$NAMESPACE" -f "$PROJECT_ROOT/manifests/monitoring/keycloak-service-monitor.yaml" || fail "Failed to apply ServiceMonitor" 5
    kubectl apply -n "$NAMESPACE" -f "$PROJECT_ROOT/manifests/monitoring/cnpg-pod-monitor.yaml" || fail "Failed to apply PodMonitor" 5
    kubectl apply -n "$NAMESPACE" -f "$PROJECT_ROOT/manifests/monitoring/keycloak-prometheus-rules.yaml" || fail "Failed to apply PrometheusRules" 5
    kubectl apply -n "$NAMESPACE" -f "$PROJECT_ROOT/manifests/monitoring/prometheus.yaml" || fail "Failed to apply Prometheus instance" 5
    info "Monitoring manifests applied."

    info "Waiting for Prometheus StatefulSet to be created by operator..."
    timeout 120s bash -c \
        "until kubectl get statefulset prometheus-keycloak -n $NAMESPACE &>/dev/null; do sleep 2; done" \
        || fail "Prometheus StatefulSet was not created by operator" 5

    info "Waiting for Prometheus StatefulSet to be ready (timeout 120s)..."
    kubectl rollout status statefulset/prometheus-keycloak -n "$NAMESPACE" --timeout=120s \
        || fail "Prometheus StatefulSet did not become ready" 5
    info "Prometheus instance ready."
fi

info "=== Deployment complete ==="
info ""
info "Instance name: $INSTANCE_NAME"
info ""
info "Next steps:"
info "  1. Port-forward: ./scripts/utils/portforward.sh $INSTANCE_NAME $NAMESPACE"
info "  2. Open: http://localhost:8080 (admin/admin)"
info "  3. Install CRD: kubectl apply -f charts/keycloak-operator/crds/"
info "  4. Create client: kubectl apply -f examples/client-example.yaml -n $NAMESPACE"
if [[ "$SKIP_MONITORING" == "false" ]]; then
info ""
info "Observability:"
info "  Jaeger UI:    kubectl port-forward -n observability svc/jaeger 16686:16686"
info "  Metrics:      kubectl port-forward -n $NAMESPACE svc/keycloak 9000:9000"
info "                curl http://localhost:9000/metrics"
info "  Prometheus:   kubectl port-forward -n $NAMESPACE svc/prometheus 9090:9090"
info "                curl http://localhost:9090/api/v1/alerts"
fi
info ""
info "To remove this instance:"
info "  ./scripts/deploy/cleanup.sh $INSTANCE_NAME $NAMESPACE"
