#!/bin/bash
# ==============================================================================
# test-crd-suite.sh - Orchestrates smoke and lifecycle CRD suites
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="${1:-}"
SUITE="${2:-full}"

if [[ -z "${NAMESPACE}" ]]; then
    echo "Usage: $0 <namespace> [smoke|lifecycle|full]" >&2
    exit 1
fi

case "${SUITE}" in
    smoke)
        "${SCRIPT_DIR}/test-crd-smoke.sh" "${NAMESPACE}"
        ;;
    lifecycle)
        "${SCRIPT_DIR}/test-crd-lifecycle.sh" "${NAMESPACE}"
        ;;
    full)
        "${SCRIPT_DIR}/test-crd-smoke.sh" "${NAMESPACE}"
        "${SCRIPT_DIR}/test-crd-lifecycle.sh" "${NAMESPACE}"
        ;;
    *)
        echo "Unknown suite '${SUITE}'. Expected: smoke|lifecycle|full" >&2
        exit 1
        ;;
esac
