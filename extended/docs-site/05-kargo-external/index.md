---
sidebar_label: Overview
description: Runtime-loaded StepPlugins in kargo-extended.
---

# Kargo External

`kargo-extended` adds runtime-loaded StepPlugins.

- A StepPlugin is an out-of-process step executor.
- It is installed as a labeled `ConfigMap`, not a served CRD, in v1.
- The host discovers StepPlugins at runtime from the Project namespace and
  `SYSTEM_RESOURCES_NAMESPACE`.
- If both namespaces define the same plugin name, the Project namespace wins.

Read these next:

- [Step Plugins](./10-step-plugins.md)
- [Build A StepPlugin](./20-step-plugin-build.md)
- [RPC Contract](./30-step-plugin-rpc.md)
