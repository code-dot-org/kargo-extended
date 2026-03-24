# Follow-Up

This file is for the next agent who needs the current state of proposal 0002.

The host slice is implemented and green. The big open items that used to live
here are done:

- watched StepPlugin discovery is implemented
- the documented `mkdir` example was built and installed for real
- a real `Stage` ran `uses: mkdir`
- the repo StepPlugin smoke path passed
- the full current `pkg/cli/tests/e2e.sh` passed

## What Is Already True

- `kargo step-plugin build DIR`
- StepPlugin `ConfigMap` parsing and emitting
- watched in-memory StepPlugin discovery
- plugin-name precedence: Project namespace over `SYSTEM_RESOURCES_NAMESPACE`
- builtin step-kind collision rejection
- StepPlugins default on; `STEP_PLUGINS_ENABLED=false` disables them
- plugin-backed Promotions routed to a per-Promotion agent pod
- shared `/workspace` workdir in the agent pod
- builtin steps executed in the agent main container
- plugin steps executed over localhost HTTP to plugin sidecars
- token file wiring under `/var/run/kargo`
- product docs under `extended/docs-site/05-kargo-external/`
- real repo smoke proof for the documented `mkdir` example
- full current repo e2e harness green after fixing the old project
  delete/recreate race

Useful code entrypoints:

- CLI build flow:
  - `extended/pkg/stepplugin/cli/root_bridge.go`
  - `extended/pkg/stepplugin/cli/io.go`
- StepPlugin spec and `ConfigMap` helpers:
  - `extended/pkg/argoworkflows/pkg/plugins/spec/plugin_types.go`
  - `extended/pkg/argoworkflows/workflow/util/plugin/configmap.go`
- runtime registry:
  - `extended/pkg/stepplugin/registry/resolver.go`
  - `extended/pkg/stepplugin/registry/store.go`
  - `extended/pkg/stepplugin/registry/watcher.go`
- engine selection:
  - `extended/pkg/stepplugin/engine.go`
- agent pod creation:
  - `extended/pkg/stepplugin/agentpod/runtime.go`
- agent HTTP server:
  - `extended/pkg/stepplugin/agent/command_bridge.go`
- agent step dispatch:
  - `extended/pkg/stepplugin/executor/dispatcher.go`
- controller bridge:
  - `extended/pkg/stepplugin/controller/controller_bridge.go`
- promotions bridge:
  - `extended/pkg/stepplugin/promotions/promotions_bridge.go`

Useful docs:

- `extended/docs-site/05-kargo-external/10-step-plugins.md`
- `extended/docs-site/05-kargo-external/20-step-plugin-build.md`
- `extended/docs-site/05-kargo-external/30-step-plugin-rpc.md`
- `extended/docs/proposals/0002-kargo-executor-plugins-from-argo-workflows/proposal.md`
- `extended/docs/proposals/0002-kargo-executor-plugins-from-argo-workflows/implementation_notes.md`
- `extended/docs/proposals/0002-kargo-executor-plugins-from-argo-workflows/implementation_checklist.md`

## Remaining Things To Remember

- On a fresh kind/Tilt cluster here, `pkg/cli/tests/e2e.sh` still assumed the
  singleton `ClusterConfig/cluster` existed already.
  - that object had to be seeded manually before the e2e runs
  - this looks like an upstream-ish harness precondition, not a StepPlugin bug
- `pkg/cli/tests/e2e.sh` now has useful split modes:
  - `STEPPLUGINS_ONLY=true ./pkg/cli/tests/e2e.sh`
  - `STEPPLUGINS_SKIP=true ./pkg/cli/tests/e2e.sh`
- If someone changes the product docs or smoke flow again, keep the docs,
  smoke script, and proposal notes in sync.

## If The Build Flow Regresses

Use these docs exactly:

- `extended/docs-site/05-kargo-external/10-step-plugins.md`
- `extended/docs-site/05-kargo-external/20-step-plugin-build.md`
- `extended/docs/proposals/0002-kargo-executor-plugins-from-argo-workflows/proposal.md`

The proof that already passed was:

1. create a temp directory with:
   - `plugin.yaml`
   - `server.py`
2. use the documented example content, not an improvised variant
3. run:

```bash
kargo step-plugin build <dir>
```

4. verify it writes:
   - `<name>-step-plugin-configmap.yaml`
   - `README.md`
5. verify the generated `ConfigMap` contains:
   - `kargo-extended.code.org/configmap-type: StepPlugin`
   - `sidecar.automountServiceAccountToken`
   - `sidecar.container`
   - `steps.yaml`
6. verify the embedded `server.py` content appears in the generated YAML
7. verify `metadata.name` and `metadata.namespace` match `plugin.yaml`

Existing automated coverage:

- `extended/pkg/stepplugin/cli/root_bridge_test.go`

## If Runtime Discovery Regresses

Current behavior is watched discovery backed by an in-memory store populated by
an informer on labeled StepPlugin `ConfigMap`s.

Implementation seams:

- `extended/pkg/stepplugin/registry/resolver.go`
- `extended/pkg/stepplugin/registry/store.go`
- `extended/pkg/stepplugin/registry/watcher.go`

Current behavior:

- watch labeled plugin `ConfigMap`s cluster-wide
- combine Project namespace and `SYSTEM_RESOURCES_NAMESPACE`
- choose by plugin name first
- then build an effective registry by step kind

What to verify:

1. labeled StepPlugin `ConfigMap`s in the Project namespace are seen
2. labeled StepPlugin `ConfigMap`s in `SYSTEM_RESOURCES_NAMESPACE` are seen
3. Project namespace overrides system namespace by plugin name
4. duplicate effective step kinds still fail
5. builtin collisions still fail
6. disabling `STEP_PLUGINS_ENABLED` stops plugin discovery cleanly

Current tests:

- `extended/pkg/stepplugin/registry/resolver_test.go`
- `extended/pkg/stepplugin/registry/watcher_test.go`
- `extended/pkg/stepplugin/controller/controller_bridge_test.go`

## If The In-Cluster `mkdir` Proof Regresses

This proof already passed in the repo smoke path. Re-run it like this:

1. bring up the local cluster and Tilt per the repo docs
2. seed `ClusterConfig/cluster` if a fresh cluster still does not have it
3. run:

```bash
STEPPLUGINS_ONLY=true ./pkg/cli/tests/e2e.sh
```

That path proves:

- the documented `mkdir` plugin builds
- the generated `ConfigMap` installs
- watched discovery sees it
- a real `Stage` uses `uses: mkdir`
- a promotion-agent pod appears
- a later builtin step observes the plugin-created directory in shared
  `/workspace`

Useful runtime sources:

- `extended/pkg/stepplugin/promotions/promotions_bridge.go`
- `extended/pkg/stepplugin/engine.go`
- `extended/pkg/stepplugin/orchestrator/orchestrator.go`
- `extended/pkg/stepplugin/agentpod/runtime.go`
- `extended/pkg/stepplugin/agent/command_bridge.go`
- `extended/pkg/stepplugin/executor/dispatcher.go`

Good places to inspect while debugging:

- controller logs
- `kubectl get pods -n <project>`
- `kubectl describe pod promotion-agent-...`
- `kubectl logs <agent-pod> -c promotion-agent`
- `kubectl logs <agent-pod> -c mkdir-step-plugin`

## If Auth / `403` Regresses

What is already true:

- the host writes and mounts bearer tokens as documented
- the host sends `Authorization: Bearer <token>`
- focused transport coverage exercises the bad-token path

If this regresses, inspect:

- `extended/pkg/stepplugin/executor/dispatcher_test.go`
- `extended/docs-site/05-kargo-external/30-step-plugin-rpc.md`

## Existing Automated Coverage

These tests already exist and are the first place to look:

- `extended/pkg/stepplugin/cli/root_bridge_test.go`
- `extended/pkg/stepplugin/registry/resolver_test.go`
- `extended/pkg/stepplugin/registry/watcher_test.go`
- `extended/pkg/stepplugin/executor/wiretypes_test.go`
- `extended/pkg/stepplugin/executor/dispatcher_test.go`
- `extended/pkg/stepplugin/promotions/promotions_bridge_test.go`
- `extended/pkg/stepplugin/controller/controller_bridge_test.go`

Useful targeted command:

```bash
go test ./extended/... ./cmd/cli ./cmd/controlplane ./pkg/controller/promotions ./pkg/promotion
```

## Outside-`extended/` Diff

The post-green diff-minimization pass has already been rerun against
`upstream/main`.

If the next agent makes more external edits, repeat that pass before calling
the work done.
