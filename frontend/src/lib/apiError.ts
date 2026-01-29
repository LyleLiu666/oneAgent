export type ApiErrorShape = {
  error?: unknown;
  code?: unknown;
  request_id?: unknown;
  hint?: unknown;
};

export type ParsedApiError = {
  message: string;
  code?: string;
  requestId?: string;
  hint?: string;
  status?: number;
};

function asString(v: unknown): string {
  if (typeof v === "string") return v.trim();
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  return "";
}

export function parseApiError(
  err: unknown,
  fallbackMessage: string = "请求失败",
): ParsedApiError {
  const fallback = String(fallbackMessage || "请求失败").trim() || "请求失败";

  const anyErr = err as any;
  const data = anyErr?.data as unknown;

  const payload = (data && typeof data === "object" ? (data as ApiErrorShape) : null) as
    | ApiErrorShape
    | null;

  const statusValue = Number(anyErr?.response?.status);
  const status = Number.isFinite(statusValue) ? statusValue : undefined;

  const message = asString(payload?.error) || asString(data) || asString(anyErr?.message) || fallback;
  const code = asString(payload?.code) || undefined;
  const hint = asString(payload?.hint) || undefined;

  const requestIdFromPayload = asString(payload?.request_id);
  const requestIdFromHeader = asString(anyErr?.response?.headers?.get?.("X-Request-ID"));
  const requestId = requestIdFromPayload || requestIdFromHeader || undefined;

  return { message, code, requestId, hint, status };
}

