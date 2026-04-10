from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request
from http import HTTPStatus
from typing import Any

from .models import AUTH_HEADER, BEARER_PREFIX


class RealSlackClient:
    def post_message(
        self,
        api_base_url: str,
        token: str,
        payload: dict[str, Any],
    ) -> dict[str, Any]:
        request = urllib.request.Request(
            urllib.parse.urljoin(api_base_url.rstrip("/") + "/", "chat.postMessage"),
            method="POST",
            headers={
                AUTH_HEADER: f"{BEARER_PREFIX}{token}",
                "Content-Type": "application/json",
            },
            data=json.dumps(payload).encode("utf-8"),
        )
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                result = json.loads(response.read().decode("utf-8"))
                if response.status >= HTTPStatus.BAD_REQUEST and not result.get("error"):
                    result["ok"] = False
                    result["error"] = response.reason
                return result
        except urllib.error.HTTPError as exc:
            body = exc.read().decode("utf-8", errors="replace")
            try:
                result = json.loads(body) if body else {}
            except json.JSONDecodeError:
                result = {}
            if "error" not in result:
                result["error"] = f"{exc.code} {exc.reason}"
            result["ok"] = False
            return result
