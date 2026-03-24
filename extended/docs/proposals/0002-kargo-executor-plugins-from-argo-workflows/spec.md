# Kargo Executor Plugins Spec

## Overview

- `kargo-extended` adds runtime-loaded executor plugins modeled on Argo
  Workflows executor plugins.
- A plugin is an out-of-process step executor. It is not a Go package imported
  into the controller binary.
- OpenTofu and `send-message` are the first plugin targets.
- Read
  [Required Host Hooks](./plugin-technical-hook-needs-list.md#required-host-hooks)
  and
  [Main Technical Constraint](./plugin-technical-hook-needs-list.md#main-technical-constraint)
  before touching host code. Those sections are required reading for this spec.

## Rules

- Use the Argo Workflows executor plugin model, not Argo CD CMP.
- Runtime discovery, not build-time imports.
- No Go `plugin`.
- No `LD_PRELOAD`.
- Host owns orchestration, retries, shared state, expression evaluation, and
  Promotion status.
- Plugins own step logic.
- Plugin CRDs, plugin RBAC, plugin images, and plugin Secrets stay with the
  plugin.
- If a Promotion contains any plugin step, run the whole Promotion in the
  agent pod.
- UI/schema discovery is optional later. It is not a v1 requirement.

## Re-using code from ArgoCD Workflow Plugin Executors

- This means Argo Workflows executor plugin code, not Argo CD CMP code.
- Where sane, extract code from Argo Workflows instead of rewriting it.
- Prefer, in order:
  - verbatim copied files
  - minimally patched copies
  - fresh Kargo code only where the Argo shape does not fit
- Do not make the Kargo design worse just to preserve a verbatim copy.
- If a copied file needs heavy surgery, stop pretending and own the forked
  version.
- Keep copied code in an obvious subtree under `extended/` so it is easy to
  diff against upstream.
- If we end up copying more than a handful of files, add a small refresh script
  that can sync from a pinned Argo Workflows checkout or commit.
- Preserve upstream license headers and mark local edits clearly.

Good first copy targets:

- `/Users/seth/src/argo-workflows/workflow/util/plugin/plugin.go`
- `/Users/seth/src/argo-workflows/workflow/util/plugin/configmap.go`
- `/Users/seth/src/argo-workflows/pkg/plugins/spec/plugin_types.go`
- `/Users/seth/src/argo-workflows/pkg/plugins/executor/template_executor_plugin.go`

Use as scaffold, not verbatim target:

- `/Users/seth/src/argo-workflows/workflow/controller/agent.go`
- `/Users/seth/src/argo-workflows/workflow/executor/agent.go`

## Discovery

- Plugin install ships a labeled `ConfigMap`.
- Discovery namespaces:
  - the Project namespace
  - `SYSTEM_RESOURCES_NAMESPACE`
- If the same plugin name exists in both, the Project namespace wins.
- The controller watches those `ConfigMap`s and keeps an in-memory plugin
  registry.
- `ConfigMap.data["plugin.yaml"]` contains the plugin spec.

Plugin spec:

```yaml
apiVersion: kargo.akuity.io/v1alpha1
kind: ExecutorPlugin
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

Rules for `steps`:

- `steps[*].kind` matches `spec.steps[].uses` in a `Promotion`.
- `defaultTimeout` and `defaultErrorThreshold` feed the same metadata paths the
  builtin registry uses today.
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
  - one sidecar for each plugin actually used by that Promotion
  - one shared `emptyDir` mounted at a fixed workdir path
- The main agent container executes builtin steps locally and plugin steps over
  localhost HTTP to sidecars.
- This is the clean path for OpenTofu. Builtin steps and plugin steps see the
  same files.

Agent pod rules:

- One agent pod per `Promotion` UID.
- Reuse it across reconciles until the `Promotion` is terminal.
- Workdir lifetime is pod lifetime.
- For agent-backed Promotions, the controller no longer owns the workdir on its
  local filesystem.

## RPC

- Controller -> agent main: `POST /api/v1/step.execute`
- Agent main -> plugin sidecar: `POST /api/v1/step.execute`
- Request body: `promotion.StepExecutionRequest`
- Response body: `promotion.StepResult`

HTTP status handling:

- `200`: use the response body
- `404`: method unsupported; cache that and stop calling it
- `503`: transient; retry
- anything else: step error

Plugin rules:

- Any language is fine if it can speak HTTP JSON.
- A plugin only needs to implement the methods it uses.
- v1 only needs `step.execute`.

## Auth And RBAC

- Copy the Argo pattern for plugin sidecars that need Kubernetes access.
- `spec.sidecar.automountServiceAccountToken: true` means the host mounts a
  token only for that sidecar.
- The plugin install ships the `ServiceAccount`, `Role`/`ClusterRole`, and
  bindings it needs.
- Do not grant plugin CRD or plugin Secret access to the host unless there is
  no other clean way.

This matters most for `send-message`. The Slack plugin should read its own CRDs
and referenced `Secret`s itself.

## Host Surface

This spec depends on these sections in
[plugin-technical-hook-needs-list.md](./plugin-technical-hook-needs-list.md):

- [Required Host Hooks](./plugin-technical-hook-needs-list.md#required-host-hooks)
- [Existing Behavior We Can Keep](./plugin-technical-hook-needs-list.md#existing-behavior-we-can-keep)
- [Plugin-Owned, Not Host Hooks](./plugin-technical-hook-needs-list.md#plugin-owned-not-host-hooks)
- [Feature Minimums](./plugin-technical-hook-needs-list.md#feature-minimums)

Read them. They are not optional background.

## Not In V1

- No webhook validation against the discovered plugin registry.
- No UI/editor/schema discovery requirement.
- No generic plugin system beyond executor plugins.
- No host-owned plugin CRDs.

## Example Plugin Specs

### OpenTofu Plugin

Scope:

- `tf-plan`
- `tf-output`
- `tf-apply`

Rules:

- This plugin requires the agent path because all three steps use the workdir.
- The sidecar image carries `tofu`.
- No new CRDs.
- No plugin Kubernetes access required in v1 unless the plugin chooses to read
  cluster state directly.
- Step config and outputs should match the public docs:
  - [`tf-plan`](../../../../docs/docs/50-user-guide/60-reference-docs/30-promotion-steps/tf-plan.md)
  - [`tf-output`](../../../../docs/docs/50-user-guide/60-reference-docs/30-promotion-steps/tf-output.md)
  - [`tf-apply`](../../../../docs/docs/50-user-guide/60-reference-docs/30-promotion-steps/tf-apply.md)

Required behavior:

- `tf-plan`
  - accepts `dir`, `out`, `vars`, `env`
  - returns `Succeeded` when changes are detected
  - returns `Skipped` when no changes are detected
  - returns output key `plan`
- `tf-output`
  - accepts `dir`, `name`, `out`, `state`, `sensitive`, `vars`, `env`
  - returns outputs in the documented shape
- `tf-apply`
  - accepts `dir`, `plan`, `vars`, `env`
  - returns output key `result`

Example plugin spec:

```yaml
apiVersion: kargo.akuity.io/v1alpha1
kind: ExecutorPlugin
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

### `send-message` Slack Plugin

Scope:

- `send-message`
- `MessageChannel`
- `ClusterMessageChannel`
- Slack only

Rules:

- Step config should match the public `send-message` docs for the Slack subset:
  - [`send-message`](../../../../docs/docs/50-user-guide/60-reference-docs/30-promotion-steps/send-message.md)
- Do not implement SMTP in this plugin.
- The plugin owns channel lookup. The host only passes the opaque step config.
- The plugin requires Kubernetes access.

Step config supported in v1:

- `channel.kind`: `MessageChannel` or `ClusterMessageChannel`
- `channel.name`
- `message`
- `encodingType`
- `slack.channelID`
- `slack.threadTS`

Output supported in v1:

- `slack.threadTS`

CRDs:

- API group: `messaging.plugins.kargo.akuity.io/v1alpha1`
- `MessageChannel`: namespaced
- `ClusterMessageChannel`: cluster-scoped
- `spec` shape mirrors the public docs
- only one channel type per object
- in v1 only `slack` is valid

`MessageChannel` rules:

- must live in the Project namespace
- `secretRef.name` resolves in the same namespace
- `secretRef` must contain Slack `apiKey`

`ClusterMessageChannel` rules:

- cluster-scoped
- `secretRef.name` resolves in `SYSTEM_RESOURCES_NAMESPACE`
- `spec` is otherwise identical to `MessageChannel`

RBAC rules:

- plugin sidecar sets `automountServiceAccountToken: true`
- plugin install ships RBAC for:
  - namespaced `MessageChannel` reads
  - cluster-scoped `ClusterMessageChannel` reads
  - `Secret` reads in the Project namespace
  - `Secret` reads in `SYSTEM_RESOURCES_NAMESPACE`
- host does not get this RBAC

Example plugin spec:

```yaml
apiVersion: kargo.akuity.io/v1alpha1
kind: ExecutorPlugin
metadata:
  name: send-message
spec:
  sidecar:
    automountServiceAccountToken: true
    container:
      name: send-message-plugin
      image: ghcr.io/yourorg/kargo-plugin-send-message:latest
      ports:
      - containerPort: 9766
      securityContext:
        runAsNonRoot: true
      resources:
        requests:
          cpu: 50m
          memory: 64Mi
        limits:
          cpu: 500m
          memory: 256Mi
  steps:
  - kind: send-message
    defaultTimeout: 1m
    defaultErrorThreshold: 1
```

Example `MessageChannel`:

```yaml
apiVersion: messaging.plugins.kargo.akuity.io/v1alpha1
kind: MessageChannel
metadata:
  name: slack
  namespace: kargo-demo
spec:
  secretRef:
    name: slack-token
  slack:
    channelID: C1234567890
```

Example `ClusterMessageChannel`:

```yaml
apiVersion: messaging.plugins.kargo.akuity.io/v1alpha1
kind: ClusterMessageChannel
metadata:
  name: devops-slack
spec:
  secretRef:
    name: slack-token
  slack:
    channelID: C1234567890
```
