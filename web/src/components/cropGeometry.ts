import type { Crop } from '../app/types';
const clamp = (v: number, min: number, max: number) => Math.max(min, Math.min(max, v));
/** A square of the image: zoom 1 is the largest square, x and y pan it from 0 to 100 %. */
export function cropAt(w: number, h: number, zoom: number, x: number, y: number): Crop {
  const z = Number.isFinite(zoom) ? Math.max(1, zoom) : 1;
  const side = Math.min(w, h) / z;
  const width = side / w,
    height = side / h;
  return {
    x: ((1 - width) * clamp(x, 0, 100)) / 100,
    y: ((1 - height) * clamp(y, 0, 100)) / 100,
    width,
    height,
  };
}
/** Crops saved by 0.1.0 are 4:3; they reopen as the largest square around the same centre. */
export function cropPosition(w: number, h: number, c: Crop) {
  const side = Math.min(c.width * w, c.height * h);
  const width = side / w,
    height = side / h;
  const x = clamp(c.x + (c.width - width) / 2, 0, 1 - width),
    y = clamp(c.y + (c.height - height) / 2, 0, 1 - height);
  return {
    zoom: Math.min(w, h) / side,
    x: 1 - width < 1e-9 ? 50 : (x / (1 - width)) * 100,
    y: 1 - height < 1e-9 ? 50 : (y / (1 - height)) * 100,
  };
}
