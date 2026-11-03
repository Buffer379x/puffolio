import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, APIError, download, setCSRF, upload } from './api';
// The external HTTP boundary is substituted; production request preparation and error parsing run unchanged.
afterEach(() => {
  vi.unstubAllGlobals();
  setCSRF('');
});
describe('Same-origin API protocol', () => {
  it('sends a write with CSRF and canonical JSON without exposing credentials externally', async () => {
    setCSRF('csrf-one');
    vi.stubGlobal('fetch', async (url: string, init: RequestInit) => {
      expect(url).toBe('/api/v1/liquids');
      expect(init.credentials).toBe('same-origin');
      const headers = new Headers(init.headers);
      expect(headers.get('X-CSRF-Token')).toBe('csrf-one');
      expect(headers.get('Content-Type')).toBe('application/json');
      expect(JSON.parse(init.body as string)).toEqual({
        name: 'Zero',
        values: { volume: { type: 'number', value: '0.000' } },
      });
      return new Response('{"id":"created"}', {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      });
    });
    expect(
      await api('/liquids', 'POST', {
        name: 'Zero',
        values: { volume: { type: 'number', value: '0.000' } },
      }),
    ).toEqual({ id: 'created' });
  });
  it('returns structured field errors and409 as recoverable API errors', async () => {
    vi.stubGlobal(
      'fetch',
      async () =>
        new Response(
          '{"error":{"code":"conflict","message":"Geändert","fields":{"date":"Ungültig"}},"requestId":"request-one"}',
          { status: 409, headers: { 'Content-Type': 'application/json' } },
        ),
    );
    const error = await api('/liquids/id', 'PATCH', { version: 1 }).catch((e) => e);
    expect(error).toBeInstanceOf(APIError);
    expect(error).toMatchObject({
      status: 409,
      code: 'conflict',
      fields: { date: 'Ungültig' },
      requestId: 'request-one',
    });
  });
  it('keeps browser multipart boundaries and the session CSRF for file upload', async () => {
    setCSRF('csrf-upload');
    vi.stubGlobal('fetch', async (_url: string, init: RequestInit) => {
      const headers = new Headers(init.headers);
      expect(headers.get('Content-Type')).toBeNull();
      expect(headers.get('X-CSRF-Token')).toBe('csrf-upload');
      expect((init.body as FormData).get('file')).toBeInstanceOf(File);
      return new Response('{"id":"media"}', { headers: { 'Content-Type': 'application/json' } });
    });
    expect(await upload('/media', new File(['pixels'], 'test.png', { type: 'image/png' }))).toEqual(
      { id: 'media' },
    );
  });
  it('keeps binary downloads binary and recognizes non-JSON server errors', async () => {
    vi.stubGlobal(
      'fetch',
      async () =>
        new Response(new Uint8Array([80, 75, 3, 4]), {
          headers: { 'Content-Type': 'application/zip' },
        }),
    );
    expect(new Uint8Array(await (await download('/exports', {})).arrayBuffer())).toEqual(
      new Uint8Array([80, 75, 3, 4]),
    );
    vi.stubGlobal(
      'fetch',
      async () => new Response('<html>Internal details</html>', { status: 503 }),
    );
    await expect(api('/me')).rejects.toMatchObject({ status: 503 });
  });
  it('does not convert cancellation into empty data or retry it', async () => {
    const controller = new AbortController();
    controller.abort();
    vi.stubGlobal('fetch', async (_url: string, init: RequestInit) => {
      expect(init.signal).toBe(controller.signal);
      throw new DOMException('Aborted', 'AbortError');
    });
    await expect(api('/liquids', 'GET', undefined, controller.signal)).rejects.toMatchObject({
      name: 'AbortError',
    });
  });
});
it('announces expired sessions without redirecting or replacing caller state', async () => {
  const listener = vi.fn();
  window.addEventListener('puffolio-session-expired', listener);
  vi.stubGlobal(
    'fetch',
    async () =>
      new Response('{"error":{"code":"unauthorized","message":"Anmelden"}}', { status: 401 }),
  );
  await expect(api('/liquids', 'PATCH', { version: 2 })).rejects.toMatchObject({ status: 401 });
  expect(listener).toHaveBeenCalledOnce();
  window.removeEventListener('puffolio-session-expired', listener);
});
it('announces expired authenticated PATCH/me but keeps initial GET/me unauthenticated', async () => {
  const listener = vi.fn();
  window.addEventListener('puffolio-session-expired', listener);
  try {
    vi.stubGlobal(
      'fetch',
      async () =>
        new Response('{"error":{"code":"unauthorized","message":"Anmelden"}}', { status: 401 }),
    );
    await expect(api('/me')).rejects.toMatchObject({ status: 401 });
    expect(listener).not.toHaveBeenCalled();
    await expect(api('/me', 'PATCH', { version: 1, displayName: 'Entwurf' })).rejects.toMatchObject(
      { status: 401 },
    );
    expect(listener).toHaveBeenCalledOnce();
  } finally {
    window.removeEventListener('puffolio-session-expired', listener);
  }
});
