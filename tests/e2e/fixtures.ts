import {
  test as base,
  expect,
  type APIRequestContext,
  type BrowserContext,
  type Page,
} from '@playwright/test';
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdir, mkdtemp, readFile, rm } from 'node:fs/promises';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import { networkInterfaces } from 'node:os';
import { createServer } from 'node:net';
import { deflateSync } from 'node:zlib';
const root = path.resolve(__dirname, '../..');
interface Server {
  url: string;
  dir: string;
  code: string;
}
export interface App {
  url: string;
  page: Page;
  context: BrowserContext;
  api: APIRequestContext;
  csrf: string;
  password: string;
  username: string;
  me: any;
  write: (path: string, method: string, data?: unknown) => Promise<any>;
}
export const test = base.extend<{ server: Server; app: App; lanHTTP: boolean }>({
  lanHTTP: [false, { option: true }],
  server: async ({ lanHTTP }, use) => {
    await mkdir(path.join(root, 'work/qa'), { recursive: true });
    const dir = await mkdtemp(path.join(root, 'work/qa/e2e-'));
    const host = lanHTTP
      ? Object.values(networkInterfaces())
          .flat()
          .find((address) => address?.family === 'IPv4' && !address.internal)?.address
      : '127.0.0.1';
    if (!host) throw new Error('LAN acceptance requires a non-loopback IPv4 interface');
    // Let the OS choose an unused port instead of intersecting the ephemeral
    // range with deterministic worker counters. Release just before spawning.
    const port = await new Promise<number>((resolve, reject) => {
      const socket = createServer();
      socket.once('error', reject);
      socket.listen(0, host, () => {
        const address = socket.address();
        if (!address || typeof address === 'string') {
          socket.close();
          reject(new Error('No TCP test port'));
          return;
        }
        socket.close((error) => (error ? reject(error) : resolve(address.port)));
      });
    });
    const url = `http://${host}:${port}`;
    let exit: string | undefined;
    const child: ChildProcess = spawn(
      path.join(root, 'work/bin/puffolio'),
      ['serve', '--data-dir', dir, '--addr', `${host}:${port}`],
      {
        cwd: root,
        env: {
          ...process.env,
          PUFFOLIO_BASE_URL: url,
          PUFFOLIO_COOKIE_SECURE: 'false',
        },
        stdio: ['ignore', 'ignore', 'pipe'],
      },
    );
    let stderr = '';
    child.stderr?.on('data', (b) => {
      stderr += String(b);
    });
    child.on('error', (e) => {
      exit = e.message;
    });
    child.on('exit', (code) => {
      exit = `exit ${code}`;
    });
    try {
      let ready = false;
      for (let i = 0; i < 150; i++) {
        if (exit) throw new Error(`Isolated test server failed: ${exit}. ${stderr}`);
        try {
          const r = await fetch(url + '/readyz');
          if (r.ok) {
            ready = true;
            break;
          }
        } catch {}
        await new Promise((r) => setTimeout(r, 100));
      }
      if (!ready) throw new Error('Isolated test server did not become ready. ' + stderr);
      const code = (await readFile(path.join(dir, 'setup-code'), 'utf8')).trim();
      await use({ url, dir, code });
    } finally {
      if (child.exitCode === null && !child.killed) {
        child.kill('SIGTERM');
        await Promise.race([
          new Promise<void>((r) => child.once('exit', () => r())),
          new Promise((r) => setTimeout(r, 3000)),
        ]);
        if (child.exitCode === null) child.kill('SIGKILL');
      }
      await rm(dir, { recursive: true, force: true });
    }
  },
  app: async ({ server, playwright, context, page }, use) => {
    const password = 'E2e-' + randomUUID();
    const username = 'admin';
    const api = await playwright.request.newContext({
      baseURL: server.url,
      extraHTTPHeaders: { Origin: server.url },
    });
    const setup = await api.post('/api/v1/setup', {
      data: {
        code: server.code,
        username,
        displayName: 'Testkonto',
        password,
        starterFields: true,
      },
    });
    expect(setup.status()).toBe(201);
    const me = await setup.json();
    await context.addCookies((await api.storageState()).cookies);
    const write = async (route: string, method: string, data?: unknown) => {
      const r = await api.fetch('/api/v1' + route, {
        method,
        data,
        headers: { 'X-CSRF-Token': me.csrfToken },
      });
      expect(r.ok(), `API ${method} ${route}: ${r.status()}`).toBeTruthy();
      return r.status() === 204 ? undefined : await r.json();
    };
    try {
      await use({
        url: server.url,
        page,
        context,
        api,
        csrf: me.csrfToken,
        password,
        username,
        me,
        write,
      });
    } finally {
      await context.close();
      await api.dispose();
    }
  },
});
export { expect };
export async function fields(app: App) {
  return (await (await app.api.get('/api/v1/fields')).json()).items as any[];
}
export async function addField(
  app: App,
  label: string,
  type: string,
  config: Record<string, unknown> = {},
) {
  return app.write('/fields', 'POST', {
    label,
    type,
    required: false,
    visibility: { form: true, detail: true, list: true, grid: true },
    config: { filterable: true, ...config },
  });
}
export async function addLiquid(
  app: App,
  name: string,
  values: Record<string, unknown> = {},
  favorite = false,
) {
  return app.write('/liquids', 'POST', { name, values, favorite });
}
export async function goto(app: App, route: string) {
  await app.page.goto(app.url + route);
  await expect(
    app.page.getByRole('heading', {
      name: route.startsWith('/liquids')
        ? 'Liquids'
        : route.startsWith('/settings')
          ? 'Einstellungen'
          : route.startsWith('/tools/nicotine')
            ? 'Nikotinrechner'
            : route.startsWith('/tools')
              ? 'Tools'
              : 'Dashboard',
      exact: true,
    }),
  ).toBeVisible();
}
export function syntheticPNG() {
  const w = 160,
    h = 120;
  const bytes = Buffer.alloc((w * 3 + 1) * h);
  for (let y = 0; y < h; y++)
    for (let x = 0; x < w; x++) {
      const i = y * (w * 3 + 1) + 1 + x * 3;
      bytes[i] = x;
      bytes[i + 1] = y;
      bytes[i + 2] = 120;
    }
  const crc = (b: Buffer) => {
    let c = 0xffffffff;
    for (const v of b) {
      c ^= v;
      for (let i = 0; i < 8; i++) c = (c >>> 1) ^ (c & 1 ? 0xedb88320 : 0);
    }
    return (c ^ 0xffffffff) >>> 0;
  };
  const chunk = (type: string, b: Buffer) => {
    const t = Buffer.from(type);
    const size = Buffer.alloc(4);
    size.writeUInt32BE(b.length);
    const sum = Buffer.alloc(4);
    sum.writeUInt32BE(crc(Buffer.concat([t, b])));
    return Buffer.concat([size, t, b, sum]);
  };
  const header = Buffer.alloc(13);
  header.writeUInt32BE(w);
  header.writeUInt32BE(h, 4);
  header[8] = 8;
  header[9] = 2;
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', header),
    chunk('IDAT', deflateSync(bytes)),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}
