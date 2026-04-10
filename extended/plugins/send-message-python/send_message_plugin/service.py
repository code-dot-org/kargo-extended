from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any

from .kubernetes import InClusterKubernetesClient
from .models import (
    AUTH_HEADER,
    AUTH_TOKEN_PATH,
    BEARER_PREFIX,
    DEFAULT_SLACK_API_BASE_URL,
    DEFAULT_SYSTEM_NAMESPACE,
    ChannelResource,
    KubernetesClient,
    RequestError,
    SlackClient,
)
from .payloads import build_slack_payload
from .slack import RealSlackClient
from .util import as_dict, string_value


class PluginServer:
    def __init__(
        self,
        *,
        expected_token_path: Path | None = None,
        system_resources_namespace: str | None = None,
        slack_api_base_url: str | None = None,
        kube_client: KubernetesClient | None = None,
        slack_client: SlackClient | None = None,
    ) -> None:
        self.expected_token_path = expected_token_path or AUTH_TOKEN_PATH
        self.system_resources_namespace = (
            system_resources_namespace
            or os.environ.get("SYSTEM_RESOURCES_NAMESPACE", "").strip()
            or DEFAULT_SYSTEM_NAMESPACE
        )
        self.slack_api_base_url = (
            slack_api_base_url
            or os.environ.get("SLACK_API_BASE_URL", "").strip()
            or DEFAULT_SLACK_API_BASE_URL
        )
        self.kube_client = kube_client or InClusterKubernetesClient()
        self.slack_client = slack_client or RealSlackClient()

    def handle(
        self,
        method: str,
        path: str,
        headers: dict[str, str],
        body: bytes,
    ) -> tuple[int, dict[str, Any] | None]:
        from http import HTTPStatus
        from .models import STEP_EXECUTE_PATH

        if path != STEP_EXECUTE_PATH:
            return HTTPStatus.NOT_FOUND, None
        if method != "POST":
            return HTTPStatus.METHOD_NOT_ALLOWED, None

        try:
            self.authorize(headers.get(AUTH_HEADER, ""))
        except Exception as exc:  # pragma: no cover
            return HTTPStatus.FORBIDDEN, errored_response(str(exc))

        try:
            request = json.loads(body.decode("utf-8"))
        except Exception as exc:
            return HTTPStatus.BAD_REQUEST, {
                "status": "Errored",
                "message": "invalid request body",
                "error": str(exc),
                "terminal": True,
            }

        return HTTPStatus.OK, self.execute(request)

    def execute(self, request: dict[str, Any]) -> dict[str, Any]:
        step = as_dict(request.get("step"))
        step_kind = string_value(step.get("kind"))
        if step_kind != "send-message":
            return errored_response(f'unsupported step kind "{step_kind}"')

        try:
            config = as_dict(step.get("config"))
            channel, secret_namespace = self.lookup_channel(request, config)
            token = self.read_slack_token(secret_namespace, channel.secret_name)
            payload, output_thread_ts = build_slack_payload(config, channel)
            slack_response = self.slack_client.post_message(
                self.slack_api_base_url,
                token,
                payload,
            )
            if not slack_response.get("ok"):
                raise RuntimeError(f'Slack API error: {slack_response.get("error", "")}')
            if not output_thread_ts:
                output_thread_ts = string_value(slack_response.get("ts"))
            return {
                "status": "Succeeded",
                "output": {
                    "slack": {
                        "threadTS": output_thread_ts,
                    },
                },
            }
        except RequestError as exc:
            return errored_response(str(exc))
        except Exception as exc:
            return failed_response(str(exc))

    def lookup_channel(
        self,
        request: dict[str, Any],
        config: dict[str, Any],
    ) -> tuple[ChannelResource, str]:
        channel_ref = as_dict(config.get("channel"))
        channel_kind = string_value(channel_ref.get("kind"))
        channel_name = string_value(channel_ref.get("name"))
        project = string_value(as_dict(request.get("context")).get("project"))

        if channel_kind == "MessageChannel":
            if not project:
                raise RequestError("step context project is required for MessageChannel")
            return self.kube_client.get_message_channel(project, channel_name), project

        if channel_kind == "ClusterMessageChannel":
            return (
                self.kube_client.get_cluster_message_channel(channel_name),
                self.system_resources_namespace,
            )

        raise RequestError(f'unsupported channel kind "{channel_kind}"')

    def read_slack_token(self, namespace: str, secret_name: str) -> str:
        try:
            secret = self.kube_client.get_secret(namespace, secret_name)
        except Exception as exc:
            raise RuntimeError(f"error getting Slack Secret: {exc}") from exc
        token = string_value(secret.get("apiKey"))
        if not token:
            raise RuntimeError('Slack Secret is missing key "apiKey"')
        return token

    def authorize(self, header_value: str) -> None:
        expected = self.expected_token_path.read_text(encoding="utf-8").strip()
        value = header_value.strip()
        if not value.startswith(BEARER_PREFIX):
            raise RuntimeError("missing bearer token")
        received = value[len(BEARER_PREFIX) :].strip()
        if received != expected:
            raise RuntimeError("invalid bearer token")


def failed_response(message: str) -> dict[str, Any]:
    return {
        "status": "Failed",
        "message": message,
        "error": message,
        "terminal": True,
    }


def errored_response(message: str) -> dict[str, Any]:
    return {
        "status": "Errored",
        "message": message,
        "error": message,
        "terminal": True,
    }
