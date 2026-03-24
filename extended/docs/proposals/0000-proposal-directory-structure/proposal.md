# 0000 Proposal Directory Structure

Status: accepted

Date: 2026-03-23

## Proposal

- Each proposal gets its own directory under `extended/docs/proposals/`.
- The directory name is `NNNN-short-name`.
- `proposal.md` is required.
- `status.yaml` is required.
- `spec.md` is optional.
- `implementation_checklist.md` is optional.
- `implementation_notes.md` is optional.

## Why

- Keep one proposal's docs together.
- Avoid filename sprawl at the top level.
- Let small proposals stay small.
- Let larger proposals carry spec, checklist, and notes without making `docs/`
  messy.

## Implementation Files

- `implementation_checklist.md` is the implementation plan, written as a
  checklist.
- Write it so if an engineer is interrupted partway through, the next engineer
  can see where the previous engineer left off.
- `implementation_notes.md` is for useful implementation notes that should not
  be shoved into the proposal or spec.

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
