from __future__ import annotations

import json
import xml.etree.ElementTree as ET
from typing import Any

import yaml

from .models import ChannelResource, RequestError
from .util import as_dict, string_value


def build_slack_payload(
    config: dict[str, Any],
    channel: ChannelResource,
) -> tuple[dict[str, Any], str]:
    encoding_type = string_value(config.get("encodingType"))
    slack_config = as_dict(config.get("slack"))

    if not encoding_type:
        channel_id = string_value(slack_config.get("channelID")) or channel.slack_channel_id
        if not channel_id:
            raise RequestError(
                f'{channel.resource_kind} "{channel.resource_name}" does not define '
                "spec.slack.channelID and config.slack.channelID is empty"
            )
        payload: dict[str, Any] = {
            "channel": channel_id,
            "text": string_value(config.get("message")),
        }
        thread_ts = string_value(slack_config.get("threadTS"))
        if thread_ts:
            payload["thread_ts"] = thread_ts
        return payload, thread_ts

    payload = decode_encoded_payload(encoding_type, string_value(config.get("message")))
    if "channel" not in payload:
        if not channel.slack_channel_id:
            raise RequestError(
                f'{channel.resource_kind} "{channel.resource_name}" does not define '
                "spec.slack.channelID"
            )
        payload["channel"] = channel.slack_channel_id
    return payload, string_value(payload.get("thread_ts"))


def decode_encoded_payload(
    encoding_type: str,
    message: str,
) -> dict[str, Any]:
    if encoding_type == "json":
        try:
            payload = json.loads(message)
        except json.JSONDecodeError as exc:
            raise RequestError(f"error decoding JSON Slack payload: {exc}") from exc
    elif encoding_type == "yaml":
        try:
            payload = yaml.safe_load(message)
        except yaml.YAMLError as exc:
            raise RequestError(f"error decoding YAML Slack payload: {exc}") from exc
    elif encoding_type == "xml":
        payload = decode_xml_slack_payload(message)
    else:
        raise RequestError(f'unsupported encodingType "{encoding_type}"')

    if not isinstance(payload, dict):
        raise RequestError("Slack payload must decode to an object")
    return payload


def decode_xml_slack_payload(message: str) -> dict[str, Any]:
    try:
        root = ET.fromstring(message)
    except ET.ParseError as exc:
        raise RequestError(f"error decoding XML Slack payload: {exc}") from exc
    payload = xml_node_to_object(root)
    if not isinstance(payload, dict):
        raise RequestError("Slack payload must decode to an object")
    return payload


def xml_node_to_object(node: ET.Element) -> dict[str, Any]:
    payload: dict[str, Any] = dict(node.attrib)
    for child in list(node):
        append_xml_value(payload, child.tag, xml_node_value(child))

    text = normalized_text(node.text)
    if text:
        payload["#text" if payload else "text"] = text
    return payload


def xml_node_value(node: ET.Element) -> Any:
    if not node.attrib and not list(node):
        return normalized_text(node.text)
    return xml_node_to_object(node)


def append_xml_value(payload: dict[str, Any], key: str, value: Any) -> None:
    if key not in payload:
        payload[key] = value
        return
    existing = payload[key]
    if isinstance(existing, list):
        existing.append(value)
        return
    payload[key] = [existing, value]


def normalized_text(value: str | None) -> str:
    return (value or "").strip()
