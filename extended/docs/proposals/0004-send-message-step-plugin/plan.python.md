# 0004 Python Plan

## Goal

- Show the same `send-message` plugin contract in a form that looks easy to
  write and easy to own.
- Keep all plugin-owned code under `extended/plugins/send-message-python/`.
- Keep the subtree standalone enough to behave like its own Git repo.
- Match the same Slack-only contract as `spec.md`.

## Design Rule

- Prefer directness over framework taste.
- Prefer a tiny dependency set over "proper" stacks.
- Prefer raw Kubernetes API reads over a large client library when that keeps
  the repo smaller and clearer.
- Prefer the first-party Slack Python SDK only if it reduces code materially.
- If a dependency does not make the plugin look simpler to a third party,
  delete it.

## Runtime Shape

- Runtime:
  - Python
- Suggested layout:
  - `extended/plugins/send-message-python/`
  - `server.py`
  - `smoke/smoke_test.py`
  - `requirements.txt`
  - `Dockerfile`
  - `plugin.yaml`
  - `manifests/`
  - `smoke/`
- Suggested server shape:
  - one small HTTP server
  - one request parser
  - one Kubernetes reader
  - one Slack sender

## Minimal Dependency Target

- Acceptable:
  - `slack_sdk`
  - `PyYAML`
- Strong preference:
  - stdlib HTTP server
  - stdlib `json`
  - stdlib `xml.etree.ElementTree`
  - direct HTTPS to Kubernetes
- Avoid:
  - large web frameworks
  - large Kubernetes client stacks
  - background workers
  - async machinery unless it removes code

## Behavior

- Implement `POST /api/v1/step.execute`.
- Enforce bearer auth from `/var/run/kargo/token`.
- Read `MessageChannel` and `ClusterMessageChannel` directly from Kubernetes.
- Read referenced `Secret` directly from Kubernetes.
- Send Slack messages from the plugin.
- Support:
  - plaintext
  - `json`
  - `yaml`
  - `xml`
- Match the response contract in `spec.md`.

## Tests

- Keep tests inside the subtree.
- Prefer a small unit-test suite over a heavy harness.
- Cover:
  - auth
  - channel lookup
  - Secret lookup
  - plaintext payload shaping
  - encoded payload shaping
  - XML decode shape
  - Slack error handling

## Smoke

- Own `smoke/smoke_test.py` inside the subtree.
- Assume Kargo already exists.
- Use Python for the smoke orchestration too.
- Build image, install manifests, create Secret and channel, run a Stage,
  assert `Succeeded`, assert non-empty `slack.threadTS`.

## Mandatory Simplify Passes

- Simplify pass 1, before full smoke:
  - ask "can I delete a dependency and still keep this clearer?"
  - ask "can I collapse this into fewer files without hiding the contract?"
- Simplify pass 2, after green:
  - ask "can I make this radically simpler?"
  - remove any helper, abstraction, or library that does not make the plugin
    look easier to write

## Current Implementation Notes

- Runtime now lives in a small package:
  - `extended/plugins/send-message-python/send_message_plugin/`
- Smoke orchestration lives in:
  - `extended/plugins/send-message-python/smoke/smoke_test.py`
  - `extended/plugins/send-message-python/smoke/lib.py`
- Keep the dependency set to:
  - stdlib
  - `PyYAML`
- Do not add:
  - Slack SDK
  - Kubernetes Python client
  - web framework
- Local validation currently proves:
  - unit tests pass
  - Python sources compile
  - Docker image builds
  - isolated kind smoke passes through `extended/tests/e2e_stepplugins.sh`
