# 0004 Implementation Notes

- Plugin subtree:
  - `extended/plugins/send-message/`
  - standalone `go.mod`
  - standalone `Dockerfile`
  - standalone `plugin.yaml`
  - standalone CRDs and RBAC under `manifests/`
  - standalone smoke helper under `smoke/`

- Runtime contract implemented in the plugin:
  - `POST /api/v1/step.execute`
  - bearer auth from `/var/run/kargo/token`
  - direct Kubernetes API reads from projected service-account credentials
  - direct Slack API call from the plugin
  - encoded `json`, `yaml`, and `xml` payload support

- V1 scope shipped:
  - `send-message`
  - `MessageChannel`
  - `ClusterMessageChannel`
  - Slack only
  - no SMTP

- Secret and channel rules implemented:
  - `secretRef.name` resolves in the Project namespace for `MessageChannel`
  - `secretRef.name` resolves in the system-resources namespace for
    `ClusterMessageChannel`
  - Slack token key is `apiKey`
  - `spec.slack.channelID` is required in CRD schema

- Smoke harness knobs:
  - `STEPPLUGIN_SEND_MESSAGE_SMOKE=true`
  - `STEPPLUGIN_SEND_MESSAGE_CHANNEL_ID=<channel-id>`
  - `STEPPLUGIN_SEND_MESSAGE_SLACK_API_KEY=<token>`
- Plugin-local smoke entrypoint:
  - `extended/plugins/send-message/smoke/smoke-test.sh`
  - repo harness calls that script rather than owning the full orchestration

- Host gap found during smoke:
  - the StepPlugin auth-token mount was not readable by non-root sidecars
  - fix was in `extended/pkg/stepplugin/agent/command_bridge.go`
  - auth directory mode is now `0755`
  - auth file mode is now `0444`

- Smoke-result note:
  - plugin response includes `output.slack.threadTS`
  - Promotion state observed in-cluster stored the value under
    `status.state.step-1.slack.threadTS`
  - the smoke harness accepts either that path or
    `status.stepExecutionMetadata[0].output.slack.threadTS`

- Encoded-message note:
  - when `encodingType` is set, `config.slack.channelID` and
    `config.slack.threadTS` are ignored
  - encoded body uses Slack-native field names such as `channel` and
    `thread_ts`
  - if encoded body omits `channel`, plugin fills it from channel resource
    `spec.slack.channelID`

- XML note:
  - no public exact upstream XML example was found
  - this implementation treats XML as a Kargo-owned alternate serialization of
    the same Slack payload object used for `json` and `yaml`
  - root name is ignored
  - repeated sibling names become arrays

- Non-plugin repo fix found during smoke:
  - `pkg/cli/cmd/promote/promote.go` assumed wrapped REST payloads
  - local smoke showed direct-object and direct-list payloads
  - decoder helpers now accept both shapes
