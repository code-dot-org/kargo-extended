# 0004 Implementation Checklist

Update this as implementation teaches us things.

## Baseline

- [x] Plugin work lives under `extended/plugins/send-message`.
- [x] The plugin subtree does not rely on repo resources outside itself.
- [x] Slack credentials stay local-only and out of git.
- [x] The host does not own channel lookup, Secret reads, or Slack API calls.

## Phase 0: read and pin the seams

- [x] Read proposal 0004 again before starting code.
- [x] Re-read the `send-message` section in proposal 0002 spec.
- [x] Read the current StepPlugin host runtime and e2e harness seams.
- [x] Record any host-side gap found during that read.

## Phase 1: create the standalone subtree

- [x] Create `extended/plugins/send-message/`.
- [x] Add plugin-owned runtime source there.
- [x] Add plugin-owned image build there.
- [x] Add plugin-owned tests there.
- [x] Add plugin-owned CRD manifests there.
- [x] Add plugin-owned RBAC manifests there.
- [x] Add plugin-owned smoke assets or smoke helper there.

## Phase 2: implement the runtime

- [x] Implement `POST /api/v1/step.execute`.
- [x] Enforce bearer-token auth from `/var/run/kargo/token`.
- [x] Return `403` on bad auth.
- [x] Reject unsupported methods cleanly.
- [x] Parse the v1 step-config subset.
- [x] Resolve `MessageChannel` from the Project namespace.
- [x] Resolve `ClusterMessageChannel` from the system-resources namespace.
- [x] Resolve referenced Secrets from the correct namespace.
- [x] Read Slack token from Secret key `apiKey`.
- [x] Send the Slack message from the plugin.
- [x] Return `slack.threadTS` output.

## Phase 3: own the OSS CRDs and RBAC

- [x] Add Slack-only `MessageChannel` CRD manifest.
- [x] Add Slack-only `ClusterMessageChannel` CRD manifest.
- [x] Keep the API group `ee.kargo.akuity.io/v1alpha1`.
- [x] Add plugin-owned RBAC manifests for channel and Secret reads.
- [x] Keep RBAC setup out of host code unless a real host gap is proven.

## Phase 4: tests inside the subtree

- [x] Add runtime tests for auth.
- [x] Add runtime tests for namespaced channel lookup.
- [x] Add runtime tests for cluster-scoped channel lookup.
- [x] Add runtime tests for referenced Secret lookup.
- [x] Add runtime tests for Slack request shaping.
- [x] Add runtime tests for `slack.channelID` override.
- [x] Add runtime tests for `slack.threadTS` override and output.
- [x] Add runtime tests for missing channel or Secret failures.
- [x] Add runtime tests for Slack API failure handling.

## Phase 5: local smoke path

- [x] Build the plugin image from `extended/plugins/send-message`.
- [x] Load the image into the local kind cluster.
- [x] Keep kube access on an isolated `KUBECONFIG`.
- [x] Keep the user's existing kube context untouched.
- [x] Install plugin CRDs and RBAC.
- [x] Generate and install the StepPlugin `ConfigMap`.
- [x] Inject a local-only Slack token into a cluster `Secret`.
- [x] Create a test `MessageChannel` or `ClusterMessageChannel`.
- [x] Run a `Stage` with `uses: send-message`.
- [x] Prove promotion success in-cluster.
- [x] Prove `slack.threadTS` output is populated.
- [x] Verify the message appeared in the target Slack channel.

## Phase 6: repo harness integration

- [x] Extend `extended/tests/e2e_stepplugins.sh` to support the real
      `send-message` smoke path.
- [x] Keep the committed harness credential-free.
- [x] Gate Slack smoke on a local env var for the token.
- [x] Keep any non-`extended/` edit to a tiny hook only, if one is needed.

## Phase Post-Green: Minimize Diff Of Files Outside ./extended Against Kargo Upstream

- [x] Fetch `upstream`.
- [x] Review every edited file outside `extended/`, if any, against
      `upstream/main`.
- [x] Move more logic behind `extended/` helpers if that shrinks the outside
      diff safely.
- [x] Re-run matching tests after each cleanup pass.
- [x] Stop only when no obvious outside-`extended/` shrink remains.
