Record user decisions verbatim or with minimal editing; do not get high on your own supply.

# Extended Docs Notes

This file is a running log of user decisions for the `extended/` fork work.

Format:

- Put `YYYY-MM-DD` at the end of each decision line. - 2026-03-23
- When a decision changes, edit the existing line and bump its date. - 2026-03-23
- For key direction changes, confirm with the user before editing the line. - 2026-03-23
- Append new decisions to the end of the decisions list. - 2026-03-23

Rules:

- Prefer the user's wording. - 2026-03-23
- Tighten lightly if needed, but do not expand for its own sake. - 2026-03-23
- If a possible new decision or direction is ambiguous, ask before adding it. - 2026-03-23

Decisions:

- `kargo-extended` is a fork with a new top-level `extended/` directory. - 2026-03-23
- Keep as much fork-specific code in `extended/` as possible. - 2026-03-23
- Keep line changes outside `extended/` to a minimum. - 2026-03-23
- Current direction for `kargo-extended` is runtime-loaded StepPlugins modeled on Argo Workflows executor plugins, not compile-time plugin imports. - 2026-03-23
- Plugins are discovered at runtime from the cluster, not imported into the controller binary. - 2026-03-23
- `kargo-extension-opentofu` implements `tf-plan`, `tf-output`, and `tf-apply`. - 2026-03-23
- `kargo-extension-send-message` uses the same interface as the pro feature as closely as public docs allow. - 2026-03-23
- `kargo-extension-send-message` implements `MessageChannel` and `ClusterMessageChannel`. - 2026-03-23
- `kargo-extension-send-message` includes Slack only. - 2026-03-23
- `kargo-extension-send-message` does not include SMTP. - 2026-03-23
- Preserve the user's style: crisp is better than verbose. - 2026-03-23
- Strong preference: find a simple implementation that allows extensions to be added to `kargo-extended` without having to add them at build time, if that can be done sanely. - 2026-03-23
- Keep `extended/docs/proposals/0002-kargo-executor-plugins-from-argo-workflows/plugin-technical-hook-needs-list.md` up to date with external source-code touchpoints outside `extended/` that the extension point system needs to hook into. - 2026-03-23
- Inspect Argo Workflows executor plugins for patterns and code we can copy, because it is adjacent in the ecosystem and people already know the model. - 2026-03-23
- If we follow the Argo Workflows model, call the pluggable units "plugins" rather than "extensions", but keep `kargo-extended` as the fork name. - 2026-03-23
- We are doing this because the Argo Workflows executor plugin model is already known in the ecosystem, close to Kargo semantically, and likely less work than inventing our own plugin system from scratch. - 2026-03-23
- Call the Kargo plugin object `StepPlugin`, not `ExecutorPlugin`. - 2026-03-23
- If we are making up a new thing, use `kargo-extended.code.org` (e.g. in k8s manifests for `apiVersion:`). If we are matching an existing Akuity API, keep that API group. - 2026-03-23
- Copy Argo's build-input-to-ConfigMap approach: typed `StepPlugin` manifest in, generated labeled `ConfigMap` out, no real `StepPlugin` CRD in v1. - 2026-03-23
- When a doc depends on detail in another doc, link to the exact section and make it clear the reader should actually go read it, not use links as a lazy "details are somewhere else" escape hatch. - 2026-03-23
- In proposal md files, all file paths should be relative to the repo root. Never use absolute local disk paths. Even files outside this repo should use `../../...` style relative paths. - 2026-03-24
- When implementing a proposal, prefer a branch named like `proposal/NNNN-proposal-dir-name`; PR title should match `proposal.md`; when implementation starts, update the PR description with GitHub links to the proposal dir and `proposal.md` on that branch. If the PR was opened during proposal writing, do not churn the description until implementation starts. - 2026-03-24
- When implementation is requested and no PR exists yet, ask whether to create a PR and a proposal-named branch, then suggest continuing in a fresh agent or fresh context on that branch. - 2026-03-24
- When adding a new proposal under `extended/docs/proposals/`, read `0000-proposal-directory-structure/proposal.md` first and follow it. - 2026-03-23
- When changing a proposal's status, update both `proposal.md` and `status.yaml` if `status.yaml` exists. - 2026-03-23
- For this project, prefer killer-robot docs over architecture-theory docs: list things, keep docs KISS, stay to the point, technical examples are good, stupid things are not. - 2026-03-23
- When you write think linux kernel mailing list not Oracle Senior J2EE Architect writing corporate trashdocs. - 2026-03-23
- IMPORTANT PAY ATTENTION AGENTS WHO ARE WRITING ENGLISH: write specs, plans and other md files like: linux kernel mailing list posts, or OpenBSD man pages, or Plan 9 / Bell Labs papers and docs with SQLite exactness. - 2026-03-23
- Fork product docs for StepPlugins should live under `extended/`, be exposed to Docusaurus through a symlinked top-level `docs/docs/05-kargo-external` section, and appear before Home in the generated docs. - 2026-03-23
- Default StepPlugins on in v1; keep `STEP_PLUGINS_ENABLED` as a controller env override through the existing Helm `controller.env` escape hatch. Minimal implementation is fine. - 2026-03-24
- Name the plugin RPC wire types `StepExecuteRequest` and `StepExecuteResponse`. - 2026-03-23
- Make the StepPlugin RPC auth contract explicit: exact token path, exact `Authorization: Bearer ...` header, and `403` on bad auth. - 2026-03-23
- PRIMARY implementation goal: minimize both changed lines and non-contiguous edit blocks in files outside `extended/`. - 2026-03-24
- Minimize outside-`extended/` diff as a balance, not an absolute; do not re-implement complex upstream subsystems under `extended/` just to avoid a small upstream edit. - 2026-03-24
- Re-implementation can lower merge-conflict risk while raising compatibility-drift risk against upstream Kargo behavior. - 2026-03-24
- Prefer the smallest reasonable design, not the smallest outside diff at any cost. - 2026-03-24
- When editing outside `extended/`, prefer thin bridge edits that load logic from helper libraries under `extended/`. - 2026-03-24
- After each outside-`extended/` edit, do a follow-up pass asking how many of those edits can be removed by moving more logic behind `extended/` helpers. - 2026-03-24
- Expect each edited file outside `extended/` to have a corresponding helper or adapter in `extended/` when that keeps the outside diff smaller. - 2026-03-24
- After editing a file outside `extended/`, diff it against upstream Kargo and look for a better whole-file strategy that reduces diff size and edit-block count. - 2026-03-24
- Treat editing outside `extended/` like code golf: iterate on how little you need to change relative to upstream. - 2026-03-24
- For every feature seam that forces an edit outside `extended/`, add tests under `extended/` so future merge-conflict repairs can be validated by unit tests. - 2026-03-24
