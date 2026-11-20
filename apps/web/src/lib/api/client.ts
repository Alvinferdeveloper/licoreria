export interface FieldError {
  field: string;
  code: string;
  message: string;
}

interface ErrorEnvelope {
  error?: {
    code: string;
    message: string;
    requestId?: string;
    details?: { fields?: FieldError[] } & Record<string, unknown>;
  };
}

/** Error returned by the API's standard envelope. Branch on `code`, never on `message`. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly fields: FieldError[] = [],
    readonly requestId?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }

  fieldMessage(field: string): string | undefined {
    return this.fields.find((f) => f.field === field)?.message;
  }
}

const NETWORK_ERROR_MESSAGE = "No hay conexión con el servidor. Revisa la red e intenta de nuevo.";

export interface ApiClientOptions {
  getAccessToken: () => string | null;
  /** Called once when a request gets 401; resolves to a fresh token or null if the session ended. */
  refreshSession: () => Promise<string | null>;
}

let options: ApiClientOptions = {
  getAccessToken: () => null,
  refreshSession: async () => null,
};

export function configureApiClient(next: ApiClientOptions) {
  options = next;
}

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

interface RequestOptions {
  body?: unknown;
  /** Multipart payload (file uploads); the browser sets its Content-Type boundary. */
  form?: FormData;
  /** Skips the automatic refresh-and-retry, used by the auth endpoints themselves. */
  skipRefresh?: boolean;
  signal?: AbortSignal;
  headers?: Record<string, string>;
  /** Gives up after this long and reports a lost connection, instead of hanging on a dead network. */
  timeoutMs?: number;
}

export async function api<T>(method: Method, path: string, init: RequestOptions = {}): Promise<T> {
  const response = await send(method, path, init, options.getAccessToken());

  if (response.status === 401 && !init.skipRefresh) {
    const token = await options.refreshSession();
    if (token) return parse<T>(await send(method, path, init, token));
  }
  return parse<T>(response);
}

/** GET a binary response (generated spreadsheets, PDFs) with the same session handling as `api`. */
export async function apiBlob(path: string): Promise<Blob> {
  let response = await send("GET", path, {}, options.getAccessToken());
  if (response.status === 401) {
    const token = await options.refreshSession();
    if (token) response = await send("GET", path, {}, token);
  }
  if (!response.ok) return parse<never>(response);
  return response.blob();
}

/** Hands a blob to the browser as a file download. */
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = Object.assign(document.createElement("a"), { href: url, download: filename });
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

async function send(method: Method, path: string, init: RequestOptions, token: string | null) {
  const headers: Record<string, string> = { ...init.headers };
  if (init.body !== undefined) headers["Content-Type"] = "application/json";
  if (token) headers.Authorization = `Bearer ${token}`;

  const timeout = init.timeoutMs ? AbortSignal.timeout(init.timeoutMs) : undefined;
  const signal = init.signal && timeout ? AbortSignal.any([init.signal, timeout]) : (init.signal ?? timeout);
  try {
    return await fetch(`/api/v1${path}`, {
      method,
      headers,
      body: init.form ?? (init.body === undefined ? undefined : JSON.stringify(init.body)),
      credentials: "same-origin",
      signal,
    });
  } catch (error) {
    // A caller's own abort propagates; a timeout (TimeoutError) means the network is gone.
    if (error instanceof DOMException && error.name === "AbortError") throw error;
    throw new ApiError(0, "network_error", NETWORK_ERROR_MESSAGE);
  }
}

/**
 * The API answers every error with a JSON envelope. A 5xx without it comes from the proxy in
 * front (the API is down or unreachable), so it counts as a lost connection.
 */
export const isUnreachable = (response: Response) =>
  response.status >= 500 && !response.headers.get("content-type")?.includes("application/json");

async function parse<T>(response: Response): Promise<T> {
  if (response.status === 204) return undefined as T;
  if (isUnreachable(response)) throw new ApiError(0, "network_error", NETWORK_ERROR_MESSAGE);

  const data: unknown = await response.json().catch(() => null);
  if (response.ok) return data as T;

  const envelope = (data ?? {}) as ErrorEnvelope;
  throw new ApiError(
    response.status,
    envelope.error?.code ?? "unknown_error",
    envelope.error?.message ?? "Ocurrió un error inesperado. Intenta de nuevo.",
    envelope.error?.details?.fields ?? [],
    envelope.error?.requestId,
  );
}

export interface Page<T> {
  data: T[];
  meta: { page: number; pageSize: number; total: number };
}
