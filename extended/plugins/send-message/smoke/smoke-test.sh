#!/usr/bin/env bash

set -euo pipefail

log_info() {
    printf '[INFO] %s\n' "$*"
}

log_pass() {
    printf '[PASS] %s\n' "$*"
}

log_fail() {
    printf '[FAIL] %s\n' "$*" >&2
}

require_env() {
    local name="$1"
    if [[ -z "${!name:-}" ]]; then
        log_fail "$name is required"
        exit 1
    fi
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

KARGO_BIN="${KARGO_BIN:-kargo}"
KARGO_FLAGS="${KARGO_FLAGS:-}"
KUBECTL_BIN="${KUBECTL_BIN:-kubectl}"
DOCKER_BIN="${DOCKER_BIN:-docker}"
KIND_BIN="${KIND_BIN:-kind}"
JQ_BIN="${JQ_BIN:-jq}"

SYSTEM_RESOURCES_NS="${SEND_MESSAGE_SMOKE_SYSTEM_RESOURCES_NAMESPACE:-kargo-system-resources}"
SECRET_NAME="${SEND_MESSAGE_SMOKE_SECRET_NAME:-send-message-slack-token}"
CHANNEL_NAME="${SEND_MESSAGE_SMOKE_CHANNEL_NAME:-send-message-smoke}"
CONFIGMAP_NAME="${SEND_MESSAGE_SMOKE_CONFIGMAP_NAME:-send-message-step-plugin}"
CLUSTER_ROLE_NAME="send-message-step-plugin-reader"

require_env SEND_MESSAGE_SMOKE_PROJECT
require_env SEND_MESSAGE_SMOKE_WAREHOUSE
require_env SEND_MESSAGE_SMOKE_FREIGHT_NAME
require_env SEND_MESSAGE_SMOKE_SLACK_API_KEY
require_env SEND_MESSAGE_SMOKE_CHANNEL_ID

TEST_PROJECT="$SEND_MESSAGE_SMOKE_PROJECT"
TEST_WAREHOUSE="$SEND_MESSAGE_SMOKE_WAREHOUSE"
SMOKE_FREIGHT_NAME="$SEND_MESSAGE_SMOKE_FREIGHT_NAME"

PLUGIN_BUILD_DIR=""
STAGE_NAME=""
CLUSTER_ROLE_BINDING_NAME=""

cleanup() {
    if [[ -n "$STAGE_NAME" ]]; then
        "$KUBECTL_BIN" delete stage.kargo.akuity.io "$STAGE_NAME" \
            -n "$TEST_PROJECT" \
            --ignore-not-found >/dev/null 2>&1 || true
    fi

    "$KUBECTL_BIN" delete messagechannel.ee.kargo.akuity.io "$CHANNEL_NAME" \
        -n "$TEST_PROJECT" \
        --ignore-not-found >/dev/null 2>&1 || true
    "$KUBECTL_BIN" delete secret "$SECRET_NAME" \
        -n "$TEST_PROJECT" \
        --ignore-not-found >/dev/null 2>&1 || true
    "$KUBECTL_BIN" delete configmap "$CONFIGMAP_NAME" \
        -n "$SYSTEM_RESOURCES_NS" \
        --ignore-not-found >/dev/null 2>&1 || true

    if [[ -n "$CLUSTER_ROLE_BINDING_NAME" ]]; then
        "$KUBECTL_BIN" delete clusterrolebinding "$CLUSTER_ROLE_BINDING_NAME" \
            --ignore-not-found >/dev/null 2>&1 || true
    fi

    "$KUBECTL_BIN" delete clusterrole "$CLUSTER_ROLE_NAME" \
        --ignore-not-found >/dev/null 2>&1 || true
    "$KUBECTL_BIN" delete -f "$PLUGIN_DIR/manifests/crds.yaml" \
        --ignore-not-found >/dev/null 2>&1 || true

    if [[ -n "$PLUGIN_BUILD_DIR" ]]; then
        rm -rf "$PLUGIN_BUILD_DIR"
    fi
}

trap cleanup EXIT

current_context="$("$KUBECTL_BIN" config current-context)"
if [[ "$current_context" != kind-* ]]; then
    log_fail "send-message smoke requires a kind context, got: $current_context"
    exit 1
fi

kind_name="${current_context#kind-}"
image_tag="send-message-step-plugin:e2e-$(date +%s)"
PLUGIN_BUILD_DIR="$(mktemp -d "/tmp/send-message-stepplugin-e2e-XXXXXX")"

log_info "Build send-message StepPlugin image"
"$DOCKER_BIN" build -t "$image_tag" "$PLUGIN_DIR"
log_pass "Build send-message StepPlugin image"

log_info "Load send-message StepPlugin image into kind"
"$KIND_BIN" load docker-image --name "$kind_name" "$image_tag"
log_pass "Load send-message StepPlugin image into kind"

log_info "Render send-message StepPlugin build dir"
"$SCRIPT_DIR/render-plugin-dir.sh" "$PLUGIN_BUILD_DIR" "$image_tag" "$SYSTEM_RESOURCES_NS"
log_pass "Render send-message StepPlugin build dir"

log_info "Build send-message StepPlugin ConfigMap"
(
    cd "$PLUGIN_BUILD_DIR"
    "$KARGO_BIN" step-plugin build .
)
log_pass "Build send-message StepPlugin ConfigMap"

log_info "Install send-message CRDs"
"$KUBECTL_BIN" apply -f "$PLUGIN_DIR/manifests/crds.yaml"
log_pass "Install send-message CRDs"

log_info "Install send-message ClusterRole"
"$KUBECTL_BIN" apply -f "$PLUGIN_DIR/manifests/rbac.yaml"
log_pass "Install send-message ClusterRole"

CLUSTER_ROLE_BINDING_NAME="send-message-step-plugin-reader-${TEST_PROJECT}"
cat > "/tmp/${CLUSTER_ROLE_BINDING_NAME}.yaml" <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ${CLUSTER_ROLE_BINDING_NAME}
subjects:
- kind: ServiceAccount
  name: default
  namespace: ${TEST_PROJECT}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ${CLUSTER_ROLE_NAME}
EOF
log_info "Bind send-message ClusterRole to test project default ServiceAccount"
"$KUBECTL_BIN" apply -f "/tmp/${CLUSTER_ROLE_BINDING_NAME}.yaml"
log_pass "Bind send-message ClusterRole to test project default ServiceAccount"

log_info "Install send-message StepPlugin ConfigMap in system resources namespace"
"$KUBECTL_BIN" apply -f "$PLUGIN_BUILD_DIR/${CONFIGMAP_NAME}-configmap.yaml"
log_pass "Install send-message StepPlugin ConfigMap in system resources namespace"

cat > "/tmp/${SECRET_NAME}.yaml" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${SECRET_NAME}
  namespace: ${TEST_PROJECT}
type: Opaque
stringData:
  apiKey: ${SEND_MESSAGE_SMOKE_SLACK_API_KEY}
EOF
log_info "Create send-message Slack Secret"
"$KUBECTL_BIN" apply -f "/tmp/${SECRET_NAME}.yaml"
log_pass "Create send-message Slack Secret"

cat > "/tmp/${CHANNEL_NAME}.yaml" <<EOF
apiVersion: ee.kargo.akuity.io/v1alpha1
kind: MessageChannel
metadata:
  name: ${CHANNEL_NAME}
  namespace: ${TEST_PROJECT}
spec:
  secretRef:
    name: ${SECRET_NAME}
  slack:
    channelID: ${SEND_MESSAGE_SMOKE_CHANNEL_ID}
EOF
log_info "Create send-message MessageChannel"
"$KUBECTL_BIN" apply -f "/tmp/${CHANNEL_NAME}.yaml"
log_pass "Create send-message MessageChannel"

STAGE_NAME="smsg-stepplugin-$(date +%s)"
message_text="send-message stepplugin smoke ${STAGE_NAME}"
cat > "/tmp/${STAGE_NAME}.yaml" <<EOF
apiVersion: kargo.akuity.io/v1alpha1
kind: Stage
metadata:
  name: ${STAGE_NAME}
  namespace: ${TEST_PROJECT}
spec:
  requestedFreight:
  - origin:
      kind: Warehouse
      name: ${TEST_WAREHOUSE}
    sources:
      direct: true
  promotionTemplate:
    spec:
      steps:
      - uses: send-message
        config:
          channel:
            kind: MessageChannel
            name: ${CHANNEL_NAME}
          message: "${message_text}"
EOF

log_info "Create send-message StepPlugin smoke stage"
stage_output="$("$KARGO_BIN" apply -f "/tmp/${STAGE_NAME}.yaml" $KARGO_FLAGS)"
printf '%s\n' "$stage_output"
if ! grep -Fq "stage.kargo.akuity.io/${STAGE_NAME}" <<<"$stage_output"; then
    log_fail "send-message smoke stage apply output did not mention ${STAGE_NAME}"
    exit 1
fi
log_pass "Create send-message StepPlugin smoke stage"

log_info "Approve freight for send-message StepPlugin smoke stage"
"$KARGO_BIN" approve \
    --project="$TEST_PROJECT" \
    --freight="$SMOKE_FREIGHT_NAME" \
    --stage="$STAGE_NAME" \
    $KARGO_FLAGS
log_pass "Approve freight for send-message StepPlugin smoke stage"

log_info "Promote freight through send-message StepPlugin smoke stage"
"$KARGO_BIN" promote \
    --project="$TEST_PROJECT" \
    --freight="$SMOKE_FREIGHT_NAME" \
    --stage="$STAGE_NAME" \
    $KARGO_FLAGS
log_pass "Promote freight through send-message StepPlugin smoke stage"

promotion_name=""
phase=""
thread_ts=""
for _ in $(seq 1 90); do
    promotion_json="$(
        "$KUBECTL_BIN" get promotion.kargo.akuity.io \
            -n "$TEST_PROJECT" \
            -o json 2>/dev/null |
            "$JQ_BIN" -r --arg stage "$STAGE_NAME" '
                [.items[] | select(.spec.stage == $stage)]
                | sort_by(.metadata.creationTimestamp)
                | last // {}
            '
    )" || true
    if [[ -n "$promotion_json" && "$promotion_json" != "null" ]]; then
        promotion_name="$(printf '%s\n' "$promotion_json" | "$JQ_BIN" -r '.metadata.name // empty')"
        phase="$(printf '%s\n' "$promotion_json" | "$JQ_BIN" -r '.status.phase // empty')"
        thread_ts="$(
            printf '%s\n' "$promotion_json" |
                "$JQ_BIN" -r '
                    .status.stepExecutionMetadata[0].output.slack.threadTS //
                    .status.state["step-1"].slack.threadTS //
                    empty
                '
        )"
    fi
    case "$phase" in
        Succeeded)
            break
            ;;
        Failed|Errored|Aborted)
            log_fail "send-message smoke promotion reached terminal phase $phase"
            "$KUBECTL_BIN" get promotion.kargo.akuity.io "$promotion_name" \
                -n "$TEST_PROJECT" \
                -o yaml
            exit 1
            ;;
    esac
    sleep 2
done

if [[ "$phase" != "Succeeded" ]]; then
    log_fail "send-message smoke promotion did not succeed in time"
    "$KUBECTL_BIN" get promotion.kargo.akuity.io -n "$TEST_PROJECT" -o yaml
    exit 1
fi

if [[ -z "$thread_ts" ]]; then
    log_fail "send-message smoke did not produce slack.threadTS output"
    "$KUBECTL_BIN" get promotion.kargo.akuity.io "$promotion_name" \
        -n "$TEST_PROJECT" \
        -o yaml
    exit 1
fi

log_pass "send-message StepPlugin smoke promotion finished successfully"
