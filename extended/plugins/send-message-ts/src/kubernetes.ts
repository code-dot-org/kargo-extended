import { readFileSync } from "node:fs";
import { request as httpsRequest } from "node:https";
import { join } from "node:path";

import {
  apiGroup,
  apiVersion,
  bearerPrefix,
  defaultKubernetesBaseURL,
  defaultServiceAccountDir,
} from "./constants.js";
import { asErrorMessage, asRecord, requiredString } from "./support.js";
import type { ChannelResource, KubernetesClient } from "./types.js";

export class InClusterKubernetesClient implements KubernetesClient {
  private readonly baseURL: URL;
  private readonly ca: string;
  private readonly token: string;

  public constructor(serviceAccountDir = defaultServiceAccountDir) {
    const host = process.env.KUBERNETES_SERVICE_HOST?.trim();
    const port = process.env.KUBERNETES_SERVICE_PORT?.trim();
    const baseURL =
      host && port
        ? `https://${host}:${port}`
        : host
          ? `https://${host}`
          : defaultKubernetesBaseURL;

    this.baseURL = new URL(baseURL);
    this.token = readFileSync(join(serviceAccountDir, "token"), "utf8").trim();
    this.ca = readFileSync(join(serviceAccountDir, "ca.crt"), "utf8");
  }

  public async getMessageChannel(
    namespace: string,
    name: string,
  ): Promise<ChannelResource> {
    const resource = await this.requestResource(
      `/apis/${apiGroup}/${apiVersion}/namespaces/${encodeURIComponent(namespace)}/messagechannels/${encodeURIComponent(name)}`,
      `message channel ${namespace}/${name}`,
    );
    return parseChannelResource(resource, "MessageChannel");
  }

  public async getClusterMessageChannel(name: string): Promise<ChannelResource> {
    const resource = await this.requestResource(
      `/apis/${apiGroup}/${apiVersion}/clustermessagechannels/${encodeURIComponent(name)}`,
      `cluster message channel ${name}`,
    );
    return parseChannelResource(resource, "ClusterMessageChannel");
  }

  public async getSecret(
    namespace: string,
    name: string,
  ): Promise<Record<string, string>> {
    const resource = await this.requestResource(
      `/api/v1/namespaces/${encodeURIComponent(namespace)}/secrets/${encodeURIComponent(name)}`,
      `Secret ${namespace}/${name}`,
    );
    const data = asRecord(resource.data, `Secret ${namespace}/${name}.data`);

    return Object.fromEntries(
      Object.entries(data).map(([key, value]) => {
        if (typeof value !== "string") {
          throw new Error(
            `Secret ${namespace}/${name}.data.${key} must be a base64 string`,
          );
        }
        return [key, Buffer.from(value, "base64").toString("utf8")];
      }),
    );
  }

  private async requestResource(
    path: string,
    displayName: string,
  ): Promise<Record<string, unknown>> {
    const { statusCode, body } = await requestJSON({
      ca: this.ca,
      headers: {
        Authorization: `${bearerPrefix}${this.token}`,
        Accept: "application/json",
      },
      host: this.baseURL.hostname,
      method: "GET",
      path,
      port: this.baseURL.port || "443",
    });

    if (statusCode === 404) {
      throw new Error(`${displayName} not found`);
    }
    if (statusCode >= 400) {
      throw new Error(
        `error reading ${displayName} from Kubernetes API: ${statusCode} ${body}`.trim(),
      );
    }

    try {
      return asRecord(JSON.parse(body), displayName);
    } catch (error) {
      throw new Error(
        `error decoding ${displayName} response: ${asErrorMessage(error)}`,
      );
    }
  }
}

function parseChannelResource(
  resource: Record<string, unknown>,
  fallbackKind: string,
): ChannelResource {
  const metadata = asRecord(resource.metadata, "metadata");
  const spec = asRecord(resource.spec, "spec");
  const secretRef = asRecord(spec.secretRef, "spec.secretRef");
  const slack = asRecord(spec.slack, "spec.slack");

  return {
    secretName: requiredString(secretRef, "name", "spec.secretRef.name"),
    slackChannelID: requiredString(slack, "channelID", "spec.slack.channelID"),
    resourceKind:
      typeof resource.kind === "string" ? resource.kind : fallbackKind,
    resourceName: requiredString(metadata, "name", "metadata.name"),
    resourceNamespace:
      typeof metadata.namespace === "string" ? metadata.namespace : undefined,
  };
}

async function requestJSON(options: {
  ca: string;
  headers: Record<string, string>;
  host: string;
  method: string;
  path: string;
  port: string;
}): Promise<{ body: string; statusCode: number }> {
  return new Promise((resolve, reject) => {
    const request = httpsRequest(
      {
        ca: options.ca,
        headers: options.headers,
        host: options.host,
        method: options.method,
        path: options.path,
        port: options.port,
      },
      (response) => {
        const chunks: Buffer[] = [];
        response.on("data", (chunk) => {
          chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
        });
        response.on("end", () => {
          resolve({
            body: Buffer.concat(chunks).toString("utf8"),
            statusCode: response.statusCode ?? 500,
          });
        });
      },
    );

    request.on("error", reject);
    request.end();
  });
}

