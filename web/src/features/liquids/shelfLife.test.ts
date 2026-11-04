import { expect, it } from 'vitest';
import { shelfColor } from './shelfLife';
it('färbt von Grün über Gelb nach Rot', () => {
  expect(shelfColor(100)).toBe('hsl(120 65% 42%)');
  expect(shelfColor(50)).toBe('hsl(60 65% 42%)');
  expect(shelfColor(0)).toBe('hsl(0 65% 42%)');
});
