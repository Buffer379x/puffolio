/** Only a session's CSRF token is held in memory; authentication remains in HttpOnly cookies. */
let csrfToken = '';
export function setCSRF(value: string): void {
  csrfToken = value;
}
export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields: Record<string, string> = {},
    public requestId = '',
  ) {
    super(message);
    this.name = 'APIError';
  }
}

async function send(
  path: string,
  method: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<Response> {
  const headers = new Headers({ Accept: 'application/json' });
  const unsafe = !['GET', 'HEAD'].includes(method);
  if (unsafe && csrfToken) headers.set('X-CSRF-Token', csrfToken);
  const multipart = body instanceof FormData;
  if (body !== undefined && !multipart) headers.set('Content-Type', 'application/json');
  const response = await fetch('/api/v1' + path, {
    method,
    credentials: 'same-origin',
    headers,
    signal,
    body: body === undefined ? undefined : multipart ? body : JSON.stringify(body),
  });
  if (!response.ok) {
    if (response.status === 401 && path !== '/auth/login' && !(path === '/me' && method === 'GET'))
      window.dispatchEvent(new Event('puffolio-session-expired'));
    let data: {
      error?: { code?: string; message?: string; fields?: Record<string, string> };
      requestId?: string;
    } = {};
    try {
      data = await response.json();
    } catch {
      /* Non-JSON failures get a neutral message. */
    }
    throw new APIError(
      response.status,
      data?.error?.code || 'request_failed',
      data?.error?.message || 'Anfrage fehlgeschlagen. Erneut versuchen.',
      data?.error?.fields || {},
      data?.requestId || response.headers.get('X-Request-ID') || '',
    );
  }
  return response;
}

export async function api<T = unknown>(
  path: string,
  method = 'GET',
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  const response = await send(path, method, body, signal);
  if (response.status === 204) return undefined as T;
  try {
    return (await response.json()) as T;
  } catch {
    throw new APIError(502, 'invalid_response', 'Serverantwort konnte nicht gelesen werden.');
  }
}
export async function upload<T = unknown>(
  path: string,
  file: File,
  signal?: AbortSignal,
): Promise<T> {
  const data = new FormData();
  data.set('file', file);
  return api<T>(path, 'POST', data, signal);
}
export async function download(path: string, body: unknown): Promise<Blob> {
  return (await send(path, 'POST', body)).blob();
}
