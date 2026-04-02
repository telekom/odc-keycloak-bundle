#!/usr/bin/env bash
set -euo pipefail

PROVIDER="external-s3"
CONTEXT="ci"
NAMESPACE=""
RUN_ID=""
OUTPUT_ENV_FILE=""
CREDENTIALS_SECRET="keycloak-backup-s3"
CLUSTER_NAME="keycloak-db"
ALLOW_NON_CI_INCLUSTER_MINIO="false"

DESTINATION_PATH=""
ENDPOINT_URL=""
ACCESS_KEY_ID=""
SECRET_ACCESS_KEY=""

MINIO_NAMESPACE="backup-ci"
MINIO_RELEASE="keycloak-backup-minio"
MINIO_IMAGE="minio/minio:latest"
MC_IMAGE="minio/mc:latest"
MINIO_ACCESS_KEY_ID=""
MINIO_SECRET_ACCESS_KEY=""

usage() {
	cat <<'EOF'
Usage:
	scripts/tests/setup-backup-provider.sh \
		--provider <external-s3|incluster-minio> \
		--context <ci|int> \
		--namespace <k8s-namespace> \
		--output-env-file <path> \
		[--run-id <id>] \
		[--credentials-secret <name>] \
		[--cluster-name <name>] \
		[--allow-non-ci-incluster-minio] \
		[--destination-path <s3://bucket/prefix>] \
		[--endpoint-url <url>] \
		[--access-key-id <id>] \
		[--secret-access-key <secret>] \
		[--minio-namespace <name>] \
		[--minio-release <name>] \
		[--minio-image <image>] \
		[--mc-image <image>] \
		[--minio-access-key-id <id>] \
		[--minio-secret-access-key <secret>]

Behavior:
	external-s3:
		- creates/upserts <credentials-secret> in target namespace
		- writes BACKUP_DESTINATION_PATH/BACKUP_ENDPOINT_URL to output env file

	incluster-minio:
		- ensures MinIO service is running in --minio-namespace
		- creates a context-specific bucket keycloak-backup-<context>
		- creates/upserts <credentials-secret> in target namespace
		- writes BACKUP_DESTINATION_PATH/BACKUP_ENDPOINT_URL to output env file
		- intended for CI smoke tests; non-CI usage requires --allow-non-ci-incluster-minio
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

write_env() {
	local key="$1"
	local value="$2"
	printf '%s=%q\n' "$key" "$value" >>"$OUTPUT_ENV_FILE"
}

decode_b64() {
	printf '%s' "$1" | base64 -d 2>/dev/null || true
}

read_secret_data_key() {
	local namespace="$1"
	local secret_name="$2"
	local key="$3"
	local encoded

	encoded="$(kubectl get secret "$secret_name" -n "$namespace" -o "jsonpath={.data.${key}}" 2>/dev/null || true)"
	[ -n "$encoded" ] || return 1
	decode_b64 "$encoded"
}

generate_token() {
	local length="$1"
	if command -v openssl >/dev/null 2>&1; then
		openssl rand -hex 32 | cut -c1-"$length"
	else
		date +%s%N | sha256sum | cut -c1-"$length"
	fi
}

create_target_secret() {
	kubectl create secret generic "$CREDENTIALS_SECRET" -n "$NAMESPACE" \
		--from-literal=ACCESS_KEY_ID="$ACCESS_KEY_ID" \
		--from-literal=SECRET_ACCESS_KEY="$SECRET_ACCESS_KEY" \
		--dry-run=client -o yaml | kubectl apply -f - >/dev/null

	ensure_cluster_secret_read_access
}

ensure_cluster_secret_read_access() {
	local service_account role_name can_read
	service_account="system:serviceaccount:${NAMESPACE}:${CLUSTER_NAME}"
	role_name="${CLUSTER_NAME}-backup-secret-reader"

	can_read="$(kubectl auth can-i --as="$service_account" -n "$NAMESPACE" get "secret/${CREDENTIALS_SECRET}" 2>/dev/null || true)"
	if [ "$can_read" = "yes" ]; then
		return 0
	fi

	kubectl create role "$role_name" -n "$NAMESPACE" \
		--verb=get \
		--resource=secrets \
		--resource-name="$CREDENTIALS_SECRET" \
		--dry-run=client -o yaml | kubectl apply -f - >/dev/null

	kubectl create rolebinding "$role_name" -n "$NAMESPACE" \
		--role="$role_name" \
		--serviceaccount="${NAMESPACE}:${CLUSTER_NAME}" \
		--dry-run=client -o yaml | kubectl apply -f - >/dev/null

	can_read="$(kubectl auth can-i --as="$service_account" -n "$NAMESPACE" get "secret/${CREDENTIALS_SECRET}" 2>/dev/null || true)"
	[ "$can_read" = "yes" ] || fail "Failed to grant secret read access for ServiceAccount ${CLUSTER_NAME} on secret ${CREDENTIALS_SECRET}"
}

setup_external_s3() {
	[ -n "$ACCESS_KEY_ID" ] || fail "--access-key-id is required for --provider external-s3"
	[ -n "$SECRET_ACCESS_KEY" ] || fail "--secret-access-key is required for --provider external-s3"
	[ -n "$DESTINATION_PATH" ] || fail "--destination-path is required for --provider external-s3"

	create_target_secret

	log "Prepared external S3 credentials secret $CREDENTIALS_SECRET in namespace $NAMESPACE"
	write_env "BACKUP_DESTINATION_PATH" "$DESTINATION_PATH"
	write_env "BACKUP_ENDPOINT_URL" "$ENDPOINT_URL"
}

setup_incluster_minio() {
	local existing_minio_user existing_minio_pass secret_changed
	existing_minio_user="$(read_secret_data_key "$MINIO_NAMESPACE" "${MINIO_RELEASE}-root" "MINIO_ROOT_USER" || true)"
	existing_minio_pass="$(read_secret_data_key "$MINIO_NAMESPACE" "${MINIO_RELEASE}-root" "MINIO_ROOT_PASSWORD" || true)"

	if [ -z "$MINIO_ACCESS_KEY_ID" ]; then
		if [ -n "$existing_minio_user" ]; then
			MINIO_ACCESS_KEY_ID="$existing_minio_user"
		else
			MINIO_ACCESS_KEY_ID="ci$(generate_token 14)"
		fi
	fi
	if [ -z "$MINIO_SECRET_ACCESS_KEY" ]; then
		if [ -n "$existing_minio_pass" ]; then
			MINIO_SECRET_ACCESS_KEY="$existing_minio_pass"
		else
			MINIO_SECRET_ACCESS_KEY="ci$(generate_token 30)"
		fi
	fi

	secret_changed="false"
	if [ "$MINIO_ACCESS_KEY_ID" != "$existing_minio_user" ] || [ "$MINIO_SECRET_ACCESS_KEY" != "$existing_minio_pass" ]; then
		secret_changed="true"
	fi

	ACCESS_KEY_ID="$MINIO_ACCESS_KEY_ID"
	SECRET_ACCESS_KEY="$MINIO_SECRET_ACCESS_KEY"

	local bucket prefix endpoint setup_pod phase
	bucket="keycloak-backup-${CONTEXT}"
	prefix="${NAMESPACE}/${RUN_ID:-manual}-$(date +%Y%m%d%H%M%S)"
	endpoint="http://${MINIO_RELEASE}.${MINIO_NAMESPACE}.svc.cluster.local:9000"

	kubectl get namespace "$MINIO_NAMESPACE" >/dev/null 2>&1 || kubectl create namespace "$MINIO_NAMESPACE" >/dev/null

	kubectl create secret generic "${MINIO_RELEASE}-root" -n "$MINIO_NAMESPACE" \
		--from-literal=MINIO_ROOT_USER="$MINIO_ACCESS_KEY_ID" \
		--from-literal=MINIO_ROOT_PASSWORD="$MINIO_SECRET_ACCESS_KEY" \
		--dry-run=client -o yaml | kubectl apply -f - >/dev/null

	kubectl apply -n "$MINIO_NAMESPACE" -f - <<EOF >/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${MINIO_RELEASE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${MINIO_RELEASE}
  template:
    metadata:
      labels:
        app: ${MINIO_RELEASE}
    spec:
      containers:
      - name: minio
        image: ${MINIO_IMAGE}
        args: ["server", "/data", "--console-address", ":9001"]
        env:
        - name: MINIO_ROOT_USER
          valueFrom:
            secretKeyRef:
              name: ${MINIO_RELEASE}-root
              key: MINIO_ROOT_USER
        - name: MINIO_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              name: ${MINIO_RELEASE}-root
              key: MINIO_ROOT_PASSWORD
        ports:
        - containerPort: 9000
          name: s3
        - containerPort: 9001
          name: console
        volumeMounts:
        - mountPath: /data
          name: data
      volumes:
      - name: data
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: ${MINIO_RELEASE}
spec:
  selector:
    app: ${MINIO_RELEASE}
  ports:
  - name: s3
    port: 9000
    targetPort: 9000
  - name: console
    port: 9001
    targetPort: 9001
EOF

	if [ "$secret_changed" = "true" ]; then
		# Env vars are sourced from the root credentials secret and need a restart on change.
		kubectl rollout restart deployment/"$MINIO_RELEASE" -n "$MINIO_NAMESPACE" >/dev/null
	fi
	kubectl rollout status deployment/"$MINIO_RELEASE" -n "$MINIO_NAMESPACE" --timeout=180s >/dev/null

	setup_pod="${MINIO_RELEASE}-mc-setup"
	kubectl delete pod "$setup_pod" -n "$MINIO_NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
	kubectl run "$setup_pod" -n "$MINIO_NAMESPACE" --restart=Never --image="$MC_IMAGE" --command -- \
		sh -ceu "mc alias set local '$endpoint' '$MINIO_ACCESS_KEY_ID' '$MINIO_SECRET_ACCESS_KEY'; mc mb --ignore-existing local/$bucket" >/dev/null

	for _ in $(seq 1 60); do
		phase="$(kubectl get pod "$setup_pod" -n "$MINIO_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
		case "$phase" in
			Succeeded)
				break
				;;
			Failed)
				kubectl logs "$setup_pod" -n "$MINIO_NAMESPACE" || true
				fail "Failed to initialize in-cluster MinIO bucket"
				;;
		esac
		sleep 2
	done

	phase="$(kubectl get pod "$setup_pod" -n "$MINIO_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
	if [ "$phase" != "Succeeded" ]; then
		kubectl logs "$setup_pod" -n "$MINIO_NAMESPACE" || true
		fail "Timed out waiting for in-cluster MinIO bucket initialization"
	fi
	kubectl delete pod "$setup_pod" -n "$MINIO_NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true

	DESTINATION_PATH="s3://${bucket}/${prefix}"
	ENDPOINT_URL="$endpoint"

	create_target_secret

	log "Prepared in-cluster MinIO endpoint ${endpoint} with destination ${DESTINATION_PATH}"
	write_env "BACKUP_DESTINATION_PATH" "$DESTINATION_PATH"
	write_env "BACKUP_ENDPOINT_URL" "$ENDPOINT_URL"
}

while [ $# -gt 0 ]; do
	case "$1" in
		--provider)
			PROVIDER="${2:-}"
			shift 2
			;;
		--context)
			CONTEXT="${2:-}"
			shift 2
			;;
		--namespace)
			NAMESPACE="${2:-}"
			shift 2
			;;
		--run-id)
			RUN_ID="${2:-}"
			shift 2
			;;
		--output-env-file)
			OUTPUT_ENV_FILE="${2:-}"
			shift 2
			;;
		--credentials-secret)
			CREDENTIALS_SECRET="${2:-}"
			shift 2
			;;
		--cluster-name)
			CLUSTER_NAME="${2:-}"
			shift 2
			;;
		--allow-non-ci-incluster-minio)
			ALLOW_NON_CI_INCLUSTER_MINIO="true"
			shift
			;;
		--destination-path)
			DESTINATION_PATH="${2:-}"
			shift 2
			;;
		--endpoint-url)
			ENDPOINT_URL="${2:-}"
			shift 2
			;;
		--access-key-id)
			ACCESS_KEY_ID="${2:-}"
			shift 2
			;;
		--secret-access-key)
			SECRET_ACCESS_KEY="${2:-}"
			shift 2
			;;
		--minio-namespace)
			MINIO_NAMESPACE="${2:-}"
			shift 2
			;;
		--minio-release)
			MINIO_RELEASE="${2:-}"
			shift 2
			;;
		--minio-image)
			MINIO_IMAGE="${2:-}"
			shift 2
			;;
		--mc-image)
			MC_IMAGE="${2:-}"
			shift 2
			;;
		--minio-access-key-id)
			MINIO_ACCESS_KEY_ID="${2:-}"
			shift 2
			;;
		--minio-secret-access-key)
			MINIO_SECRET_ACCESS_KEY="${2:-}"
			shift 2
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

require_cmd kubectl

[ -n "$NAMESPACE" ] || fail "--namespace is required"
[ -n "$OUTPUT_ENV_FILE" ] || fail "--output-env-file is required"
mkdir -p "$(dirname "$OUTPUT_ENV_FILE")"
: >"$OUTPUT_ENV_FILE"

case "$PROVIDER" in
	external-s3)
		setup_external_s3
		;;
	incluster-minio)
		if [ "$CONTEXT" != "ci" ] && [ "$ALLOW_NON_CI_INCLUSTER_MINIO" != "true" ]; then
			fail "--provider incluster-minio is CI-only by default. Use --context ci or explicitly pass --allow-non-ci-incluster-minio."
		fi
		setup_incluster_minio
		;;
	*)
		fail "Unsupported --provider: $PROVIDER"
		;;
esac

write_env "BACKUP_PROVIDER" "$PROVIDER"

log "Backup provider setup complete (${PROVIDER})"
