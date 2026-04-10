import assert from "node:assert/strict";
import { cp, mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { setTimeout as delay } from "node:timers/promises";

const pluginDir = resolve(import.meta.dirname, "..", "..");

const kargoBin = process.env.KARGO_BIN || "kargo";
const kubectlBin = process.env.KUBECTL_BIN || "kubectl";
const dockerBin = process.env.DOCKER_BIN || "docker";
const kindBin = process.env.KIND_BIN || "kind";

const systemResourcesNamespace =
  process.env.SEND_MESSAGE_SMOKE_SYSTEM_RESOURCES_NAMESPACE ||
  "kargo-system-resources";
const secretName =
  process.env.SEND_MESSAGE_SMOKE_SECRET_NAME || "send-message-slack-token";
const channelName =
  process.env.SEND_MESSAGE_SMOKE_CHANNEL_NAME || "send-message-smoke";

const project = requireEnv("SEND_MESSAGE_SMOKE_PROJECT");
const warehouse = requireEnv("SEND_MESSAGE_SMOKE_WAREHOUSE");
const freightName = requireEnv("SEND_MESSAGE_SMOKE_FREIGHT_NAME");
const slackAPIKey = requireEnv("SEND_MESSAGE_SMOKE_SLACK_API_KEY");
const slackChannelID = requireEnv("SEND_MESSAGE_SMOKE_CHANNEL_ID");

const stageName = `smsg-ts-${Date.now()}`;
const clusterRoleBindingName = `send-message-step-plugin-reader-${project}`;
const imageTag =
  process.env.SEND_MESSAGE_SMOKE_IMAGE ||
  `send-message-step-plugin-ts:e2e-${Date.now()}`;

let buildDir = "";

try {
  const kindName = currentKindCluster();
  buildDir = await mkdtemp(join(tmpdir(), "send-message-ts-smoke-"));

  logInfo(`build send-message StepPlugin image ${imageTag}`);
  runCommand(dockerBin, ["build", "-t", imageTag, pluginDir]);
  logPass("build image");

  logInfo(`load ${imageTag} into kind cluster ${kindName}`);
  runCommand(kindBin, ["load", "docker-image", "--name", kindName, imageTag]);
  logPass("load image");

  logInfo("render plugin build dir");
  await renderPluginBuildDir(buildDir, imageTag, systemResourcesNamespace);
  logPass("render plugin build dir");

  logInfo("build StepPlugin ConfigMap");
  runCommand(kargoBin, ["step-plugin", "build", "."], { cwd: buildDir });
  const configMapPath = await findConfigMapPath(buildDir);
  logPass("build StepPlugin ConfigMap");

  logInfo("install CRDs and RBAC");
  runCommand(kubectlBin, ["apply", "-f", join(pluginDir, "manifests", "crds.yaml")]);
  runCommand(kubectlBin, ["apply", "-f", join(pluginDir, "manifests", "rbac.yaml")]);
  runCommand(kubectlBin, ["apply", "-f", "-"], {
    input: yaml(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ${clusterRoleBindingName}
subjects:
- kind: ServiceAccount
  name: default
  namespace: ${project}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: send-message-step-plugin-reader
`),
  });
  logPass("install CRDs and RBAC");

  logInfo("install StepPlugin ConfigMap");
  runCommand(kubectlBin, ["apply", "-f", configMapPath]);
  logPass("install StepPlugin ConfigMap");

  logInfo("create smoke Secret and MessageChannel");
  runCommand(kubectlBin, ["apply", "-f", "-"], {
    input: yaml(`
apiVersion: v1
kind: Secret
metadata:
  name: ${secretName}
  namespace: ${project}
type: Opaque
stringData:
  apiKey: ${slackAPIKey}
---
apiVersion: ee.kargo.akuity.io/v1alpha1
kind: MessageChannel
metadata:
  name: ${channelName}
  namespace: ${project}
spec:
  secretRef:
    name: ${secretName}
  slack:
    channelID: ${slackChannelID}
`),
  });
  logPass("create smoke Secret and MessageChannel");

  logInfo("create smoke Stage");
  runCommand(kargoBin, ["apply", "-f", "-"], {
    input: yaml(`
apiVersion: kargo.akuity.io/v1alpha1
kind: Stage
metadata:
  name: ${stageName}
  namespace: ${project}
spec:
  requestedFreight:
  - origin:
      kind: Warehouse
      name: ${warehouse}
    sources:
      direct: true
  promotionTemplate:
    spec:
      steps:
      - uses: send-message
        config:
          channel:
            kind: MessageChannel
            name: ${channelName}
          message: TypeScript send-message StepPlugin smoke from ${stageName}
`),
  });
  logPass("create smoke Stage");

  logInfo("approve and promote freight");
  runCommand(kargoBin, [
    "approve",
    `--project=${project}`,
    `--freight=${freightName}`,
    `--stage=${stageName}`,
  ]);
  runPromoteCommand([
    "promote",
    `--project=${project}`,
    `--freight=${freightName}`,
    `--stage=${stageName}`,
  ]);
  logPass("approve and promote freight");

  logInfo("wait for Promotion success");
  const promotion = await waitForPromotion(project, stageName);
  const threadTS = readThreadTS(promotion);
  assert.notEqual(threadTS, "", "expected non-empty slack.threadTS");
  logPass(`promotion succeeded with slack.threadTS=${threadTS}`);
} finally {
  await cleanup().catch((error: unknown) => {
    logInfo(`cleanup warning: ${String(error)}`);
  });
}

async function renderPluginBuildDir(
  outDir: string,
  image: string,
  systemNamespace: string,
) {
  await cp(join(pluginDir, "plugin.yaml"), join(outDir, "plugin.yaml"));
  const pluginYAML = await readFile(join(outDir, "plugin.yaml"), "utf8");
  const rendered = pluginYAML
    .replace("namespace: kargo-system-resources", `namespace: ${systemNamespace}`)
    .replace("image: send-message-step-plugin-ts:dev", `image: ${image}`)
    .replace("value: kargo-system-resources", `value: ${systemNamespace}`);
  await writeFile(join(outDir, "plugin.yaml"), rendered, "utf8");
}

async function findConfigMapPath(outDir: string) {
  const entries = await readdir(outDir);
  const configMaps = entries.filter((entry) => entry.endsWith("-configmap.yaml"));
  assert.equal(configMaps.length, 1, "expected exactly one generated ConfigMap");
  return join(outDir, configMaps[0]);
}

function currentKindCluster() {
  const context = runCommand(kubectlBin, ["config", "current-context"]).trim();
  assert.match(
    context,
    /^kind-/,
    `send-message-ts smoke requires a kind context, got ${context}`,
  );
  return context.slice("kind-".length);
}

function runPromoteCommand(args: string[]) {
  const result = spawnSync(kargoBin, args, {
    encoding: "utf8",
    env: process.env,
  });
  if (result.stdout) {
    process.stdout.write(result.stdout);
  }
  if (result.stderr) {
    process.stderr.write(result.stderr);
  }

  const output = `${result.stdout || ""}${result.stderr || ""}`;
  if (result.status === 0) {
    return;
  }
  if (
    output.includes(
      "panic: runtime error: invalid memory address or nil pointer dereference",
    )
  ) {
    logInfo(
      "kargo promote hit the known printer panic after submit; continuing and verifying via Promotion state",
    );
    return;
  }

  throw new Error(
    `command failed (${kargoBin} ${args.join(" ")}): ${output.trim()}`,
  );
}

async function waitForPromotion(projectName: string, stage: string) {
  for (let attempt = 0; attempt < 90; attempt += 1) {
    const output = runCommand(kubectlBin, [
      "get",
      "promotion.kargo.akuity.io",
      "-n",
      projectName,
      "-o",
      "json",
    ]);
    const promotionList = JSON.parse(output) as {
      items?: Array<Record<string, unknown>>;
    };
    const matchingPromotions = (promotionList.items || [])
      .filter(
        (item) =>
          ((item.spec as Record<string, unknown> | undefined)?.stage as string | undefined) ===
          stage,
      )
      .sort((left, right) => {
        const leftTime =
          ((left.metadata as Record<string, unknown> | undefined)?.creationTimestamp as
            | string
            | undefined) || "";
        const rightTime =
          ((right.metadata as Record<string, unknown> | undefined)?.creationTimestamp as
            | string
            | undefined) || "";
        return leftTime.localeCompare(rightTime);
      });

    const promotion = matchingPromotions.at(-1);
    if (promotion) {
      const phase =
        ((promotion.status as Record<string, unknown> | undefined)?.phase as
          | string
          | undefined) || "";
      if (phase === "Succeeded") {
        return promotion;
      }
      if (["Failed", "Errored", "Aborted"].includes(phase)) {
        throw new Error(`promotion ended in phase ${phase}`);
      }
    }

    await delay(2000);
  }

  throw new Error(`timed out waiting for Promotion for Stage ${stage}`);
}

function readThreadTS(promotion: Record<string, unknown>) {
  const status = promotion.status as Record<string, unknown> | undefined;
  const metadata = status?.stepExecutionMetadata as
    | Array<Record<string, unknown>>
    | undefined;
  const metadataThreadTS = metadata?.[0]?.output as Record<string, unknown> | undefined;
  const metadataSlack = metadataThreadTS?.slack as Record<string, unknown> | undefined;
  if (typeof metadataSlack?.threadTS === "string") {
    return metadataSlack.threadTS;
  }

  const state = status?.state as Record<string, unknown> | undefined;
  const stepState = state?.["step-1"] as Record<string, unknown> | undefined;
  const slack = stepState?.slack as Record<string, unknown> | undefined;
  return typeof slack?.threadTS === "string" ? slack.threadTS : "";
}

function requireEnv(name: string) {
  const value = process.env[name]?.trim();
  assert.ok(value, `${name} is required`);
  return value;
}

function runCommand(
  command: string,
  args: string[],
  options: {
    cwd?: string;
    input?: string;
  } = {},
) {
  const result = spawnSync(command, args, {
    cwd: options.cwd,
    encoding: "utf8",
    input: options.input,
    stdio: ["pipe", "pipe", "pipe"],
  });

  if (result.status !== 0) {
    throw new Error(
      `${command} ${args.join(" ")} failed with exit ${result.status}\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`,
    );
  }
  return result.stdout;
}

async function cleanup() {
  runDelete(kargoBin, ["delete", "stage", stageName, `--project=${project}`]);
  runDelete(kubectlBin, [
    "delete",
    "messagechannel.ee.kargo.akuity.io",
    channelName,
    "-n",
    project,
    "--ignore-not-found",
  ]);
  runDelete(kubectlBin, [
    "delete",
    "secret",
    secretName,
    "-n",
    project,
    "--ignore-not-found",
  ]);
  runDelete(kubectlBin, [
    "delete",
    "clusterrolebinding",
    clusterRoleBindingName,
    "--ignore-not-found",
  ]);

  if (buildDir) {
    await rm(buildDir, {
      force: true,
      recursive: true,
    });
  }
}

function runDelete(command: string, args: string[]) {
  spawnSync(command, args, {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
}

function yaml(document: string) {
  return `${document.trim()}\n`;
}

function logInfo(message: string) {
  process.stdout.write(`[INFO] ${message}\n`);
}

function logPass(message: string) {
  process.stdout.write(`[PASS] ${message}\n`);
}
