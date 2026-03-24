# Plugin Technical Hook Needs List

## Overview

1. This file lists source files outside `extended/` that the plugin system
needs to touch.
2. Each item says what exists now, what change we need, who owns it, and which
plugin needs it.
3. Plugin-owned CRDs and RBAC are listed when they affect integration, but they
are not host hooks.
4. Keep this list current.

## Required Host Hooks

`cmd/controlplane/controller.go`
- Today:
  - blank-imports builtin step runners
  - builds `promotion.NewLocalEngine(...)`
- Change:
  - bootstrap plugin discovery
  - register builtins plus discovered plugins
  - build a plugin-aware promotion engine/executor
- Owner: host
- Needed by: OpenTofu, `send-message`

`pkg/promotion/registry.go`
- Today:
  - in-memory registry
  - assumes compile-time registration
- Change:
  - keep the registry
  - add runtime registration from discovered plugins
  - make plugin metadata available anywhere timeout/default logic reads it
- Owner: host
- Needed by: OpenTofu, `send-message`

`pkg/promotion/local_executor.go`
- Today:
  - looks up a runner
  - injects capabilities
  - calls `runner.Run(...)` in-process
- Change:
  - dispatch builtin steps in-process
  - dispatch plugin steps over plugin transport
  - keep the current `StepContext` -> `StepResult` contract
- Owner: host
- Needed by: OpenTofu, `send-message`

`pkg/controller/promotions/promotions.go`
- Today:
  - keeps a per-Promotion workdir on the controller filesystem
  - reuses that workdir across reconciles
  - `calculateRequeueInterval()` reads metadata from `promotion.DefaultStepRunnerRegistry`
- Change:
  - `calculateRequeueInterval()` must use the plugin-aware registry
  - if a Promotion uses any plugin step, do not create or use a controller-local
    workdir for it
  - agent-backed Promotions use an agent-owned workdir instead
- Owner: host
- Needed by:
  - OpenTofu: yes, this is the main filesystem and execution-mode constraint
  - `send-message`: registry lookup only

`pkg/promotion/runner/builtin/schema_loader.go`
`hack/codegen/promotion-step-configs.sh`
- Today:
  - builtin step schemas are embedded and generated at build time
- Change:
  - plugin schemas must be publishable without editing builtin schema lists
  - if we skip this in v1, runtime can still work, but schema-driven validation
    and authoring will be builtin-only
- Owner: host for discovery, plugin for schemas
- Needed by: OpenTofu, `send-message`

`ui/hack/generate-other-schema.mjs`
`ui/src/features/promotion-directives/registry/use-discover-registries.ts`
- Today:
  - UI promotion directive schemas are hardcoded to builtin steps
- Change:
  - discover plugin step schemas dynamically
  - or accept no UI support for plugin steps in v1
- Owner: host for discovery, plugin for schemas
- Needed by: OpenTofu, `send-message`

## Existing Behavior We Can Keep

`pkg/webhook/kubernetes/promotion_steps.go`
`pkg/webhook/kubernetes/stage/webhook.go`
- Today:
  - does not validate step kinds against a known registry
  - only validates alias rules and task-ref rules
- Change:
  - none required for v1
  - optional later: validate against discovered plugins
- Owner: host
- Needed by: nobody for v1

`pkg/expressions/function/functions.go`
- Today:
  - step config expressions already support `secret()`, `sharedSecret()`,
    `configMap()`, `sharedConfigMap()`, `freightMetadata()`, `stageMetadata()`
- Change:
  - none required for v1
- Owner: no change
- Needed by:
  - OpenTofu: use existing secret/config access
  - `send-message`: can use existing expression support in step config

## Plugin-Owned, Not Host Hooks

`MessageChannel` and `ClusterMessageChannel` CRDs
- Today:
  - do not exist in OSS Kargo
- Change:
  - the `send-message` plugin ships and installs its own CRDs
- Owner: plugin
- Needed by: `send-message`

Plugin RBAC
- Today:
  - host chart only grants built-in permissions
- Change:
  - the plugin ships its own RBAC
  - if the host process reads plugin CRDs or their referenced Secrets, plugin
    install must also bind that RBAC to the host service account
- Owner: plugin install
- Needed by: `send-message`

Plugin runtime image
- Today:
  - controller image only contains builtin host code
- Change:
  - each plugin ships its own runtime image
  - OpenTofu plugin image carries `tofu`
  - `send-message` plugin image carries Slack logic
- Owner: plugin
- Needed by: OpenTofu, `send-message`

`charts/kargo/templates/controller/configmap.yaml`
- Today:
  - controller already gets `SYSTEM_RESOURCES_NAMESPACE`
  - controller already gets `SHARED_RESOURCES_NAMESPACE`
- Change:
  - plugin runtime needs the same namespace config if it reads cluster-scoped
    channel secrets or shared resources
  - easiest path is to have plugin install read the same Helm values
- Owner: plugin install
- Needed by: `send-message`

## Feature Minimums

OpenTofu
- dynamic step registration
- plugin transport from host executor
- access to the Promotion workdir
- optional plugin schemas for UI/validation
- no new CRDs

`send-message`
- dynamic step registration
- plugin transport from host executor
- plugin-owned `MessageChannel`
- plugin-owned `ClusterMessageChannel`
- system-resources namespace config
- host RBAC only if the host process reads channel CRDs or referenced Secrets

## Main Technical Constraint

OpenTofu
- Decision: do not try to share the controller's local filesystem with plugins.
- If a Promotion has a plugin step, run that Promotion in the agent path.
- Builtin steps in that Promotion must run there too if they touch the workdir.
- `tf-plan`, `tf-apply`, and `tf-output` all want local files.

`send-message`
- Does not care about the Promotion workdir.
- Its hard part is the channel resource model, not local filesystem access.

## Not On This List

- If a thing is not listed here, it is not a hook yet.
- Do not add architecture theory here.
- Add paths, ownership, and exact technical change only.
