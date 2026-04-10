# Upstream Kargo merge procedure

When merging latest upstream Kargo into this fork, use one dated branch, one
YAML report, and one pull request.

Branch name:

```text
upstream-kargo-merge/YYYY-MM-DD
```

Report path:

```text
extended/docs/upstream-kargo-merge-reports/YYYY-MM-DD.yaml
```

The report is mandatory, even if the merge has no conflicts.

If the merge had conflicts, the pull request must also carry inline review
comments on the resolved diff ranges. The YAML report is the durable record.
The PR comments are the fast path for review.

## Procedure

1. Start from clean fork `main`.

   ```bash
   date="$(date +%F)"
   branch="upstream-kargo-merge/$date"
   report="extended/docs/upstream-kargo-merge-reports/$date.yaml"

   git fetch origin upstream --tags
   git switch main
   git pull --ff-only origin main
   git switch -c "$branch"

   fork_base_commit="$(git rev-parse HEAD)"
   upstream_commit="$(git rev-parse upstream/main)"

   git merge --no-ff --no-commit "$upstream_commit" || true
   ```

1. If there are conflicts, capture the file list before resolving anything.

   ```bash
   git diff --name-only --diff-filter=U | sort
   ```

1. Create the YAML report immediately and keep it current while resolving the
   merge.

   Use this shape exactly:

   ```yaml
   date: YYYY-MM-DD
   merge_branch: upstream-kargo-merge/YYYY-MM-DD
   target_branch: main
   target_remote: origin
   source_remote: upstream
   source_branch: main
   fork_base_commit: <sha>
   upstream_commit: <sha>
   conflicted_files: []
   conflicts: []
   post_merge_adjustments: []
   validation:
     make_test_unit: pending
     make_lint: pending
     make_build_cli_with_ui: pending
     go_build_cmd_controlplane: pending
     git_diff_check: pending
     stepplugins_only_e2e: skipped
   notes: []
   ```

   If the merge had no conflicts, leave:

   ```yaml
   conflicted_files: []
   conflicts: []
   ```

   Each `conflicts` entry has this shape:

   ```yaml
   - path: pkg/foo/bar.go
     area: outside-extended
     kind: content
     fork_change: |
       What this fork changed before the merge.
     upstream_change: |
       What upstream changed in the same area.
     resolution: combined
     pr_comment_status: pending
     notes: |
       Why this resolution is correct.
   ```

   `area` is one of `extended`, `outside-extended`, `generated`, `docs`,
   `other`.

   `kind` is one of `content`, `modify/delete`, `delete/modify`,
   `rename/rename`, `other`.

   `resolution` is one of `keep-fork`, `take-upstream`, `combined`, `custom`.

   `pr_comment_status` is one of `pending`, `added`, `not-needed`.

1. Resolve the merge.

   Fork rule:

   - Keep fork-specific logic under `extended/`.
   - For files outside `extended/`, keep the local diff as small as possible.

1. If the merge changed generated inputs, run:

   ```bash
   make codegen
   ```

1. Run validation. All required checks must pass before commit.

   ```bash
   make test-unit
   make lint
   make build-cli-with-ui
   go build ./cmd/controlplane
   git diff --check
   ```

   If the merge touched `extended/`, `pkg/cli/tests/e2e.sh`, StepPlugin wiring,
   or thin bridge code that loads logic from `extended/`, also run this if a
   local kind/Tilt environment is already up:

   ```bash
   STEPPLUGINS_ONLY=true ./pkg/cli/tests/e2e.sh
   ```

   Update the report after each command to `pass`, `fail`, or `skipped`.

1. Commit the merge and the report together.

   ```bash
   git add -A
   git commit -s -m "Merge upstream Kargo $date"
   ```

1. Push the branch and open the pull request.

   ```bash
   git push -u origin "$branch"
   gh pr create \
     --repo code-dot-org/kargo-extended \
     --base main \
     --head "$branch" \
     --title "Merge upstream Kargo $date" \
     --body "Merges upstream Kargo commit \`$upstream_commit\` into our fork.
   
   Merge report: \`$report\`."
   gh pr view --repo code-dot-org/kargo-extended --web
   ```

1. If the merge had conflicts, add inline comments on the PR in
   <hlt>Files changed</hlt>.

   Add one comment per resolved conflict range. Put the comment on the final
   resolved lines, not on the whole file.

   Use this shape:

   ```text
   Upstream had:
   <short summary>

   Our fork had:
   <short summary>

   Resolution:
   <kept fork | took upstream | combined | custom>

   Why:
   <short reason>
   ```

   Keep each comment tight. Do not paste whole hunks. Summarize the behavior.
   After adding the comment, update that conflict entry in the YAML report from
   `pr_comment_status: pending` to `pr_comment_status: added`.

   If the merge had no conflicts, do not add PR inline comments for conflict
   resolution.

## Why the report exists

- It records which files conflict repeatedly.
- It separates `extended/` conflicts from outside-`extended/` conflicts.
- It leaves a trail of how each conflict was resolved.
