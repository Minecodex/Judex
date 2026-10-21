export class APIError extends Error {
  status: number;
  code: string;
  details?: unknown;
  retryable?: boolean;
  constructor(status: number, code: string, message: string, details?: unknown, retryable?: boolean) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
    this.retryable = retryable;
  }
}

type RequestOptions = RequestInit & { idempotencyKey?: string };
const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

// An uncertain write retains its key until its result has actually been decoded.
// State belongs to this session client and is cleared on logout.
export function createRequester(base: string, unauthorized: () => void) {
  let csrfToken = '';
  const pendingWrites = new Map<string, string>();
  async function request<T>(path: string, init?: RequestOptions): Promise<T> {
    const headers = new Headers(init?.headers);
    if (!headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
    const method = (init?.method || 'GET').toUpperCase();
    const writing = method !== 'GET' && method !== 'HEAD';
    // Distinct bytes must not collide as "[object ArrayBuffer]".
    const body = init?.body instanceof ArrayBuffer
      ? Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', init.body)), b => b.toString(16).padStart(2, '0')).join('')
      : String(init?.body ?? '');
    const explicitKey = init?.idempotencyKey ?? headers.get('Idempotency-Key');
    // Composite operations own their stage identities. Another explicit intent
    // with identical bytes must not inherit an older uncertain request's key.
    const signature = method + ' ' + path + ' ' + body + ' ' + (explicitKey ?? '');
    let key: string | undefined;
    if (writing) {
      if (csrfToken) headers.set('X-CSRF-Token', csrfToken);
      key = explicitKey ?? pendingWrites.get(signature) ?? crypto.randomUUID();
      pendingWrites.set(signature, key);
      headers.set('Idempotency-Key', key);
    }
    const acknowledge = () => {
      if (key && pendingWrites.get(signature) === key) pendingWrites.delete(signature);
    };
    let response: Response;
    try {
      response = await fetch(base + path, {...init, credentials: 'same-origin', headers});
    } catch {
      throw new APIError(0, 'NETWORK', 'network error', undefined, true);
    }
    if (response.status === 401 && !path.startsWith('/auth/')) unauthorized();
    if (response.status === 204) {
      acknowledge();
      return undefined as T;
    }
    let payload: unknown;
    try {
      payload = await response.json();
    } catch {
      throw invalidResponse(response.status);
    }
    if (response.ok) {
      if (!isObject(payload) || !Object.hasOwn(payload, 'data') || Object.hasOwn(payload, 'error')) {
        throw invalidResponse(response.status);
      }
      acknowledge();
      return payload.data as T;
    }
    const error = isObject(payload) && payload.error;
    if (!isObject(error) || typeof error.code !== 'string' || typeof error.message !== 'string') {
      throw invalidResponse(response.status);
    }
    // A valid client error is definitive. A server failure may follow a
    // committed effect, so its retry still uses the original key.
    if (response.status < 500) acknowledge();
    throw new APIError(response.status, error.code, error.message, error.details, error.retryable === true);
  }
  return {
    request,
    setCSRFToken: (token: string) => { csrfToken = token; },
    clearCSRFToken: () => { csrfToken = ''; pendingWrites.clear(); },
  };
}

function invalidResponse(status: number) {
  return new APIError(status, 'INVALID_RESPONSE', 'The response could not be read. Retry the same action to confirm its result.', undefined, true);
}
