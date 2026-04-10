import type { RequestListener } from "node:http";

export interface StepExecuteRequest {
  context: StepContext;
  step: Step;
}

export interface StepContext {
  project?: string;
}

export interface Step {
  kind: string;
  config: SendMessageConfig;
}

export interface SendMessageConfig {
  channel: ChannelRef;
  message: string;
  encodingType?: string;
  slack?: SlackOptions;
}

export interface ChannelRef {
  kind: string;
  name: string;
}

export interface SlackOptions {
  channelID?: string;
  threadTS?: string;
}

export interface StepExecuteResponse {
  status: "Succeeded" | "Failed" | "Errored";
  message?: string;
  output?: Record<string, unknown>;
  error?: string;
  terminal?: boolean;
}

export interface ChannelResource {
  secretName: string;
  slackChannelID: string;
  resourceKind: string;
  resourceName: string;
  resourceNamespace?: string;
}

export interface SlackPostMessageResponse {
  ok: boolean;
  error?: string;
  ts?: string;
}

export interface KubernetesClient {
  getMessageChannel(
    namespace: string,
    name: string,
  ): Promise<ChannelResource>;
  getClusterMessageChannel(name: string): Promise<ChannelResource>;
  getSecret(
    namespace: string,
    name: string,
  ): Promise<Record<string, string>>;
}

export interface SlackClient {
  postMessage(
    token: string,
    payload: Record<string, unknown>,
  ): Promise<SlackPostMessageResponse>;
}

export interface ServerOptions {
  expectedTokenPath?: string;
  kubeClient?: KubernetesClient;
  slackAPIBaseURL?: string;
  slackClient?: SlackClient;
  systemResourcesNamespace?: string;
}

export interface StepPluginServer {
  execute(request: StepExecuteRequest): Promise<StepExecuteResponse>;
  handler(): RequestListener;
}

