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
- Keep the plugin runtime out of `kargo-extended`.
- Match the public Kargo step shape closely enough to be a drop-in for users.
- Reuse the shared-workdir agent path from 0002.
- Do not add new host hooks unless the plugin proves one is really needed.

## Why

- OpenTofu was a stated design target for 0002.
- It is the clearest proof that the shared-workdir plugin model is useful.
- It is a natural next slice after the `mkdir` proof.

## Initial Success Looks Like

- A real OpenTofu plugin image exists outside this repo.
- `tf-plan`, `tf-output`, and `tf-apply` run through the 0002 StepPlugin host.
- The plugin reads and writes the same workdir as surrounding builtin steps.
- No new host-side seams are needed, or any required new seam is documented
  precisely.

## Not Decided Yet

- exact plugin repo layout
- exact test fixture shape
- exact compatibility target for edge-case flags and outputs
