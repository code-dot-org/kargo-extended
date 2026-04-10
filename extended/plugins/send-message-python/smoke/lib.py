from __future__ import annotations

import json
import os
import shlex
import subprocess
import time
from pathlib import Path


class SmokeEnv:
    def __init__(self) -> None:
        self.project = os.environ.get("SEND_MESSAGE_SMOKE_PROJECT", "")
        self.warehouse = os.environ.get("SEND_MESSAGE_SMOKE_WAREHOUSE", "")
        self.freight_name = os.environ.get("SEND_MESSAGE_SMOKE_FREIGHT_NAME", "")
        self.slack_api_key = os.environ.get("SEND_MESSAGE_SMOKE_SLACK_API_KEY", "")
        self.channel_id = os.environ.get("SEND_MESSAGE_SMOKE_CHANNEL_ID", "")
        self.system_namespace = os.environ.get(
            "SEND_MESSAGE_SMOKE_SYSTEM_RESOURCES_NAMESPACE",
            "kargo-system-resources",
        )
        self.secret_name = os.environ.get(
            "SEND_MESSAGE_SMOKE_SECRET_NAME",
            "send-message-slack-token",
        )
        self.channel_name = os.environ.get(
            "SEND_MESSAGE_SMOKE_CHANNEL_NAME",
            "send-message-smoke",
        )
        self.cluster_role_name = "send-message-step-plugin-reader"
        self.kargo_bin = os.environ.get("KARGO_BIN", "kargo")
        self.kubectl_bin = os.environ.get("KUBECTL_BIN", "kubectl")
        self.docker_bin = os.environ.get("DOCKER_BIN", "docker")
        self.kind_bin = os.environ.get("KIND_BIN", "kind")
        self.kargo_flags = shlex.split(os.environ.get("KARGO_FLAGS", ""))

    def require(self, *names: str) -> None:
        for name in names:
            if not os.environ.get(name):
                fail(f"{name} is required")

    def run(self, *args: str, cwd: Path | None = None) -> None:
        log("INFO", " ".join(args))
        subprocess.run(args, cwd=str(cwd) if cwd else None, check=True)

    def capture(self, *args: str, cwd: Path | None = None) -> str:
        completed = subprocess.run(
            args,
            cwd=str(cwd) if cwd else None,
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        return completed.stdout


class SmokeResources:
    def __init__(self, env: SmokeEnv, plugin_dir: Path, tmp_dir: Path) -> None:
        self.env = env
        self.plugin_dir = plugin_dir
        self.tmp_dir = tmp_dir
        self.rendered_plugin_dir = tmp_dir / "plugin"
        self.configmap_path: Path | None = None
        self.cluster_role_binding_name = f"send-message-step-plugin-reader-{env.project}"
        self.stage_name = f"smsg-python-{int(time.time())}"
        self.promotion_name = f"{self.stage_name}-promotion"
        self.message_text = (
            f"send-message Python implementation smoke {self.stage_name}"
        )

    def write_plugin_dir(self, image_tag: str) -> None:
        self.rendered_plugin_dir.mkdir(parents=True, exist_ok=True)
        rendered = (self.plugin_dir / "plugin.yaml").read_text(encoding="utf-8")
        rendered = rendered.replace(
            "namespace: kargo-system-resources",
            f"namespace: {self.env.system_namespace}",
        )
        rendered = rendered.replace(
            "image: send-message-step-plugin-python:dev",
            f"image: {image_tag}",
        )
        rendered = rendered.replace(
            "value: kargo-system-resources",
            f"value: {self.env.system_namespace}",
        )
        (self.rendered_plugin_dir / "plugin.yaml").write_text(rendered, encoding="utf-8")

    def build_image(self, image_tag: str) -> None:
        self.env.run(self.env.docker_bin, "build", "-t", image_tag, str(self.plugin_dir))

    def load_image(self, kind_name: str, image_tag: str) -> None:
        self.env.run(
            self.env.kind_bin,
            "load",
            "docker-image",
            "--name",
            kind_name,
            image_tag,
        )

    def build_configmap(self) -> None:
        self.env.run(
            self.env.kargo_bin,
            "step-plugin",
            "build",
            ".",
            cwd=self.rendered_plugin_dir,
        )
        matches = sorted(self.rendered_plugin_dir.glob("*-configmap.yaml"))
        if len(matches) != 1:
            fail(
                "expected exactly one rendered StepPlugin ConfigMap, got "
                f"{[path.name for path in matches]}"
            )
        self.configmap_path = matches[0]

    def install_crds(self) -> None:
        self.apply_file(self.plugin_dir / "manifests" / "crds.yaml")

    def install_rbac(self) -> None:
        self.apply_file(self.plugin_dir / "manifests" / "rbac.yaml")

    def bind_cluster_role(self) -> None:
        binding_path = self.tmp_dir / "clusterrolebinding.yaml"
        binding_path.write_text(
            "\n".join(
                [
                    "apiVersion: rbac.authorization.k8s.io/v1",
                    "kind: ClusterRoleBinding",
                    "metadata:",
                    f"  name: {self.cluster_role_binding_name}",
                    "subjects:",
                    "- kind: ServiceAccount",
                    "  name: default",
                    f"  namespace: {self.env.project}",
                    "roleRef:",
                    "  apiGroup: rbac.authorization.k8s.io",
                    "  kind: ClusterRole",
                    f"  name: {self.env.cluster_role_name}",
                    "",
                ]
            ),
            encoding="utf-8",
        )
        self.apply_file(binding_path)

    def install_configmap(self) -> None:
        if self.configmap_path is None:
            fail("StepPlugin ConfigMap was not built")
        self.apply_file(self.configmap_path)

    def create_secret(self) -> None:
        path = self.tmp_dir / "secret.yaml"
        path.write_text(
            "\n".join(
                [
                    "apiVersion: v1",
                    "kind: Secret",
                    "metadata:",
                    f"  name: {self.env.secret_name}",
                    f"  namespace: {self.env.project}",
                    "type: Opaque",
                    "stringData:",
                    f"  apiKey: {self.env.slack_api_key}",
                    "",
                ]
            ),
            encoding="utf-8",
        )
        self.apply_file(path)

    def create_channel(self) -> None:
        path = self.tmp_dir / "messagechannel.yaml"
        path.write_text(
            "\n".join(
                [
                    "apiVersion: ee.kargo.akuity.io/v1alpha1",
                    "kind: MessageChannel",
                    "metadata:",
                    f"  name: {self.env.channel_name}",
                    f"  namespace: {self.env.project}",
                    "spec:",
                    "  secretRef:",
                    f"    name: {self.env.secret_name}",
                    "  slack:",
                    f"    channelID: {self.env.channel_id}",
                    "",
                ]
            ),
            encoding="utf-8",
        )
        self.apply_file(path)

    def create_stage(self) -> None:
        path = self.tmp_dir / "stage.yaml"
        path.write_text(
            "\n".join(
                [
                    "apiVersion: kargo.akuity.io/v1alpha1",
                    "kind: Stage",
                    "metadata:",
                    f"  name: {self.stage_name}",
                    f"  namespace: {self.env.project}",
                    "spec:",
                    "  requestedFreight:",
                    "  - origin:",
                    "      kind: Warehouse",
                    f"      name: {self.env.warehouse}",
                    "    sources:",
                    "      direct: true",
                    "  promotionTemplate:",
                    "    spec:",
                    "      steps:",
                    "      - uses: send-message",
                    "        config:",
                    "          channel:",
                    "            kind: MessageChannel",
                    f"            name: {self.env.channel_name}",
                    f'          message: "{self.message_text}"',
                    "",
                ]
            ),
            encoding="utf-8",
        )
        self.env.run(self.env.kargo_bin, "apply", "-f", str(path), *self.env.kargo_flags)

    def approve_freight(self) -> None:
        self.env.run(
            self.env.kargo_bin,
            "approve",
            f"--project={self.env.project}",
            f"--freight={self.env.freight_name}",
            f"--stage={self.stage_name}",
            *self.env.kargo_flags,
        )

    def promote_freight(self) -> None:
        path = self.tmp_dir / "promotion.yaml"
        path.write_text(
            "\n".join(
                [
                    "apiVersion: kargo.akuity.io/v1alpha1",
                    "kind: Promotion",
                    "metadata:",
                    f"  name: {self.promotion_name}",
                    f"  namespace: {self.env.project}",
                    "spec:",
                    f"  stage: {self.stage_name}",
                    f"  freight: {self.env.freight_name}",
                    "  steps:",
                    "  - uses: send-message",
                    "    config:",
                    "      channel:",
                    "        kind: MessageChannel",
                    f"        name: {self.env.channel_name}",
                    f'      message: "{self.message_text}"',
                    "",
                ]
            ),
            encoding="utf-8",
        )
        self.apply_file(path)

    def wait_for_success(self) -> None:
        deadline = time.time() + 180
        last_phase = ""
        last_promotion_name = ""
        last_thread_ts = ""

        while time.time() < deadline:
            stdout = self.env.capture(
                self.env.kubectl_bin,
                "get",
                "promotion.kargo.akuity.io",
                "-n",
                self.env.project,
                "-o",
                "json",
            )
            promotion_name, phase, thread_ts = find_latest_promotion(stdout, self.stage_name)
            if promotion_name:
                last_promotion_name = promotion_name
                last_phase = phase
                last_thread_ts = thread_ts

            if phase == "Succeeded":
                if not thread_ts:
                    fail("send-message smoke did not produce slack.threadTS output")
                return
            if phase in {"Failed", "Errored", "Aborted"}:
                fail(
                    "send-message smoke promotion reached terminal phase "
                    f"{phase} ({promotion_name})"
                )
            time.sleep(2)

        fail(
            "send-message smoke promotion did not succeed in time"
            f" (last promotion={last_promotion_name}, last phase={last_phase}, "
            f"last threadTS={last_thread_ts})"
        )

    def cleanup(self) -> None:
        delete(
            self.env.kubectl_bin,
            "promotion.kargo.akuity.io",
            self.promotion_name,
            "-n",
            self.env.project,
        )
        delete(self.env.kubectl_bin, "stage.kargo.akuity.io", self.stage_name, "-n", self.env.project)
        delete(
            self.env.kubectl_bin,
            "messagechannel.ee.kargo.akuity.io",
            self.env.channel_name,
            "-n",
            self.env.project,
        )
        delete(self.env.kubectl_bin, "secret", self.env.secret_name, "-n", self.env.project)
        if self.configmap_path is not None:
            delete(
                self.env.kubectl_bin,
                "configmap",
                self.configmap_path.stem.removesuffix("-configmap"),
                "-n",
                self.env.system_namespace,
            )
        delete(self.env.kubectl_bin, "clusterrolebinding", self.cluster_role_binding_name)
        delete(self.env.kubectl_bin, "clusterrole", self.env.cluster_role_name)
        subprocess.run(
            [
                self.env.kubectl_bin,
                "delete",
                "-f",
                str(self.plugin_dir / "manifests" / "crds.yaml"),
                "--ignore-not-found",
            ],
            check=False,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

    def apply_file(self, path: Path) -> None:
        self.env.run(self.env.kubectl_bin, "apply", "-f", str(path))


def delete(command: str, resource: str, name: str, *extra: str) -> None:
    subprocess.run(
        [command, "delete", resource, name, *extra, "--ignore-not-found"],
        check=False,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def find_latest_promotion(payload: str, stage_name: str) -> tuple[str, str, str]:
    data = json.loads(payload)
    items = [
        item
        for item in data.get("items", [])
        if item.get("spec", {}).get("stage") == stage_name
    ]
    if not items:
        return "", "", ""
    items.sort(key=lambda item: item.get("metadata", {}).get("creationTimestamp", ""))
    promotion = items[-1]
    metadata = promotion.get("metadata", {})
    status = promotion.get("status", {})
    thread_ts = (
        deep_get(status, "stepExecutionMetadata", 0, "output", "slack", "threadTS")
        or deep_get(status, "state", "step-1", "slack", "threadTS")
        or ""
    )
    return metadata.get("name", ""), status.get("phase", ""), thread_ts


def deep_get(value: object, *path: object) -> str:
    current = value
    for key in path:
        if isinstance(key, int):
            if not isinstance(current, list) or key >= len(current):
                return ""
            current = current[key]
            continue
        if not isinstance(current, dict):
            return ""
        current = current.get(key)
    return current if isinstance(current, str) else ""


def log(level: str, message: str) -> None:
    print(f"[{level}] {message}")


def fail(message: str) -> None:
    log("FAIL", message)
    raise SystemExit(1)
