#!/usr/bin/env bash
set -euo pipefail

# Generates throwaway Kubernetes Secret manifests for local testing.
# Output is written outside the repository and must not be committed.

NAMESPACE="${NAMESPACE:-keycloak-system}"
OUTPUT_FILE="${OUTPUT_FILE:-/tmp/keycloak-test-secrets.yaml}"

password="$(openssl rand -hex 16)"

cat >"$OUTPUT_FILE" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: keycloak-admin
  namespace: ${NAMESPACE}
type: Opaque
stringData:
  username: admin
  password: ${password}
EOF

echo "Generated ${OUTPUT_FILE} for local testing only."
