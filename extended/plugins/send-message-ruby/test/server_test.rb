require "json"
require "logger"
require "minitest/autorun"
require "tmpdir"

require_relative "../lib/send_message_plugin"

class SendMessagePluginTest < Minitest::Test
  def test_rejects_missing_bearer_token
    app = build_app

    status, response = app.call(
      method: "POST",
      path: SendMessagePlugin::STEP_EXECUTE_PATH,
      headers: {},
      body: JSON.generate(minimal_request)
    )

    assert_equal 403, status
    assert_equal "Errored", response["status"]
    assert_equal "missing bearer token", response["error"]
  end

  def test_rejects_invalid_bearer_token
    app = build_app

    status, response = app.call(
      method: "POST",
      path: SendMessagePlugin::STEP_EXECUTE_PATH,
      headers: { "Authorization" => "Bearer wrong-token" },
      body: JSON.generate(minimal_request)
    )

    assert_equal 403, status
    assert_equal "Errored", response["status"]
    assert_equal "invalid bearer token", response["error"]
  end

  def test_rejects_invalid_request_body
    app = build_app

    status, response = app.call(
      method: "POST",
      path: SendMessagePlugin::STEP_EXECUTE_PATH,
      headers: { "Authorization" => "Bearer expected-token" },
      body: "{not-json"
    )

    assert_equal 400, status
    assert_equal "Errored", response["status"]
    assert_equal "invalid request body", response["message"]
  end

  def test_uses_namespaced_message_channel
    kube = FakeKubernetesClient.new(
      message_channels: {
        "demo/send" => {
          "secret_name" => "slack-token",
          "slack_channel_id" => "C123",
          "resource_kind" => "MessageChannel",
          "resource_name" => "send"
        }
      },
      secrets: {
        "demo/slack-token" => { "apiKey" => "xoxb-demo" }
      }
    )
    slack = FakeSlackClient.new(
      "ok" => true,
      "ts" => "1712345678.000100"
    )
    app = build_app(kube: kube, slack: slack)

    request = minimal_request
    request["context"] = { "project" => "demo" }
    response = execute_request(app, request)

    assert_equal "Succeeded", response["status"]
    assert_equal(
      {
        "channel" => "C123",
        "text" => "hello from plugin"
      },
      slack.last_payload
    )
    assert_equal(
      "1712345678.000100",
      response.dig("output", "slack", "threadTS")
    )
  end

  def test_uses_cluster_message_channel
    kube = FakeKubernetesClient.new(
      cluster_message_channels: {
        "send" => {
          "secret_name" => "slack-token",
          "slack_channel_id" => "C777",
          "resource_kind" => "ClusterMessageChannel",
          "resource_name" => "send"
        }
      },
      secrets: {
        "kargo-system-resources/slack-token" => { "apiKey" => "xoxb-demo" }
      }
    )
    slack = FakeSlackClient.new(
      "ok" => true,
      "ts" => "1712345678.000200"
    )
    app = build_app(kube: kube, slack: slack)

    request = minimal_request
    request["context"] = { "project" => "demo" }
    request["step"]["config"]["channel"] = {
      "kind" => "ClusterMessageChannel",
      "name" => "send"
    }
    response = execute_request(app, request)

    assert_equal "Succeeded", response["status"]
    assert_equal(
      {
        "channel" => "C777",
        "text" => "hello from plugin"
      },
      slack.last_payload
    )
  end

  def test_plaintext_honors_slack_overrides
    kube = standard_kube
    slack = FakeSlackClient.new(
      "ok" => true,
      "ts" => "1712345678.000300"
    )
    app = build_app(kube: kube, slack: slack)

    request = minimal_request
    request["context"] = { "project" => "demo" }
    request["step"]["config"]["slack"] = {
      "channelID" => "C999",
      "threadTS" => "1700000000.000001"
    }
    response = execute_request(app, request)

    assert_equal "Succeeded", response["status"]
    assert_equal(
      {
        "channel" => "C999",
        "text" => "hello from plugin",
        "thread_ts" => "1700000000.000001"
      },
      slack.last_payload
    )
    assert_equal(
      "1700000000.000001",
      response.dig("output", "slack", "threadTS")
    )
  end

  def test_supports_json_payloads
    slack = FakeSlackClient.new(
      "ok" => true,
      "ts" => "1712345678.000400"
    )
    app = build_app(kube: standard_kube, slack: slack)

    request = minimal_request
    request["context"] = { "project" => "demo" }
    request["step"]["config"]["encodingType"] = "json"
    request["step"]["config"]["message"] = <<~JSON
      {"text":"rich","blocks":[{"type":"section"}]}
    JSON
    response = execute_request(app, request)

    assert_equal "Succeeded", response["status"]
    assert_equal(
      {
        "channel" => "C123",
        "text" => "rich",
        "blocks" => [
          { "type" => "section" }
        ]
      },
      slack.last_payload
    )
  end

  def test_supports_yaml_payloads
    slack = FakeSlackClient.new(
      "ok" => true,
      "ts" => "1712345678.000410"
    )
    app = build_app(kube: standard_kube, slack: slack)

    request = minimal_request
    request["context"] = { "project" => "demo" }
    request["step"]["config"]["encodingType"] = "yaml"
    request["step"]["config"]["message"] = <<~YAML
      text: rich
      blocks:
      - type: section
    YAML
    response = execute_request(app, request)

    assert_equal "Succeeded", response["status"]
    assert_equal(
      {
        "channel" => "C123",
        "text" => "rich",
        "blocks" => [
          { "type" => "section" }
        ]
      },
      slack.last_payload
    )
  end

  def test_encoded_payload_ignores_outer_slack_overrides
    slack = FakeSlackClient.new(
      "ok" => true,
      "ts" => "1712345678.000450"
    )
    app = build_app(kube: standard_kube, slack: slack)

    request = minimal_request
    request["context"] = { "project" => "demo" }
    request["step"]["config"]["encodingType"] = "json"
    request["step"]["config"]["message"] = '{"text":"rich","thread_ts":"1700000000.000002"}'
    request["step"]["config"]["slack"] = {
      "channelID" => "C999",
      "threadTS" => "1700000000.000001"
    }
    response = execute_request(app, request)

    assert_equal "Succeeded", response["status"]
    assert_equal(
      {
        "channel" => "C123",
        "text" => "rich",
        "thread_ts" => "1700000000.000002"
      },
      slack.last_payload
    )
    assert_equal(
      "1700000000.000002",
      response.dig("output", "slack", "threadTS")
    )
  end

  def test_supports_xml_payloads
    slack = FakeSlackClient.new(
      "ok" => true,
      "ts" => "1712345678.000500"
    )
    app = build_app(kube: standard_kube, slack: slack)

    request = minimal_request
    request["context"] = { "project" => "demo" }
    request["step"]["config"]["encodingType"] = "xml"
    request["step"]["config"]["message"] = <<~XML
      <message>
        <text>rich</text>
        <thread_ts>1700000000.000003</thread_ts>
      </message>
    XML
    response = execute_request(app, request)

    assert_equal "Succeeded", response["status"]
    assert_equal(
      {
        "channel" => "C123",
        "text" => "rich",
        "thread_ts" => "1700000000.000003"
      },
      slack.last_payload
    )
    assert_equal(
      "1700000000.000003",
      response.dig("output", "slack", "threadTS")
    )
  end

  def test_xml_decode_repeats_siblings_into_arrays
    slack = FakeSlackClient.new(
      "ok" => true,
      "ts" => "1712345678.000510"
    )
    app = build_app(kube: standard_kube, slack: slack)

    request = minimal_request
    request["context"] = { "project" => "demo" }
    request["step"]["config"]["encodingType"] = "xml"
    request["step"]["config"]["message"] = <<~XML
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
    XML
    response = execute_request(app, request)

    assert_equal "Succeeded", response["status"]
    assert_equal(
      {
        "channel" => "C123",
        "text" => "rich",
        "blocks" => [
          {
            "type" => "section",
            "text" => {
              "type" => "mrkdwn",
              "text" => "*hello*"
            }
          },
          {
            "type" => "divider"
          }
        ]
      },
      slack.last_payload
    )
  end

  def test_secret_lookup_failure_returns_failed
    kube = FakeKubernetesClient.new(
      message_channels: {
        "demo/send" => {
          "secret_name" => "slack-token",
          "slack_channel_id" => "C123",
          "resource_kind" => "MessageChannel",
          "resource_name" => "send"
        }
      }
    )
    app = build_app(kube: kube, slack: FakeSlackClient.new("ok" => true, "ts" => "0"))

    request = minimal_request
    request["context"] = { "project" => "demo" }
    response = execute_request(app, request)

    assert_equal "Failed", response["status"]
    assert_match(/error getting Slack Secret/, response["error"])
  end

  def test_slack_rejection_returns_failed
    app = build_app(
      kube: standard_kube,
      slack: FakeSlackClient.new(
        "ok" => false,
        "error" => "channel_not_found"
      )
    )

    request = minimal_request
    request["context"] = { "project" => "demo" }
    response = execute_request(app, request)

    assert_equal "Failed", response["status"]
    assert_equal "Slack API error: channel_not_found", response["error"]
  end

  def test_bad_yaml_is_errored
    app = build_app(kube: standard_kube, slack: FakeSlackClient.new("ok" => true, "ts" => "0"))

    request = minimal_request
    request["context"] = { "project" => "demo" }
    request["step"]["config"]["encodingType"] = "yaml"
    request["step"]["config"]["message"] = "text: [oops"
    response = execute_request(app, request)

    assert_equal "Errored", response["status"]
    assert_match(/error decoding YAML Slack payload/, response["error"])
  end

  def test_slack_transport_error_returns_failed
    app = build_app(
      kube: standard_kube,
      slack: FakeSlackClient.new(
        { "ok" => true, "ts" => "0" },
        error: StandardError.new("dial tcp timeout")
      )
    )

    request = minimal_request
    request["context"] = { "project" => "demo" }
    response = execute_request(app, request)

    assert_equal "Failed", response["status"]
    assert_equal "dial tcp timeout", response["error"]
  end

  private

  def build_app(kube: FakeKubernetesClient.new, slack: FakeSlackClient.new("ok" => true, "ts" => "0"))
    token_dir = Dir.mktmpdir
    token_path = File.join(token_dir, "token")
    File.write(token_path, "expected-token")

    SendMessagePlugin::App.new(
      token_path: token_path,
      system_resources_namespace: "kargo-system-resources",
      kubernetes_client: kube,
      slack_client: slack,
      logger: Logger.new(File::NULL)
    )
  end

  def execute_request(app, request)
    status, response = app.call(
      method: "POST",
      path: SendMessagePlugin::STEP_EXECUTE_PATH,
      headers: { "Authorization" => "Bearer expected-token" },
      body: JSON.generate(request)
    )

    assert_equal 200, status
    response
  end

  def minimal_request
    {
      "step" => {
        "kind" => "send-message",
        "config" => {
          "channel" => {
            "kind" => "MessageChannel",
            "name" => "send"
          },
          "message" => "hello from plugin"
        }
      }
    }
  end

  def standard_kube
    FakeKubernetesClient.new(
      message_channels: {
        "demo/send" => {
          "secret_name" => "slack-token",
          "slack_channel_id" => "C123",
          "resource_kind" => "MessageChannel",
          "resource_name" => "send"
        }
      },
      secrets: {
        "demo/slack-token" => { "apiKey" => "xoxb-demo" }
      }
    )
  end
end

class FakeKubernetesClient
  def initialize(message_channels: {}, cluster_message_channels: {}, secrets: {})
    @message_channels = message_channels
    @cluster_message_channels = cluster_message_channels
    @secrets = secrets
  end

  def get_message_channel(namespace, name)
    @message_channels.fetch("#{namespace}/#{name}") do
      raise SendMessagePlugin::ExecutionError, "message channel not found"
    end
  end

  def get_cluster_message_channel(name)
    @cluster_message_channels.fetch(name) do
      raise SendMessagePlugin::ExecutionError, "cluster message channel not found"
    end
  end

  def get_secret(namespace, name)
    @secrets.fetch("#{namespace}/#{name}") do
      raise SendMessagePlugin::ExecutionError, "error getting Slack Secret: secret not found"
    end
  end
end

class FakeSlackClient
  attr_reader :last_payload

  def initialize(response, error: nil)
    @response = response
    @error = error
  end

  def post_message(api_base_url:, token:, payload:)
    @last_payload = payload
    raise @error if @error

    @response
  end
end
