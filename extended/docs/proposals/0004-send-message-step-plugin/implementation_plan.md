# 0004 Implementation Plan

## Goal

- Deliver a real Slack-only `send-message` StepPlugin under
  `extended/plugins/send-message`.
- Keep that subtree self-contained enough to behave like its own Git repo.
- Do not rely on code, files, build helpers, tests, or other resources outside
  `extended/plugins/send-message` for the plugin implementation itself.
- Use the StepPlugin host slice from proposal
  [0002-kargo-executor-plugins-from-argo-workflows](../0002-kargo-executor-plugins-from-argo-workflows/proposal.md)
  without widening host responsibilities.
- Keep channel lookup and referenced Secret reads in the plugin, not the host.
- Use local-only Slack credentials for smoke testing. Do not commit any real
  token or local token source.
- Make the plugin match the documented Slack subset, including encoded-message
  behavior, except for SMTP which stays out of scope.

## Implementation Shape

- Put all plugin-owned source, manifests, tests, and smoke assets under:
  - `extended/plugins/send-message/`
- Treat that subtree as a standalone plugin repo:
  - own runtime source
  - own image build
  - own CRDs
  - own RBAC manifests
  - own tests
  - own smoke-test helpers
  - own smoke entrypoint script
- It is acceptable for repo-level smoke harness code outside that subtree to
  call into the subtree's smoke entrypoint.
- It is not acceptable for plugin runtime code inside that subtree to import or
  shell out to helpers elsewhere in this repo.

## Plugin Runtime

- Build a plugin-owned runtime image from `extended/plugins/send-message/`.
- Implement `POST /api/v1/step.execute`.
- Enforce the StepPlugin bearer-token contract:
  - read `/var/run/kargo/token`
  - require `Authorization: Bearer ...`
  - return `403` on bad auth
- Read Kubernetes credentials from the mounted service account projection when
  present.
- Use direct Kubernetes API reads from the plugin:
  - `MessageChannel` in the Project namespace
  - `ClusterMessageChannel` cluster-scoped
  - referenced `Secret` in the Project namespace or system-resources namespace
- Call Slack from the plugin runtime, not through the host.

## V1 Behavior

- Scope:
  - `send-message`
  - `MessageChannel`
  - `ClusterMessageChannel`
  - Slack only
- Do not implement SMTP.
- Support this step-config subset in v1:
  - `channel.kind`
  - `channel.name`
  - `message`
  - `encodingType`
  - `slack.channelID`
  - `slack.threadTS`
- Support this output subset in v1:
  - `slack.threadTS`
- Plaintext behavior:
  - `message` is sent as Slack `text`
  - `slack.channelID` overrides channel resource `spec.slack.channelID`
  - `slack.threadTS` is sent as Slack `thread_ts`
  - output `slack.threadTS` is `config.slack.threadTS` if set, else Slack
    response `ts`
- Encoded-message behavior:
  - support `json`, `yaml`, and `xml`
  - when `encodingType` is set, treat the body as the Slack payload object
  - ignore `config.slack.channelID` and `config.slack.threadTS`
  - if encoded body omits `channel`, fill it from channel resource
    `spec.slack.channelID`
  - if encoded body sets `thread_ts`, return that value in output
  - otherwise return Slack response `ts` in output
- XML behavior:
  - XML is a Kargo-owned alternate serialization of the same Slack payload
    object used for `json` and `yaml`
  - root element name is ignored
  - attributes become keys
  - repeated sibling element names become arrays
  - nested elements become nested objects
- `MessageChannel` lookup rules:
  - resource kind is `MessageChannel`
  - resource namespace is the Project namespace from the step context
  - `secretRef.name` resolves in the same namespace
  - `spec.slack.channelID` is required
  - Slack token key is `apiKey`
- `ClusterMessageChannel` lookup rules:
  - resource kind is `ClusterMessageChannel`
  - `secretRef.name` resolves in the system-resources namespace configured for
    the plugin install
  - `spec.slack.channelID` is required
  - Slack token key is `apiKey`

## CRDs And RBAC

- The plugin subtree owns the OSS CRD manifests for:
  - `MessageChannel`
  - `ClusterMessageChannel`
- Use the existing Akuity API group:
  - `ee.kargo.akuity.io/v1alpha1`
- Keep schemas minimal but structurally valid for the Slack-only slice.
- Ship plugin-owned RBAC manifests under the subtree.
- For the local smoke path:
  - bind only the test Project default `ServiceAccount`
  - grant channel reads and referenced Secret reads needed by the plugin
  - keep that setup in the smoke assets or smoke helper, not in host code

## Tests

- Keep plugin tests under `extended/plugins/send-message/`.
- Cover at least:
  - bearer-token auth
  - `MessageChannel` lookup
  - `ClusterMessageChannel` lookup
  - referenced Secret lookup
  - Slack request shaping for plain text
  - `slack.channelID` override
  - `slack.threadTS` override
  - `slack.threadTS` output
  - encoded-body behavior ignores `config.slack.*`
  - `xml` decode path
  - failure path when channel or Secret is missing
  - failure path when Slack returns an error

## Smoke Test

- Put the primary smoke script in:
  - `extended/plugins/send-message/smoke/smoke-test.sh`
- That script assumes Kargo is already available and takes its inputs from env.
- Keep the committed smoke path credential-free.
- Expect a local env var for the Slack API token during local smoke testing.
- Build the plugin image from `extended/plugins/send-message/`.
- Load that image into the local kind cluster without touching the user's
  global kube context.
- Install plugin CRDs, RBAC, and generated StepPlugin `ConfigMap`.
- Create either a `MessageChannel` or `ClusterMessageChannel` test resource and
  the referenced Secret in-cluster from the local-only token source.
- Run a `Stage` with `uses: send-message`.
- Prove in-cluster success from `Promotion` status, including non-empty
  `slack.threadTS`.
- For interactive local verification, also confirm the message appeared in the
  target Slack channel.
- The repo harness may call this script at the right point in e2e flow, but the
  smoke orchestration lives in the plugin subtree.

## Host Changes

- Prefer zero host changes.
- If the plugin proves a host gap, keep any host fix thin and behind
  `extended/`.
- Do not move channel lookup, Secret reads, or Slack API calls into the host.
- Host gap actually found:
  - non-root sidecars could not read `/var/run/kargo/token`
  - keep the fix thin in `extended/pkg/stepplugin/agent/command_bridge.go`
  - auth directory mode must permit traversal by non-root sidecars
  - auth file mode must permit read by non-root sidecars

## Phase Post-Green: Minimize Diff Of Files Outside ./extended Against Kargo Upstream

1. Get the feature green first.
2. Fetch `upstream/main`.
3. Review every edited file outside `extended/`, if any.
4. Move logic behind `extended/` helpers where that shrinks the conflict
   surface.
5. Re-run the matching tests after each cleanup pass.
6. Stop when no obvious helper extraction or edit-block reduction remains.
