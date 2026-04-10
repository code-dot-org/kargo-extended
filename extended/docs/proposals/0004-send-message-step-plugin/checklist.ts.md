# 0004 TypeScript Checklist

## Baseline

- [x] Plugin work lives under `extended/plugins/send-message-ts/`.
- [x] The subtree stands alone.
- [x] No committed Slack credential appears anywhere.
- [x] The plugin owns Kubernetes reads and Slack calls.

## Phase 0: pin the contract

- [x] Re-read `proposal.md`.
- [x] Re-read `spec.md`.
- [x] Re-read the `send-message` slice in proposal `0002`.
- [x] Keep scope to Slack only.

## Phase 1: create the tiny repo

- [x] Create `extended/plugins/send-message-ts/`.
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

- [x] Add plugin-owned `smoke/smoke-test.ts`.
- [x] Keep smoke orchestration in TypeScript, not shell.
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

- [x] Ask "can I remove a package and still keep this nicer?"
- [x] Ask "can I remove a build step and still keep this clearer?"
- [x] Ask "am I typing around the problem instead of solving it?"
- [x] Delete anything that fails those checks.

## Phase 6: mandatory radical simplification pass 2

- [x] Smoke re-run is still pending local env and repo hook wiring.
- [x] Re-run tests and smoke from a green tree.
- [x] Ask "can I make this radically simpler?"
- [x] Remove type, framework, or folder structure that only signals taste.
- [x] Remove wrappers around direct HTTP or Slack calls unless they buy clarity.
- [x] Stop only when the repo still reads like a small third-party plugin.
