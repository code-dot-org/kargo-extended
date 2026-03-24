# Follow-Up

This file is for the next agent who will verify and finish the remaining open
items from proposal 0002.

Primary missing proof: build the documented `mkdir` StepPlugin from source
files, install it into a cluster, and prove a real `Stage` can run
`uses: mkdir`.

## What Is Already True

These are already implemented:

- `kargo step-plugin build DIR`
- StepPlugin `ConfigMap` parsing and emitting
- plugin-name precedence: Project namespace over `SYSTEM_RESOURCES_NAMESPACE`
- builtin step-kind collision rejection
- plugin-backed Promotions routed to a per-Promotion agent pod
- shared `/workspace` workdir in the agent pod
- builtin steps executed in the agent main container
- plugin steps executed over localhost HTTP to plugin sidecars
- token file wiring under `/var/run/kargo`
- product docs under `extended/docs-site/05-kargo-external/`

Useful code entrypoints:

- CLI build flow:
  - `extended/pkg/stepplugin/cli/root_bridge.go`
  - `extended/pkg/stepplugin/cli/io.go`
- StepPlugin spec and `ConfigMap` helpers:
  - `extended/pkg/argoworkflows/pkg/plugins/spec/plugin_types.go`
  - `extended/pkg/argoworkflows/workflow/util/plugin/configmap.go`
- runtime registry:
  - `extended/pkg/stepplugin/registry/resolver.go`
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

## Open Items

From `implementation_notes.md`:

- Discovery currently resolves plugin `ConfigMap`s on demand through the
  controller client. It does not yet maintain a watched in-memory registry.

From `implementation_checklist.md`:

- [ ] Watch the Project namespace.
- [ ] Watch `SYSTEM_RESOURCES_NAMESPACE`.
- [ ] Build the documented `mkdir` plugin example.
- [ ] Generate the documented `mkdir` discovery `ConfigMap`.
- [ ] Install it into `kargo-system-resources` or a Project namespace.
- [ ] Prove discovery works.
- [ ] Prove a `Stage` can run `uses: mkdir`.

From chat:

- I did not run an in-cluster end-to-end `mkdir` proof yet.
- The host sends the bearer token and mounts token paths as specified, but
  actual `403` enforcement is still on the plugin sidecar implementation side.

## What The Next Agent Should Test

### 1. Build Tool

Confirm the documented `mkdir` example can really be built from source files,
not just from unit tests.

Use these exact docs:

- `extended/docs-site/05-kargo-external/10-step-plugins.md`
- `extended/docs-site/05-kargo-external/20-step-plugin-build.md`
- `extended/docs/proposals/0002-kargo-executor-plugins-from-argo-workflows/proposal.md`

The proposal already contains concrete example contents for:

- `plugin.yaml`
- `server.py`
- `stage.yaml`
- `mkdir-step-plugin-configmap.yaml`

The docs page `10-step-plugins.md` also shows the minimal `Stage` usage:

```yaml
spec:
  promotionTemplate:
    spec:
      steps:
      - uses: mkdir
        config:
          path: demo/subdir
```

Minimum proof:

1. Create a temp directory with:
   - `plugin.yaml`
   - one `server.py`
2. Use the documented example content, not an improvised variant.
3. Run:

```bash
kargo step-plugin build <dir>
```

4. Verify it writes:
   - `<name>-step-plugin-configmap.yaml`
   - `README.md`
5. Verify the generated `ConfigMap` contains:
   - `kargo-extended.code.org/configmap-type: StepPlugin`
   - `sidecar.automountServiceAccountToken`
   - `sidecar.container`
   - `steps.yaml`
6. Verify the embedded `server.py` content appears in the generated YAML.
7. Verify `metadata.name` and `metadata.namespace` match `plugin.yaml`.

Expected build behavior from `20-step-plugin-build.md`:

- output file name is `<name>-step-plugin-configmap.yaml`
- `README.md` is emitted beside it
- if there is a single `server.*` file, its contents are stored in
  `spec.sidecar.container.args[0]` before YAML emission

There is already narrow automated coverage for this in:

- `extended/pkg/stepplugin/cli/root_bridge_test.go`

That test is useful for expected output shape, but it is not a substitute for
running the real CLI binary.

If the next agent changes the build output shape, update:

- docs under `extended/docs-site/05-kargo-external/`
- `implementation_checklist.md`

### 2. Runtime Discovery

Current behavior is on-demand resolution, not a watched in-memory registry.

The next agent needs to decide whether to:

- leave that as-is and explicitly narrow the proposal/checklist/docs, or
- add the watch-based registry the proposal still calls for

Current implementation seam:

- `extended/pkg/stepplugin/registry/resolver.go`

Current behavior:

- list/read plugin `ConfigMap`s when resolving a plugin-backed Promotion
- combine Project namespace and `SYSTEM_RESOURCES_NAMESPACE`
- choose by plugin name first
- then build an effective registry by step kind

If implementing watches, the most natural seam is still:

- keep real logic in `extended/pkg/stepplugin/registry/`
- keep `cmd/controlplane/controller.go` unchanged except for thin wiring

What to verify if watches are added:

1. labeled StepPlugin `ConfigMap`s in the Project namespace are seen
2. labeled StepPlugin `ConfigMap`s in `SYSTEM_RESOURCES_NAMESPACE` are seen
3. Project namespace overrides system namespace by plugin name
4. duplicate effective step kinds still fail
5. builtin collisions still fail
6. disabling `STEP_PLUGINS_ENABLED` stops plugin discovery cleanly

Current tests that cover the non-watch part:

- `extended/pkg/stepplugin/registry/resolver_test.go`

Also inspect:

- `extended/pkg/stepplugin/controller/controller_bridge.go`
- `extended/pkg/stepplugin/engine.go`

### 3. In-Cluster `mkdir` Proof

This is the biggest missing proof.

Target outcome:

- a real `Stage` uses `uses: mkdir`
- the controller creates a promotion-agent pod
- the agent pod includes:
  - the agent main container
  - the `mkdir` plugin sidecar
  - shared `/workspace`
- the plugin step succeeds
- the created directory exists in the shared workdir during execution

You may need Tilt or a local cluster.

Relevant repo commands from the root `AGENTS.md`:

```bash
make hack-kind-up
make hack-tilt-up
```

or whatever equivalent local cluster setup you prefer in this repo.

Likely practical flow:

1. Build and deploy the current controller image into the local dev cluster.
2. Enable StepPlugins through controller env:

```yaml
controller:
  env:
  - name: STEP_PLUGINS_ENABLED
    value: "true"
```

3. Build the `mkdir` plugin `ConfigMap` with `kargo step-plugin build`.
4. Apply that generated `ConfigMap` into:
   - `kargo-system-resources`, or
   - a Project namespace
5. Create or adapt a test `Stage` whose promotion template includes:

```yaml
- uses: mkdir
  config:
    path: demo/subdir
```

6. Trigger a `Promotion`.
7. Verify:
   - the step kind resolves
   - the `Promotion` goes through the plugin-aware engine path
   - an agent pod named like `promotion-agent-<promotion-uid>` appears
   - the pod contains the plugin sidecar
   - the `Promotion` finishes successfully

The most useful runtime source files are:

- `extended/pkg/stepplugin/promotions/promotions_bridge.go`
- `extended/pkg/stepplugin/engine.go`
- `extended/pkg/stepplugin/orchestrator/orchestrator.go`
- `extended/pkg/stepplugin/agentpod/runtime.go`
- `extended/pkg/stepplugin/agent/command_bridge.go`
- `extended/pkg/stepplugin/executor/dispatcher.go`

The generated plugin `ConfigMap` should produce a sidecar container named:

- `mkdir-step-plugin`

The proposal and docs expect the plugin to listen on the first declared
container port, which for the example is:

- `9765`

Do not stop at "the step returned `200`". The better proof is that the plugin
really touched the shared workdir. Best case:

- the plugin creates `demo/subdir`
- a later builtin step in the same Promotion observes or uses that path

Good places to inspect while debugging:

- controller logs
- `kubectl get pods -n <project>`
- `kubectl describe pod promotion-agent-...`
- `kubectl logs <agent-pod> -c promotion-agent`
- `kubectl logs <agent-pod> -c mkdir-step-plugin`

### 4. Token/Auth Contract

The host side currently does this:

- init container writes `/var/run/kargo/<sidecar-container-name>/token`
- agent main reads that file
- plugin sidecar gets `/var/run/kargo/token` via subPath mount
- agent HTTP client sends `Authorization: Bearer <token>`

What is not proven yet:

- that a real plugin sidecar checks the token and returns `403` on bad auth

The next agent should test this with a plugin server that:

1. reads `/var/run/kargo/token`
2. compares it to `Authorization: Bearer ...`
3. returns `403` on mismatch
4. returns `200` on match

The exact contract is already documented in:

- `extended/docs-site/05-kargo-external/30-step-plugin-rpc.md`

That doc is meant to be normative for:

- `POST /api/v1/step.execute`
- `StepExecuteRequest`
- `StepExecuteResponse`
- bearer token path and header
- `403` on bad auth
- `404` unsupported method caching
- `503` transient retry behavior

If reality differs from that doc, either the code or the docs need to change.

This can be done with either:

- a focused Go/unit-style test around the agent/dispatcher/client pieces, or
- a real in-cluster plugin sidecar used in the `mkdir` proof

The second option is better if it is not too painful.

Likely code paths to inspect:

- `extended/pkg/stepplugin/agentpod/remote_executor.go`
- `extended/pkg/stepplugin/executor/dispatcher.go`
- `extended/pkg/stepplugin/executor/wiretypes.go`

### 5. Docs vs Reality Pass

After proving the real flow, re-read:

- `extended/docs-site/05-kargo-external/10-step-plugins.md`
- `extended/docs-site/05-kargo-external/20-step-plugin-build.md`
- `extended/docs-site/05-kargo-external/30-step-plugin-rpc.md`
- `extended/docs/proposals/0002-kargo-executor-plugins-from-argo-workflows/implementation_checklist.md`
- `extended/docs/proposals/0002-kargo-executor-plugins-from-argo-workflows/implementation_notes.md`

and make them match reality exactly.

The most likely mismatches to clean up are:

- watch-based discovery versus on-demand resolution
- exact pod naming or container naming
- exact test/demo commands
- first-port RPC behavior
- token mount paths
- how much of the `403` auth contract is host-side versus plugin-side

If the next agent lands the watch-based registry or the end-to-end `mkdir`
proof, update the proposal docs in the same change.

## Existing Automated Coverage

These tests already exist and are a good starting point:

- `extended/pkg/stepplugin/cli/root_bridge_test.go`
- `extended/pkg/stepplugin/registry/resolver_test.go`
- `extended/pkg/stepplugin/executor/wiretypes_test.go`
- `extended/pkg/stepplugin/promotions/promotions_bridge_test.go`
- `extended/pkg/stepplugin/controller/controller_bridge_test.go`

Useful targeted command:

```bash
go test ./extended/... ./cmd/cli ./cmd/controlplane ./pkg/controller/promotions ./pkg/promotion
```

That gave good coverage for the bridge seams and some wire-shape behavior, but
it does not prove:

- real cluster discovery
- real agent pod assembly
- real sidecar RPC auth enforcement
- real shared-workdir behavior across builtin and plugin steps

## Outside-`extended/` Diff

I already did one post-green diff pass against `upstream/main`.

The current external edits are still small and look reasonable:

- `cmd/cli/root.go`
- `cmd/controlplane/controller.go`
- `cmd/controlplane/root.go`
- `pkg/controller/promotions/promotions.go`
- `pkg/controller/promotions/promotions_test.go`
- `pkg/promotion/evaluator.go`
- `pkg/promotion/promotion.go`

If the next agent makes more external edits, repeat that diff-minimization pass
again before calling the work done.
