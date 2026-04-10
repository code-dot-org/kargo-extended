#!/usr/bin/env python3

from __future__ import annotations

import shutil
import sys
import tempfile
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from smoke.lib import SmokeEnv, SmokeResources, fail, log


def main() -> int:
    plugin_dir = Path(__file__).resolve().parents[1]
    env = SmokeEnv()
    env.require(
        "SEND_MESSAGE_SMOKE_PROJECT",
        "SEND_MESSAGE_SMOKE_WAREHOUSE",
        "SEND_MESSAGE_SMOKE_FREIGHT_NAME",
        "SEND_MESSAGE_SMOKE_SLACK_API_KEY",
        "SEND_MESSAGE_SMOKE_CHANNEL_ID",
    )

    current_context = env.capture(
        env.kubectl_bin,
        "config",
        "current-context",
    ).strip()
    if not current_context.startswith("kind-"):
        fail(f"send-message smoke requires a kind context, got: {current_context}")

    kind_name = current_context.removeprefix("kind-")
    image_tag = f"send-message-step-plugin-python:e2e-{int(time.time())}"
    tmp_dir = Path(tempfile.mkdtemp(prefix="send-message-python-smoke-"))

    resources = SmokeResources(env, plugin_dir, tmp_dir)
    try:
        resources.write_plugin_dir(image_tag)
        resources.build_image(image_tag)
        resources.load_image(kind_name, image_tag)
        resources.build_configmap()
        resources.install_crds()
        resources.install_rbac()
        resources.bind_cluster_role()
        resources.install_configmap()
        resources.create_secret()
        resources.create_channel()
        resources.create_stage()
        resources.approve_freight()
        resources.promote_freight()
        resources.wait_for_success()
    finally:
        resources.cleanup()
        shutil.rmtree(tmp_dir, ignore_errors=True)

    log("PASS", "send-message StepPlugin smoke promotion finished successfully")
    return 0


if __name__ == "__main__":
    sys.exit(main())
