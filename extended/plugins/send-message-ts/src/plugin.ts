import { createServer as createHTTPServer, type IncomingMessage, type RequestListener, type ServerResponse } from "node:http";
import { BearerTokenAuthorizer } from "./auth.js";
import {
  defaultAuthTokenPath,
  defaultSlackAPIBaseURL,
  defaultSystemResourcesNamespace,
  stepExecutePath,
} from "./constants.js";
import { InClusterKubernetesClient } from "./kubernetes.js";
import { buildSlackPayload, decodeXMLSlackPayload } from "./payload.js";
import { SlackWebAPIClient } from "./slack.js";
import {
  asErrorMessage,
  erroredResponse,
  failedResponse,
  isRequestError,
  RequestError,
} from "./support.js";
import type {
  ChannelResource,
  KubernetesClient,
  SendMessageConfig,
  ServerOptions,
  SlackClient,
  SlackPostMessageResponse,
  Step,
  StepContext,
  StepExecuteRequest,
  StepExecuteResponse,
} from "./types.js";

export { decodeXMLSlackPayload, buildSlackPayload };
export { InClusterKubernetesClient } from "./kubernetes.js";
export { SlackWebAPIClient } from "./slack.js";
export { stepExecutePath } from "./constants.js";
export type {
  ChannelResource,
  KubernetesClient,
  SendMessageConfig,
  ServerOptions,
  SlackClient,
  SlackPostMessageResponse,
  Step,
  StepContext,
  StepExecuteRequest,
  StepExecuteResponse,
} from "./types.js";

export class Server {
  private readonly expectedTokenPath: string;
  private readonly kubeClient: KubernetesClient;
  private readonly slackClient: SlackClient;
  private readonly systemResourcesNamespace: string;
  private readonly authorizer: BearerTokenAuthorizer;

  public constructor(options: ServerOptions = {}) {
    this.expectedTokenPath = options.expectedTokenPath ?? defaultAuthTokenPath;
    this.systemResourcesNamespace =
      options.systemResourcesNamespace?.trim() ||
      process.env.SYSTEM_RESOURCES_NAMESPACE?.trim() ||
      defaultSystemResourcesNamespace;
    this.kubeClient = options.kubeClient ?? new InClusterKubernetesClient();
    this.slackClient =
      options.slackClient ??
      new SlackWebAPIClient(
        options.slackAPIBaseURL?.trim() ||
          process.env.SLACK_API_BASE_URL?.trim() ||
          defaultSlackAPIBaseURL,
      );
    this.authorizer = new BearerTokenAuthorizer(this.expectedTokenPath);
  }

  public handler(): RequestListener {
    return (request, response) => {
      void this.handle(request, response);
    };
  }

  public async execute(
    request: StepExecuteRequest,
  ): Promise<StepExecuteResponse> {
    if (request.step.kind !== "send-message") {
      return erroredResponse(
        `unsupported step kind ${JSON.stringify(request.step.kind)}`,
      );
    }

    try {
      const { channel, secretNamespace } = await this.lookupChannel(request);
      const secret = await this.kubeClient.getSecret(
        secretNamespace,
        channel.secretName,
      );
      const token = secret.apiKey?.trim();
      if (!token) {
        return failedResponse('Slack Secret is missing key "apiKey"');
      }

      const { payload, outputThreadTS } = buildSlackPayload(
        request.step.config,
        channel,
      );
      const slackResponse = await this.slackClient.postMessage(token, payload);
      if (!slackResponse.ok) {
        return failedResponse(
          `Slack API error: ${slackResponse.error ?? "unknown_error"}`,
        );
      }

      return {
        status: "Succeeded",
        output: {
          slack: {
            threadTS: outputThreadTS || slackResponse.ts || "",
          },
        },
      };
    } catch (error) {
      if (isRequestError(error)) {
        return erroredResponse(asErrorMessage(error));
      }
      return failedResponse(asErrorMessage(error));
    }
  }

  private async handle(
    request: IncomingMessage,
    response: ServerResponse,
  ): Promise<void> {
    try {
      const requestPath = new URL(
        request.url ?? "/",
        "http://step-plugin.local",
      ).pathname;
      if (requestPath !== stepExecutePath) {
        response.statusCode = 404;
        response.end();
        return;
      }
      if (request.method !== "POST") {
        response.statusCode = 405;
        response.end();
        return;
      }

      const authResult = await this.authorizer.authorize(request.headers);
      if (authResult !== null) {
        this.writeJSON(response, 403, authResult);
        return;
      }

      const body = await readRequestBody(request);
      let parsedRequest: StepExecuteRequest;
      try {
        parsedRequest = JSON.parse(body) as StepExecuteRequest;
      } catch (error) {
        this.writeJSON(response, 400, {
          status: "Errored",
          message: "invalid request body",
          error: asErrorMessage(error),
          terminal: true,
        });
        return;
      }

      const result = await this.execute(parsedRequest);
      this.writeJSON(response, 200, result);
    } catch (error) {
      this.writeJSON(response, 500, erroredResponse(asErrorMessage(error)));
    }
  }

  private async lookupChannel(
    request: StepExecuteRequest,
  ): Promise<{ channel: ChannelResource; secretNamespace: string }> {
    const ref = request.step.config.channel;
    switch (ref.kind) {
      case "MessageChannel": {
        const project = request.context.project?.trim();
        if (!project) {
          throw new RequestError(
            "step context project is required for MessageChannel",
          );
        }
        return {
          channel: await this.kubeClient.getMessageChannel(project, ref.name),
          secretNamespace: project,
        };
      }
      case "ClusterMessageChannel":
        return {
          channel: await this.kubeClient.getClusterMessageChannel(ref.name),
          secretNamespace: this.systemResourcesNamespace,
        };
      default:
        throw new RequestError(
          `unsupported channel kind ${JSON.stringify(ref.kind)}`,
        );
    }
  }

  private writeJSON(
    response: ServerResponse,
    statusCode: number,
    body: StepExecuteResponse,
  ): void {
    response.setHeader("content-type", "application/json");
    response.statusCode = statusCode;
    response.end(JSON.stringify(body));
  }

  public listen(port = 9765) {
    return createHTTPServer(this.handler()).listen(port);
  }
}

async function readRequestBody(request: IncomingMessage): Promise<string> {
  const chunks: Buffer[] = [];
  for await (const chunk of request) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  }
  return Buffer.concat(chunks).toString("utf8");
}
