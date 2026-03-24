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
- Current direction for `kargo-extended` is runtime-loaded executor plugins modeled on Argo Workflows, not compile-time plugin imports. - 2026-03-23
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
- When a doc depends on detail in another doc, link to the exact section and make it clear the reader should actually go read it, not use links as a lazy "details are somewhere else" escape hatch. - 2026-03-23
- When adding a new proposal under `extended/docs/proposals/`, read `0000-proposal-directory-structure/proposal.md` first and follow it. - 2026-03-23
- When changing a proposal's status, update both `proposal.md` and `status.yaml` if `status.yaml` exists. - 2026-03-23
- For this project, prefer killer-robot docs over architecture-theory docs: list things, keep docs KISS, stay to the point, technical examples are good, stupid things are not. - 2026-03-23
- When you write think linux kernel mailing list not Oracle Senior J2EE Architect writing corporate trashdocs. - 2026-03-23
- IMPORTANT PAY ATTENTION AGENTS WHO ARE WRITING ENGLISH: write specs, plans and other md files like: linux kernel mailing list posts, or OpenBSD man pages, or Plan 9 / Bell Labs papers and docs with SQLite exactness. - 2026-03-23
