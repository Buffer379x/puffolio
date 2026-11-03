import { expect, it } from 'vitest';
import { visualLineCount } from '../test/textGeometry';
it('counts mixed font sizes on one baseline as a single visual line', () => {
  expect(
    visualLineCount([
      { top: 100, bottom: 116 },
      { top: 102, bottom: 116 },
    ]),
  ).toBe(1);
});
it('detects a real second line and ignores duplicate inline rectangles', () => {
  expect(
    visualLineCount([
      { top: 100, bottom: 116 },
      { top: 100, bottom: 116 },
      { top: 120, bottom: 136 },
    ]),
  ).toBe(2);
  expect(visualLineCount([])).toBe(0);
});
