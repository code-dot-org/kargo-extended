# 0002 Implementation Notes

- Host-side StepPlugin code lives under `extended/pkg/stepplugin/`.
- Argo-shaped copied or adapted helpers live under
  `extended/pkg/argoworkflows/`.
- The public CLI command is `kargo step-plugin build DIR`.
- The controller binary now includes `promotion-agent`.
- Plugin-backed Promotions use a per-Promotion agent pod named
  `promotion-agent-<promotion-uid>`.
- The agent pod main container runs builtin steps locally and plugin steps over
  localhost HTTP.
- The agent pod clones the current controller pod's main container env,
  envFrom, image, and mounted volumes, then adds:
  - shared workdir `emptyDir`
  - shared token `emptyDir`
  - explicit service account projected volume
- Plugin sidecars get only:
  - their declared container spec
  - `/workspace`
  - `/var/run/kargo`
  - optional service account projection
- Discovery currently resolves plugin `ConfigMap`s on demand through the
  controller client. It does not yet maintain a watched in-memory registry.
