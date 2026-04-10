from __future__ import annotations

import base64
import json
import os
import ssl
import urllib.error
import urllib.parse
import urllib.request
from http import HTTPStatus
from typing import Any

from .models import (
    AUTH_HEADER,
    BEARER_PREFIX,
    DEFAULT_KUBERNETES_URL,
    ChannelResource,
    SERVICE_ACCOUNT_CA,
    SERVICE_ACCOUNT_TOKEN,
)
from .util import as_dict, deep_get, string_value


class InClusterKubernetesClient:
    def __init__(self) -> None:
        self.token = SERVICE_ACCOUNT_TOKEN.read_text(encoding="utf-8").strip()
        self.base_url = kubernetes_base_url()
        self.ssl_context = ssl.create_default_context(cafile=str(SERVICE_ACCOUNT_CA))

    def get_message_channel(self, namespace: str, name: str) -> ChannelResource:
        response = self._get_json(
            "/apis/ee.kargo.akuity.io/v1alpha1/namespaces/"
            f"{quote(namespace)}/messagechannels/{quote(name)}"
        )
        return ChannelResource(
            secret_name=string_value(deep_get(response, "spec", "secretRef", "name")),
            slack_channel_id=string_value(deep_get(response, "spec", "slack", "channelID")),
            resource_kind="MessageChannel",
            resource_name=string_value(deep_get(response, "metadata", "name")),
            resource_namespace=string_value(deep_get(response, "metadata", "namespace")),
        )

    def get_cluster_message_channel(self, name: str) -> ChannelResource:
        response = self._get_json(
            "/apis/ee.kargo.akuity.io/v1alpha1/clustermessagechannels/"
            f"{quote(name)}"
        )
        return ChannelResource(
            secret_name=string_value(deep_get(response, "spec", "secretRef", "name")),
            slack_channel_id=string_value(deep_get(response, "spec", "slack", "channelID")),
            resource_kind="ClusterMessageChannel",
            resource_name=string_value(deep_get(response, "metadata", "name")),
        )

    def get_secret(self, namespace: str, name: str) -> dict[str, str]:
        response = self._get_json(
            f"/api/v1/namespaces/{quote(namespace)}/secrets/{quote(name)}"
        )
        decoded: dict[str, str] = {}
        for key, value in as_dict(response.get("data")).items():
            try:
                raw = base64.b64decode(string_value(value))
            except Exception as exc:
                raise RuntimeError(f'error decoding secret key "{key}": {exc}') from exc
            decoded[str(key)] = raw.decode("utf-8")
        return decoded

    def _get_json(self, path: str) -> dict[str, Any]:
        request = urllib.request.Request(
            urllib.parse.urljoin(self.base_url.rstrip("/") + "/", path.lstrip("/")),
            method="GET",
            headers={AUTH_HEADER: f"{BEARER_PREFIX}{self.token}"},
        )
        try:
            with urllib.request.urlopen(
                request,
                context=self.ssl_context,
                timeout=30,
            ) as response:
                return json.loads(response.read().decode("utf-8"))
        except urllib.error.HTTPError as exc:
            body = exc.read().decode("utf-8", errors="replace").strip()
            if exc.code == HTTPStatus.NOT_FOUND:
                raise RuntimeError(f"resource not found at {path}") from exc
            raise RuntimeError(
                f"Kubernetes API error {exc.code} {exc.reason}: {body}"
            ) from exc


def kubernetes_base_url() -> str:
    host = os.environ.get("KUBERNETES_SERVICE_HOST", "").strip()
    if not host:
        return DEFAULT_KUBERNETES_URL
    port = (
        os.environ.get("KUBERNETES_SERVICE_PORT_HTTPS", "").strip()
        or os.environ.get("KUBERNETES_SERVICE_PORT", "").strip()
        or "443"
    )
    return f"https://{host}:{port}"


def quote(value: str) -> str:
    return urllib.parse.quote(value, safe="")
