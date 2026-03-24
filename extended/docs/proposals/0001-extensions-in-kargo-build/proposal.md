# 0001 Must Build Extensions Into Kargo Build

Status: rejected

Date: 2026-03-23

## Proposal

- `kargo-extended` exposes host hooks.
- Plugin repos live outside this repo.
- The final Kargo binary imports those repos at build time.
- There is one composition point under `extended/`.
- There is no runtime plugin discovery.

This was the model described in:

- [Overview](./spec.md#overview)
- [Rules](./spec.md#rules)
- [Host Surface](./spec.md#host-surface)
- [Build](./spec.md#build)

## Why It Looked Good

- Small host delta.
- Fits the existing in-process step runner registry.
- Easy first path for OpenTofu.
- Easy first path for a Slack-backed `send-message` step.

## Why We Rejected It

- It fails the main product preference: add plugins without rebuilding Kargo.
- Every plugin set needs a new controller image build.
- Plugin install becomes a source/build problem, not a cluster install problem.
- There is no runtime discovery story.
- It is less familiar in this ecosystem than the Argo Workflows executor plugin
  model.
- It locks plugin code into the host process when an out-of-process model is a
  better fit for long-term isolation.

## Decision

Reject this model.

Use runtime-loaded executor plugins modeled on Argo Workflows instead.

See:

- [Overview](../0002-kargo-executor-plugins-from-argo-workflows/spec.md#overview)
- [Discovery](../0002-kargo-executor-plugins-from-argo-workflows/spec.md#discovery)
- [Execution Model](../0002-kargo-executor-plugins-from-argo-workflows/spec.md#execution-model)
- [Example Plugin Specs](../0002-kargo-executor-plugins-from-argo-workflows/spec.md#example-plugin-specs)

## Consequences

- `spec.md` remains useful as a rejected alternative.
- New host work should follow the executor plugin spec, not this one.
- If a future change brings build-time composition back, it needs a new
  proposal.
