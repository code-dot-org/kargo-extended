# Hardcoded Build Spec

## Overview

1. `kargo-extended`: a fork with a new `extended/` directory that keeps as
much code there as possible, with as few line changes outside that directory
as possible to support needed features, and provides the ability to write the
extension points we want outside the repo.
2. `kargo-extension-opentofu`: implements `tf-plan`, `tf-output`, and
`tf-apply`, and uses `kargo-extended` extension points to be loaded.
3. `kargo-extension-send-message`: uses the same interface as the pro feature,
implements `MessageChannel` and `ClusterMessageChannel` CRDs, and contains a
simple baked-into-repo implementation of the Slack feature, without SMTP,
mimicking the pro feature in every way we know how from public docs.

## Rules

- Build-time composition only.
- No runtime loading.
- No Go `plugin`.
- No `LD_PRELOAD`.
- The final binary imports the chosen repos.
- Keep that import point in one place under `extended/`.
- Keep changes outside `extended/` to the minimum needed to call into the host.

## Tree

```text
extended/
  host/
  cmd/kargo-extended/
  charts/
  docs/
```

`extended/host/`: host hooks and shared helpers

`extended/cmd/kargo-extended/`: final binary composition point

`extended/charts/`: fork-owned manifests

`extended/docs/`: fork docs

## Host Surface

- Step runner registration.
- Optional scheme registration for typed CRD clients.
- Shared helper APIs only if a repo needs them.

Step runners are the main hook. That covers OpenTofu and Slack-backed
`send-message`.

Extensions may use unstructured or dynamic clients and skip scheme
registration.

Extension-owned CRDs do not need to live in this repo.

## Repo Scope

`kargo-extension-opentofu`: `tf-plan`, `tf-output`, `tf-apply`

`kargo-extension-send-message`:
- `send-message`
- `MessageChannel`
- `ClusterMessageChannel`
- Slack only
- no SMTP
- own API group, not `ee.kargo.akuity.io`

`kargo-extension-send-message` also owns:
- CRDs
- RBAC
- secret ref semantics
- cluster/project scope rules

## Build

- `kargo-extended` provides the host hooks.
- `extended/cmd/kargo-extended/` imports the selected repos.
- No other build path matters for this spec.
