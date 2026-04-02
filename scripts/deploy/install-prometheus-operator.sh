#!/bin/bash
# ==============================================================================
# install-prometheus-operator.sh - Install Prometheus Operator (cluster-wide)
# ==============================================================================
#
# PURPOSE:
#   Installs the Prometheus Operator via the upstream bundle manifest, which
#   provides the CRDs and controller needed to manage Prometheus, Alertmanager,
#   ServiceMonitor, PodMonitor, and PrometheusRule resources in the cluster.
#   This is a one-time, cluster-wide installation.
#
# USAGE:
#   ./scripts/deploy/install-prometheus-operator.sh [version]
#
# ARGUMENTS:
#   version   Optional. Prometheus Operator version (default: "0.80.1")
#
# EXAMPLES:
#   ./scripts/deploy/install-prometheus-operator.sh              # Install v0.80.1
#   ./scripts/deploy/install-prometheus-operator.sh 0.81.0       # Install specific version
#
# CREATES:
#   - CRDs for Prometheus Operator (alertmanagerconfigs.monitoring.coreos.com,
#     alertmanagers.monitoring.coreos.com, podmonitors.monitoring.coreos.com,
#     probes.monitoring.coreos.com, prometheusagents.monitoring.coreos.com,
#     prometheuses.monitoring.coreos.com, prometheusrules.monitoring.coreos.com,
#     scrapeconfigs.monitoring.coreos.com, servicemonitors.monitoring.coreos.com,
#     thanosrulers.monitoring.coreos.com)
#   - Deployment "prometheus-operator" in namespace "default"
#   - ClusterRole and ClusterRoleBinding for the operator
#
# NOTES:
#   - Requires cluster-admin permissions
#   - Only needs to be run once per cluster
#   - Uses --server-side apply to handle large CRD manifests safely
#   - deploy-all.sh calls this automatically if needed
#
# SEE ALSO:
#   https://github.com/prometheus-operator/prometheus-operator
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

PROM_VERSION="${1:-0.80.1}"

info "Installing Prometheus Operator v${PROM_VERSION}..."

kubectl apply --server-side -f "https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v${PROM_VERSION}/bundle.yaml" || fail "Failed to apply Prometheus Operator manifests" 1

info "Waiting for Prometheus Operator to be ready..."
kubectl wait --for=condition=available --timeout=120s deployment/prometheus-operator -n default || fail "Prometheus Operator did not become ready" 2

info "Prometheus Operator installed."
kubectl get crd | grep monitoring.coreos.com
