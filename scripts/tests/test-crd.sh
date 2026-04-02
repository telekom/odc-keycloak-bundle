#!/bin/bash
# ==============================================================================
# test-crd.sh - Backward-compatible wrapper around modular CRD test suites
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "[INFO] test-crd.sh is a compatibility wrapper. Delegating to test-crd-suite.sh."
exec "${SCRIPT_DIR}/test-crd-suite.sh" "$@"
