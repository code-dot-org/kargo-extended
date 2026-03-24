# 0002 Implementation Checklist

Update this as implementation teaches us things.

## Baseline

- [x] `StepPlugin` is build input, not a served CRD.
- [x] The first host proof is `mkdir`.
- [x] Plugin RPC types are `StepExecuteRequest` and `StepExecuteResponse`.
- [x] RPC auth and env-var enablement are explicit.
- [x] New invented plugin surface uses `kargo-extended.code.org`.
- [x] External seams should stay thin and load logic from `extended/`.
- [x] External seams need tests under `extended/`.

## Phase 0: read a lot

- [x] Prefer a branch named:
  - [x] `proposal/0002-kargo-executor-plugins-from-argo-workflows`
- [x] If a PR exists for implementation:
  - [x] set PR title to match the current title in `proposal.md`
  - [x] update the PR description when implementation starts
  - [x] include a GitHub link to the proposal directory on that branch
  - [x] include a direct GitHub link to `proposal.md` on that branch
- [x] If the PR was opened during proposal writing, do not churn the
      description until implementation starts.
- [x] Read source code and docs for the Argo Worfklows executor plugin.
- [x] Read all files outside `extended/` expected to be patched.
- [x] Read the implementation plan, spec, and proposal docs.
- [x] Read the local Argo source/docs needed for this implementation. No extra
      web material was needed.

## Phase 1: Copy In Argo Source

- [x] Copy and lightly patch:
  - [x] `../../../../../argo-workflows/cmd/argo/commands/executorplugin/root.go`
  - [x] `../../../../../argo-workflows/cmd/argo/commands/executorplugin/build.go`
  - [x] `../../../../../argo-workflows/cmd/argo/commands/executorplugin/io.go`
- [x] Copy and patch:
  - [x] `../../../../../argo-workflows/workflow/util/plugin/plugin.go`
  - [x] `../../../../../argo-workflows/workflow/util/plugin/configmap.go`
- [x] Fork:
  - [x] `../../../../../argo-workflows/pkg/plugins/spec/plugin_types.go`
- [x] Keep copied code under `extended/pkg/argoworkflows/`.
- [x] Preserve upstream file headers where present.
- [x] Record upstream source path and commit on copied files.
- [x] Own the forked `plugin_types.go` instead of pretending it is a light
      patch.
- [x] Keep as-is from copied build-tool files:
  - [x] `plugin.yaml` load
  - [x] optional `server.*` embed
  - [x] manifest -> `ConfigMap` flow
- [x] Patch in copied build-tool files:
  - [x] command names
  - [x] filenames
  - [x] Kargo strings
  - [x] `steps.yaml`
- [x] Keep as-is from copied transport/discovery helpers:
  - [x] generic HTTP JSON call path
  - [x] unsupported-method cache
  - [x] transient retry behavior
  - [x] sidecar `ConfigMap` encoding shape
- [x] Patch in copied transport/discovery helpers:
  - [x] Kargo label key/value
  - [x] Kargo spec type
  - [x] `steps.yaml`
- [x] Use as reference only:
  - [x] `../../../../../argo-workflows/pkg/plugins/executor/template_executor_plugin.go`
  - [x] `../../../../../argo-workflows/workflow/controller/agent.go`
  - [x] `../../../../../argo-workflows/workflow/executor/agent.go`
  - [x] `../../../../../argo-workflows/cmd/argoexec/commands/agent.go`
- [x] Use as product-docs source material:
  - [x] `../../../../../argo-workflows/docs/executor_plugins.md`
  - [x] `../../../../../argo-workflows/docs/cli/argo_executor-plugin.md`
  - [x] `../../../../../argo-workflows/docs/cli/argo_executor-plugin_build.md`
  - [x] `../../../../../argo-workflows/docs/executor_swagger.md`

## External Seams

- [x] `cmd/cli/root.go`
  - [x] keep to one import block change
  - [x] keep to one `cmd.AddCommand(...)` line
  - [x] keep command logic in `extended/pkg/stepplugin/cli/root_bridge.go`
  - [x] keep build logic in `extended/pkg/stepplugin/cli/build.go`
  - [x] protect this seam with `extended/pkg/stepplugin/cli/root_bridge_test.go`
- [x] `cmd/controlplane/controller.go`
  - [x] keep to one import block and one setup block in `setupReconcilers()`
  - [x] have the helper read `STEP_PLUGINS_ENABLED`
  - [x] keep discovery and engine wiring in
        `extended/pkg/stepplugin/controller/controller_bridge.go`
  - [x] do not spread StepPlugin env parsing across `controllerOptions`
  - [x] do not inline discovery logic here
  - [x] protect this seam with
        `extended/pkg/stepplugin/controller/controller_bridge_test.go`
- [x] `cmd/controlplane/root.go`
  - [x] keep the external edit to thin command wiring only
  - [x] keep command logic in `extended/pkg/stepplugin/agent/command_bridge.go`
- [x] `pkg/controller/promotions/promotions.go`
  - [x] keep edits to workdir setup in `promote()`
  - [x] keep edits to timeout/requeue logic in
        `calculateRequeueInterval()`
  - [x] leave delete cleanup and `cleanupWorkDir()` alone
  - [x] keep helper logic in
        `extended/pkg/stepplugin/promotions/promotions_bridge.go`
  - [x] do not add special-case logic in many small places
  - [x] protect this seam with
        `extended/pkg/stepplugin/promotions/promotions_bridge_test.go`
- [x] `pkg/promotion/registry.go`
  - [x] avoid touching this file
  - [x] keep plugin metadata resolution in
        `extended/pkg/stepplugin/registry/resolver.go`
  - [x] no `registry_bridge.go` was needed
  - [x] protect this seam with
        `extended/pkg/stepplugin/registry/resolver_test.go`
- [x] `pkg/promotion/local_executor.go`
  - [x] avoid touching this file
  - [x] keep builtin-only Promotions on the existing local engine
  - [x] keep supporting logic in
        `extended/pkg/stepplugin/executor/dispatcher.go`
  - [x] keep wire adaptation in
        `extended/pkg/stepplugin/executor/wiretypes.go`
  - [x] no forced executor bridge was needed in `pkg/promotion/local_executor.go`
  - [x] protect the used seam with:
    - [x] `extended/pkg/stepplugin/executor/dispatcher_test.go`
    - [x] `extended/pkg/stepplugin/executor/wiretypes_test.go`

## Phase 2: Authoring Manifest And Build Tool

- [x] Add Kargo StepPlugin spec types under `extended/pkg/argoworkflows/...`.
- [x] Add `kargo step-plugin` root command.
- [x] Add `kargo step-plugin build DIR`.
- [x] Load `plugin.yaml`.
- [x] Embed optional single `server.*`.
- [x] Write `<name>-step-plugin-configmap.yaml`.
- [x] Preserve `metadata.namespace` from `plugin.yaml`.
- [x] Keep Argo sidecar validation shape.
- [x] Keep any external edit in this phase to a thin bridge only.
- [x] Use the first declared container port as the plugin RPC port.

## Phase 3: Discovery And Spec Parsing

- [x] Parse labeled StepPlugin `ConfigMap`s.
- [x] Use `kargo-extended.code.org/configmap-type: StepPlugin`.
- [x] Parse and emit `sidecar.automountServiceAccountToken`.
- [x] Parse and emit `sidecar.container`.
- [x] Parse and emit `steps.yaml`.
- [x] Reject invalid sidecar config.
- [x] Reject missing or duplicate step kinds.
- [x] Generated `ConfigMap` names use `-step-plugin`.

## Phase 4: Runtime Plugin Registry

- [x] Add Kargo-owned discovery code under `extended/pkg/stepplugin/`.
- [ ] Watch the Project namespace.
- [ ] Watch `SYSTEM_RESOURCES_NAMESPACE`.
- [x] Resolve effective plugins by plugin name first.
- [x] Make Project namespace win over `SYSTEM_RESOURCES_NAMESPACE`.
- [x] Build a resolved step registry keyed by step kind.
- [x] Reject collisions with builtin step kinds.
- [x] Reject collisions between effective plugins.
- [x] Add public enablement through env var `STEP_PLUGINS_ENABLED`.
- [x] Document that users set it through the chart's existing `controller.env`.
- [x] When disabled, ignore StepPlugin discovery.
- [x] Keep this resolver in `extended/`.

## Phase 5: Agent-Backed Promotion Execution

- [x] Keep builtin-only Promotions on the current local engine.
- [x] Route plugin-backed Promotions to the agent path.
- [x] Put builtin and plugin steps in one shared workdir.
- [x] Add one plugin sidecar per used plugin.
- [x] Reuse the agent pod across reconciles for one Promotion UID.
- [x] Stop using controller-local workdir for plugin-backed Promotions.
- [x] Keep the controller owning:
  - [x] step ordering
  - [x] expression evaluation
  - [x] shared state
  - [x] retries and backoff
  - [x] `Promotion` status writes
- [x] Keep the agent owning:
  - [x] the shared workdir
  - [x] builtin step execution inside the pod
  - [x] plugin step RPC calls to sidecars

## Phase 6: Plugin Transport And Agent Runtime

- [x] Add `POST /api/v1/step.execute`.
- [x] Define `StepExecuteRequest`.
- [x] Define `StepExecuteResponse`.
- [x] Use normal JSON on the wire.
- [x] Do not base64-encode `step.config`.
- [x] Keep internal promotion types behind the transport boundary.
- [x] Init container writes one random token per plugin sidecar container name.
- [x] Init and main mount `/var/run/kargo`.
- [x] Agent main reads `/var/run/kargo/<sidecar-container-name>/token`.
- [x] Each plugin sidecar sees `/var/run/kargo/token`.
- [x] Agent sends `Authorization: Bearer <token>`.
- [x] Bad auth returns `403`.
- [x] Keep RPC auth separate from optional Kubernetes service account token
      mounts.
- [x] Keep plugin transport generic behavior:
  - [x] `404` caches unsupported method
  - [x] `503` retries as transient
  - [x] other `4xx/5xx` fail hard

## Phase 7: First Deliverable Boundary

- [ ] Build the documented `mkdir` plugin example.
- [ ] Generate the documented `mkdir` discovery `ConfigMap`.
- [ ] Install it into `kargo-system-resources` or a Project namespace.
- [ ] Prove discovery works.
- [ ] Prove a `Stage` can run `uses: mkdir`.
- [x] Keep OpenTofu and `send-message` out of this repo's first host slice.
- [x] Reach roughly Argo executor plugin doc depth by copying and rewriting
      where that keeps work small.

## Tests

- [x] Build-tool tests.
- [x] `plugin.yaml` -> generated `ConfigMap` -> parsed discovery object
      round-trip test.
- [x] Config parser tests.
- [x] Registry precedence and collision tests.
- [x] Transport tests for `200`, `403`, `404`, `503`, timeouts, and connection
      refused.
- [x] Test that bearer token is sent on plugin RPC.
- [x] Test that `step.config` is normal JSON on the wire.
- [x] Test that StepPlugin disablement leaves builtin-only behavior alone.
- [x] Controller tests for local vs agent execution path.
- [x] Agent tests for shared workdir and token mounts.
- [ ] End-to-end `mkdir` proof from `Stage`.
- [x] `extended/pkg/stepplugin/cli/root_bridge_test.go` covers the CLI bridge.
- [x] `extended/pkg/stepplugin/controller/controller_bridge_test.go` covers the
      controller bridge.
- [x] `extended/pkg/stepplugin/promotions/promotions_bridge_test.go` covers the
      Promotions bridge.
- [x] `extended/pkg/stepplugin/registry/resolver_test.go` covers the runtime
      plugin registry seam.
- [x] No `local_executor_bridge_test.go` was needed because
      `pkg/promotion/local_executor.go` was not touched.
- [x] `extended/pkg/stepplugin/executor/dispatcher_test.go` covers plugin step
      dispatch.
- [x] `extended/pkg/stepplugin/executor/wiretypes_test.go` covers
      internal-to-wire adaptation.

## Phase 8: Post-Green External Diff Minimization

1. Comparison base
   - [x] Get the feature working first.
   - [x] Get the relevant tests green first.
   - [x] Use the real upstream Kargo history as the comparison base.
   - [x] Prefer the existing `upstream` remote.
   - [x] Existing `upstream` remote was present, so no add was needed.
   - [x] `git fetch upstream`.
2. Measure the current outside-`extended/` diff
   - [x] Review every file edited outside `extended/`.
   - [x] Diff each edited external file against `upstream/main`.
   - [x] Count changed lines and non-contiguous edit blocks in each.
3. Try to shrink each spicy file
   - [x] Move any avoidable logic behind `extended/` helpers.
   - [x] Re-check whether any outside-`extended/` edit can be removed entirely.
   - [x] See if a better whole-file strategy reduces edit blocks or total diff
         size.
   - [x] Ask which edits were only convenient during development and can now be
         collapsed or removed.
4. Guard against overdoing diff minimization
   - [x] Re-check whether shrinking the outside diff would create a large
         duplicate subsystem under `extended/`.
   - [x] Re-check whether shrinking the outside diff would increase the
         compatibility burden with upstream Kargo more than it lowers
         merge-conflict risk.
5. Re-test after cleanup
   - [x] After cleanup on outside-`extended/` files, rerun the matching
         `extended/` tests.
   - [x] After cleanup, rerun broader targeted tests that touch those seams.
   - [x] Matching `extended/` tests are green again.

## Product Docs

- [x] Create `extended/docs-site/05-kargo-external/_category_.json`.
- [x] Create `extended/docs-site/05-kargo-external/index.md`.
- [x] Create `extended/docs-site/05-kargo-external/10-step-plugins.md`.
- [x] Create `extended/docs-site/05-kargo-external/20-step-plugin-build.md`.
- [x] Create `extended/docs-site/05-kargo-external/30-step-plugin-rpc.md`.
- [x] Add symlink `docs/docs/05-kargo-external ->
      ../../extended/docs-site/05-kargo-external`.
- [x] Make `Kargo External` appear before `Home`.
- [x] Keep fork-owned docs under `extended/docs-site/05-kargo-external/`.
- [x] Rely on the docs loader following symlinks instead of moving doc source.
- [x] Do not edit `docs/sidebars.js` unless ordering fails.
- [x] Document `plugin.yaml` and `server.*` embedding.
- [x] Document env-var enablement through existing `controller.env`.
- [x] Document install into `kargo-system-resources` or a Project namespace.
- [x] Document Project namespace precedence.
- [x] Document exact RPC auth contract and token paths.
- [x] Document `403`, `404`, and `503`.
- [x] Document the minimal `mkdir` example and `Stage` usage.
- [x] Reach roughly Argo executor plugin doc depth by copying and rewriting
      where that keeps work small.

## Follow-Up Plans After Initial Implementation

- [ ] Decide whether v1 will keep on-demand discovery or still implement the
      watch-based in-memory registry.
- [ ] If v1 keeps on-demand discovery, update `proposal.md`,
      `implementation_notes.md`, `implementation_checklist.md`, and product
      docs to say that explicitly.
- [ ] After the real `mkdir` proof, do a docs-versus-reality pass on:
      `extended/docs-site/05-kargo-external/10-step-plugins.md`,
      `extended/docs-site/05-kargo-external/20-step-plugin-build.md`, and
      `extended/docs-site/05-kargo-external/30-step-plugin-rpc.md`.
- [ ] Prove shared-workdir behavior across a plugin step and a later builtin
      step in one `Promotion`, not just that the plugin RPC returned success.
- [x] Prove bad auth returns `403` with a real token-checking plugin sidecar or
      a focused transport test that exercises the same contract.
