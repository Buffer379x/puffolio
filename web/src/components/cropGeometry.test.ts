import { it, expect } from 'vitest';
import { cropAt, cropPosition } from './cropGeometry';
it('stays square and inside the image at every pan boundary', () => {
  for (const [w, h] of [
    [1600, 1200],
    [600, 1600],
    [1600, 600],
    [80, 80],
  ])
    for (const x of [0, 50, 100])
      for (const y of [0, 50, 100]) {
        const c = cropAt(w, h, 2, x, y);
        expect(c.x).toBeGreaterThanOrEqual(0);
        expect(c.y).toBeGreaterThanOrEqual(0);
        expect(c.x + c.width).toBeLessThanOrEqual(1.000001);
        expect(c.y + c.height).toBeLessThanOrEqual(1.000001);
        expect(c.width * w).toBeCloseTo(c.height * h);
        const p = cropPosition(w, h, c);
        const again = cropAt(w, h, p.zoom, p.x, p.y);
        for (const key of ['x', 'y', 'width', 'height'] as const)
          expect(again[key]).toBeCloseTo(c[key]);
      }
});
it('uses the largest square at zoom 1', () => {
  expect(cropAt(1600, 1200, 1, 50, 50)).toEqual({ x: 0.125, y: 0, width: 0.75, height: 1 });
});
it('reopens a 4:3 crop from 0.1.0 as a square around the same centre', () => {
  const p = cropPosition(1600, 1200, { x: 0, y: 0, width: 1, height: 1 });
  expect(p).toEqual({ zoom: 1, x: 50, y: 50 });
  const c = cropAt(1600, 1200, p.zoom, p.x, p.y);
  expect(c.width * 1600).toBeCloseTo(c.height * 1200);
});
