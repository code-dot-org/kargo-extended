---
sidebar_label: Build
description: Build a StepPlugin authoring manifest into the discovery ConfigMap.
---

# Build A StepPlugin

Build a StepPlugin from a directory containing:

- `plugin.yaml`
- optional single `server.*`

Run:

```bash
kargo step-plugin build .
```

This writes:

- `<name>-step-plugin-configmap.yaml`
- `README.md`

## `server.*` Embedding

If the plugin directory contains a single `server.*` file, `kargo step-plugin
build` reads that file and stores its contents in
`spec.sidecar.container.args[0]`.

This matches the Argo Workflows executor-plugin build flow.

## Generated `ConfigMap`

The generated `ConfigMap`:

- keeps `metadata.name` and `metadata.namespace` from `plugin.yaml`
- adds the discovery label
- stores the sidecar container spec in `data["sidecar.container"]`
- stores plugin step metadata in `data["steps.yaml"]`

Once applied, the controller's StepPlugin watcher picks it up from either
`kargo-system-resources` or the Project namespace.

Example install:

```bash
kubectl apply -f mkdir-step-plugin-configmap.yaml
```
