import { readFile } from "node:fs/promises";

import { authHeader, bearerPrefix } from "./constants.js";
import { erroredResponse } from "./support.js";
import type { StepExecuteResponse } from "./types.js";

export class BearerTokenAuthorizer {
  public constructor(private readonly tokenPath: string) {}

  public async authorize(
    headers: Record<string, string | string[] | undefined>,
  ): Promise<StepExecuteResponse | null> {
    const expectedToken = (await readFile(this.tokenPath, "utf8")).trim();
    const headerValue = readHeader(headers, authHeader)?.trim();
    if (!headerValue?.startsWith(bearerPrefix)) {
      return erroredResponse("missing bearer token");
    }

    const receivedToken = headerValue.slice(bearerPrefix.length).trim();
    if (receivedToken !== expectedToken) {
      return erroredResponse("invalid bearer token");
    }
    return null;
  }
}

function readHeader(
  headers: Record<string, string | string[] | undefined>,
  name: string,
) {
  const value = headers[name];
  if (Array.isArray(value)) {
    return value[0];
  }
  return value;
}

