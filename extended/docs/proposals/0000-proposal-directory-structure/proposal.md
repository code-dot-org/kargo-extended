# 0000 Proposal Directory Structure

Status: accepted

Date: 2026-03-23

## Proposal

- Each proposal gets its own directory under `extended/docs/proposals/`.
- The directory name is `NNNN-short-name`.
- `proposal.md` is required.
- `spec.md` is optional.
- `plan.md` is optional.
- `notes.md` is optional.
- `status.yaml` is optional.

## Why

- Keep one proposal's docs together.
- Avoid filename sprawl at the top level.
- Let small proposals stay small.
- Let larger proposals carry spec, plan, and notes without making `docs/`
  messy.

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
