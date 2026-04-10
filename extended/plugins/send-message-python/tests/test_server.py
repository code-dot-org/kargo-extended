from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import send_message_plugin as server


class FakeKubernetesClient:
    def __init__(
        self,
        *,
        message_channels: dict[tuple[str, str], server.ChannelResource] | None = None,
        cluster_message_channels: dict[str, server.ChannelResource] | None = None,
        secrets: dict[tuple[str, str], dict[str, str]] | None = None,
    ) -> None:
        self.message_channels = message_channels or {}
        self.cluster_message_channels = cluster_message_channels or {}
        self.secrets = secrets or {}

    def get_message_channel(self, namespace: str, name: str) -> server.ChannelResource:
        try:
            return self.message_channels[(namespace, name)]
        except KeyError as exc:
            raise RuntimeError("message channel not found") from exc

    def get_cluster_message_channel(self, name: str) -> server.ChannelResource:
        try:
            return self.cluster_message_channels[name]
        except KeyError as exc:
            raise RuntimeError("cluster message channel not found") from exc

    def get_secret(self, namespace: str, name: str) -> dict[str, str]:
        try:
            return self.secrets[(namespace, name)]
        except KeyError as exc:
            raise RuntimeError("secret not found") from exc


class FakeSlackClient:
    def __init__(
        self,
        *,
        response: dict[str, object] | None = None,
        error: Exception | None = None,
    ) -> None:
        self.response = response or {"ok": True, "ts": "0"}
        self.error = error
        self.last_payload: dict[str, object] | None = None

    def post_message(
        self,
        api_base_url: str,
        token: str,
        payload: dict[str, object],
    ) -> dict[str, object]:
        del api_base_url
        del token
        self.last_payload = payload
        if self.error is not None:
            raise self.error
        return self.response


class PluginServerTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.token_path = Path(self.temp_dir.name) / "token"
        self.token_path.write_text("expected-token", encoding="utf-8")

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def new_server(
        self,
        *,
        kube_client: server.KubernetesClient | None = None,
        slack_client: server.SlackClient | None = None,
    ) -> server.PluginServer:
        return server.PluginServer(
            expected_token_path=self.token_path,
            system_resources_namespace="kargo-system-resources",
            kube_client=kube_client or FakeKubernetesClient(),
            slack_client=slack_client or FakeSlackClient(),
        )

    def test_handler_rejects_missing_bearer_token(self) -> None:
        plugin = self.new_server()

        status_code, response = plugin.handle(
            "POST",
            server.STEP_EXECUTE_PATH,
            {},
            json.dumps(minimal_request()).encode("utf-8"),
        )

        self.assertEqual(403, status_code)
        self.assertEqual("Errored", response["status"])

    def test_handler_rejects_invalid_bearer_token(self) -> None:
        plugin = self.new_server()

        status_code, response = plugin.handle(
            "POST",
            server.STEP_EXECUTE_PATH,
            {server.AUTH_HEADER: f"{server.BEARER_PREFIX}wrong-token"},
            json.dumps(minimal_request()).encode("utf-8"),
        )

        self.assertEqual(403, status_code)
        self.assertEqual("Errored", response["status"])

    def test_handler_rejects_invalid_request_body(self) -> None:
        plugin = self.new_server()

        status_code, response = plugin.handle(
            "POST",
            server.STEP_EXECUTE_PATH,
            {server.AUTH_HEADER: f"{server.BEARER_PREFIX}expected-token"},
            b"{not-json",
        )

        self.assertEqual(400, status_code)
        self.assertEqual("Errored", response["status"])
        self.assertEqual("invalid request body", response["message"])

    def test_execute_uses_namespaced_message_channel(self) -> None:
        kube_client = FakeKubernetesClient(
            message_channels={
                ("demo", "send"): server.ChannelResource(
                    secret_name="slack-token",
                    slack_channel_id="C123",
                    resource_kind="MessageChannel",
                    resource_name="send",
                    resource_namespace="demo",
                ),
            },
            secrets={
                ("demo", "slack-token"): {"apiKey": "xoxb-demo"},
            },
        )
        slack_client = FakeSlackClient(response={"ok": True, "ts": "1712345678.000100"})
        plugin = self.new_server(kube_client=kube_client, slack_client=slack_client)

        response = execute_request(
            plugin,
            {
                "context": {"project": "demo"},
                "step": minimal_request()["step"],
            },
        )

        self.assertEqual("Succeeded", response["status"])
        self.assertEqual("1712345678.000100", response["output"]["slack"]["threadTS"])
        self.assertEqual(
            {
                "channel": "C123",
                "text": "hello from plugin",
            },
            slack_client.last_payload,
        )

    def test_execute_uses_cluster_message_channel(self) -> None:
        kube_client = FakeKubernetesClient(
            cluster_message_channels={
                "send": server.ChannelResource(
                    secret_name="slack-token",
                    slack_channel_id="C777",
                    resource_kind="ClusterMessageChannel",
                    resource_name="send",
                ),
            },
            secrets={
                ("kargo-system-resources", "slack-token"): {"apiKey": "xoxb-demo"},
            },
        )
        slack_client = FakeSlackClient(response={"ok": True, "ts": "1712345678.000200"})
        plugin = self.new_server(kube_client=kube_client, slack_client=slack_client)

        response = execute_request(
            plugin,
            {
                "context": {"project": "demo"},
                "step": {
                    "kind": "send-message",
                    "config": {
                        "channel": {
                            "kind": "ClusterMessageChannel",
                            "name": "send",
                        },
                        "message": "hello from plugin",
                    },
                },
            },
        )

        self.assertEqual("Succeeded", response["status"])
        self.assertEqual(
            {
                "channel": "C777",
                "text": "hello from plugin",
            },
            slack_client.last_payload,
        )

    def test_execute_honors_plaintext_slack_overrides(self) -> None:
        plugin = self.new_server(
            kube_client=FakeKubernetesClient(
                message_channels={
                    ("demo", "send"): server.ChannelResource(
                        secret_name="slack-token",
                        slack_channel_id="C123",
                        resource_kind="MessageChannel",
                        resource_name="send",
                        resource_namespace="demo",
                    ),
                },
                secrets={
                    ("demo", "slack-token"): {"apiKey": "xoxb-demo"},
                },
            ),
            slack_client=FakeSlackClient(response={"ok": True, "ts": "1712345678.000300"}),
        )

        request = minimal_request()
        request["context"] = {"project": "demo"}
        request["step"]["config"]["slack"] = {
            "channelID": "C999",
            "threadTS": "1700000000.000001",
        }

        response = execute_request(plugin, request)

        self.assertEqual("Succeeded", response["status"])
        self.assertEqual(
            {
                "channel": "C999",
                "text": "hello from plugin",
                "thread_ts": "1700000000.000001",
            },
            plugin.slack_client.last_payload,
        )
        self.assertEqual(
            "1700000000.000001",
            response["output"]["slack"]["threadTS"],
        )

    def test_execute_supports_json_encoding(self) -> None:
        plugin = self.new_server(
            kube_client=FakeKubernetesClient(
                message_channels={
                    ("demo", "send"): server.ChannelResource(
                        secret_name="slack-token",
                        slack_channel_id="C123",
                        resource_kind="MessageChannel",
                        resource_name="send",
                        resource_namespace="demo",
                    ),
                },
                secrets={
                    ("demo", "slack-token"): {"apiKey": "xoxb-demo"},
                },
            ),
            slack_client=FakeSlackClient(response={"ok": True, "ts": "1712345678.000400"}),
        )

        request = minimal_request()
        request["context"] = {"project": "demo"}
        request["step"]["config"]["encodingType"] = "json"
        request["step"]["config"]["message"] = (
            '{"text":"rich","blocks":[{"type":"section"}]}'
        )

        response = execute_request(plugin, request)

        self.assertEqual("Succeeded", response["status"])
        self.assertEqual(
            {
                "channel": "C123",
                "text": "rich",
                "blocks": [{"type": "section"}],
            },
            plugin.slack_client.last_payload,
        )

    def test_execute_supports_yaml_encoding(self) -> None:
        plugin = self.new_server(
            kube_client=FakeKubernetesClient(
                message_channels={
                    ("demo", "send"): server.ChannelResource(
                        secret_name="slack-token",
                        slack_channel_id="C123",
                        resource_kind="MessageChannel",
                        resource_name="send",
                        resource_namespace="demo",
                    ),
                },
                secrets={
                    ("demo", "slack-token"): {"apiKey": "xoxb-demo"},
                },
            ),
            slack_client=FakeSlackClient(response={"ok": True, "ts": "1712345678.000410"}),
        )

        request = minimal_request()
        request["context"] = {"project": "demo"}
        request["step"]["config"]["encodingType"] = "yaml"
        request["step"]["config"]["message"] = (
            "text: rich\nblocks:\n- type: section\n"
        )

        response = execute_request(plugin, request)

        self.assertEqual("Succeeded", response["status"])
        self.assertEqual(
            {
                "channel": "C123",
                "text": "rich",
                "blocks": [{"type": "section"}],
            },
            plugin.slack_client.last_payload,
        )

    def test_execute_returns_errored_for_bad_yaml(self) -> None:
        plugin = self.new_server(
            kube_client=FakeKubernetesClient(
                message_channels={
                    ("demo", "send"): server.ChannelResource(
                        secret_name="slack-token",
                        slack_channel_id="C123",
                        resource_kind="MessageChannel",
                        resource_name="send",
                        resource_namespace="demo",
                    ),
                },
                secrets={
                    ("demo", "slack-token"): {"apiKey": "xoxb-demo"},
                },
            ),
            slack_client=FakeSlackClient(),
        )

        request = minimal_request()
        request["context"] = {"project": "demo"}
        request["step"]["config"]["encodingType"] = "yaml"
        request["step"]["config"]["message"] = "text: [oops"

        response = execute_request(plugin, request)

        self.assertEqual("Errored", response["status"])
        self.assertIn("error decoding YAML Slack payload", response["error"])

    def test_execute_supports_xml_encoding(self) -> None:
        plugin = self.new_server(
            kube_client=FakeKubernetesClient(
                message_channels={
                    ("demo", "send"): server.ChannelResource(
                        secret_name="slack-token",
                        slack_channel_id="C123",
                        resource_kind="MessageChannel",
                        resource_name="send",
                        resource_namespace="demo",
                    ),
                },
                secrets={
                    ("demo", "slack-token"): {"apiKey": "xoxb-demo"},
                },
            ),
            slack_client=FakeSlackClient(response={"ok": True, "ts": "1712345678.000500"}),
        )

        request = minimal_request()
        request["context"] = {"project": "demo"}
        request["step"]["config"]["encodingType"] = "xml"
        request["step"]["config"]["message"] = """
        <message>
          <text>rich</text>
          <thread_ts>1700000000.000003</thread_ts>
        </message>
        """

        response = execute_request(plugin, request)

        self.assertEqual("Succeeded", response["status"])
        self.assertEqual(
            {
                "channel": "C123",
                "text": "rich",
                "thread_ts": "1700000000.000003",
            },
            plugin.slack_client.last_payload,
        )
        self.assertEqual(
            "1700000000.000003",
            response["output"]["slack"]["threadTS"],
        )

    def test_execute_encoded_payload_ignores_slack_overrides(self) -> None:
        plugin = self.new_server(
            kube_client=FakeKubernetesClient(
                message_channels={
                    ("demo", "send"): server.ChannelResource(
                        secret_name="slack-token",
                        slack_channel_id="C123",
                        resource_kind="MessageChannel",
                        resource_name="send",
                        resource_namespace="demo",
                    ),
                },
                secrets={
                    ("demo", "slack-token"): {"apiKey": "xoxb-demo"},
                },
            ),
            slack_client=FakeSlackClient(response={"ok": True, "ts": "1712345678.000450"}),
        )

        request = minimal_request()
        request["context"] = {"project": "demo"}
        request["step"]["config"]["encodingType"] = "json"
        request["step"]["config"]["message"] = (
            '{"text":"rich","thread_ts":"1700000000.000002"}'
        )
        request["step"]["config"]["slack"] = {
            "channelID": "C999",
            "threadTS": "1700000000.000001",
        }

        response = execute_request(plugin, request)

        self.assertEqual("Succeeded", response["status"])
        self.assertEqual(
            {
                "channel": "C123",
                "text": "rich",
                "thread_ts": "1700000000.000002",
            },
            plugin.slack_client.last_payload,
        )
        self.assertEqual(
            "1700000000.000002",
            response["output"]["slack"]["threadTS"],
        )

    def test_execute_fails_when_secret_missing(self) -> None:
        plugin = self.new_server(
            kube_client=FakeKubernetesClient(
                message_channels={
                    ("demo", "send"): server.ChannelResource(
                        secret_name="slack-token",
                        slack_channel_id="C123",
                        resource_kind="MessageChannel",
                        resource_name="send",
                        resource_namespace="demo",
                    ),
                },
            ),
            slack_client=FakeSlackClient(),
        )

        response = execute_request(
            plugin,
            {
                "context": {"project": "demo"},
                "step": minimal_request()["step"],
            },
        )

        self.assertEqual("Failed", response["status"])
        self.assertIn("secret not found", response["error"])

    def test_execute_fails_when_slack_errors(self) -> None:
        plugin = self.new_server(
            kube_client=FakeKubernetesClient(
                message_channels={
                    ("demo", "send"): server.ChannelResource(
                        secret_name="slack-token",
                        slack_channel_id="C123",
                        resource_kind="MessageChannel",
                        resource_name="send",
                        resource_namespace="demo",
                    ),
                },
                secrets={
                    ("demo", "slack-token"): {"apiKey": "xoxb-demo"},
                },
            ),
            slack_client=FakeSlackClient(
                response={"ok": False, "error": "channel_not_found"}
            ),
        )

        response = execute_request(
            plugin,
            {
                "context": {"project": "demo"},
                "step": minimal_request()["step"],
            },
        )

        self.assertEqual("Failed", response["status"])
        self.assertIn("channel_not_found", response["error"])

    def test_xml_decode_shape(self) -> None:
        payload = server.decode_xml_slack_payload(
            """
            <message icon_emoji=":wave:">
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
            """
        )

        self.assertEqual(
            {
                "icon_emoji": ":wave:",
                "text": "rich",
                "blocks": [
                    {
                        "type": "section",
                        "text": {
                            "type": "mrkdwn",
                            "text": "*hello*",
                        },
                    },
                    {
                        "type": "divider",
                    },
                ],
            },
            payload,
        )


def execute_request(
    plugin: server.PluginServer,
    request: dict[str, object],
) -> dict[str, object]:
    status_code, response = plugin.handle(
        "POST",
        server.STEP_EXECUTE_PATH,
        {server.AUTH_HEADER: f"{server.BEARER_PREFIX}expected-token"},
        json.dumps(request).encode("utf-8"),
    )
    if status_code != 200:
        raise AssertionError(f"unexpected status code {status_code}: {response}")
    assert response is not None
    return response


def minimal_request() -> dict[str, object]:
    return {
        "step": {
            "kind": "send-message",
            "config": {
                "channel": {
                    "kind": "MessageChannel",
                    "name": "send",
                },
                "message": "hello from plugin",
            },
        },
    }


if __name__ == "__main__":
    unittest.main()
