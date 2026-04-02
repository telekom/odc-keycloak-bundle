#!/usr/bin/env bash
set -euo pipefail

MODE="static"
NAMESPACE=""
CLUSTER_NAME="keycloak-db"
TARGET_CLUSTER_NAME=""
OBJECT_STORE_NAME=""
DESTINATION_PATH=""
CREDENTIALS_SECRET=""
ENDPOINT_URL=""
TIMEOUT_SECONDS="300"
KUBECONFIG_FILE=""
KEEP_BACKUP_CR="false"
BACKUP_LAST_ERROR=""
BACKUP_LAST_PHASE=""

PLUGIN_NAME="barman-cloud.cloudnative-pg.io"

usage() {
  cat <<'EOF'
Usage:
  scripts/tests/test-backup-restore.sh [--live --namespace <ns> --destination-path <s3://...> --credentials-secret <secret> [--endpoint-url <url>] [--cluster-name <name>] [--object-store-name <name>] [--timeout-seconds <n>] [--restore-cluster-name <name>] [--kubeconfig <path>] [--keep-backup-cr]]

Modes:
  static (default)
    - Validates CNPG-native backup/restore consistency in repository files.

  live
    - Runs static checks and then triggers a real on-demand CNPG Backup.
    - Uses the CNPG Barman Cloud Plugin (ObjectStore + method=plugin).
    - Waits for backup completion and then applies a non-destructive CNPG recovery cluster manifest.
EOF
}

log() {
  printf '[INFO] %s\n' "$*"
}

fail() {
  printf '[ERROR] %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Required command not found: $1"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --live)
      MODE="live"
      shift
      ;;
    --namespace)
      NAMESPACE="${2:-}"
      shift 2
      ;;
    --cluster-name)
      CLUSTER_NAME="${2:-}"
      shift 2
      ;;
    --restore-cluster-name)
      TARGET_CLUSTER_NAME="${2:-}"
      shift 2
      ;;
    --object-store-name)
      OBJECT_STORE_NAME="${2:-}"
      shift 2
      ;;
    --destination-path)
      DESTINATION_PATH="${2:-}"
      shift 2
      ;;
    --credentials-secret)
      CREDENTIALS_SECRET="${2:-}"
      shift 2
      ;;
    --endpoint-url)
      ENDPOINT_URL="${2:-}"
      shift 2
      ;;
    --timeout-seconds)
      TIMEOUT_SECONDS="${2:-}"
      shift 2
      ;;
    --kubeconfig)
      KUBECONFIG_FILE="${2:-}"
      shift 2
      ;;
    --keep-backup-cr)
      KEEP_BACKUP_CR="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "Unknown argument: $1"
      ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

check_file() {
  local file="$1"
  [[ -f "$ROOT_DIR/$file" ]] || fail "Missing file: $file"
}

check_contains() {
  local file="$1"
  local pattern="$2"
  grep -Eq "$pattern" "$ROOT_DIR/$file" || fail "Expected pattern not found in $file: $pattern"
}

check_not_contains() {
  local file="$1"
  local pattern="$2"
  if grep -Eq "$pattern" "$ROOT_DIR/$file"; then
    fail "Unexpected legacy pattern found in $file: $pattern"
  fi
}

run_static_checks() {
  log "Running static CNPG-native backup/restore consistency checks"

  check_file "operator/api/v1alpha1/groupversion_info.go"
  check_file "operator/config/rbac/role.yaml"
  check_file "charts/keycloak-operator/files/role.yaml"
  check_file "component-constructor.yaml"
  check_file "examples/backup-example.yaml"
  check_file "examples/restore-cluster-example.yaml"
  check_file "docs/UPGRADE.md"

  check_not_contains "operator/api/v1alpha1/groupversion_info.go" "&Backup\\{\\}, &BackupList\\{\\}"
  check_not_contains "operator/cmd/main.go" "BackupReconciler"
  check_not_contains "operator/config/rbac/role.yaml" "^  - backups$|^  - backups/status$|^  - backups/finalizers$|^  - scheduledbackups$|^  - objectstores$"
  check_not_contains "charts/keycloak-operator/files/role.yaml" "^  - backups$|^  - backups/status$|^  - backups/finalizers$|^  - scheduledbackups$|^  - objectstores$"

  check_not_contains "component-constructor.yaml" "backup-crd|keycloakbackup-crd"

  check_contains "examples/backup-example.yaml" "apiVersion: postgresql.cnpg.io/v1"
  check_contains "examples/backup-example.yaml" "apiVersion: barmancloud.cnpg.io/v1"
  check_contains "examples/backup-example.yaml" "kind: ObjectStore"
  check_contains "examples/backup-example.yaml" "kind: Backup"
  check_contains "examples/backup-example.yaml" "kind: ScheduledBackup"
  check_contains "examples/backup-example.yaml" "method: plugin"
  check_contains "examples/backup-example.yaml" "pluginConfiguration:"
  check_contains "examples/restore-cluster-example.yaml" "apiVersion: postgresql.cnpg.io/v1"
  check_contains "examples/restore-cluster-example.yaml" "kind: Cluster"
  check_contains "examples/restore-cluster-example.yaml" "bootstrap:"
  check_contains "examples/restore-cluster-example.yaml" "recovery:"
  check_contains "examples/restore-cluster-example.yaml" "plugin:"
  check_contains "examples/restore-cluster-example.yaml" "barmanObjectName:"

  check_contains "docs/UPGRADE.md" "## 8\\. Backup and Restore \\(CNPG-native\\)"
  check_contains "docs/UPGRADE.md" "### 8\\.4 Restoring from a backup"
  check_contains "docs/UPGRADE.md" "kind: Backup"
  check_contains "docs/UPGRADE.md" "kind: Cluster"
  check_contains "docs/UPGRADE.md" "kind: ObjectStore"
  check_contains "docs/UPGRADE.md" "method: plugin"

  check_not_contains "component-constructor.yaml" "keycloak-client-operator|keycloakbackup-crd|restore-crd"
  check_not_contains "examples/backup-example.yaml" "keycloak\\.ocm\\.software|KeycloakBackup"
  check_not_contains "scripts/deploy/cleanup.sh" "keycloak\\.ocm\\.software|keycloakbackup"
  check_not_contains "operator/api/v1alpha1/groupversion_info.go" "Restore"
  check_not_contains "examples/backup-example.yaml" "method:\\s*barmanObjectStore"
  check_not_contains "examples/restore-cluster-example.yaml" "barmanObjectStore:"
  check_not_contains "docs/UPGRADE.md" "method:\\s*barmanObjectStore"

  log "Static checks passed"
}

run_live_restore_smoke() {
  prepare_restore_cluster_name

  if kubectl get "clusters.postgresql.cnpg.io/${TARGET_CLUSTER_NAME}" -n "$NAMESPACE" >/dev/null 2>&1; then
    fail "Restore target cluster already exists in namespace ${NAMESPACE}: ${TARGET_CLUSTER_NAME}. Use --restore-cluster-name to provide a unique name."
  fi

  local manifest
  manifest="$(mktemp)"

  cat >"$manifest" <<EOF
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: ${TARGET_CLUSTER_NAME}
  namespace: ${NAMESPACE}
spec:
  instances: 1
  bootstrap:
    recovery:
      source: keycloak-backup
  externalClusters:
  - name: keycloak-backup
    plugin:
      name: ${PLUGIN_NAME}
      parameters:
        barmanObjectName: ${OBJECT_STORE_NAME}
        serverName: ${CLUSTER_NAME}
  storage:
    size: 5Gi
EOF

  log "Applying live CNPG recovery cluster: ${TARGET_CLUSTER_NAME}"
  kubectl apply -f "$manifest"

  log "Waiting for recovery cluster readiness"
  wait_for_restore_cluster_ready

  log "Live restore smoke passed"
  kubectl get "clusters.postgresql.cnpg.io/${TARGET_CLUSTER_NAME}" -n "$NAMESPACE" -o yaml | sed -n '1,140p'

  log "Note: target recovery cluster '${TARGET_CLUSTER_NAME}' is intentionally not deleted by this test"
  rm -f "$manifest"
}

prepare_restore_cluster_name() {
  [[ -n "$TARGET_CLUSTER_NAME" ]] || TARGET_CLUSTER_NAME="${CLUSTER_NAME}-restore-smoke-$(date +%s)"

  # CNPG creates a bootstrap job named "<restore-cluster>-1-full-recovery".
  # Kubernetes object names are limited to 63 chars; fail early with a clear hint.
  local recovery_job_name
  recovery_job_name="${TARGET_CLUSTER_NAME}-1-full-recovery"
  if [ ${#recovery_job_name} -gt 63 ]; then
    fail "Restore cluster name '${TARGET_CLUSTER_NAME}' is too long for CNPG bootstrap job naming. Keep restore name <= 47 chars (current full-recovery job name length: ${#recovery_job_name})."
  fi
}

print_restore_diagnostics() {
  local recovery_job
  recovery_job="${TARGET_CLUSTER_NAME}-1-full-recovery"

  log "Restore diagnostics: cluster ${TARGET_CLUSTER_NAME}"
  kubectl get "clusters.postgresql.cnpg.io/${TARGET_CLUSTER_NAME}" -n "$NAMESPACE" -o yaml | sed -n '1,260p' || true

  log "Restore diagnostics: pods for cluster ${TARGET_CLUSTER_NAME}"
  kubectl get pods -n "$NAMESPACE" -l "cnpg.io/cluster=${TARGET_CLUSTER_NAME}" -o wide || true

  log "Restore diagnostics: jobs for cluster ${TARGET_CLUSTER_NAME}"
  kubectl get jobs -n "$NAMESPACE" -l "cnpg.io/cluster=${TARGET_CLUSTER_NAME}" -o wide || true

  log "Restore diagnostics: bootstrap job logs (${recovery_job})"
  kubectl logs -n "$NAMESPACE" "job/${recovery_job}" --all-containers=true --tail=200 || true

  log "Restore diagnostics: recent namespace events"
  kubectl get events -n "$NAMESPACE" --sort-by=.metadata.creationTimestamp | tail -n 80 || true
}

wait_for_restore_cluster_ready() {
  local started_at now elapsed phase
  started_at="$(date +%s)"

  while true; do
    if kubectl wait "clusters.postgresql.cnpg.io/${TARGET_CLUSTER_NAME}" -n "$NAMESPACE" \
      --for=condition=Ready \
      --timeout=10s >/dev/null 2>&1; then
      return 0
    fi

    phase="$(kubectl get "clusters.postgresql.cnpg.io/${TARGET_CLUSTER_NAME}" -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    now="$(date +%s)"
    elapsed="$((now - started_at))"

    if (( elapsed >= TIMEOUT_SECONDS )); then
      print_restore_diagnostics
      fail "Timed out after ${TIMEOUT_SECONDS}s waiting for recovery cluster ${TARGET_CLUSTER_NAME} readiness (last phase: ${phase:-unknown})"
    fi

    log "Waiting for recovery cluster readiness (phase: ${phase:-unknown}, ${elapsed}s elapsed)"
    sleep 5
  done
}

ensure_kube_context() {
  if ! kubectl config current-context >/dev/null 2>&1; then
    fail "kubectl current-context is not set. Configure a kube context or run with --kubeconfig <path>."
  fi
}

configure_cluster_backup_store() {
  local objectstore_manifest cluster_patch

  objectstore_manifest="$(mktemp)"
  cluster_patch="$(mktemp)"

  cat >"$objectstore_manifest" <<EOF
apiVersion: barmancloud.cnpg.io/v1
kind: ObjectStore
metadata:
  name: ${OBJECT_STORE_NAME}
  namespace: ${NAMESPACE}
spec:
  configuration:
    destinationPath: ${DESTINATION_PATH}
    s3Credentials:
      accessKeyId:
        name: ${CREDENTIALS_SECRET}
        key: ACCESS_KEY_ID
      secretAccessKey:
        name: ${CREDENTIALS_SECRET}
        key: SECRET_ACCESS_KEY
    wal:
      compression: gzip
    data:
      compression: gzip
EOF

  if [[ -n "$ENDPOINT_URL" ]]; then
    cat >>"$objectstore_manifest" <<EOF
    endpointURL: ${ENDPOINT_URL}
  instanceSidecarConfiguration:
    env:
    - name: AWS_REQUEST_CHECKSUM_CALCULATION
      value: when_required
    - name: AWS_RESPONSE_CHECKSUM_VALIDATION
      value: when_required
EOF
  fi

  log "Applying ObjectStore ${OBJECT_STORE_NAME}"
  kubectl apply -f "$objectstore_manifest" >/dev/null

  # Remove deprecated in-tree barmanObjectStore config first; CNPG rejects
  # plugin WAL archiving while legacy backup config is present.
  kubectl patch "clusters.postgresql.cnpg.io/${CLUSTER_NAME}" -n "$NAMESPACE" --type json \
    -p='[{"op":"remove","path":"/spec/backup/barmanObjectStore"}]' >/dev/null 2>&1 || true

  cat >"$cluster_patch" <<EOF
spec:
  plugins:
  - name: ${PLUGIN_NAME}
    isWALArchiver: true
    parameters:
      barmanObjectName: ${OBJECT_STORE_NAME}
EOF

  log "Patching CNPG cluster plugin config on ${CLUSTER_NAME}"
  kubectl patch "clusters.postgresql.cnpg.io/${CLUSTER_NAME}" -n "$NAMESPACE" --type merge --patch-file "$cluster_patch" >/dev/null

  rm -f "$objectstore_manifest" "$cluster_patch"
}

wait_for_cluster_ready_with_plugin() {
  local started_at now elapsed plugins
  started_at="$(date +%s)"

  while true; do
    if kubectl wait "clusters.postgresql.cnpg.io/${CLUSTER_NAME}" -n "$NAMESPACE" \
      --for=condition=Ready \
      --timeout=10s >/dev/null 2>&1; then
      plugins="$(kubectl get "clusters.postgresql.cnpg.io/${CLUSTER_NAME}" -n "$NAMESPACE" -o jsonpath='{range .status.pluginStatus[*]}{.name}{"\n"}{end}' 2>/dev/null || true)"
      if printf '%s\n' "$plugins" | grep -qx "$PLUGIN_NAME"; then
        log "Cluster is Ready and plugin is available: ${PLUGIN_NAME}"
        return 0
      fi
    fi

    now="$(date +%s)"
    elapsed="$((now - started_at))"
    if (( elapsed >= TIMEOUT_SECONDS )); then
      kubectl get "clusters.postgresql.cnpg.io/${CLUSTER_NAME}" -n "$NAMESPACE" -o yaml | sed -n '1,220p' || true
      fail "Timed out after ${TIMEOUT_SECONDS}s waiting for cluster readiness with plugin ${PLUGIN_NAME}"
    fi

    log "Waiting for cluster/plugin readiness (${elapsed}s elapsed)"
    sleep 5
  done
}

ensure_cluster_can_read_backup_secret() {
  local sa="system:serviceaccount:${NAMESPACE}:${CLUSTER_NAME}"
  local can_read

  can_read="$(kubectl auth can-i --as="$sa" -n "$NAMESPACE" get "secret/${CREDENTIALS_SECRET}" 2>/dev/null || true)"
  if [[ "$can_read" != "yes" ]]; then
    fail "ServiceAccount ${CLUSTER_NAME} cannot read secret ${CREDENTIALS_SECRET} in namespace ${NAMESPACE}. Grant secret read access before running live backup smoke."
  fi
}

ensure_plugin_installed() {
  kubectl get crd objectstores.barmancloud.cnpg.io >/dev/null 2>&1 || fail "Missing CRD objectstores.barmancloud.cnpg.io. Install the Barman Cloud Plugin in the CNPG operator namespace first."
}

wait_for_backup_terminal_phase() {
  local backup_name="$1"
  local started_at now elapsed phase err_msg

  started_at="$(date +%s)"

  while true; do
    phase="$(kubectl get "backups.postgresql.cnpg.io/${backup_name}" -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    err_msg="$(kubectl get "backups.postgresql.cnpg.io/${backup_name}" -n "$NAMESPACE" -o jsonpath='{.status.error}' 2>/dev/null || true)"

    case "$phase" in
      completed)
        BACKUP_LAST_ERROR=""
        BACKUP_LAST_PHASE="completed"
        log "Backup completed"
        return 0
        ;;
      failed)
        BACKUP_LAST_PHASE="failed"
        BACKUP_LAST_ERROR="${err_msg:-no error message in status}"
        return 2
        ;;
      "")
        phase="pending"
        ;;
    esac

    now="$(date +%s)"
    elapsed="$((now - started_at))"
    if (( elapsed >= TIMEOUT_SECONDS )); then
      BACKUP_LAST_PHASE="$phase"
      BACKUP_LAST_ERROR="Timed out after ${TIMEOUT_SECONDS}s waiting for backup completion (last phase: ${phase}; error: ${err_msg:-n/a})"
      return 1
    fi

    log "Backup phase: ${phase} (${elapsed}s elapsed)"
    sleep 5
  done
}

run_live_backup_smoke() {
  require_cmd kubectl

  [[ -n "$NAMESPACE" ]] || fail "--namespace is required in --live mode"
  [[ -n "$DESTINATION_PATH" ]] || fail "--destination-path is required in --live mode"
  [[ -n "$CREDENTIALS_SECRET" ]] || fail "--credentials-secret is required in --live mode"
  [[ -n "$OBJECT_STORE_NAME" ]] || OBJECT_STORE_NAME="${CLUSTER_NAME}-backup-store"

  ensure_kube_context
  ensure_plugin_installed

  kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || fail "Namespace not found: $NAMESPACE"
  kubectl get secret "$CREDENTIALS_SECRET" -n "$NAMESPACE" >/dev/null 2>&1 || fail "Credentials secret not found in namespace ${NAMESPACE}: ${CREDENTIALS_SECRET}"
  kubectl get "clusters.postgresql.cnpg.io/${CLUSTER_NAME}" -n "$NAMESPACE" >/dev/null 2>&1 || fail "CNPG cluster not found in namespace ${NAMESPACE}: ${CLUSTER_NAME}"

  ensure_cluster_can_read_backup_secret
  configure_cluster_backup_store
  wait_for_cluster_ready_with_plugin

  local max_attempts attempt backup_name manifest wait_rc
  max_attempts=2
  attempt=1

  while (( attempt <= max_attempts )); do
    backup_name="backup-smoke-$(date +%s)-a${attempt}"
    manifest="$(mktemp)"

    cat >"$manifest" <<EOF
apiVersion: postgresql.cnpg.io/v1
kind: Backup
metadata:
  name: ${backup_name}
  namespace: ${NAMESPACE}
spec:
  cluster:
    name: ${CLUSTER_NAME}
  method: plugin
  pluginConfiguration:
    name: ${PLUGIN_NAME}
EOF

    log "Applying live backup smoke CR: ${backup_name} (attempt ${attempt}/${max_attempts})"
    kubectl apply -f "$manifest"

    log "Waiting for backup to complete"
    wait_rc=0
    wait_for_backup_terminal_phase "$backup_name" || wait_rc=$?

    if [[ $wait_rc -eq 0 ]]; then
      log "Live backup smoke passed"
      kubectl get "backups.postgresql.cnpg.io/${backup_name}" -n "$NAMESPACE" -o yaml | sed -n '1,120p'

      if [[ "$KEEP_BACKUP_CR" == "true" ]]; then
        log "Keeping smoke Backup CR for post-run inspection: ${backup_name}"
      else
        log "Cleaning up smoke Backup CR"
        kubectl delete "backups.postgresql.cnpg.io/${backup_name}" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
      fi
      rm -f "$manifest"
      return 0
    fi

    log "Backup attempt failed (phase: ${BACKUP_LAST_PHASE:-unknown}, error: ${BACKUP_LAST_ERROR:-unknown})"
    kubectl get "backups.postgresql.cnpg.io/${backup_name}" -n "$NAMESPACE" -o yaml | sed -n '1,160p' || true

    if [[ "$KEEP_BACKUP_CR" != "true" ]]; then
      kubectl delete "backups.postgresql.cnpg.io/${backup_name}" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
    fi

    rm -f "$manifest"

    if [[ $attempt -lt $max_attempts && "$BACKUP_LAST_ERROR" == *"requested plugin is not available"* ]]; then
      log "Detected transient CNPG plugin race. Waiting for plugin settle and retrying backup."
      sleep 20
      wait_for_cluster_ready_with_plugin
      attempt=$((attempt + 1))
      continue
    fi

    fail "Backup failed early: ${BACKUP_LAST_ERROR:-unknown error}"
  done
}

run_static_checks

if [[ "$MODE" == "live" ]]; then
  if [[ -n "$KUBECONFIG_FILE" ]]; then
    export KUBECONFIG="$KUBECONFIG_FILE"
  fi
  prepare_restore_cluster_name
  run_live_backup_smoke
  run_live_restore_smoke
else
  log "Live mode skipped (run with --live to trigger CNPG backup + restore smoke tests)"
fi

log "Done"
