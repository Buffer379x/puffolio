import { it, expect } from 'vitest';
import { cropAt, cropPosition } from './cropGeometry';
it('fills landscape and portrait without empty edges at every pan boundary', () => {
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
        expect((c.width * w) / (c.height * h)).toBeCloseTo(4 / 3);
        const p = cropPosition(w, h, c);
        expect(cropAt(w, h, p.zoom, p.x, p.y)).toEqual(c);
      }
});
