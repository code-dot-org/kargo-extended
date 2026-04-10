# 0004 Python Checklist

## Baseline

- [x] Plugin work lives under `extended/plugins/send-message-python/`.
- [x] The subtree stands alone.
- [x] No committed Slack credential appears anywhere.
- [x] The plugin owns Kubernetes reads and Slack calls.

## Phase 0: pin the contract

- [x] Re-read `proposal.md`.
- [x] Re-read `spec.md`.
- [x] Re-read the `send-message` slice in proposal `0002`.
- [x] Keep scope to Slack only.

## Phase 1: create the tiny repo

- [x] Create `extended/plugins/send-message-python/`.
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

- [x] Add plugin-owned `smoke/smoke_test.py`.
- [x] Keep smoke orchestration in Python, not shell.
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

- [x] Ask "can I make this look easier by deleting a dependency?"
- [x] Ask "can I merge files without making the contract harder to read?"
- [x] Ask "am I using a framework just because it is familiar?"
- [x] Delete anything that fails those checks.

## Phase 6: mandatory radical simplification pass 2

- [x] Re-run tests and smoke from a green tree.
- [x] Ask "can I make this radically simpler?"
- [x] Remove abstractions that only serve style.
- [x] Remove helpers that only save a few lines.
- [x] Stop only when the repo still reads like a small third-party plugin.

Refactor pass notes:
- Split the runtime into small package files for:
  - app flow
  - HTTP entrypoint
  - Kubernetes and Slack clients
  - payload decoding and shaping
- Split smoke support out of the entrypoint into `smoke/lib.py`.
- Re-ran unit tests after refactor.
- Re-ran `py_compile` across all Python files after refactor.
- Re-ran isolated kind smoke through `extended/tests/e2e_stepplugins.sh`.
