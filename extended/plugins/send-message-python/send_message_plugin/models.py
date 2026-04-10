from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any, Protocol

STEP_EXECUTE_PATH = "/api/v1/step.execute"
AUTH_HEADER = "Authorization"
BEARER_PREFIX = "Bearer "

AUTH_TOKEN_PATH = Path("/var/run/kargo/token")
SERVICE_ACCOUNT_DIR = Path("/var/run/secrets/kubernetes.io/serviceaccount")
SERVICE_ACCOUNT_TOKEN = SERVICE_ACCOUNT_DIR / "token"
SERVICE_ACCOUNT_CA = SERVICE_ACCOUNT_DIR / "ca.crt"

DEFAULT_SYSTEM_NAMESPACE = "kargo-system-resources"
DEFAULT_KUBERNETES_URL = "https://kubernetes.default.svc"
DEFAULT_SLACK_API_BASE_URL = "https://slack.com/api"


class RequestError(RuntimeError):
    pass


@dataclass
class ChannelResource:
    secret_name: str
    slack_channel_id: str
    resource_kind: str
    resource_name: str
    resource_namespace: str = ""


class KubernetesClient(Protocol):
    def get_message_channel(
        self,
        namespace: str,
        name: str,
    ) -> ChannelResource:
        ...

    def get_cluster_message_channel(self, name: str) -> ChannelResource:
        ...

    def get_secret(self, namespace: str, name: str) -> dict[str, str]:
        ...


class SlackClient(Protocol):
    def post_message(
        self,
        api_base_url: str,
        token: str,
        payload: dict[str, Any],
    ) -> dict[str, Any]:
        ...
