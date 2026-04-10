# 0004 Spec

## Scope

- plugin dir:
  - `extended/plugins/send-message`
- step kind:
  - `send-message`
- channel resources:
  - `MessageChannel`
  - `ClusterMessageChannel`
- transport:
  - Slack only
- out of scope:
  - SMTP

## Separate-Repo Rule

- Treat `extended/plugins/send-message` as if it were its own Git repo.
- Plugin runtime, manifests, tests, image build, and smoke assets live under
  that subtree.
- The plugin runtime must not require code, files, build helpers, or tests from
  elsewhere in `kargo-extended`.
- Repo-level harness code may call the subtree's smoke entrypoint, but the
  plugin must own the smoke procedure.

## Runtime Contract

- HTTP endpoint:
  - `POST /api/v1/step.execute`
- auth:
  - sidecar reads `/var/run/kargo/token`
  - request must send `Authorization: Bearer <token>`
  - bad or missing token returns `403`
- Kubernetes access:
  - plugin reads projected service-account credentials
  - plugin reads the Kubernetes API directly
- host responsibilities:
  - mount workdir
  - mount auth token
  - optionally mount projected service-account credentials
  - pass opaque step config
- host does not:
  - resolve channels
  - resolve referenced Secrets
  - call Slack
  - reshape encoded message bodies

## Step Config

Supported config keys:

- `channel.kind`
- `channel.name`
- `message`
- `encodingType`
- `slack.channelID`
- `slack.threadTS`

Channel reference:

- `channel.kind` must be `MessageChannel` or `ClusterMessageChannel`
- `channel.name` names the referenced channel resource

## Channel Rules

`MessageChannel`:

- namespaced
- resource namespace is the step context Project namespace
- `secretRef.name` resolves in the same namespace
- `spec.slack.channelID` is required
- Secret key is `apiKey`

`ClusterMessageChannel`:

- cluster-scoped
- `secretRef.name` resolves in `SYSTEM_RESOURCES_NAMESPACE`
- `spec.slack.channelID` is required
- Secret key is `apiKey`

## Plaintext Behavior

- `encodingType` omitted or empty means plaintext
- plaintext mode sends Slack `text = config.message`
- plaintext mode resolves `channel` in this order:
  - `config.slack.channelID`, if set
  - else `channel.spec.slack.channelID`
- plaintext mode sets `thread_ts = config.slack.threadTS` if set
- plaintext output:
  - `slack.threadTS = config.slack.threadTS` if set
  - else `slack.threadTS = Slack response ts`

Plaintext example:

```yaml
channel:
  kind: MessageChannel
  name: send-message-smoke
message: hello from plugin
slack:
  channelID: C999
  threadTS: "1700000000.000001"
```

Produces Slack payload:

```json
{
  "channel": "C999",
  "text": "hello from plugin",
  "thread_ts": "1700000000.000001"
}
```

## Encoded Message Behavior

- supported encodings:
  - `json`
  - `yaml`
  - `xml`
- encoded mode means `message` is the Slack body, not plain text
- encoded mode owns the Slack body shape
- encoded body uses Slack field names, not Kargo config names
- when `encodingType` is set:
  - ignore `config.slack.channelID`
  - ignore `config.slack.threadTS`
- if encoded body omits `channel`, fill it from `channel.spec.slack.channelID`
- encoded output:
  - `slack.threadTS = payload.thread_ts` if set
  - else `slack.threadTS = Slack response ts`

Encoded body keys may include, for example:

- `channel`
- `thread_ts`
- `text`
- `blocks`
- `icon_emoji`
- `icon_url`

JSON example:

```yaml
channel:
  kind: MessageChannel
  name: send-message-smoke
encodingType: json
message: |
  {
    "text": "rich",
    "thread_ts": "1700000000.000002",
    "blocks": [
      {
        "type": "section",
        "text": {
          "type": "mrkdwn",
          "text": "*hello*"
        }
      }
    ]
  }
slack:
  channelID: C999
  threadTS: "1700000000.000001"
```

Produces Slack payload:

```json
{
  "channel": "<channel.spec.slack.channelID>",
  "text": "rich",
  "thread_ts": "1700000000.000002",
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "*hello*"
      }
    }
  ]
}
```

The outer `slack.*` keys are ignored in this mode.
The encoded body did not set `channel`, so the plugin filled it from the
channel resource.

## XML Mapping

- There is no known Slack-standard XML format here.
- XML is a Kargo-owned alternate serialization of the same logical Slack body
  object used for `json` and `yaml`.
- The implementation mapping is:
  - root element name is ignored
  - root attributes become top-level keys
  - child element names become object keys
  - repeated sibling element names become arrays
  - leaf elements become strings
  - nested elements become nested objects
  - mixed content stores text under `#text`

Simple XML example:

```xml
<message>
  <channel>C1234567890</channel>
  <text>hello</text>
  <thread_ts>1700000000.000003</thread_ts>
</message>
```

Decodes to:

```json
{
  "channel": "C1234567890",
  "text": "hello",
  "thread_ts": "1700000000.000003"
}
```

Array/object XML example:

```xml
<message>
  <text>rich</text>
  <blocks>
    <type>section</type>
    <text>
      <type>mrkdwn</type>
      <text>*hello*</text>
    </text>
  </blocks>
  <blocks>
    <type>divider</type>
  </blocks>
</message>
```

Decodes to:

```json
{
  "text": "rich",
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "*hello*"
      }
    },
    {
      "type": "divider"
    }
  ]
}
```

## Response Contract

Success:

- status `Succeeded`
- output key `slack.threadTS`

Failure classes:

- status `Failed`:
  - Slack API rejects the request
  - Kubernetes lookup needed for execution fails after request validation
- status `Errored`:
  - request shape is invalid
  - auth is bad or missing
  - config cannot be decoded
  - required inputs are missing

## CRDs And RBAC

- API group:
  - `ee.kargo.akuity.io/v1alpha1`
- plugin-owned CRDs:
  - `MessageChannel`
  - `ClusterMessageChannel`
- plugin-owned RBAC grants:
  - read `MessageChannel`
  - read `ClusterMessageChannel`
  - read referenced `Secret`
- local smoke binds the plugin reader role only to the test Project default
  `ServiceAccount`

## Smoke Contract

- primary smoke entrypoint:
  - `extended/plugins/send-message/smoke/smoke-test.sh`
- script assumes Kargo is already available
- script requires:
  - `SEND_MESSAGE_SMOKE_PROJECT`
  - `SEND_MESSAGE_SMOKE_WAREHOUSE`
  - `SEND_MESSAGE_SMOKE_FREIGHT_NAME`
  - `SEND_MESSAGE_SMOKE_SLACK_API_KEY`
  - `SEND_MESSAGE_SMOKE_CHANNEL_ID`
- script may override:
  - `SEND_MESSAGE_SMOKE_SYSTEM_RESOURCES_NAMESPACE`
  - `SEND_MESSAGE_SMOKE_SECRET_NAME`
  - `SEND_MESSAGE_SMOKE_CHANNEL_NAME`
  - `SEND_MESSAGE_SMOKE_CONFIGMAP_NAME`
  - `KARGO_BIN`
  - `KARGO_FLAGS`
  - `KUBECTL_BIN`
  - `DOCKER_BIN`
  - `KIND_BIN`
  - `JQ_BIN`
- script behavior:
  - require a `kind-*` kube context
  - build the plugin image
  - load the image into `kind`
  - render the plugin build dir
  - build the StepPlugin discovery `ConfigMap`
  - install CRDs and RBAC
  - create test Secret and `MessageChannel`
  - create a test `Stage`
  - approve and promote the requested freight
  - wait for `Succeeded`
  - assert non-empty `slack.threadTS`
  - clean up test resources on exit
- repo harness may call this script, but this script is the plugin-owned smoke
  path

## Host Prerequisite Found During 0004

- Non-root third-party sidecars must be able to read `/var/run/kargo/token`.
- The base host slice was missing that property.
- The current host bridge for that requirement lives under
  `extended/pkg/stepplugin/agent/`.
- Current required auth mount behavior for plugin sidecars:
  - auth directory mode `0755`
  - auth file mode `0444`
