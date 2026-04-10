import assert from "node:assert/strict";
import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { createServer } from "node:http";

import {
  type ChannelResource,
  buildSlackPayload,
  decodeXMLSlackPayload,
  type KubernetesClient,
  type SlackClient,
  type SlackPostMessageResponse,
  type StepExecuteResponse,
  type StepExecuteRequest,
  Server,
  stepExecutePath,
} from "../src/plugin.js";

class FakeKubernetesClient implements KubernetesClient {
  public readonly clusterMessageChannels = new Map<string, ChannelResource>();
  public readonly messageChannels = new Map<string, ChannelResource>();
  public readonly secrets = new Map<string, Record<string, string>>();
  public readonly secretLookups: string[] = [];

  public async getMessageChannel(
    namespace: string,
    name: string,
  ): Promise<ChannelResource> {
    const channel = this.messageChannels.get(`${namespace}/${name}`);
    if (!channel) {
      throw new Error("message channel not found");
    }
    return channel;
  }

  public async getClusterMessageChannel(name: string): Promise<ChannelResource> {
    const channel = this.clusterMessageChannels.get(name);
    if (!channel) {
      throw new Error("cluster message channel not found");
    }
    return channel;
  }

  public async getSecret(
    namespace: string,
    name: string,
  ): Promise<Record<string, string>> {
    const key = `${namespace}/${name}`;
    this.secretLookups.push(key);
    const secret = this.secrets.get(key);
    if (!secret) {
      throw new Error("secret not found");
    }
    return secret;
  }
}

class FakeSlackClient implements SlackClient {
  public lastPayload: Record<string, unknown> | undefined;
  public response: SlackPostMessageResponse = {
    ok: true,
    ts: "1712345678.000100",
  };

  public async postMessage(
    _token: string,
    payload: Record<string, unknown>,
  ): Promise<SlackPostMessageResponse> {
    this.lastPayload = payload;
    return this.response;
  }
}

test("handler rejects a missing bearer token", async () => {
  const server = await newTestServer();
  await withHTTPServer(server, async (baseURL) => {
    const response = await fetch(`${baseURL}${stepExecutePath}`, {
      body: JSON.stringify(minimalRequest()),
      headers: {
        "content-type": "application/json",
      },
      method: "POST",
    });

    assert.equal(response.status, 403);
    const body = (await response.json()) as Record<string, unknown>;
    assert.equal(body.status, "Errored");
    assert.equal(body.error, "missing bearer token");
  });
});

test("handler rejects an invalid bearer token", async () => {
  const server = await newTestServer();
  await withHTTPServer(server, async (baseURL) => {
    const response = await fetch(`${baseURL}${stepExecutePath}`, {
      body: JSON.stringify(minimalRequest()),
      headers: {
        authorization: "Bearer wrong-token",
        "content-type": "application/json",
      },
      method: "POST",
    });

    assert.equal(response.status, 403);
    const body = (await response.json()) as Record<string, unknown>;
    assert.equal(body.status, "Errored");
    assert.equal(body.error, "invalid bearer token");
  });
});

test("handler rejects an invalid request body", async () => {
  const server = await newTestServer();
  await withHTTPServer(server, async (baseURL) => {
    const response = await fetch(`${baseURL}${stepExecutePath}`, {
      body: "{not-json",
      headers: {
        authorization: "Bearer expected-token",
        "content-type": "application/json",
      },
      method: "POST",
    });

    assert.equal(response.status, 400);
    const body = (await response.json()) as Record<string, unknown>;
    assert.equal(body.status, "Errored");
    assert.equal(body.message, "invalid request body");
  });
});

test("execute uses MessageChannel and its Secret namespace", async () => {
  const kube = new FakeKubernetesClient();
  kube.messageChannels.set("demo/send", {
    resourceKind: "MessageChannel",
    resourceName: "send",
    resourceNamespace: "demo",
    secretName: "slack-token",
    slackChannelID: "C123",
  });
  kube.secrets.set("demo/slack-token", { apiKey: "xoxb-demo" });

  const slack = new FakeSlackClient();
  const server = await newTestServer({ kubeClient: kube, slackClient: slack });

  const request = minimalRequest();
  request.context.project = "demo";

  const response = await server.execute(request);

  assert.equal(response.status, "Succeeded");
  assert.deepEqual(slack.lastPayload, {
    channel: "C123",
    text: "hello from plugin",
  });
  assert.deepEqual(kube.secretLookups, ["demo/slack-token"]);
  assert.equal(readThreadTS(response), "1712345678.000100");
});

test("execute uses ClusterMessageChannel and system resources Secret", async () => {
  const kube = new FakeKubernetesClient();
  kube.clusterMessageChannels.set("send", {
    resourceKind: "ClusterMessageChannel",
    resourceName: "send",
    secretName: "slack-token",
    slackChannelID: "C777",
  });
  kube.secrets.set("kargo-system-resources/slack-token", { apiKey: "xoxb-demo" });

  const slack = new FakeSlackClient();
  slack.response = {
    ok: true,
    ts: "1712345678.000200",
  };
  const server = await newTestServer({ kubeClient: kube, slackClient: slack });

  const request = minimalRequest();
  request.context.project = "demo";
  request.step.config.channel = {
    kind: "ClusterMessageChannel",
    name: "send",
  };

  const response = await server.execute(request);

  assert.equal(response.status, "Succeeded");
  assert.deepEqual(slack.lastPayload, {
    channel: "C777",
    text: "hello from plugin",
  });
  assert.deepEqual(kube.secretLookups, ["kargo-system-resources/slack-token"]);
});

test("plaintext mode honors config.slack overrides", () => {
  const result = buildSlackPayload(
    {
      channel: {
        kind: "MessageChannel",
        name: "send",
      },
      message: "hello from plugin",
      slack: {
        channelID: "C999",
        threadTS: "1700000000.000001",
      },
    },
    {
      resourceKind: "MessageChannel",
      resourceName: "send",
      secretName: "slack-token",
      slackChannelID: "C123",
    },
  );

  assert.deepEqual(result.payload, {
    channel: "C999",
    text: "hello from plugin",
    thread_ts: "1700000000.000001",
  });
  assert.equal(result.outputThreadTS, "1700000000.000001");
});

test("encoded mode ignores config.slack and fills channel from the resource", async () => {
  const kube = new FakeKubernetesClient();
  kube.messageChannels.set("demo/send", {
    resourceKind: "MessageChannel",
    resourceName: "send",
    resourceNamespace: "demo",
    secretName: "slack-token",
    slackChannelID: "C123",
  });
  kube.secrets.set("demo/slack-token", { apiKey: "xoxb-demo" });

  const slack = new FakeSlackClient();
  slack.response = {
    ok: true,
    ts: "1712345678.000300",
  };
  const server = await newTestServer({ kubeClient: kube, slackClient: slack });

  const request = minimalRequest();
  request.context.project = "demo";
  request.step.config.encodingType = "json";
  request.step.config.message =
    '{"text":"rich","thread_ts":"1700000000.000002","blocks":[{"type":"section"}]}';
  request.step.config.slack = {
    channelID: "C999",
    threadTS: "1700000000.000001",
  };

  const response = await server.execute(request);

  assert.equal(response.status, "Succeeded");
  assert.deepEqual(slack.lastPayload, {
    blocks: [{ type: "section" }],
    channel: "C123",
    text: "rich",
    thread_ts: "1700000000.000002",
  });
  assert.equal(readThreadTS(response), "1700000000.000002");
});

test("execute supports YAML payloads", async () => {
  const kube = new FakeKubernetesClient();
  kube.messageChannels.set("demo/send", {
    resourceKind: "MessageChannel",
    resourceName: "send",
    resourceNamespace: "demo",
    secretName: "slack-token",
    slackChannelID: "C123",
  });
  kube.secrets.set("demo/slack-token", { apiKey: "xoxb-demo" });

  const slack = new FakeSlackClient();
  slack.response = {
    ok: true,
    ts: "1712345678.000410",
  };
  const server = await newTestServer({ kubeClient: kube, slackClient: slack });

  const request = minimalRequest();
  request.context.project = "demo";
  request.step.config.encodingType = "yaml";
  request.step.config.message = "text: rich\nblocks:\n- type: section\n";

  const response = await server.execute(request);

  assert.equal(response.status, "Succeeded");
  assert.deepEqual(slack.lastPayload, {
    blocks: [{ type: "section" }],
    channel: "C123",
    text: "rich",
  });
  assert.equal(readThreadTS(response), "1712345678.000410");
});

test("execute supports XML payloads", async () => {
  const kube = new FakeKubernetesClient();
  kube.messageChannels.set("demo/send", {
    resourceKind: "MessageChannel",
    resourceName: "send",
    resourceNamespace: "demo",
    secretName: "slack-token",
    slackChannelID: "C123",
  });
  kube.secrets.set("demo/slack-token", { apiKey: "xoxb-demo" });

  const slack = new FakeSlackClient();
  slack.response = {
    ok: true,
    ts: "1712345678.000500",
  };
  const server = await newTestServer({ kubeClient: kube, slackClient: slack });

  const request = minimalRequest();
  request.context.project = "demo";
  request.step.config.encodingType = "xml";
  request.step.config.message = `
<message>
  <text>rich</text>
  <thread_ts>1700000000.000003</thread_ts>
</message>`;

  const response = await server.execute(request);

  assert.equal(response.status, "Succeeded");
  assert.deepEqual(slack.lastPayload, {
    channel: "C123",
    text: "rich",
    thread_ts: "1700000000.000003",
  });
  assert.equal(readThreadTS(response), "1700000000.000003");
});

test("execute returns Errored for bad YAML", async () => {
  const kube = new FakeKubernetesClient();
  kube.messageChannels.set("demo/send", {
    resourceKind: "MessageChannel",
    resourceName: "send",
    resourceNamespace: "demo",
    secretName: "slack-token",
    slackChannelID: "C123",
  });
  kube.secrets.set("demo/slack-token", { apiKey: "xoxb-demo" });

  const server = await newTestServer({ kubeClient: kube });
  const request = minimalRequest();
  request.context.project = "demo";
  request.step.config.encodingType = "yaml";
  request.step.config.message = "text: [oops";

  const response = await server.execute(request);

  assert.equal(response.status, "Errored");
  assert.match(String(response.error), /error decoding YAML Slack payload/);
});

test("decodeXMLSlackPayload preserves the Kargo-owned XML mapping", () => {
  const payload = decodeXMLSlackPayload(`
<message icon_emoji=":wave:">
  <text>rich</text>
  <blocks>
    <type>section</type>
    <text>
      <type>mrkdwn</type>
      <text>*hello*</text>
    </text>
  </blocks>
  <blocks>
    <type>divider</type>
  </blocks>
</message>`);

  assert.deepEqual(payload, {
    blocks: [
      {
        text: {
          text: "*hello*",
          type: "mrkdwn",
        },
        type: "section",
      },
      {
        type: "divider",
      },
    ],
    icon_emoji: ":wave:",
    text: "rich",
  });
});

test("execute surfaces Slack failures", async () => {
  const kube = new FakeKubernetesClient();
  kube.messageChannels.set("demo/send", {
    resourceKind: "MessageChannel",
    resourceName: "send",
    resourceNamespace: "demo",
    secretName: "slack-token",
    slackChannelID: "C123",
  });
  kube.secrets.set("demo/slack-token", { apiKey: "xoxb-demo" });

  const slack = new FakeSlackClient();
  slack.response = {
    ok: false,
    error: "channel_not_found",
  };
  const server = await newTestServer({ kubeClient: kube, slackClient: slack });

  const request = minimalRequest();
  request.context.project = "demo";

  const response = await server.execute(request);

  assert.equal(response.status, "Failed");
  assert.match(String(response.error), /channel_not_found/);
});

test("execute returns Failed when the Slack Secret lookup fails", async () => {
  const kube = new FakeKubernetesClient();
  kube.messageChannels.set("demo/send", {
    resourceKind: "MessageChannel",
    resourceName: "send",
    resourceNamespace: "demo",
    secretName: "slack-token",
    slackChannelID: "C123",
  });

  const server = await newTestServer({ kubeClient: kube });
  const request = minimalRequest();
  request.context.project = "demo";

  const response = await server.execute(request);

  assert.equal(response.status, "Failed");
  assert.match(String(response.error), /secret not found/);
});

async function newTestServer(options: {
  kubeClient?: KubernetesClient;
  slackClient?: SlackClient;
} = {}) {
  const tokenDir = await mkdtemp(join(tmpdir(), "send-message-ts-test-"));
  const tokenPath = join(tokenDir, "token");
  await writeFile(tokenPath, "expected-token", "utf8");

  return new Server({
    expectedTokenPath: tokenPath,
    kubeClient: options.kubeClient ?? new FakeKubernetesClient(),
    slackClient: options.slackClient ?? new FakeSlackClient(),
    systemResourcesNamespace: "kargo-system-resources",
  });
}

async function withHTTPServer(
  pluginServer: Server,
  run: (baseURL: string) => Promise<void>,
) {
  const httpServer = createServer(pluginServer.handler());
  await new Promise<void>((resolve) => {
    httpServer.listen(0, "127.0.0.1", resolve);
  });

  const address = httpServer.address();
  assert.ok(address && typeof address === "object");

  try {
    await run(`http://127.0.0.1:${address.port}`);
  } finally {
    await new Promise<void>((resolve, reject) => {
      httpServer.close((error) => {
        if (error) {
          reject(error);
          return;
        }
        resolve();
      });
    });
  }
}

function minimalRequest(): StepExecuteRequest {
  return {
    context: {},
    step: {
      kind: "send-message",
      config: {
        channel: {
          kind: "MessageChannel",
          name: "send",
        },
        message: "hello from plugin",
      },
    },
  };
}

function readThreadTS(response: StepExecuteResponse) {
  const output = response.output as Record<string, unknown>;
  const slack = output.slack as Record<string, unknown>;
  return String(slack.threadTS ?? "");
}
