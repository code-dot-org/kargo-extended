import type { StepExecuteResponse } from "./types.js";

export class RequestError extends Error {}

export function asErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function failedResponse(message: string): StepExecuteResponse {
  return {
    status: "Failed",
    message,
    error: message,
    terminal: true,
  };
}

export function erroredResponse(message: string): StepExecuteResponse {
  return {
    status: "Errored",
    message,
    error: message,
    terminal: true,
  };
}

export function asRecord(
  value: unknown,
  label: string,
): Record<string, unknown> {
  if (!isRecord(value)) {
    throw new RequestError(`${label} must be an object`);
  }
  return value;
}

export function requiredString(
  record: Record<string, unknown>,
  key: string,
  label: string,
): string {
  const value = record[key];
  if (typeof value !== "string" || value.trim() === "") {
    throw new RequestError(`${label} must be a non-empty string`);
  }
  return value;
}

export function isRequestError(error: unknown): error is RequestError {
  return error instanceof RequestError;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
