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
- Put the plugin work under `extended/plugins/send-message`.
- Treat `extended/plugins/send-message` as if it were its own Git repo.
- Do not rely on code, files, build helpers, tests, or other resources outside
  `extended/plugins/send-message`.
- Use that subtree to test whether a third party can implement the plugin
  without merge permissions in the main repo.
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

- A real Slack-only plugin image is built from
  `extended/plugins/send-message`.
- A Promotion step using `send-message` runs through the 0002 StepPlugin host.
- The plugin resolves `MessageChannel` and `ClusterMessageChannel` itself.
- The plugin reads the referenced Slack secret with plugin-owned RBAC.
- The plugin subtree can be lifted out and treated like its own repo without
  needing resources from elsewhere in `kargo-extended`.
- No new host-side seams are needed, or any required new seam is documented
  precisely.

## Not Decided Yet

- exact Slack payload and output compatibility edges
- exact in-cluster test fixture shape
