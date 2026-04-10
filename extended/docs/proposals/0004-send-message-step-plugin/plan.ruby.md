# 0004 Ruby Plan

## Goal

- Show the same `send-message` plugin contract in a form that is tiny,
  readable, and comfortable for a scripting-language user.
- Keep all plugin-owned code under `extended/plugins/send-message-ruby/`.
- Keep the subtree standalone enough to behave like its own Git repo.
- Match the same Slack-only contract as `spec.md`.

## Design Rule

- Prefer stdlib unless a gem removes real complexity.
- Prefer direct HTTPS to Kubernetes and Slack if that keeps the repo smaller.
- Do not chase a Ruby-flavored framework stack.
- If a gem exists only to save a small amount of obvious code, do not use it.

## Runtime Shape

- Runtime:
  - Ruby
- Suggested layout:
  - `extended/plugins/send-message-ruby/`
  - `server.rb`
  - `smoke/smoke_test.rb`
  - `Gemfile`
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

- Strong preference:
  - stdlib `webrick`
  - stdlib `json`
  - stdlib `psych`
  - stdlib `rexml`
  - stdlib `net/http`
- Avoid:
  - Rails-adjacent stacks
  - Sinatra unless it clearly removes code
  - large Kubernetes or Slack client wrappers

## Behavior

- Implement `POST /api/v1/step.execute`.
- Enforce bearer auth from `/var/run/kargo/token`.
- Read `MessageChannel` and `ClusterMessageChannel` directly from Kubernetes.
- Read referenced `Secret` directly from Kubernetes.
- Send Slack messages directly or through a tiny client if one clearly helps.
- Support:
  - plaintext
  - `json`
  - `yaml`
  - `xml`
- Match the response contract in `spec.md`.

## Tests

- Keep tests inside the subtree.
- Prefer a small test file and direct fixtures.
- Cover:
  - auth
  - channel lookup
  - Secret lookup
  - plaintext payload shaping
  - encoded payload shaping
  - XML decode shape
  - Slack error handling

## Smoke

- Own `smoke/smoke_test.rb` inside the subtree.
- Assume Kargo already exists.
- Use Ruby for the smoke orchestration too.
- Build image, install manifests, create Secret and channel, run a Stage,
  assert `Succeeded`, assert non-empty `slack.threadTS`.

## Mandatory Simplify Passes

- Simplify pass 1, before full smoke:
  - ask "can I replace this gem with stdlib and make the repo smaller?"
  - ask "can I collapse this into fewer files?"
- Simplify pass 2, after green:
  - ask "can I make this radically simpler?"
  - remove any Ruby structure that exists to feel proper rather than to keep
    the plugin clear
