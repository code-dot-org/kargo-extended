# 0002 Kargo Step Plugins From Argo Workflows

Status: trial

Date: 2026-03-23

## Proposal

- `kargo-extended` adds runtime-loaded StepPlugins modeled on Argo Workflows
  executor plugins.
- Plugin discovery is runtime, not build-time.
- `plugin.yaml` is a typed build input. `kargo step-plugin build DIR`
  generates the labeled discovery `ConfigMap`.
- Plugins are out-of-process containers, not Go packages imported into the
  controller binary.
- Plugin RPC uses Kargo-owned wire types with normal JSON:
  - `StepExecuteRequest`
  - `StepExecuteResponse`
- Internal promotion types stay internal.
- New invented plugin surface uses `kargo-extended.code.org`.
- Existing Akuity APIs keep their existing groups.
- `StepPlugin` is not a real CRD in v1.
- StepPlugins are on by default in v1. `STEP_PLUGINS_ENABLED` can disable them
  through the chart's existing `controller.env`.
- Where sane, copy Argo Workflows executor plugin code instead of rewriting it.
- Keep real fork logic under `extended/`.
- Keep files outside `extended/` to thin bridge edits with tests under
  `extended/`.

## Why

- This matches Kargo step execution better than Argo CD CMP.
- This avoids rebuilding Kargo for every plugin set.
- This is already a known model in the Argo/Kubernetes ecosystem.
- We can likely reuse real code, not just ideas.

## First Host Proof

- `mkdir`

## Later Targets

- OpenTofu:
  - `tf-plan`
  - `tf-output`
  - `tf-apply`
- `send-message`:
  - `send-message`
  - `MessageChannel`
  - `ClusterMessageChannel`
  - Slack only

## Main Constraint

- OpenTofu needs shared workdir access.
- Current direction: if a `Promotion` uses any plugin step, run the whole
  `Promotion` in an agent pod with a shared workdir.
- This proposal is not accepted until that execution path is proven clean
  enough.

## Success Looks Like

- `kargo step-plugin build DIR` turns a typed `StepPlugin` manifest into a
  labeled discovery `ConfigMap`.
- Kargo discovers labeled plugin `ConfigMap`s at runtime from
  `kargo-system-resources` (`$SYSTEM_RESOURCES_NAMESPACE`) or the Project
  namespace.
- Project namespace wins over `SYSTEM_RESOURCES_NAMESPACE` for the same plugin
  name.
- Effective step kinds are unambiguous. v1 does not permit plugins to shadow
  builtin step kinds or each other.
- Plugin RPC uses clean JSON on the wire. It does not marshal internal
  promotion structs directly.
- Plugin RPC auth is explicit:
  - plugin sidecar reads `/var/run/kargo/token`
  - agent sends `Authorization: Bearer <token>`
  - bad auth returns `403`
- The minimal `mkdir` plugin works end to end:
  - build
  - install
  - discovery
  - step execution from a `Stage`
- Product docs exist for:
  - writing a StepPlugin
  - building and installing it
  - the `kargo step-plugin build` command
  - the plugin RPC contract
- Product docs reach roughly Argo executor plugin doc depth, copied and
  rewritten where that keeps work small.
- Product docs include a top-level `Kargo External` section before `Home`.
- Outside-`extended/` edits stay thin and are re-reviewed after the feature is
  green.
- A plugin-backed `Promotion` runs builtin and plugin steps in one agent pod.
- OpenTofu can read and write the same workdir as surrounding builtin steps.
- `send-message` Slack can resolve existing `MessageChannel` resources.
- OpenTofu and `send-message` are design targets for the host slice. Their real
  plugin runtimes land later.

## Links

- [spec.md](./spec.md)
- [implementation_plan.md](./implementation_plan.md)
- [plugin-technical-hook-needs-list.md](./plugin-technical-hook-needs-list.md)

## Minimal `mkdir` Example

**Source Files**

`plugin.yaml`

```yaml
apiVersion: kargo-extended.code.org/v1alpha1
kind: StepPlugin
metadata:
  name: mkdir
  namespace: kargo-system-resources
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

**Use In Kargo**

`stage.yaml`

```yaml
spec:
  promotionTemplate:
    spec:
      steps:
      - uses: mkdir
        config:
          path: demo/subdir
```

**Generated ConfigMap**

`mkdir-step-plugin-configmap.yaml`

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
