#!/bin/bash
# ==============================================================================
# test-airgapped-ocm-images.sh - Verify OCM image coverage for air-gapped deploys
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/../utils/common.sh"

PROJECT_ROOT="$(cd "$(dirname "$(dirname "$SCRIPT_DIR")")" && pwd)"
COMPONENT_CONSTRUCTOR="$PROJECT_ROOT/component-constructor.yaml"
TMP_DIR="$(mktemp -d)"

cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

COMPONENT_IMAGE_REFS="$TMP_DIR/component-image-refs.txt"
COMPONENT_IMAGE_NAMES="$TMP_DIR/component-image-names.txt"
RUNTIME_IMAGE_REFS="$TMP_DIR/runtime-image-refs.tsv"
MISSING_IMAGE_REFS="$TMP_DIR/missing-image-refs.tsv"
UNPINNED_COMPONENT_REFS="$TMP_DIR/unpinned-component-refs.txt"

strip_value() {
    local value="$1"
    value="${value%%#*}"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    value="${value%\"}"
    value="${value#\"}"
    echo "$value"
}

has_component_image_ref() {
    local ref="$1"
    grep -Fxq "$ref" "$COMPONENT_IMAGE_REFS"
}

has_component_image_name() {
    local name="$1"
    grep -Fxq "$name" "$COMPONENT_IMAGE_NAMES"
}

is_dynamic_ref() {
    local ref="$1"
    [[ "$ref" == *'${'* || "$ref" == *'{{'* ]]
}

is_operator_default_ref() {
    local ref="$1"
    [[ "$ref" == */keycloak-operator:* ]] && has_component_image_name "keycloak-operator-image"
}

extract_component_images() {
    awk '
        function strip(value) {
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
            gsub(/^"|"$/, "", value)
            return value
        }
        /^[[:space:]]*-[[:space:]]name:/ {
            name = $0
            sub(/^[[:space:]]*-[[:space:]]name:[[:space:]]*/, "", name)
            name = strip(name)
        }
        /^[[:space:]]*type:[[:space:]]*ociImage[[:space:]]*$/ {
            if (name != "") {
                print name
            }
        }
    ' "$COMPONENT_CONSTRUCTOR" | sort -u >"$COMPONENT_IMAGE_NAMES"

    awk '
        function strip(value) {
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
            gsub(/^"|"$/, "", value)
            return value
        }
        /^[[:space:]]*imageReference:[[:space:]]*/ {
            ref = $0
            sub(/^[[:space:]]*imageReference:[[:space:]]*/, "", ref)
            print strip(ref)
        }
    ' "$COMPONENT_CONSTRUCTOR" | sort -u >"$COMPONENT_IMAGE_REFS"
}

extract_manifest_images() {
    while IFS= read -r -d '' file; do
        awk -v file="${file#$PROJECT_ROOT/}" '
            /^[[:space:]]*(image|imageName):[[:space:]]*/ {
                ref = $0
                sub(/^[[:space:]]*(image|imageName):[[:space:]]*/, "", ref)
                sub(/[[:space:]]+#.*$/, "", ref)
                gsub(/^"|"$/, "", ref)
                if (ref != "") {
                    print ref "\t" file ":" FNR
                }
            }
            /^[[:space:]]*default:[[:space:]]*".*(quay\.io|ghcr\.io|docker\.io|registry\.k8s\.io|busybox|keycloak-operator).*"/ {
                ref = $0
                sub(/^[[:space:]]*default:[[:space:]]*/, "", ref)
                gsub(/^"|"$/, "", ref)
                if (ref != "") {
                    print ref "\t" file ":" FNR
                }
            }
        ' "$file" >>"$RUNTIME_IMAGE_REFS"
    done < <(find "$PROJECT_ROOT/manifests" "$PROJECT_ROOT/kro" -type f \( -name '*.yaml' -o -name '*.yml' \) -print0)
}

extract_chart_default_images() {
    awk -v file="charts/keycloak-operator/values.yaml" '
        function clean(value) {
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
            gsub(/^"|"$/, "", value)
            return value
        }
        /^image:[[:space:]]*$/ {
            section = "operator"
            next
        }
        /^[^[:space:]]/ && $0 !~ /^image:[[:space:]]*$/ {
            if (section == "operator") {
                section = ""
            }
        }
        section == "operator" && /^[[:space:]]+repository:/ {
            operator_repo = clean($2)
            next
        }
        section == "operator" && /^[[:space:]]+tag:/ {
            operator_tag = clean($2)
            next
        }
        /^[[:space:]]+configCliImage:[[:space:]]*$/ {
            section = "configCli"
            next
        }
        section == "configCli" && /^[[:space:]]+repository:/ {
            config_cli_repo = clean($2)
            next
        }
        section == "configCli" && /^[[:space:]]+tag:/ {
            config_cli_tag = clean($2)
            next
        }
        END {
            if (operator_repo != "" && operator_tag != "") {
                print operator_repo ":" operator_tag "\t" file ":image"
            }
            if (config_cli_repo != "" && config_cli_tag != "") {
                print config_cli_repo ":" config_cli_tag "\t" file ":operator.configCliImage"
            }
        }
    ' "$PROJECT_ROOT/charts/keycloak-operator/values.yaml" >>"$RUNTIME_IMAGE_REFS"
}

verify_component_image_pinning() {
    while IFS= read -r ref; do
        [[ -n "$ref" ]] || continue
        if is_dynamic_ref "$ref"; then
            continue
        fi
        if [[ "$ref" != *@sha256:* ]]; then
            echo "$ref" >>"$UNPINNED_COMPONENT_REFS"
        fi
    done <"$COMPONENT_IMAGE_REFS"

    if [[ -s "$UNPINNED_COMPONENT_REFS" ]]; then
        fail "OCM ociImage resources must use digest-pinned image references: $(tr '\n' ' ' <"$UNPINNED_COMPONENT_REFS")" 1
    fi
}

verify_runtime_image_coverage() {
    sort -u "$RUNTIME_IMAGE_REFS" -o "$RUNTIME_IMAGE_REFS"

    while IFS=$'\t' read -r ref source; do
        ref="$(strip_value "$ref")"
        [[ -n "$ref" ]] || continue
        if is_dynamic_ref "$ref"; then
            continue
        fi
        if is_operator_default_ref "$ref"; then
            continue
        fi
        if ! has_component_image_ref "$ref"; then
            printf '%s\t%s\n' "$ref" "$source" >>"$MISSING_IMAGE_REFS"
        fi
    done <"$RUNTIME_IMAGE_REFS"

    if [[ -s "$MISSING_IMAGE_REFS" ]]; then
        echo "[FAIL] Static runtime images missing from component-constructor.yaml OCM ociImage resources:" >&2
        while IFS=$'\t' read -r ref source; do
            echo "  - $ref ($source)" >&2
        done <"$MISSING_IMAGE_REFS"
        exit 1
    fi
}

extract_component_images
: >"$RUNTIME_IMAGE_REFS"
extract_manifest_images
extract_chart_default_images
verify_component_image_pinning
verify_runtime_image_coverage

info "Air-gapped OCM image coverage passed."
info "Static runtime image references: $(cut -f1 "$RUNTIME_IMAGE_REFS" | sort -u | wc -l | tr -d ' ')"
info "OCM ociImage resources: $(wc -l <"$COMPONENT_IMAGE_REFS" | tr -d ' ')"
