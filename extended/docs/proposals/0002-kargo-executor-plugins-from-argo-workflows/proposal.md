# 0002 Kargo Executor Plugins From Argo Workflows

Status: trial

Date: 2026-03-23

## Proposal

- `kargo-extended` adds runtime-loaded executor plugins modeled on Argo
  Workflows executor plugins.
- Plugin discovery is runtime, not build-time.
- Plugins are out-of-process containers, not Go packages imported into the
  controller binary.
- Where sane, copy Argo Workflows executor plugin code instead of rewriting it.
- Prefer, in order:
  - verbatim copied files
  - minimally patched copies
  - new Kargo code only where the Argo shape does not fit

## Why

- It matches Kargo step execution better than Argo CD CMP.
- It matches the user's preference to avoid rebuilding Kargo for every plugin
  set.
- It is already a known model in the Argo/Kubernetes ecosystem.
- We can likely reuse real code, not just ideas.

## First Targets

- OpenTofu:
  - `tf-plan`
  - `tf-output`
  - `tf-apply`
- `send-message`:
  - `send-message`
  - `MessageChannel`
  - `ClusterMessageChannel`
  - Slack only

## First Argo Copy Targets

- `/Users/seth/src/argo-workflows/workflow/util/plugin/plugin.go`
- `/Users/seth/src/argo-workflows/workflow/util/plugin/configmap.go`
- `/Users/seth/src/argo-workflows/pkg/plugins/spec/plugin_types.go`
- `/Users/seth/src/argo-workflows/pkg/plugins/executor/template_executor_plugin.go`

Scaffold only:

- `/Users/seth/src/argo-workflows/workflow/controller/agent.go`
- `/Users/seth/src/argo-workflows/workflow/executor/agent.go`

## Main Open Question

- OpenTofu needs shared workdir access.
- Current direction: if a `Promotion` uses any plugin step, run the whole
  `Promotion` in an agent pod with a shared workdir.
- This proposal is not accepted until that execution path is proven clean
  enough.

## Success Looks Like

- Kargo discovers plugin specs at runtime from the cluster.
- A plugin-backed `Promotion` runs builtin and plugin steps in one agent pod.
- OpenTofu can read and write the same workdir as surrounding builtin steps.
- `send-message` Slack can resolve plugin-owned `MessageChannel` resources.

## Links

- [Re-using code from ArgoCD Workflow Plugin Executors](./spec.md#re-using-code-from-argocd-workflow-plugin-executors)
- [Discovery](./spec.md#discovery)
- [Execution Model](./spec.md#execution-model)
- [RPC](./spec.md#rpc)
- [Example Plugin Specs](./spec.md#example-plugin-specs)
- [Required Host Hooks](../../plugin-technical-hook-needs-list.md#required-host-hooks)
- [Main Technical Constraint](../../plugin-technical-hook-needs-list.md#main-technical-constraint)
