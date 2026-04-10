#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 3 ]]; then
    echo "usage: $0 OUT_DIR IMAGE SYSTEM_RESOURCES_NAMESPACE" >&2
    exit 1
fi

OUT_DIR="$1"
IMAGE="$2"
SYSTEM_RESOURCES_NAMESPACE="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

mkdir -p "$OUT_DIR"
cp "$PLUGIN_DIR/plugin.yaml" "$OUT_DIR/plugin.yaml"

python3 - "$OUT_DIR/plugin.yaml" "$IMAGE" "$SYSTEM_RESOURCES_NAMESPACE" <<'PY'
import pathlib
import sys

plugin_yaml = pathlib.Path(sys.argv[1])
image = sys.argv[2]
system_ns = sys.argv[3]

data = plugin_yaml.read_text()
data = data.replace("namespace: kargo-system-resources", f"namespace: {system_ns}")
data = data.replace("image: send-message-step-plugin:dev", f"image: {image}")
data = data.replace("value: kargo-system-resources", f"value: {system_ns}")
plugin_yaml.write_text(data)
PY
