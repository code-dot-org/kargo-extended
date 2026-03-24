# Kargo Step Plugins Spec

## Overview

- `kargo-extended` adds runtime-loaded StepPlugins modeled on Argo Workflows
  executor plugins.
- A plugin is an out-of-process step executor. It is not a Go package imported
  into the controller binary.
- OpenTofu and `send-message` drive the design.
- The first host proof is a minimal `mkdir` plugin.
- Read
  [Required Host Hooks](./plugin-technical-hook-needs-list.md#required-host-hooks)
  and
  [Main Technical Constraint](./plugin-technical-hook-needs-list.md#main-technical-constraint)
  before touching host code.

## Rules

- Use the Argo Workflows executor plugin model, not Argo CD CMP.
- Runtime discovery, not build-time imports.
- `StepPlugin` is build input, not a served CRD, in v1.
- No Go `plugin`.
- No `LD_PRELOAD`.
- New invented plugin surface uses `kargo-extended.code.org`.
- Existing Akuity APIs keep their existing groups.
- Internal Kargo promotion types stay internal.
- Plugin RPC uses dedicated Kargo wire types with normal JSON.
- Host owns orchestration, retries, shared state, expression evaluation, and
  `Promotion` status.
- The agent owns step execution and the shared workdir for plugin-backed
  Promotions. It does not own orchestration or `Promotion` status.
- Plugins own step logic.
- Plugin CRDs, plugin RBAC, plugin images, and plugin Secrets stay with the
  plugin.
- If a `Promotion` contains any plugin step, run the whole `Promotion` in the
  agent pod.
- Step kinds are global. v1 does not let plugins shadow builtin step kinds or
  other effective plugin step kinds.
- UI/schema discovery is optional later. It is not a v1 requirement.
- Keep real implementation logic under `extended/` whenever possible.
- Files outside `extended/` should stay thin bridge edits with seam-protection
  tests under `extended/`.

## Reuse From Argo Workflows

- Reuse Argo Workflows executor plugin code where it keeps the design simple.
- Prefer, in order:
  - verbatim copied files
  - minimally patched copies
  - fresh Kargo code only where the Argo shape does not fit
- If a copied file needs heavy surgery, own the forked version.
- Keep copied code under `extended/pkg/argoworkflows/`.
- Preserve upstream file headers and record the upstream source path and commit.

## Authoring And Build

- `plugin.yaml` is a typed authoring manifest.
- It keeps Kubernetes object shape:
  - `apiVersion`
  - `kind`
  - `metadata`
  - `spec`
- It is not a real cluster object in v1.
- Build command:
  - `kargo step-plugin build DIR`
- Build inputs:
  - `DIR/plugin.yaml`
  - optional single `DIR/server.*`
- Build outputs:
  - `<name>-step-plugin-configmap.yaml`
  - generated `README.md`
- The generated `ConfigMap` keeps `metadata.name` and `metadata.namespace`
  from `plugin.yaml`.
- Keep Argo's `server.*` embedding behavior in v1.
- Keep Argo's sidecar validation shape in v1:
  - at least one `containerPort`
  - `resources.requests`
  - `resources.limits`
  - `securityContext`
- The plugin RPC port is the first declared
  `spec.sidecar.container.ports[*].containerPort`.

Example `plugin.yaml`:

```yaml
apiVersion: kargo-extended.code.org/v1alpha1
kind: StepPlugin
metadata:
  name: opentofu
spec:
  sidecar:
    automountServiceAccountToken: false
    container:
      name: opentofu-plugin
      image: ghcr.io/yourorg/kargo-plugin-opentofu:latest
      ports:
      - containerPort: 9765
      securityContext:
        runAsNonRoot: true
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          cpu: "1"
          memory: 1Gi
  steps:
  - kind: tf-plan
    defaultTimeout: 10m
    defaultErrorThreshold: 1
  - kind: tf-output
    defaultTimeout: 10m
    defaultErrorThreshold: 1
  - kind: tf-apply
    defaultTimeout: 0s
    defaultErrorThreshold: 1
```

## Product Docs

- Product docs are part of v1. Proposal docs are not enough.
- Aim for roughly Argo Workflows executor plugin doc depth.
- Copy and rewrite where that keeps work small.
- Use these Argo docs as source material:
  - `../../../../../argo-workflows/docs/executor_plugins.md`
  - `../../../../../argo-workflows/docs/cli/argo_executor-plugin.md`
  - `../../../../../argo-workflows/docs/cli/argo_executor-plugin_build.md`
  - `../../../../../argo-workflows/docs/executor_swagger.md`
- Keep fork-owned docs under:
  - `extended/docs-site/05-kargo-external/`
- Expose them in generated docs with one symlink:
  - `docs/docs/05-kargo-external ->
    ../../extended/docs-site/05-kargo-external`
- Keep `Kargo External` before `Home`.
- Minimum pages:
  - `index.md`
  - `10-step-plugins.md`
  - `20-step-plugin-build.md`
  - `30-step-plugin-rpc.md`
- Minimum content:
  - what a StepPlugin is
  - `plugin.yaml`
  - `server.*` embedding
  - enablement through `controller.env`
  - `kargo step-plugin build`
  - install into `kargo-system-resources` or a Project namespace
  - Project namespace precedence
  - exact RPC auth contract
  - `403`, `404`, and `503`
  - the minimal `mkdir` example
  - `Stage` usage

## Enablement

- StepPlugins are enabled by default.
- v1 public enablement is controller env var:

```yaml
controller:
  env:
  - name: STEP_PLUGINS_ENABLED
    value: "false"
```

- Minimal implementation is fine:
  - the controller may just read `STEP_PLUGINS_ENABLED`
  - the chart does not need a dedicated StepPlugin value
  - no runtime toggle is required
- When StepPlugins are disabled:
  - the controller does not watch StepPlugin `ConfigMap`s
  - builtin-only Promotions keep current behavior
  - Promotions that reference plugin step kinds fail as unknown step kinds

## Discovery

- Plugin install ships a labeled `ConfigMap`.
- Use this label for discovery:
  - `kargo-extended.code.org/configmap-type: StepPlugin`
- Discovery namespaces:
  - the Project namespace
  - `SYSTEM_RESOURCES_NAMESPACE` (default `kargo-system-resources`)
- A generated plugin `ConfigMap` can go in either:
  - `kargo-system-resources` (`$SYSTEM_RESOURCES_NAMESPACE`)
  - the Project namespace
- If the same plugin name exists in both, the Project namespace wins.
- The controller watches those `ConfigMap`s and keeps an in-memory plugin
  registry.
- The discovery `ConfigMap` stores sidecar fields in Argo-shaped keys:
  - `sidecar.automountServiceAccountToken`
  - `sidecar.container`
- Kargo step registration metadata lives in `ConfigMap.data["steps.yaml"]`.

Generated discovery `ConfigMap` shape:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: opentofu-step-plugin
  labels:
    kargo-extended.code.org/configmap-type: StepPlugin
data:
  sidecar.automountServiceAccountToken: "false"
  sidecar.container: |
    name: opentofu-plugin
    image: ghcr.io/yourorg/kargo-plugin-opentofu:latest
    ports:
    - containerPort: 9765
    securityContext:
      runAsNonRoot: true
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: "1"
        memory: 1Gi
  steps.yaml: |
    - kind: tf-plan
      defaultTimeout: 10m
      defaultErrorThreshold: 1
    - kind: tf-output
      defaultTimeout: 10m
      defaultErrorThreshold: 1
    - kind: tf-apply
      defaultTimeout: 0s
      defaultErrorThreshold: 1
```

Rules for `steps`:

- `steps[*].kind` matches `uses:` in a `Promotion` or `PromotionTask` step.
- `defaultTimeout` and `defaultErrorThreshold` feed the same metadata paths the
  builtin registry uses today.
- The resolved step registry is keyed by step kind.
- A plugin-backed registry entry contains:
  - plugin name
  - plugin namespace
  - step metadata
- After plugin-name precedence is applied, duplicate plugin step kinds are
  rejected.
- Plugin step kinds that collide with builtin step kinds are rejected.
- v1 does not require schemas here.

## Execution Model

- Builtin-only Promotions may keep the current local engine.
- Any Promotion with a plugin step uses the agent path.
- The controller still:
  - builds `promotion.StepContext`
  - evaluates `if`
  - updates shared state
  - applies retry/backoff rules
  - writes `Promotion` status
- The agent pod owns the workdir for plugin-backed Promotions.
- The agent pod contains:
  - one main `kargo-promotion-agent` container
  - one sidecar for each plugin actually used by that `Promotion`
  - one shared `emptyDir` mounted at a fixed workdir path
- The main agent container executes builtin steps locally and plugin steps over
  localhost HTTP to sidecars.
- The agent is an execution host. It is not a second orchestrator.

Agent pod rules:

- One agent pod per `Promotion` UID.
- Reuse it across reconciles until the `Promotion` is terminal.
- Workdir lifetime is pod lifetime.
- For agent-backed Promotions, the controller no longer owns the workdir on its
  local filesystem.

## RPC

- Controller -> agent main: `POST /api/v1/step.execute`
- Agent main -> plugin sidecar: `POST /api/v1/step.execute`
- Request type: `StepExecuteRequest`
- Response type: `StepExecuteResponse`
- Do not marshal `promotion.StepExecutionRequest` directly on the wire.
- Do not marshal `promotion.StepResult` directly on the wire.
- Adapt between internal promotion types and plugin RPC wire types at the
  transport boundary.
- Use normal JSON on the wire.
- `step.config` is normal JSON in the request body, not base64-encoded bytes.

HTTP status handling:

- `200`: use the response body
- `404`: method unsupported; cache that and stop calling it
- `503`: transient; retry
- anything else: step error

Plugin rules:

- Any language is fine if it can speak HTTP JSON.
- A plugin only needs to implement the methods it uses.
- v1 only needs `step.execute`.

## RPC Auth

- Copy Argo's per-plugin bearer-token pattern, but use Kargo paths.
- The agent init container creates one random token per plugin sidecar
  container name.
- The agent init container and agent main container mount a shared volume at:
  - `/var/run/kargo`
- The agent main container reads each plugin token from:
  - `/var/run/kargo/<sidecar-container-name>/token`
- Each plugin sidecar mounts only its own subpath at:
  - `/var/run/kargo`
- Inside the plugin sidecar, the token file path is:
  - `/var/run/kargo/token`
- Every agent -> plugin request must send:
  - `Authorization: Bearer <token>`
- The plugin sidecar compares that header to the contents of
  `/var/run/kargo/token`.
- Missing or wrong bearer token returns:
  - `403`
- `404` still means unsupported method.
- `503` still means transient error.

## Kubernetes Access And RBAC

- This section is about Kubernetes API access, not plugin RPC auth.
- `spec.sidecar.automountServiceAccountToken: true` means the host mounts a
  Kubernetes service account token only for that sidecar.
- The plugin install ships the `ServiceAccount`, `Role`/`ClusterRole`, and
  bindings it needs.
- Do not grant plugin CRD or plugin Secret access to the host unless there is
  no other clean way.
- This matters most for `send-message`. The Slack plugin should read its own
  CRDs and referenced `Secret`s itself.

## Later Target Constraints

OpenTofu:

- scope:
  - `tf-plan`
  - `tf-output`
  - `tf-apply`
- this plugin requires the agent path because all three steps use the workdir
- the sidecar image carries `tofu`
- no new CRDs
- no Kubernetes access required in v1 unless the plugin chooses to read cluster
  state directly
- step config and outputs should match the public step docs closely enough to
  be drop-in for users:
  - [`tf-plan`](../../../../docs/docs/50-user-guide/60-reference-docs/30-promotion-steps/tf-plan.md)
  - [`tf-output`](../../../../docs/docs/50-user-guide/60-reference-docs/30-promotion-steps/tf-output.md)
  - [`tf-apply`](../../../../docs/docs/50-user-guide/60-reference-docs/30-promotion-steps/tf-apply.md)
- required behavior:
  - `tf-plan` accepts `dir`, `out`, `vars`, `env`
  - `tf-plan` returns output key `plan`
  - `tf-plan` returns `Succeeded` when changes exist and `Skipped` when none do
  - `tf-output` accepts `dir`, `name`, `out`, `state`, `sensitive`, `vars`,
    `env`
  - `tf-output` returns outputs in the documented shape
  - `tf-apply` accepts `dir`, `plan`, `vars`, `env`
  - `tf-apply` returns output key `result`

`send-message`:

- scope:
  - `send-message`
  - `MessageChannel`
  - `ClusterMessageChannel`
  - Slack only
- do not implement SMTP in this plugin
- step config should match the public `send-message` docs for the Slack subset:
  - [`send-message`](../../../../docs/docs/50-user-guide/60-reference-docs/30-promotion-steps/send-message.md)
- v1 step config subset:
  - `channel.kind`
  - `channel.name`
  - `message`
  - `encodingType`
  - `slack.channelID`
  - `slack.threadTS`
- v1 output subset:
  - `slack.threadTS`
- the plugin owns channel lookup; the host passes opaque step config
- the plugin requires Kubernetes access
- use the existing Akuity CRD surface:
  - `ee.kargo.akuity.io/v1alpha1`
  - `MessageChannel`
  - `ClusterMessageChannel`
- `MessageChannel` rules:
  - namespaced
  - lives in the Project namespace
  - `secretRef.name` resolves in the same namespace
  - `secretRef` contains Slack `apiKey`
- `ClusterMessageChannel` rules:
  - cluster-scoped
  - `secretRef.name` resolves in `SYSTEM_RESOURCES_NAMESPACE`
  - `secretRef` contains Slack `apiKey`
- RBAC rules:
  - plugin sidecar sets `automountServiceAccountToken: true`
  - plugin install ships RBAC for channel reads and referenced `Secret` reads
  - host does not get this RBAC by default

## Not In V1

- No real `StepPlugin` CRD.
- No webhook validation against the discovered plugin registry.
- No UI/editor/schema discovery requirement.
- No generic plugin system beyond StepPlugins.
- No host-owned plugin CRDs.
- No real OpenTofu plugin runtime in the first host slice.
- No real `send-message` plugin runtime in the first host slice.

## Minimal `mkdir` Example

Source files:

`plugin.yaml`

```yaml
apiVersion: kargo-extended.code.org/v1alpha1
kind: StepPlugin
metadata:
  name: mkdir
  namespace: kargo-system-resources # could also be a Project namespace
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
```

`server.py`

```python
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
```

Build and install:

```bash
kargo step-plugin build .
kubectl apply -f mkdir-step-plugin-configmap.yaml
```

Enablement:

```yaml
controller:
  env:
  - name: STEP_PLUGINS_ENABLED
    value: "false"
```

Generated discovery `ConfigMap`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mkdir-step-plugin
  namespace: kargo-system-resources
  labels:
    kargo-extended.code.org/configmap-type: StepPlugin
data:
  sidecar.automountServiceAccountToken: "false"
  sidecar.container: |
    args:
    - |
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
    command:
    - python
    - -u
    - -c
    image: python:alpine3.23
    name: mkdir-step-plugin
    ports:
    - containerPort: 9765
    resources:
      requests:
        cpu: 50m
        memory: 32Mi
      limits:
        cpu: 100m
        memory: 64Mi
    securityContext:
      runAsNonRoot: true
      runAsUser: 65534
  steps.yaml: |
    - kind: mkdir
```

Use in a `Stage`:

```yaml
spec:
  promotionTemplate:
    spec:
      steps:
      - uses: mkdir
        config:
          path: demo/subdir
```
