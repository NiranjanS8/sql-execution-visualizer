const DEFAULT_BASE_URL = "http://127.0.0.1:8787";

export const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL ?? DEFAULT_BASE_URL).replace(
  /\/+$/,
  "",
);

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly line?: number;
  readonly column?: number;

  constructor(message: string, status: number, code: string, line?: number, column?: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.line = line;
    this.column = column;
  }
}

type ErrorEnvelope = {
  error?: {
    code?: string;
    message?: string;
    location?: {
      line?: number;
      column?: number;
    };
  };
};

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    headers: { Accept: "application/json", ...init.headers },
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as ErrorEnvelope | null;
    throw new ApiError(
      body?.error?.message ?? `Request failed with status ${response.status}.`,
      response.status,
      body?.error?.code ?? "unknown_error",
      body?.error?.location?.line,
      body?.error?.location?.column,
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}
