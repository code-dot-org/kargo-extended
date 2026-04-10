# 0003 OpenTofu Step Plugin

Status: proposed

Date: 2026-03-24

## Proposal

- Follow proposal
  [0002-kargo-executor-plugins-from-argo-workflows](../0002-kargo-executor-plugins-from-argo-workflows/proposal.md).
- Build an OpenTofu StepPlugin that uses the host slice from 0002.
- Scope:
  - `tf-plan`
  - `tf-output`
  - `tf-apply`
- Put the plugin work under `extended/plugins/opentofu`.
- Treat `extended/plugins/opentofu` as if it were its own Git repo.
- Do not rely on code, files, build helpers, tests, or other resources outside
  `extended/plugins/opentofu`.
- Use that subtree to test whether a third party can implement the plugin
  without merge permissions in the main repo.
- Match the public Kargo step shape closely enough to be a drop-in for users.
- Reuse the shared-workdir agent path from 0002.
- Do not add new host hooks unless the plugin proves one is really needed.

## Why

- OpenTofu was a stated design target for 0002.
- It is the clearest proof that the shared-workdir plugin model is useful.
- It is a natural next slice after the `mkdir` proof.

## Initial Success Looks Like

- A real OpenTofu plugin image is built from `extended/plugins/opentofu`.
- `tf-plan`, `tf-output`, and `tf-apply` run through the 0002 StepPlugin host.
- The plugin reads and writes the same workdir as surrounding builtin steps.
- The plugin subtree can be lifted out and treated like its own repo without
  needing resources from elsewhere in `kargo-extended`.
- No new host-side seams are needed, or any required new seam is documented
  precisely.

## Not Decided Yet

- exact test fixture shape
- exact compatibility target for edge-case flags and outputs
