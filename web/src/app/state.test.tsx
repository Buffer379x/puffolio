import { render, cleanup } from '@testing-library/react';
import { it, expect, vi, afterEach } from 'vitest';
import { Theme } from './state';
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
it('follows running system appearance changes and removes listener on unmount', () => {
  let handler: () => void = () => {};
  const remove = vi.fn();
  const media = {
    matches: false,
    addEventListener: (_n: string, fn: () => void) => {
      handler = fn;
    },
    removeEventListener: remove,
  };
  vi.stubGlobal('matchMedia', () => media);
  const r = render(<Theme theme="system" />);
  expect(document.documentElement.dataset.theme).toBe('light');
  media.matches = true;
  handler();
  expect(document.documentElement.dataset.theme).toBe('dark');
  r.unmount();
  expect(remove).toHaveBeenCalled();
});
