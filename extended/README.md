# kargo-extended

`kargo-extended` **adds a PLUGIN SYSTEM** to [Kargo](https://github.com/akuity/kargo)!

We would be very happy to merge our plugin system into upstream, if they would have us? 🥺

NOW: you can write your own plugins to kargo, e.g. in python, and they're
very easy (see below). Plugins can be defined externally to the repo, and
are defined as Kubernetes objects.

In particular, you can now defined new steps, for use in promotions, like:
- mkdir
- create-yaml
- transmogrify-world
- post-to-my-internal-system
- etc.

We based our plugin system on [Argo Workflow's ExecutorPlugins](https://argo-workflows.readthedocs.io/en/latest/executor_plugins/),
as in, literally, we used a lot of their source code. This gave us a well thought
out model to start from.

## Quick Start: `mkdir` StepPlugin

This is the minimal `mkdir` example from
`extended/docs/proposals/0002-kargo-executor-plugins-from-argo-workflows/proposal.md`.

Create a work directory:

```bash
mkdir -p /tmp/kargo-mkdir-plugin
cd /tmp/kargo-mkdir-plugin
```

Write `plugin.yaml`:

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

Write `server.py`:

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

Build and install it:

```bash
kargo step-plugin build .
kubectl apply -f mkdir-step-plugin-configmap.yaml
```

Use it in a `Stage`:

```yaml
spec:
  promotionTemplate:
    spec:
      steps:
      - uses: mkdir
        config:
          path: demo/subdir
```

## 95% of fork-specific code lives in [`extended/`](./extended/)

As much of the fork's code, docs, and test logic as possible live under
[`extended/`](./extended/). Files outside that directory should stay thin when
they can: wiring, bridges, small chart changes, and similar seams.

The repo-root `README.md` is a symlink to this file. Treat this file as the
fork homepage.

## What Lives Here

- `docs/`
  - fork docs and proposal directories
- `docs-site/`
  - fork product docs exposed through the main Docusaurus site
- `pkg/argoworkflows/`
  - copied or adapted Argo executor-plugin helpers and types
- `pkg/stepplugin/`
  - the host-side StepPlugin implementation
- `tests/`
  - fork-owned e2e helpers

## StepPlugin Architecture

The current fork feature is runtime-loaded `StepPlugin`s modeled on Argo
Workflows executor plugins.

High-level flow:

1. `kargo step-plugin build DIR` reads `plugin.yaml` and optional `server.*`.
2. It writes a labeled discovery `ConfigMap` plus a generated `README.md`.
3. The controller resolves StepPlugins from those labeled `ConfigMap`s.
4. Builtin-only Promotions stay on the normal upstream local engine.
5. Promotions that use plugin steps run through a per-Promotion agent pod.
6. The agent main container runs builtin steps locally and calls plugin
   sidecars over localhost HTTP.
7. Builtin and plugin steps share `/workspace`.
8. RPC auth uses bearer tokens under `/var/run/kargo`.

Main packages:

- `pkg/stepplugin/cli/`
  - `kargo step-plugin` command wiring
- `pkg/stepplugin/registry/`
  - StepPlugin discovery and step-kind resolution
- `pkg/stepplugin/agentpod/`
  - promotion-agent pod construction and remote execution runtime
- `pkg/stepplugin/executor/`
  - RPC wire types and dispatcher
- `pkg/stepplugin/controller/`
  - controller bridge wiring
- `pkg/stepplugin/promotions/`
  - thin bridge code for upstream promotion-controller seams

## Proposal System

Proposals live under:

- `extended/docs/proposals/NNNN-short-name/`

Required files:

- `proposal.md`
- `status.yaml`

Common implementation files:

- `implementation_plan.md`
- `implementation_checklist.md`
- `implementation_notes.md`

Normal flow:

1. Write `implementation_plan.md` when implementation starts.
2. Derive `implementation_checklist.md` from that plan.
3. Update the checklist as implementation teaches things.
4. Record useful handoff detail in `implementation_notes.md`.
5. If proposal status changes, update both `proposal.md` and `status.yaml`.

Before adding or reshaping proposals, read:

- `extended/docs/AGENTS.md`
- `extended/docs/proposals/0000-proposal-directory-structure/proposal.md`

To find active work, inspect `status.yaml` files and unfinished
`implementation_checklist.md` items.

## E2E Tests

First read:

- `docs/docs/60-contributor-guide/10-hacking-on-kargo.md`

That doc covers local cluster and Tilt setup. The e2e script does not create
the cluster or deploy Kargo for you.

Common setup:

```bash
make hack-build-cli
make hack-kind-up      # or use Docker Desktop Kubernetes
make hack-tilt-up
```

Full CLI/API e2e:

```bash
./pkg/cli/tests/e2e.sh
```

Useful targeted checks:

```bash
go test ./extended/...
make lint-go
```

## Working Rules

- Keep as much fork code as possible under `extended/`.
- Keep edits outside `extended/` thin and boring to avoid merge conflicts.
- Add tests under `extended/` for every external seam you introduce.
- After the feature is green, do the required post-green pass against
  `upstream/main` and try to shrink outside-`extended/` edits again.

## `e2e.sh` Env Vars

Read `docs/docs/60-contributor-guide/10-hacking-on-kargo.md` first. These
flags only select which part of the existing e2e harness to run. They do not
create the cluster or deploy Kargo for you.

Run only the fork StepPlugin smoke path:

```bash
STEPPLUGINS_ONLY=true ./pkg/cli/tests/e2e.sh
```

Skip the fork StepPlugin smoke path and run the rest of the harness:

```bash
STEPPLUGINS_SKIP=true ./pkg/cli/tests/e2e.sh
```
