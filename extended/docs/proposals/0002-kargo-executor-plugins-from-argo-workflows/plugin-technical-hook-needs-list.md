# Plugin Technical Hook Needs List

## Overview

1. This file lists source files outside `extended/` that the plugin system may
   need to touch.
2. Keep this list current.
3. Any file listed here should stay a thin bridge if possible.
4. Any feature seam listed here needs tests under `extended/`.

## Required Host Hooks

`cmd/controlplane/controller.go`

- Today:
  - blank-imports builtin step runners
  - builds `promotion.NewLocalEngine(...)`
- Need:
  - respect `STEP_PLUGINS_ENABLED`
  - bootstrap plugin discovery from labeled plugin `ConfigMap`s
  - resolve effective plugins by name with Project namespace over
    `SYSTEM_RESOURCES_NAMESPACE`
  - build a plugin-aware promotion engine
- Prefer:
  - keep this file to wiring and delegation
  - put implementation under `extended/`
- Needed by:
  - OpenTofu
  - `send-message`

`cmd/controlplane/root.go`

- Today:
  - wires subcommands for the controlplane binary
- Need:
  - add the `promotion-agent` host command so the controller image can serve as
    the execution host inside plugin-backed Promotion pods
- Prefer:
  - one import block change
  - one `AddCommand(...)` line
  - keep real agent logic under `extended/`
- Needed by:
  - plugin-backed Promotion execution

`pkg/controller/promotions/promotions.go`

- Today:
  - keeps a per-Promotion workdir on the controller filesystem
  - reuses that workdir across reconciles
  - `calculateRequeueInterval()` reads metadata from
    `promotion.DefaultStepRunnerRegistry`
- Need:
  - if a Promotion uses any plugin step, do not create or use a
    controller-local workdir for it
  - agent-backed Promotions use an agent-owned workdir instead
  - `calculateRequeueInterval()` must use plugin-aware metadata
- Prefer:
  - keep this file to policy and wiring
  - put helper logic under `extended/`
- Needed by:
  - OpenTofu
  - `send-message` registry lookup

`pkg/promotion/registry.go`

- Today:
  - in-memory registry
  - assumes compile-time registration
- Need:
  - plugin metadata must be available anywhere timeout/default logic reads it
- Prefer:
  - avoid touching this file if a parallel resolver under `extended/` can keep
    the outside diff smaller
- Needed by:
  - OpenTofu
  - `send-message`

`pkg/promotion/local_executor.go`

- Today:
  - looks up a runner
  - injects capabilities
  - calls `runner.Run(...)` in-process
- Need:
  - builtin steps stay local
  - plugin steps go over plugin transport
  - internal promotion types adapt to clean plugin RPC wire types at the
    transport boundary
- Prefer:
  - avoid touching this file if a separate plugin-aware engine path under
    `extended/` can own plugin-backed Promotions
  - if touched, keep it to one thin dispatch bridge
- Needed by:
  - OpenTofu
  - `send-message`

`pkg/promotion/runner/builtin/schema_loader.go`
`hack/codegen/promotion-step-configs.sh`

- Today:
  - builtin step schemas are embedded and generated at build time
- Need:
  - plugin schemas must be publishable without editing builtin schema lists
- Prefer:
  - if skipped in v1, runtime can still work; only schema-driven validation and
    authoring stay builtin-only
- Needed by:
  - OpenTofu
  - `send-message`

`ui/hack/generate-other-schema.mjs`
`ui/src/features/promotion-directives/registry/use-discover-registries.ts`

- Today:
  - UI promotion directive schemas are hardcoded to builtin steps
- Need:
  - discover plugin step schemas dynamically
  - or accept no UI support for plugin steps in v1
- Needed by:
  - OpenTofu
  - `send-message`

## Existing Behavior We Can Keep

`pkg/webhook/kubernetes/promotion_steps.go`
`pkg/webhook/kubernetes/stage/webhook.go`

- Today:
  - do not validate step kinds against a known registry
  - only validate alias rules and task-ref rules
- Keep:
  - none required for v1
  - optional later: validate against discovered plugins

`pkg/expressions/function/functions.go`

- Today:
  - step config expressions already support `secret()`, `sharedSecret()`,
    `configMap()`, `sharedConfigMap()`, `freightMetadata()`, `stageMetadata()`
- Keep:
  - none required for v1

## Plugin-Owned, Not Host Hooks

`MessageChannel` and `ClusterMessageChannel` CRDs

- Today:
  - do not exist in OSS Kargo
- Plugin owns:
  - those CRDs in OSS
  - the existing Akuity GVKs, not fork-only ones
- Needed by:
  - `send-message`

Plugin RBAC

- Today:
  - host chart only grants built-in permissions
- Plugin owns:
  - its own RBAC
  - any extra binding if the host must read plugin CRDs or referenced `Secret`s
- Needed by:
  - `send-message`

Plugin runtime image

- Today:
  - controller image only contains builtin host code
- Plugin owns:
  - its own runtime image
  - `tofu` in the OpenTofu image
  - Slack logic in the `send-message` image
- Needed by:
  - OpenTofu
  - `send-message`

## Feature Minimums

OpenTofu:

- dynamic step registration
- plugin transport from host executor
- access to the Promotion workdir
- optional plugin schemas for UI/validation
- no new CRDs

`send-message`:

- dynamic step registration
- plugin transport from host executor
- existing `MessageChannel`
- existing `ClusterMessageChannel`
- system-resources namespace config
- host RBAC only if the host process reads channel CRDs or referenced `Secret`s

## Main Technical Constraint

OpenTofu:

- do not try to share the controller's local filesystem with plugins
- if a Promotion has a plugin step, run that Promotion in the agent path
- builtin steps in that Promotion must run there too if they touch the workdir
- `tf-plan`, `tf-apply`, and `tf-output` all want local files

`send-message`:

- does not care about the Promotion workdir
- its hard part is the channel resource model, not local filesystem access

## Not On This List

- If a thing is not listed here, it is not a hook yet.
- Do not add architecture theory here.
- Add paths, exact technical change, and whether we can avoid the edit.
