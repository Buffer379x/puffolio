import type { Crop } from '../app/types';
const clamp = (v: number, min: number, max: number) => Math.max(min, Math.min(max, v));
export function cropAt(w: number, h: number, zoom: number, x: number, y: number): Crop {
  const z = Number.isFinite(zoom) ? Math.max(1, zoom) : 1;
  const scale = Math.max(4 / w, 3 / h) * z;
  const width = 4 / (w * scale),
    height = 3 / (h * scale);
  return {
    x: ((1 - width) * clamp(x, 0, 100)) / 100,
    y: ((1 - height) * clamp(y, 0, 100)) / 100,
    width,
    height,
  };
}
export function cropPosition(w: number, h: number, c: Crop) {
  const zoom = 4 / (w * c.width) / Math.max(4 / w, 3 / h);
  return {
    zoom,
    x: 1 - c.width < 1e-9 ? 50 : (c.x / (1 - c.width)) * 100,
    y: 1 - c.height < 1e-9 ? 50 : (c.y / (1 - c.height)) * 100,
  };
}
