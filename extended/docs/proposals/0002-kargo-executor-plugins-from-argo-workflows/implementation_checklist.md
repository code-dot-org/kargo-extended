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

- [ ] Prefer a branch named:
  - [ ] `proposal/0002-kargo-executor-plugins-from-argo-workflows`
- [ ] If a PR exists for implementation:
  - [ ] set PR title to match the current title in `proposal.md`
  - [ ] update the PR description when implementation starts
  - [ ] include a GitHub link to the proposal directory on that branch
  - [ ] include a direct GitHub link to `proposal.md` on that branch
- [ ] If the PR was opened during proposal writing, do not churn the
  description until implementation starts.
- [ ] Read source code and docs for the Argo Worfklows executor plugin
- [ ] Read all files outside ./external you expect to be patching
- [ ] Read the implementation_plan.md, spec, etc (all proposal docs)
- [ ] Read any web material you deem relevant to this implementation after
      reading the plan.

## Phase 1: Copy In Argo Source

- [ ] Copy and lightly patch:
  - [ ] `../../../../../argo-workflows/cmd/argo/commands/executorplugin/root.go`
  - [ ] `../../../../../argo-workflows/cmd/argo/commands/executorplugin/build.go`
  - [ ] `../../../../../argo-workflows/cmd/argo/commands/executorplugin/io.go`
- [ ] Copy and patch:
  - [ ] `../../../../../argo-workflows/workflow/util/plugin/plugin.go`
  - [ ] `../../../../../argo-workflows/workflow/util/plugin/configmap.go`
- [ ] Fork:
  - [ ] `../../../../../argo-workflows/pkg/plugins/spec/plugin_types.go`
- [ ] Keep copied code under `extended/pkg/argoworkflows/`.
- [ ] Preserve upstream file headers.
- [ ] Record upstream source path and commit on copied files.
- [ ] If a copied file needs heavy surgery, own the forked version instead of
  pretending it is still a light patch.
- [ ] Keep as-is from copied build-tool files:
  - [ ] `plugin.yaml` load
  - [ ] optional `server.*` embed
  - [ ] manifest -> `ConfigMap` flow
- [ ] Patch in copied build-tool files:
  - [ ] command names
  - [ ] filenames
  - [ ] Kargo strings
  - [ ] `steps.yaml`
- [ ] Keep as-is from copied transport/discovery helpers:
  - [ ] generic HTTP JSON call path
  - [ ] unsupported-method cache
  - [ ] transient retry behavior
  - [ ] sidecar `ConfigMap` encoding shape
- [ ] Patch in copied transport/discovery helpers:
  - [ ] Kargo label key/value
  - [ ] Kargo spec type
  - [ ] `steps.yaml`
- [ ] Use as reference only:
  - [ ] `../../../../../argo-workflows/pkg/plugins/executor/template_executor_plugin.go`
  - [ ] `../../../../../argo-workflows/workflow/controller/agent.go`
  - [ ] `../../../../../argo-workflows/workflow/executor/agent.go`
  - [ ] `../../../../../argo-workflows/cmd/argoexec/commands/agent.go`
- [ ] Use as product-docs source material:
  - [ ] `../../../../../argo-workflows/docs/executor_plugins.md`
  - [ ] `../../../../../argo-workflows/docs/cli/argo_executor-plugin.md`
  - [ ] `../../../../../argo-workflows/docs/cli/argo_executor-plugin_build.md`
  - [ ] `../../../../../argo-workflows/docs/executor_swagger.md`

## External Seams

- [ ] `cmd/cli/root.go`
  - [ ] keep to one import block change
  - [ ] keep to one `cmd.AddCommand(...)` line
  - [ ] keep command logic in `extended/pkg/stepplugin/cli/root_bridge.go`
  - [ ] keep build logic in `extended/pkg/stepplugin/cli/build.go`
  - [ ] protect this seam with `extended/pkg/stepplugin/cli/root_bridge_test.go`
- [ ] `cmd/controlplane/controller.go`
  - [ ] keep to one import block and one setup block in `setupReconcilers()`
  - [ ] have the helper read `STEP_PLUGINS_ENABLED`
  - [ ] keep discovery and engine wiring in
        `extended/pkg/stepplugin/controller/controller_bridge.go`
  - [ ] do not spread StepPlugin env parsing across `controllerOptions`
  - [ ] do not inline discovery logic here
  - [ ] protect this seam with
        `extended/pkg/stepplugin/controller/controller_bridge_test.go`
- [ ] `pkg/controller/promotions/promotions.go`
  - [ ] keep edits to workdir setup in `promote()`
  - [ ] keep edits to timeout/requeue logic in
        `calculateRequeueInterval()`
  - [ ] leave delete cleanup and `cleanupWorkDir()` alone unless proven
        necessary
  - [ ] keep helper logic in
        `extended/pkg/stepplugin/promotions/promotions_bridge.go`
  - [ ] do not add special-case logic in many small places
  - [ ] protect this seam with
        `extended/pkg/stepplugin/promotions/promotions_bridge_test.go`
- [ ] `pkg/promotion/registry.go`
  - [ ] avoid touching this file if possible
  - [ ] keep plugin metadata resolution in
        `extended/pkg/stepplugin/registry/resolver.go`
  - [ ] if forced, keep it to one tiny adapter block
  - [ ] if forced, use
        `extended/pkg/stepplugin/registry/registry_bridge.go`
  - [ ] stop and revisit the design if this starts turning into a broad edit
  - [ ] protect this seam with
        `extended/pkg/stepplugin/registry/resolver_test.go`
- [ ] `pkg/promotion/local_executor.go`
  - [ ] avoid touching this file if a separate plugin-aware engine path under
        `extended/` can own plugin-backed Promotions
  - [ ] keep builtin-only Promotions on the existing local engine
  - [ ] if forced, keep it to one dispatch branch and one delegation call
  - [ ] keep bridge logic in
        `extended/pkg/stepplugin/executor/local_executor_bridge.go`
  - [ ] keep supporting logic in
        `extended/pkg/stepplugin/executor/dispatcher.go`
  - [ ] keep wire adaptation in
        `extended/pkg/stepplugin/executor/wiretypes.go`
  - [ ] stop and revisit the design if touching this also forces broad edits to
        `pkg/promotion/local_orchestrator.go`
  - [ ] protect this seam with:
    - [ ] `extended/pkg/stepplugin/executor/local_executor_bridge_test.go`
    - [ ] `extended/pkg/stepplugin/executor/dispatcher_test.go`
    - [ ] `extended/pkg/stepplugin/executor/wiretypes_test.go`

## Phase 2: Authoring Manifest And Build Tool

- [ ] Add Kargo StepPlugin spec types under `extended/pkg/argoworkflows/...`.
- [ ] Add `kargo step-plugin` root command.
- [ ] Add `kargo step-plugin build DIR`.
- [ ] Load `plugin.yaml`.
- [ ] Embed optional single `server.*`.
- [ ] Write `<name>-step-plugin-configmap.yaml`.
- [ ] Preserve `metadata.namespace` from `plugin.yaml`.
- [ ] Keep Argo sidecar validation shape.
- [ ] Keep any external edit in this phase to a thin bridge only.
- [ ] Use the first declared container port as the plugin RPC port.

## Phase 3: Discovery And Spec Parsing

- [ ] Parse labeled StepPlugin `ConfigMap`s.
- [ ] Use `kargo-extended.code.org/configmap-type: StepPlugin`.
- [ ] Parse and emit `sidecar.automountServiceAccountToken`.
- [ ] Parse and emit `sidecar.container`.
- [ ] Parse and emit `steps.yaml`.
- [ ] Reject invalid sidecar config.
- [ ] Reject missing or duplicate step kinds.
- [ ] Generated `ConfigMap` names use `-step-plugin`.

## Phase 4: Runtime Plugin Registry

- [ ] Add Kargo-owned discovery code under `extended/pkg/stepplugin/`.
- [ ] Watch the Project namespace.
- [ ] Watch `SYSTEM_RESOURCES_NAMESPACE`.
- [ ] Resolve effective plugins by plugin name first.
- [ ] Make Project namespace win over `SYSTEM_RESOURCES_NAMESPACE`.
- [ ] Build a resolved step registry keyed by step kind.
- [ ] Reject collisions with builtin step kinds.
- [ ] Reject collisions between effective plugins.
- [ ] Add public enablement through env var `STEP_PLUGINS_ENABLED`.
- [ ] Document that users set it through the chart's existing `controller.env`.
- [ ] When disabled, ignore StepPlugin discovery.
- [ ] Keep this resolver in `extended/`.

## Phase 5: Agent-Backed Promotion Execution

- [ ] Keep builtin-only Promotions on the current local engine.
- [ ] Route plugin-backed Promotions to the agent path.
- [ ] Put builtin and plugin steps in one shared workdir.
- [ ] Add one plugin sidecar per used plugin.
- [ ] Reuse the agent pod across reconciles for one Promotion UID.
- [ ] Stop using controller-local workdir for plugin-backed Promotions.
- [ ] Keep the controller owning:
  - [ ] step ordering
  - [ ] expression evaluation
  - [ ] shared state
  - [ ] retries and backoff
  - [ ] `Promotion` status writes
- [ ] Keep the agent owning:
  - [ ] the shared workdir
  - [ ] builtin step execution inside the pod
  - [ ] plugin step RPC calls to sidecars

## Phase 6: Plugin Transport And Agent Runtime

- [ ] Add `POST /api/v1/step.execute`.
- [ ] Define `StepExecuteRequest`.
- [ ] Define `StepExecuteResponse`.
- [ ] Use normal JSON on the wire.
- [ ] Do not base64-encode `step.config`.
- [ ] Keep internal promotion types behind the transport boundary.
- [ ] Init container writes one random token per plugin sidecar container name.
- [ ] Init and main mount `/var/run/kargo`.
- [ ] Agent main reads `/var/run/kargo/<sidecar-container-name>/token`.
- [ ] Each plugin sidecar sees `/var/run/kargo/token`.
- [ ] Agent sends `Authorization: Bearer <token>`.
- [ ] Bad auth returns `403`.
- [ ] Keep RPC auth separate from optional Kubernetes service account token
  mounts.
- [ ] Keep plugin transport generic behavior:
  - [ ] `404` caches unsupported method
  - [ ] `503` retries as transient
  - [ ] other `4xx/5xx` fail hard

## Phase 7: First Deliverable Boundary

- [ ] Build the documented `mkdir` plugin example.
- [ ] Generate the documented `mkdir` discovery `ConfigMap`.
- [ ] Install it into `kargo-system-resources` or a Project namespace.
- [ ] Prove discovery works.
- [ ] Prove a `Stage` can run `uses: mkdir`.
- [ ] Keep OpenTofu and `send-message` out of this repo's first host slice.
- [ ] Reach roughly Argo executor plugin doc depth by copying and rewriting
  where that keeps work small.

## Tests

- [ ] Build-tool tests.
- [ ] `plugin.yaml` -> generated `ConfigMap` -> parsed discovery object
      round-trip test.
- [ ] Config parser tests.
- [ ] Registry precedence and collision tests.
- [ ] Transport tests for `200`, `403`, `404`, `503`, timeouts, and connection
  refused.
- [ ] Test that bearer token is sent on plugin RPC.
- [ ] Test that `step.config` is normal JSON on the wire.
- [ ] Test that StepPlugin disablement leaves builtin-only behavior alone.
- [ ] Controller tests for local vs agent execution path.
- [ ] Agent tests for shared workdir and token mounts.
- [ ] End-to-end `mkdir` proof from `Stage`.
- [ ] `extended/pkg/stepplugin/cli/root_bridge_test.go` covers the CLI bridge.
- [ ] `extended/pkg/stepplugin/controller/controller_bridge_test.go` covers the
  controller bridge.
- [ ] `extended/pkg/stepplugin/promotions/promotions_bridge_test.go` covers the
  Promotions bridge.
- [ ] `extended/pkg/stepplugin/registry/resolver_test.go` covers the runtime
  plugin registry seam.
- [ ] `extended/pkg/stepplugin/executor/local_executor_bridge_test.go` covers
  any forced executor bridge.
- [ ] `extended/pkg/stepplugin/executor/dispatcher_test.go` covers plugin step
  dispatch.
- [ ] `extended/pkg/stepplugin/executor/wiretypes_test.go` covers
  internal-to-wire adaptation.

## Phase 8: Post-Green External Diff Minimization

1. Comparison base
   - [ ] Get the feature working first.
   - [ ] Get the relevant tests green first.
   - [ ] Use the real upstream Kargo history as the comparison base.
   - [ ] Prefer the existing `upstream` remote.
   - [ ] If `upstream` is missing, add `https://github.com/akuity/kargo.git`.
   - [ ] `git fetch upstream`.
2. Measure the current outside-`extended/` diff
   - [ ] Review every file edited outside `extended/`.
   - [ ] Diff each edited external file against `upstream/main`.
   - [ ] Count changed lines and non-contiguous edit blocks in each.
3. Try to shrink each spicy file
   - [ ] Move any avoidable logic behind `extended/` helpers.
   - [ ] Re-check whether any outside-`extended/` edit can be removed entirely.
   - [ ] See if a better whole-file strategy reduces edit blocks or total diff
         size.
   - [ ] Ask which edits were only convenient during development and can now be
         collapsed or removed.
4. Guard against overdoing diff minimization
   - [ ] Re-check whether shrinking the outside diff would create a large
         duplicate subsystem under `extended/`.
   - [ ] Re-check whether shrinking the outside diff would increase the
         compatibility burden with upstream Kargo more than it lowers
         merge-conflict risk.
5. Re-test after cleanup
   - [ ] After every cleanup pass on an outside-`extended/` file, rerun the
         matching `extended/` tests.
   - [ ] After every cleanup pass, rerun any broader targeted tests that touch
         that seam.
   - [ ] After any merge-conflict repair in an outside-`extended/` file, rerun
         the matching `extended/` tests.

## Product Docs

- [ ] Create `extended/docs-site/05-kargo-external/_category_.json`.
- [ ] Create `extended/docs-site/05-kargo-external/index.md`.
- [ ] Create `extended/docs-site/05-kargo-external/10-step-plugins.md`.
- [ ] Create `extended/docs-site/05-kargo-external/20-step-plugin-build.md`.
- [ ] Create `extended/docs-site/05-kargo-external/30-step-plugin-rpc.md`.
- [ ] Add symlink `docs/docs/05-kargo-external ->
  ../../extended/docs-site/05-kargo-external`.
- [ ] Make `Kargo External` appear before `Home`.
- [ ] Keep fork-owned docs under `extended/docs-site/05-kargo-external/`.
- [ ] Rely on the docs loader following symlinks instead of moving doc source.
- [ ] Do not edit `docs/sidebars.js` unless ordering fails.
- [ ] Document `plugin.yaml` and `server.*` embedding.
- [ ] Document env-var enablement through existing `controller.env`.
- [ ] Document install into `kargo-system-resources` or a Project namespace.
- [ ] Document Project namespace precedence.
- [ ] Document exact RPC auth contract and token paths.
- [ ] Document `403`, `404`, and `503`.
- [ ] Document the minimal `mkdir` example and `Stage` usage.
- [ ] Reach roughly Argo executor plugin doc depth by copying and rewriting
  where that keeps work small.
