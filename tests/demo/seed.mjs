// Fills a fresh Puffolio instance with sample data. Used by demo.mjs and screenshots.mjs.
import { spawn } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { deflateSync } from 'node:zlib';

export const DEMO_LOGIN = { username: 'demo', password: 'Puffolio-Demo-2026!' };

function encodePNG(width, height, rgb) {
  const crc = (b) => {
    let c = 0xffffffff;
    for (const v of b) {
      c ^= v;
      for (let i = 0; i < 8; i++) c = (c >>> 1) ^ (c & 1 ? 0xedb88320 : 0);
    }
    return (c ^ 0xffffffff) >>> 0;
  };
  const chunk = (type, b) => {
    const name = Buffer.from(type);
    const size = Buffer.alloc(4);
    size.writeUInt32BE(b.length);
    const sum = Buffer.alloc(4);
    sum.writeUInt32BE(crc(Buffer.concat([name, b])));
    return Buffer.concat([size, name, b, sum]);
  };
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width);
  header.writeUInt32BE(height, 4);
  header[8] = 8;
  header[9] = 2;
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', header),
    chunk('IDAT', deflateSync(rgb)),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

// Signed distance to a rounded box, so the drawing stays smooth when scaled down.
const box = (x, y, cx, cy, hw, hh, r) => {
  const qx = Math.abs(x - cx) - hw + r;
  const qy = Math.abs(y - cy) - hh + r;
  return Math.hypot(Math.max(qx, 0), Math.max(qy, 0)) + Math.min(Math.max(qx, qy), 0) - r;
};
const cover = (distance) => Math.min(1, Math.max(0, 0.5 - distance));
const mix = (a, b, t) => a.map((v, k) => v + (b[k] - v) * t);

/** A synthetic product shot: nothing here comes from a real photo or brand. */
function bottlePhoto({ light, dark, liquid, label }) {
  const size = 800;
  const rgb = Buffer.alloc((size * 3 + 1) * size);
  for (let y = 0; y < size; y++)
    for (let x = 0; x < size; x++) {
      const vignette = Math.min(1, Math.hypot(x - 400, y - 340) / 620);
      let color = mix(light, dark, vignette * 0.85 + (y / size) * 0.15);
      const floor = Math.max(0, 1 - Math.hypot((x - 400) / 230, (y - 700) / 34));
      color = mix(color, [20, 24, 20], floor * 0.45);
      const glass = box(x, y, 400, 470, 130, 230, 46);
      const neck = box(x, y, 400, 234, 30, 30, 4);
      if (neck < 1 || glass < 1) {
        const body = mix(liquid, [18, 20, 16], 0.25 + (0.4 * Math.abs(x - 400)) / 130);
        color = mix(color, body, cover(Math.min(neck, glass)));
      }
      const band = box(x, y, 400, 482, 130, 118, 0);
      if (band < 1) {
        const paper = mix(label, [255, 255, 255], 0.12 + (y - 364) / 900);
        color = mix(color, paper, cover(Math.max(band, glass)));
        const motif = Math.hypot(x - 400, y - 462) - 52;
        color = mix(color, liquid, cover(motif) * 0.85);
        color = mix(color, [28, 32, 26], cover(box(x, y, 400, 566, 130, 16, 0)) * cover(glass));
      }
      const shine = box(x, y, 300, 470, 7, 190, 7);
      if (shine < 1) color = mix(color, [255, 255, 255], cover(shine) * 0.28);
      const cap = box(x, y, 400, 165, 42, 56, 8);
      if (cap < 1) {
        const ridge = 12 + 10 * (Math.sin(x * 0.75) * 0.5 + 0.5);
        color = mix(color, [ridge, ridge + 2, ridge], cover(cap));
      }
      const i = y * (size * 3 + 1) + 1 + x * 3;
      for (let k = 0; k < 3; k++) rgb[i + k] = Math.round(Math.min(255, Math.max(0, color[k])));
    }
  return encodePNG(size, size, rgb);
}

const yearsAgo = (years, days = 0) => {
  const d = new Date(Date.now() - (years * 365.25 + days) * 86400000);
  return d.toISOString().slice(0, 10);
};

const photo = (light, dark, liquid, label) => ({ light, dark, liquid, label });
const SAMPLES = [
  {
    name: 'Citrus Mint',
    maker: 'Nordwind',
    rating: 4,
    tags: ['Zitrus', 'Menthol'],
    nicotine: 3,
    status: 'In Gebrauch',
    purchased: yearsAgo(0, 30),
    favorite: true,
    photo: photo([214, 226, 170], [84, 118, 94], [132, 170, 70], [244, 246, 224]),
    notes: 'Frisch, wenig Süße. Nach zwei Wochen deutlich runder.',
  },
  {
    name: 'Mango Ice',
    maker: 'Tropic Lab',
    rating: 5,
    tags: ['Frucht', 'Menthol'],
    nicotine: 6,
    status: 'In Gebrauch',
    purchased: yearsAgo(1),
    favorite: true,
    photo: photo([246, 196, 120], [176, 84, 64], [222, 128, 44], [252, 242, 222]),
  },
  {
    name: 'Vanilla Custard',
    maker: 'Old Mill',
    rating: 4,
    tags: ['Dessert'],
    nicotine: 3,
    status: 'Vorrat',
    purchased: yearsAgo(2),
    photo: photo([232, 216, 178], [146, 108, 70], [196, 150, 84], [250, 244, 226]),
  },
  {
    name: 'Forest Berry',
    maker: 'Tropic Lab',
    rating: 3,
    tags: ['Beere', 'Frucht'],
    nicotine: 0,
    status: 'Vorrat',
    purchased: yearsAgo(3),
    ownShelfLife: '24',
  },
  {
    name: 'Virginia Reserve',
    maker: 'Old Mill',
    rating: 5,
    tags: ['Tabak'],
    nicotine: 12,
    status: 'In Gebrauch',
    purchased: yearsAgo(4),
    favorite: true,
    photo: photo([168, 128, 92], [66, 44, 32], [110, 70, 36], [236, 222, 196]),
  },
  {
    name: 'Cool Grape',
    maker: 'Nordwind',
    tags: ['Beere', 'Menthol'],
    nicotine: 3,
    status: 'Vorrat',
  },
  {
    name: 'Lemon Tart',
    maker: 'Backstube',
    rating: 4,
    tags: ['Dessert', 'Zitrus'],
    nicotine: 3,
    status: 'Aufgebraucht',
    purchased: yearsAgo(5, 20),
    favorite: true,
  },
  {
    name: 'Peach Tea',
    maker: 'Teahouse',
    rating: 3,
    tags: ['Frucht'],
    nicotine: 0,
    status: 'Vorrat',
    purchased: yearsAgo(4, 200),
  },
];
const TAGS = [
  ['Frucht', 'orange'],
  ['Menthol', 'cyan'],
  ['Tabak', 'amber'],
  ['Dessert', 'pink'],
  ['Beere', 'violet'],
  ['Zitrus', 'lime'],
];

export async function seed(url, dataDir) {
  let cookie = '';
  let csrf = '';
  const call = async (path, method = 'GET', body, form) => {
    const r = await fetch(url + '/api/v1' + path, {
      method,
      headers: {
        Origin: url,
        ...(cookie ? { Cookie: cookie } : {}),
        ...(csrf ? { 'X-CSRF-Token': csrf } : {}),
        ...(form ? {} : { 'Content-Type': 'application/json' }),
      },
      body: form || (body === undefined ? undefined : JSON.stringify(body)),
    });
    const set = r.headers.getSetCookie?.() || [];
    if (set.length) cookie = set.map((c) => c.split(';')[0]).join('; ');
    const text = await r.text();
    if (!r.ok) throw new Error(`${method} ${path}: ${r.status} ${text}`);
    return text ? JSON.parse(text) : undefined;
  };
  const me = await call('/setup', 'POST', {
    code: readFileSync(dataDir + '/setup-code', 'utf8').trim(),
    displayName: 'Demo',
    starterFields: true,
    ...DEMO_LOGIN,
  });
  csrf = me.csrfToken;
  const fields = Object.fromEntries((await call('/fields')).items.map((f) => [f.label, f]));
  const tag = {};
  for (const [name, paletteId] of TAGS)
    tag[name] = (await call(`/fields/${fields.Tags.id}/tags`, 'POST', { name, paletteId })).id;
  const status = Object.fromEntries(fields.Status.config.options.map((o) => [o.label, o.id]));
  const upload = async (colors) => {
    const form = new FormData();
    form.append('file', new Blob([bottlePhoto(colors)], { type: 'image/png' }), 'sample.png');
    const media = await call('/media', 'POST', undefined, form);
    return (await call(`/media/${media.id}/crop`, 'POST', { x: 0, y: 0, width: 1, height: 1 })).id;
  };
  for (const sample of SAMPLES) {
    const values = {};
    const set = (label, value) => {
      if (value !== null && value !== undefined)
        values[fields[label].id] = { type: fields[label].type, value };
    };
    set('Hersteller', sample.maker);
    set('Bewertung', sample.rating);
    set(
      'Tags',
      sample.tags.map((t) => tag[t]),
    );
    set('Nikotin', sample.nicotine.toFixed(fields.Nikotin.config.scale ?? 1));
    set('Status', status[sample.status]);
    set('Kaufdatum', sample.purchased);
    set('Eigene Haltedauer', sample.ownShelfLife);
    set('Notizen', sample.notes);
    if (sample.photo) set('Bild', await upload(sample.photo));
    await call('/liquids', 'POST', {
      name: sample.name,
      values,
      favorite: !!sample.favorite,
    });
  }
}

/** A throwaway instance with sample data in a temporary directory. */
export async function startInstance(port) {
  const root = resolve(import.meta.dirname, '../..');
  const url = `http://127.0.0.1:${port}`;
  const dir = mkdtempSync(join(tmpdir(), 'puffolio-demo-'));
  const server = spawn(
    process.env.PUFFOLIO_BIN || join(root, 'work/bin/puffolio'),
    ['serve', '--data-dir', dir, '--addr', `127.0.0.1:${port}`],
    { stdio: 'ignore' },
  );
  const stop = () => {
    server.kill('SIGTERM');
    rmSync(dir, { recursive: true, force: true });
  };
  for (let i = 0; i < 100; i++) {
    const ready = await fetch(url + '/readyz').then(
      (r) => r.ok,
      () => false,
    );
    if (ready) break;
    await new Promise((r) => setTimeout(r, 100));
  }
  await seed(url, dir);
  return { url, stop };
}
