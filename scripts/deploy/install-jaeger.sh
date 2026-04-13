#!/bin/bash
# ==============================================================================
# install-jaeger.sh - Deploy Jaeger all-in-one (ephemeral) into observability namespace
# ==============================================================================
#
# PURPOSE:
#   Deploys the Jaeger all-in-one image (in-memory, no persistence) into a
#   dedicated "observability" namespace. Exposes the Jaeger UI and OTLP
#   ingestion endpoints (gRPC and HTTP) via a ClusterIP Service.
#   Intended for CI pipelines and ephemeral dev clusters only — trace data
#   is lost when the pod restarts.
#
# USAGE:
#   ./scripts/deploy/install-jaeger.sh [version]
#
# ARGUMENTS:
#   version   Optional. Jaeger all-in-one image tag (default: "1.58.1")
#
# EXAMPLES:
#   ./scripts/deploy/install-jaeger.sh              # Deploy v1.58.1
#   ./scripts/deploy/install-jaeger.sh 1.60.0       # Deploy specific version
#
# CREATES:
#   - Namespace "observability"
#   - Deployment "jaeger" in namespace "observability"
#   - Service "jaeger" in namespace "observability" (ClusterIP)
#     * port 16686 — Jaeger UI
#     * port 4317  — OTLP gRPC ingestion
#     * port 4318  — OTLP HTTP ingestion
#
# NOTES:
#   - No persistence: all trace data is held in memory and lost on pod restart
#   - Requires cluster-admin permissions (creates Namespace)
#   - Only needs to be run once per cluster lifecycle
#   - Not suitable for production use
#   - deploy-all.sh calls this automatically if needed
#
# SEE ALSO:
#   https://www.jaegertracing.io/docs/latest/deployment/
#   https://quay.io/repository/jaegertracing/all-in-one
#
# ==============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

JAEGER_VERSION="${1:-1.58.1}"

info "Deploying Jaeger all-in-one v${JAEGER_VERSION} (in-memory) into namespace 'observability'..."

kubectl apply -f - <<EOF || fail "Failed to apply Jaeger manifests" 1
---
apiVersion: v1
kind: Namespace
metadata:
  name: observability
  labels:
    app.kubernetes.io/managed-by: install-jaeger
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
  namespace: observability
  labels:
    app: jaeger
    app.kubernetes.io/name: jaeger
    app.kubernetes.io/component: all-in-one
    app.kubernetes.io/managed-by: install-jaeger
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jaeger
  template:
    metadata:
      labels:
        app: jaeger
        app.kubernetes.io/name: jaeger
        app.kubernetes.io/component: all-in-one
    spec:
      securityContext:
        runAsUser: 1000
        runAsNonRoot: true
      containers:
        - name: jaeger
          image: quay.io/jaegertracing/all-in-one:${JAEGER_VERSION}
          env:
            - name: COLLECTOR_OTLP_ENABLED
              value: "true"
          ports:
            - name: ui
              containerPort: 16686
              protocol: TCP
            - name: otlp-grpc
              containerPort: 4317
              protocol: TCP
            - name: otlp-http
              containerPort: 4318
              protocol: TCP
          resources:
            requests:
              cpu: 50m
              memory: 128Mi
            limits:
              cpu: 200m
              memory: 256Mi
          securityContext:
            runAsUser: 1000
            runAsNonRoot: true
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger
  namespace: observability
  labels:
    app: jaeger
    app.kubernetes.io/name: jaeger
    app.kubernetes.io/component: all-in-one
    app.kubernetes.io/managed-by: install-jaeger
spec:
  type: ClusterIP
  selector:
    app: jaeger
  ports:
    - name: ui
      port: 16686
      targetPort: ui
      protocol: TCP
    - name: otlp-grpc
      port: 4317
      targetPort: otlp-grpc
      protocol: TCP
    - name: otlp-http
      port: 4318
      targetPort: otlp-http
      protocol: TCP
EOF

info "Waiting for Jaeger to be ready..."
kubectl wait --for=condition=available --timeout=120s deployment/jaeger -n observability || fail "Jaeger did not become ready" 2

info "Jaeger all-in-one deployed."
info "OTLP/gRPC endpoint: http://jaeger.observability.svc:4317"
