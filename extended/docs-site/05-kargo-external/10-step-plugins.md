---
sidebar_label: Step Plugins
description: What StepPlugins are and how kargo-extended uses them.
---

# Step Plugins

StepPlugins let `kargo-extended` execute promotion steps in sidecar containers
discovered at runtime.

## Enablement

StepPlugins are on by default.

Disable them with the controller's existing `controller.env` escape hatch:

```yaml
controller:
  env:
  - name: STEP_PLUGINS_ENABLED
    value: "false"
```

## Discovery

Install a generated StepPlugin `ConfigMap` in either:

- `kargo-system-resources`
- the Project namespace

Discovery uses this label:

```yaml
kargo-extended.code.org/configmap-type: StepPlugin
```

Project namespace wins over `SYSTEM_RESOURCES_NAMESPACE` for the same plugin
name.

The controller watches labeled StepPlugin `ConfigMap`s and keeps an in-memory
registry for runtime resolution.

v1 rejects:

- StepPlugin step kinds that collide with builtin step kinds
- duplicate effective plugin step kinds after namespace precedence is applied

## Execution Model

- Builtin-only Promotions stay on the current local execution path.
- If any step uses a plugin kind, the whole Promotion runs through a per-
  Promotion agent pod.
- That pod holds one shared `emptyDir` mounted at `/workspace`.
- Builtin steps run in the agent main container.
- Plugin steps run over localhost HTTP to plugin sidecars in the same pod.

## `plugin.yaml`

Minimal `plugin.yaml`:

```yaml
apiVersion: kargo-extended.code.org/v1alpha1
kind: StepPlugin
metadata:
  name: mkdir
  namespace: kargo-system-resources
spec:
  sidecar:
    automountServiceAccountToken: false
    container:
      name: mkdir-step-plugin
      image: python:alpine3.23
      command:
      - python
      - -u
      - -c
      ports:
      - containerPort: 9765
      securityContext:
        runAsNonRoot: true
      resources:
        requests:
          cpu: 50m
          memory: 32Mi
        limits:
          cpu: 100m
          memory: 64Mi
  steps:
  - kind: mkdir
```

## Stage Usage

Use a plugin step exactly like a builtin step, but with the plugin step kind in
`uses:`:

```yaml
spec:
  promotionTemplate:
    spec:
      steps:
      - uses: mkdir
        config:
          path: demo/subdir
```
