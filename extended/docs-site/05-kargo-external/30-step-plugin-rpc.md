---
sidebar_label: RPC
description: HTTP JSON contract for StepPlugin execution.
---

# RPC Contract

StepPlugin RPC uses HTTP JSON.

## Method

Plugins implement:

```text
POST /api/v1/step.execute
```

Request type:

- `StepExecuteRequest`

Response type:

- `StepExecuteResponse`

`step.config` is normal JSON. It is not base64-encoded.

## Auth

The agent init container writes one random bearer token per plugin sidecar.

- Agent main reads:
  - `/var/run/kargo/<sidecar-container-name>/token`
- Plugin sidecar sees:
  - `/var/run/kargo/token`
- Agent request header:
  - `Authorization: Bearer <token>`

Bad auth returns:

- `403`

## Status Handling

- `200`: use the response body
- `404`: method unsupported; caller caches that
- `503`: transient; caller retries
- other `4xx/5xx`: hard failure

Controller-to-agent calls use the same path and JSON types. The auth contract
above is for agent-to-plugin sidecar calls.
