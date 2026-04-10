from __future__ import annotations

import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from .service import PluginServer


class RequestHandler(BaseHTTPRequestHandler):
    plugin_server: PluginServer

    def do_POST(self) -> None:  # noqa: N802
        self._handle()

    def do_GET(self) -> None:  # noqa: N802
        self._handle()

    def log_message(self, format: str, *args: Any) -> None:
        return

    def _handle(self) -> None:
        content_length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(content_length) if content_length else b""
        status_code, response = self.plugin_server.handle(
            self.command,
            self.path,
            {key: value for key, value in self.headers.items()},
            body,
        )
        self.send_response(status_code)
        if response is None:
            self.end_headers()
            return
        data = json.dumps(response).encode("utf-8")
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def make_handler(plugin_server: PluginServer) -> type[RequestHandler]:
    class Handler(RequestHandler):
        pass

    Handler.plugin_server = plugin_server
    return Handler


def serve() -> None:
    port = int(os.environ.get("PORT", "9765"))
    address = os.environ.get("HOST", "0.0.0.0")
    server = ThreadingHTTPServer((address, port), make_handler(PluginServer()))
    server.serve_forever()
