#!/usr/bin/env bash

STEPPLUGIN_TEST_STAGE=""
STEPPLUGIN_CONFIGMAP_NAME="mkdir-step-plugin"
STEPPLUGIN_PLUGIN_DIR=""

stepplugin_e2e_end() {
    local status="$1"

    log_info "STEPPLUGIN_E2E_END status=${status}"
}

stepplugin_e2e_fail() {
    stepplugin_e2e_end "failed"
    exit 1
}

cleanup_stepplugin_e2e() {
    if [[ -n "${STEPPLUGIN_TEST_STAGE:-}" && -n "${TEST_PROJECT:-}" ]]; then
        kubectl delete stage.kargo.akuity.io "$STEPPLUGIN_TEST_STAGE" \
            -n "$TEST_PROJECT" \
            --ignore-not-found >/dev/null 2>&1 || true
    fi

    if [[ -n "${STEPPLUGIN_CONFIGMAP_NAME:-}" && -n "${SYSTEM_RESOURCES_NS:-}" ]]; then
        kubectl delete configmap "$STEPPLUGIN_CONFIGMAP_NAME" \
            -n "$SYSTEM_RESOURCES_NS" \
            --ignore-not-found >/dev/null 2>&1 || true
    fi

    if [[ -n "${STEPPLUGIN_PLUGIN_DIR:-}" ]]; then
        rm -rf "$STEPPLUGIN_PLUGIN_DIR"
    fi
}

wait_for_stepplugin_promotion() {
    local promotion_name="$1"
    local phase=""
    local mkdir_status=""
    local copy_status=""
    local agent_pod_seen=""

    log_test "Wait for StepPlugin smoke promotion to finish"

    for _ in $(seq 1 90); do
        local promotion_json
        promotion_json=$(
            kubectl get promotion.kargo.akuity.io "$promotion_name" \
                -n "$TEST_PROJECT" \
                -o json 2>/dev/null
        ) || true
        if [[ -n "$promotion_json" ]]; then
            phase=$(echo "$promotion_json" | jq -r '.status.phase // empty')
            mkdir_status=$(
                echo "$promotion_json" |
                    jq -r '.status.stepExecutionMetadata[0].status // empty'
            )
            copy_status=$(
                echo "$promotion_json" |
                    jq -r '.status.stepExecutionMetadata[1].status // empty'
            )
        fi

        if [[ -z "$agent_pod_seen" ]]; then
            agent_pod_seen=$(
                kubectl get pods \
                    -n "$TEST_PROJECT" \
                    -l "kargo-extended.code.org/promotion=${promotion_name},kargo-extended.code.org/promotion-agent=true" \
                    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true
            )
        fi

        case "$phase" in
            Succeeded)
                break
                ;;
            Failed|Errored|Aborted)
                log_error "StepPlugin smoke promotion reached terminal phase $phase"
                kubectl get promotion.kargo.akuity.io "$promotion_name" \
                    -n "$TEST_PROJECT" \
                    -o yaml
                stepplugin_e2e_fail
                ;;
        esac

        sleep 2
    done

    if [[ "$phase" != "Succeeded" ]]; then
        log_error "StepPlugin smoke promotion did not succeed in time"
        kubectl get promotion.kargo.akuity.io "$promotion_name" \
            -n "$TEST_PROJECT" \
            -o yaml
        stepplugin_e2e_fail
    fi

    if [[ "$mkdir_status" != "Succeeded" ]]; then
        log_error "mkdir plugin step did not succeed"
        kubectl get promotion.kargo.akuity.io "$promotion_name" \
            -n "$TEST_PROJECT" \
            -o yaml
        stepplugin_e2e_fail
    fi

    if [[ "$copy_status" != "Succeeded" ]]; then
        log_error "builtin copy witness step did not succeed"
        kubectl get promotion.kargo.akuity.io "$promotion_name" \
            -n "$TEST_PROJECT" \
            -o yaml
        stepplugin_e2e_fail
    fi

    if [[ -n "$agent_pod_seen" ]]; then
        log_success "Observed promotion-agent pod for StepPlugin smoke test"
    else
        log_info "Did not observe a promotion-agent pod before completion"
        ((TESTS_PASSED++))
    fi

    log_success "StepPlugin smoke promotion finished successfully"
}

run_stepplugin_e2e_tests() {
    log_section "7A. STEPPLUGIN SMOKE TESTS"
    log_info "STEPPLUGIN_E2E_START"

    local smoke_freight_alias="stepplugin-smoke-$(date +%s)"
    local smoke_freight_name=""

    local controller_deployment
    controller_deployment=$(
        kubectl get deployment \
            -n "$KARGO_NS" \
            -l app.kubernetes.io/component=controller \
            -o jsonpath='{.items[?(@.metadata.name=="kargo-controller")].metadata.name}'
    )
    if [[ -z "$controller_deployment" ]]; then
        controller_deployment=$(
            kubectl get deployment \
                -n "$KARGO_NS" \
                -l app.kubernetes.io/component=controller \
                -o jsonpath='{.items[0].metadata.name}'
        )
    fi
    if [[ -z "$controller_deployment" ]]; then
        log_error "Could not find controller deployment in namespace $KARGO_NS"
        stepplugin_e2e_fail
    fi

    STEPPLUGIN_PLUGIN_DIR="/tmp/stepplugin-e2e-$(date +%s)"
    run_test "Create StepPlugin smoke temp dir" "mkdir -p $STEPPLUGIN_PLUGIN_DIR"

    cat > "${STEPPLUGIN_PLUGIN_DIR}/plugin.yaml" <<EOF
apiVersion: kargo-extended.code.org/v1alpha1
kind: StepPlugin
metadata:
  name: mkdir
  namespace: ${SYSTEM_RESOURCES_NS}
spec:
  sidecar:
    automountServiceAccountToken: false
    container:
      name: mkdir-step-plugin
      image: python:alpine3.23
      command:
      - python
      - -u
      - -c
      ports:
      - containerPort: 9765
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
      resources:
        requests:
          cpu: 50m
          memory: 32Mi
        limits:
          cpu: 100m
          memory: 64Mi
  steps:
  - kind: mkdir
EOF

    cat > "${STEPPLUGIN_PLUGIN_DIR}/server.py" <<'EOF'
import json, os
from http.server import BaseHTTPRequestHandler, HTTPServer

class MkdirHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        config = request["step"]["config"]
        os.makedirs(
            f'{request["context"]["workDir"]}/{config["path"]}',
            exist_ok=True,
        )
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{"status":"Succeeded"}')

HTTPServer(("", 9765), MkdirHandler).serve_forever()
EOF

    run_test \
        "Build documented mkdir StepPlugin example" \
        "cd $STEPPLUGIN_PLUGIN_DIR && $KARGO_BIN step-plugin build ."
    run_test \
        "Build emits StepPlugin README" \
        "test -f $STEPPLUGIN_PLUGIN_DIR/README.md"
    run_test \
        "Build emits StepPlugin ConfigMap YAML" \
        "test -f $STEPPLUGIN_PLUGIN_DIR/mkdir-step-plugin-configmap.yaml"
    run_test \
        "Generated ConfigMap has StepPlugin discovery label" \
        "grep -F 'kargo-extended.code.org/configmap-type: StepPlugin' $STEPPLUGIN_PLUGIN_DIR/mkdir-step-plugin-configmap.yaml"
    run_test \
        "Generated ConfigMap has sidecar automount key" \
        "grep -F 'sidecar.automountServiceAccountToken:' $STEPPLUGIN_PLUGIN_DIR/mkdir-step-plugin-configmap.yaml"
    run_test \
        "Generated ConfigMap has sidecar container key" \
        "grep -F 'sidecar.container:' $STEPPLUGIN_PLUGIN_DIR/mkdir-step-plugin-configmap.yaml"
    run_test \
        "Generated ConfigMap has steps key" \
        "grep -F 'steps.yaml:' $STEPPLUGIN_PLUGIN_DIR/mkdir-step-plugin-configmap.yaml"
    run_test \
        "Generated ConfigMap embeds server source" \
        "grep -F 'HTTPServer((\"\", 9765), MkdirHandler).serve_forever()' $STEPPLUGIN_PLUGIN_DIR/mkdir-step-plugin-configmap.yaml"

    run_test \
        "Install StepPlugin ConfigMap in system resources namespace" \
        "kubectl apply -f $STEPPLUGIN_PLUGIN_DIR/mkdir-step-plugin-configmap.yaml"
    kubectl_assert_exists \
        "StepPlugin ConfigMap installed" \
        "configmap" \
        "$STEPPLUGIN_CONFIGMAP_NAME" \
        "$SYSTEM_RESOURCES_NS"

    cat > "/tmp/${smoke_freight_alias}.yaml" <<EOF
apiVersion: kargo.akuity.io/v1alpha1
kind: Freight
metadata:
  name: placeholder
  namespace: ${TEST_PROJECT}
alias: ${smoke_freight_alias}
origin:
  kind: Warehouse
  name: ${TEST_WAREHOUSE}
images:
- repoURL: nginx
  tag: 1.27.0
EOF

    run_test \
        "Create StepPlugin smoke freight" \
        "kubectl apply -f /tmp/${smoke_freight_alias}.yaml"
    for _ in $(seq 1 20); do
        smoke_freight_name=$(
            kubectl get freight.kargo.akuity.io \
                -n "$TEST_PROJECT" \
                -l "kargo.akuity.io/alias=${smoke_freight_alias}" \
                -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true
        )
        if [[ -n "$smoke_freight_name" ]]; then
            break
        fi
        sleep 1
    done
    if [[ -z "$smoke_freight_name" ]]; then
        log_error "StepPlugin smoke freight was not created"
        kubectl get freight.kargo.akuity.io -n "$TEST_PROJECT" -o yaml
        stepplugin_e2e_fail
    fi

    STEPPLUGIN_TEST_STAGE="test-stepplugin-$(date +%s)"
    cat > "/tmp/${STEPPLUGIN_TEST_STAGE}.yaml" <<EOF
apiVersion: kargo.akuity.io/v1alpha1
kind: Stage
metadata:
  name: ${STEPPLUGIN_TEST_STAGE}
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
      - uses: mkdir
        config:
          path: demo/subdir
      - uses: copy
        config:
          inPath: demo/subdir
          outPath: copied/subdir
EOF

    run_test_assert_contains \
        "Create StepPlugin smoke stage" \
        "stage.kargo.akuity.io/${STEPPLUGIN_TEST_STAGE}" \
        "$KARGO_BIN apply -f /tmp/${STEPPLUGIN_TEST_STAGE}.yaml $KARGO_FLAGS"
    run_test \
        "Approve freight for StepPlugin smoke stage" \
        "$KARGO_BIN approve --project=$TEST_PROJECT --freight=$smoke_freight_name --stage=$STEPPLUGIN_TEST_STAGE $KARGO_FLAGS"

    local approved_for=""
    for _ in $(seq 1 20); do
        approved_for=$(
            kubectl get freight.kargo.akuity.io "$smoke_freight_name" \
                -n "$TEST_PROJECT" \
                -o jsonpath='{.status.approvedFor}' 2>/dev/null || true
        )
        if echo "$approved_for" | grep -q "$STEPPLUGIN_TEST_STAGE"; then
            break
        fi
        sleep 1
    done
    if echo "$approved_for" | grep -q "$STEPPLUGIN_TEST_STAGE"; then
        log_success "Freight approved for StepPlugin smoke stage"
    else
        log_error "Freight approval for StepPlugin smoke stage was not observed"
        kubectl get freight.kargo.akuity.io "$smoke_freight_name" \
            -n "$TEST_PROJECT" \
            -o yaml
        stepplugin_e2e_fail
    fi

    run_test \
        "Promote freight through StepPlugin smoke stage" \
        "$KARGO_BIN promote --project=$TEST_PROJECT --freight=$smoke_freight_name --stage=$STEPPLUGIN_TEST_STAGE $KARGO_FLAGS"

    local promotion_name
    for _ in $(seq 1 20); do
        promotion_name=$(
            kubectl get promotion.kargo.akuity.io \
                -n "$TEST_PROJECT" \
                -o json 2>/dev/null |
                jq -r --arg stage "$STEPPLUGIN_TEST_STAGE" '
                    [.items[] | select(.spec.stage == $stage)]
                    | sort_by(.metadata.creationTimestamp)
                    | last
                    | .metadata.name // empty
                '
        )
        if [[ -n "$promotion_name" ]]; then
            break
        fi
        sleep 1
    done
    if [[ -z "$promotion_name" ]]; then
        log_error "Could not parse StepPlugin smoke promotion name"
        kubectl get promotion.kargo.akuity.io -n "$TEST_PROJECT" -o yaml
        stepplugin_e2e_fail
    fi

    wait_for_stepplugin_promotion "$promotion_name"
    stepplugin_e2e_end "success"
}
