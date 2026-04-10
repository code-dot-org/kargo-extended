# 0004 Ruby Checklist

## Baseline

- [x] Plugin work lives under `extended/plugins/send-message-ruby/`.
- [x] The subtree stands alone.
- [x] No committed Slack credential appears anywhere.
- [x] The plugin owns Kubernetes reads and Slack calls.

## Phase 0: pin the contract

- [x] Re-read `proposal.md`.
- [x] Re-read `spec.md`.
- [x] Re-read the `send-message` slice in proposal `0002`.
- [x] Keep scope to Slack only.

## Phase 1: create the tiny repo

- [x] Create `extended/plugins/send-message-ruby/`.
- [x] Add runtime source.
- [x] Add image build.
- [x] Add `plugin.yaml`.
- [x] Add CRD manifests.
- [x] Add RBAC manifests.
- [x] Add smoke assets.

## Phase 2: implement the runtime

- [x] Implement `POST /api/v1/step.execute`.
- [x] Enforce bearer auth from `/var/run/kargo/token`.
- [x] Read `MessageChannel`.
- [x] Read `ClusterMessageChannel`.
- [x] Read referenced `Secret`.
- [x] Send plaintext Slack payloads.
- [x] Send encoded Slack payloads.
- [x] Return `slack.threadTS`.

## Phase 3: test the contract

- [x] Add auth tests.
- [x] Add channel lookup tests.
- [x] Add Secret lookup tests.
- [x] Add plaintext payload tests.
- [x] Add encoded payload tests.
- [x] Add XML decode tests.
- [x] Add Slack failure tests.

## Phase 4: smoke

- [x] Add plugin-owned `smoke/smoke_test.rb`.
- [x] Keep smoke orchestration in Ruby, not shell.
- [x] Build the image.
- [x] Load it into kind.
- [x] Install CRDs and RBAC.
- [x] Install StepPlugin `ConfigMap`.
- [x] Create local-only test Secret.
- [x] Create test `MessageChannel`.
- [x] Run a `Stage` with `uses: send-message`.
- [x] Assert `Succeeded`.
- [x] Assert non-empty `slack.threadTS`.

## Phase 5: mandatory radical simplification pass 1

- [x] Ask "can I replace this gem with stdlib and make the repo smaller?"
- [x] Ask "can I collapse this into fewer files and still keep it readable?"
- [x] Ask "am I carrying Ruby framework habits into a tiny sidecar?"
- [x] Delete anything that fails those checks.

## Phase 6: mandatory radical simplification pass 2

- [x] Re-run tests and smoke from a green tree.
- [x] Ask "can I make this radically simpler?"
- [x] Remove wrappers that only save a few obvious lines.
- [x] Remove folder structure that makes the plugin look more official than
      small.
- [x] Stop only when the repo still reads like a small third-party plugin.
