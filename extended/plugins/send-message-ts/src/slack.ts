import { WebClient } from "@slack/web-api";
import type { ChatPostMessageArguments } from "@slack/web-api";

import { defaultSlackAPIBaseURL } from "./constants.js";
import type { SlackClient, SlackPostMessageResponse } from "./types.js";

export class SlackWebAPIClient implements SlackClient {
  public constructor(private readonly apiBaseURL = defaultSlackAPIBaseURL) {}

  public async postMessage(
    token: string,
    payload: Record<string, unknown>,
  ): Promise<SlackPostMessageResponse> {
    const client = new WebClient(token, {
      slackApiUrl: this.apiBaseURL,
    });
    const result = await client.chat.postMessage(
      payload as unknown as ChatPostMessageArguments,
    );
    return {
      ok: result.ok ?? false,
      error: result.error,
      ts: result.ts,
    };
  }
}

