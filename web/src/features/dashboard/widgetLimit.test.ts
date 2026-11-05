import { expect, it } from 'vitest';
import { parseWidgetLimit } from './widgetLimit';
it('accepts whole numbers from 1 to 10', () => {
  expect(parseWidgetLimit('1')).toBe(1);
  expect(parseWidgetLimit('10')).toBe(10);
  expect(parseWidgetLimit(' 7 ')).toBe(7);
});
it.each(['', '0', '11', '2,5', '2.5', '-1', 'abc', '1e1'])('rejects %j', (text) => {
  expect(parseWidgetLimit(text)).toBeNull();
});
