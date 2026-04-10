import { XMLParser } from "fast-xml-parser";
import { parse as parseYAML } from "yaml";

import { asErrorMessage, asRecord, RequestError } from "./support.js";
import type { ChannelResource, SendMessageConfig } from "./types.js";

const xmlParser = new XMLParser({
  attributeNamePrefix: "",
  ignoreAttributes: false,
  parseAttributeValue: false,
  parseTagValue: false,
  textNodeName: "#text",
  trimValues: true,
});

export function buildSlackPayload(
  config: SendMessageConfig,
  channel: ChannelResource,
): {
  payload: Record<string, unknown>;
  outputThreadTS: string;
} {
  const encoding = config.encodingType?.trim() ?? "";

  if (encoding === "") {
    return buildPlaintextPayload(config, channel);
  }

  const payload = parseEncodedPayload(encoding, config.message);
  if (!Object.hasOwn(payload, "channel")) {
    const channelID = channel.slackChannelID.trim();
    if (!channelID) {
      throw new RequestError(
        `${channel.resourceKind} ${JSON.stringify(channel.resourceName)} does not define spec.slack.channelID`,
      );
    }
    payload.channel = channelID;
  }

  return {
    payload,
    outputThreadTS:
      typeof payload.thread_ts === "string" ? payload.thread_ts : "",
  };
}

export function decodeXMLSlackPayload(message: string): Record<string, unknown> {
  let parsed: unknown;
  try {
    parsed = xmlParser.parse(message);
  } catch (error) {
    throw new RequestError(
      `error decoding XML Slack payload: ${asErrorMessage(error)}`,
    );
  }

  const root = asRecord(parsed, "Slack XML payload");
  const rootValue = Object.values(root)[0];
  if (rootValue === undefined) {
    throw new RequestError("Slack payload must decode to an object");
  }

  const normalized = normalizeXMLRoot(rootValue);
  return asRecord(normalized, "Slack XML payload");
}

function buildPlaintextPayload(
  config: SendMessageConfig,
  channel: ChannelResource,
) {
  const channelID =
    config.slack?.channelID?.trim() || channel.slackChannelID.trim();
  if (!channelID) {
    throw new RequestError(
      `${channel.resourceKind} ${JSON.stringify(channel.resourceName)} does not define spec.slack.channelID and config.slack.channelID is empty`,
    );
  }

  const payload: Record<string, unknown> = {
    channel: channelID,
    text: config.message,
  };
  const threadTS = config.slack?.threadTS?.trim() ?? "";
  if (threadTS) {
    payload.thread_ts = threadTS;
  }

  return {
    payload,
    outputThreadTS: threadTS,
  };
}

function parseEncodedPayload(
  encoding: string,
  message: string,
): Record<string, unknown> {
  let decoded: unknown;

  switch (encoding) {
    case "json":
      try {
        decoded = JSON.parse(message);
      } catch (error) {
        throw new RequestError(
          `error decoding JSON Slack payload: ${asErrorMessage(error)}`,
        );
      }
      break;
    case "yaml":
      try {
        decoded = parseYAML(message);
      } catch (error) {
        throw new RequestError(
          `error decoding YAML Slack payload: ${asErrorMessage(error)}`,
        );
      }
      break;
    case "xml":
      decoded = decodeXMLSlackPayload(message);
      break;
    default:
      throw new RequestError(
        `unsupported encodingType ${JSON.stringify(encoding)}`,
      );
  }

  return asRecord(decoded, "Slack payload");
}

function normalizeXMLRoot(value: unknown): Record<string, unknown> | unknown {
  if (isRecord(value)) {
    const normalized = normalizeXMLValue(value);
    if (isRecord(normalized)) {
      return normalized;
    }
    return { text: stringifyXMLScalar(normalized) };
  }
  return { text: stringifyXMLScalar(value) };
}

function normalizeXMLValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => normalizeXMLValue(item));
  }
  if (!isRecord(value)) {
    return stringifyXMLScalar(value);
  }

  const result: Record<string, unknown> = {};
  for (const [key, rawValue] of Object.entries(value)) {
    const normalized = normalizeXMLValue(rawValue);
    if (key === "#text" && normalized === "") {
      continue;
    }
    result[key] = normalized;
  }

  const keys = Object.keys(result);
  if (keys.length === 1 && keys[0] === "#text") {
    return result["#text"];
  }
  return result;
}

function stringifyXMLScalar(value: unknown): string {
  if (typeof value === "string") {
    return value.trim();
  }
  if (value === null || value === undefined) {
    return "";
  }
  return String(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
