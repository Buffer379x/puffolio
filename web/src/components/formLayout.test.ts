import { it, expect } from 'vitest';
import { formSpans } from './formLayout';
it('pairs simple fields and widens a lone remainder before wide content without reordering', () => {
  expect(formSpans([true, false, false, false, true, false, true])).toEqual([
    true,
    false,
    false,
    true,
    true,
    true,
    true,
  ]);
  expect(formSpans([false, false, true, false, false, false])).toEqual([
    false,
    false,
    true,
    false,
    false,
    true,
  ]);
});
