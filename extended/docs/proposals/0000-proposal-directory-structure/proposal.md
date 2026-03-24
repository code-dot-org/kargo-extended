# 0000 Proposal Directory Structure

Status: accepted

Date: 2026-03-23

## Proposal

- Each proposal gets its own directory under `extended/docs/proposals/`.
- The directory name is `NNNN-short-name`.
- `proposal.md` is required.
- `status.yaml` is required.
- `spec.md` is optional.
- `implementation_plan.md` is optional.
- `implementation_checklist.md` is optional.
- `implementation_notes.md` is optional.

## Why

- Keep one proposal's docs together.
- Avoid filename sprawl at the top level.
- Let small proposals stay small.
- Let larger proposals carry spec, plan, checklist, and notes without making
  `docs/` messy.

## Implementation Files

- `implementation_plan.md` is the implementation plan.
- Write `implementation_plan.md` first when implementation starts.
- `implementation_checklist.md` is derived from `implementation_plan.md`.
- Write `implementation_checklist.md` as phased checklist items.
- Both files can change as implementation teaches us things.
- Write both so if an engineer is interrupted partway through, the next
  engineer can see where the previous engineer left off.
- Prefer changing `implementation_checklist.md` freely as work moves.
- `implementation_notes.md` is for useful implementation notes that should not
  be shoved into the proposal or spec.

## Branches And PRs

- When implementing a proposal, prefer a branch named like:
  - `proposal/NNNN-proposal-dir-name`
- PR title should match the current title in `proposal.md`.
- If implementation is requested and no PR exists yet, ask whether to create a
  PR and a proposal-named branch.
- Then suggest continuing implementation in a fresh agent or fresh context on
  that branch.
- When implementation starts, update the PR description to include:
  - a GitHub link to the proposal directory on that branch
  - a direct GitHub link to `proposal.md` on that branch
- The description may include proposal text once implementation is underway.
- If the PR was opened during proposal writing, do not churn the description
  until implementation starts.

## `status.yaml`

- Use `status.yaml` when a proposal has active state we want to track outside
  the prose.
- `status.yaml` currently uses these fields:

```yaml
status: accepted
updated: 2026-03-23
```

- `proposal.md` remains the human-readable source of the decision.
- `status.yaml` is for current state, not for repeating the whole proposal.

Allowed `status` values:

- `proposed`
- `trial`
- `accepted`
- `rejected`
- `superseded`

When `status: superseded`, add:

```yaml
superseded_by: ../NNNN-other-proposal/proposal.md
```

`superseded_by` points to the superseding proposal.

## Decision

Use proposal directories.

Use `status.yaml` when it is useful.
