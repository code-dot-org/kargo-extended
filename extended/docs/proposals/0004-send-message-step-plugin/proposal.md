# 0004 Send-Message Step Plugin

Status: proposed

Date: 2026-03-24

## Proposal

- Follow proposal
  [0002-kargo-executor-plugins-from-argo-workflows](../0002-kargo-executor-plugins-from-argo-workflows/proposal.md).
- Build a `send-message` StepPlugin that uses the host slice from 0002.
- Scope:
  - `send-message`
  - `MessageChannel`
  - `ClusterMessageChannel`
  - Slack only
- Keep the plugin runtime out of `kargo-extended`.
- Match the public Kargo `send-message` step shape for the Slack subset as
  closely as public docs allow.
- Keep channel lookup and referenced Secret reads in the plugin, not the host.
- Do not implement SMTP in this slice.

## Why

- `send-message` was a stated design target for 0002.
- It proves the model for plugin-owned Kubernetes access and plugin-owned CRD
  reads.
- It is the clean next slice after the generic StepPlugin host is done.

## Initial Success Looks Like

- A real Slack-only plugin image exists outside this repo.
- A Promotion step using `send-message` runs through the 0002 StepPlugin host.
- The plugin resolves `MessageChannel` and `ClusterMessageChannel` itself.
- The plugin reads the referenced Slack secret with plugin-owned RBAC.
- No new host-side seams are needed, or any required new seam is documented
  precisely.

## Not Decided Yet

- exact plugin repo layout
- exact Slack payload and output compatibility edges
- exact in-cluster test fixture shape
