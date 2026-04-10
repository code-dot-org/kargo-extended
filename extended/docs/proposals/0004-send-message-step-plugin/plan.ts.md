# 0004 TypeScript Plan

## Goal

- Show the same `send-message` plugin contract in a form that benefits from
  the strongest Slack SDK support.
- Keep all plugin-owned code under `extended/plugins/send-message-ts/`.
- Keep the subtree standalone enough to behave like its own Git repo.
- Match the same Slack-only contract as `spec.md`.

## Design Rule

- Prefer the smallest runtime that still looks natural to a TypeScript user.
- Use Slack's first-party Node SDK if it reduces code and ambiguity.
- Do not pull in a Kubernetes client unless it is clearly smaller than direct
  HTTPS calls.
- If typing machinery becomes louder than the plugin logic, cut it back.

## Runtime Shape

- Runtime:
  - TypeScript on Node
- Suggested layout:
  - `extended/plugins/send-message-ts/`
  - `src/server.ts`
  - `smoke/smoke-test.ts`
  - `package.json`
  - `tsconfig.json`
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
  - `@slack/web-api`
  - `yaml`
  - `fast-xml-parser`
- Strong preference:
  - built-in Node HTTP server
  - built-in `fetch`
  - direct HTTPS to Kubernetes
- Avoid:
  - Express, Nest, or similar frameworks
  - generated API clients
  - build chains that are bigger than the plugin

## Behavior

- Implement `POST /api/v1/step.execute`.
- Enforce bearer auth from `/var/run/kargo/token`.
- Read `MessageChannel` and `ClusterMessageChannel` directly from Kubernetes.
- Read referenced `Secret` directly from Kubernetes.
- Send Slack messages with the Slack Web API client or direct HTTP if smaller.
- Support:
  - plaintext
  - `json`
  - `yaml`
  - `xml`
- Match the response contract in `spec.md`.

## Tests

- Keep tests inside the subtree.
- Prefer a small test runner and direct fixtures.
- Cover:
  - auth
  - channel lookup
  - Secret lookup
  - plaintext payload shaping
  - encoded payload shaping
  - XML decode shape
  - Slack error handling

## Smoke

- Own `smoke/smoke-test.ts` inside the subtree.
- Assume Kargo already exists.
- Use TypeScript for the smoke orchestration too.
- Build image, install manifests, create Secret and channel, run a Stage,
  assert `Succeeded`, assert non-empty `slack.threadTS`.

## Mandatory Simplify Passes

- Simplify pass 1, before full smoke:
  - ask "can I delete a package and keep the code clearer?"
  - ask "can I shrink the build chain?"
- Simplify pass 2, after green:
  - ask "can I make this radically simpler?"
  - remove type or framework structure that makes the plugin look harder than
    it is
