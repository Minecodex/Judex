export const dataMode =
  import.meta.env.VITE_DATA_MODE === "demo" ? "demo" : "api";

export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public details?: unknown,
    public retryable?: boolean,
  ) {
    super(message);
  }
}

type Envelope<T> = { data: T; meta?: { requestId?: string; serverTime?: string; eventCursor?: number } };

// Session-scoped CSRF token captured from GET /auth/session and kept in
// memory only (docs/plans/v1/02 §2: cookie 不进 JS 可写存储)。
let csrfToken = "";
const pendingWrites = new Map<string, string>();

export const setCSRFToken = (token: string) => {
  csrfToken = token;
};
export const clearCSRFToken = () => {
  csrfToken = "";
  pendingWrites.clear();
};

const base = (import.meta.env.VITE_API_BASE_URL || "") + "/api/v1";

// EventSource 等非 fetch 通道使用的完整地址。
export const apiUrl = (path: string) => base + path;

export async function request<T>(
  path: string,
  init?: RequestInit & { idempotencyKey?: string },
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(init?.headers as Record<string, string> | undefined),
  };
  const method = (init?.method || "GET").toUpperCase();
  if (method !== "GET" && method !== "HEAD") {
    if (csrfToken) headers["X-CSRF-Token"] = csrfToken;
    headers["Idempotency-Key"] = init?.idempotencyKey ?? headers["Idempotency-Key"] ?? crypto.randomUUID();
  }
  const signature = method + " " + path + " " + String(init?.body ?? "");
  if (headers["Idempotency-Key"]) {
    headers["Idempotency-Key"] = pendingWrites.get(signature) ?? headers["Idempotency-Key"];
    pendingWrites.set(signature, headers["Idempotency-Key"]);
  }
  let response: Response;
  try {
    response = await fetch(base + path, { ...init, credentials: "same-origin", headers });
  } catch (cause) {
    throw new APIError(0, "NETWORK", "network error", undefined, true);
  }
  if (response.status < 500) pendingWrites.delete(signature);
  if (response.status === 401 && !path.startsWith("/auth/")) window.dispatchEvent(new Event("judex:unauthorized"));
  if (response.status === 204) return undefined as T;
  const payload = await response.json().catch(() => null);
  if (!response.ok) {
    const error = payload?.error;
    throw new APIError(
      response.status,
      error?.code || "HTTP_ERROR",
      error?.message || "Request failed",
      error?.details,
      error?.retryable,
    );
  }
  return (payload as Envelope<T>).data !== undefined || payload === null
    ? ((payload as Envelope<T>)?.data as T)
    : (payload as T);
}

export type SystemInfo = {
  name: string;
  version: string;
  commit?: string;
  protocolVersion: string;
  schemaRange: string;
  capabilities: Record<string, boolean>;
};
