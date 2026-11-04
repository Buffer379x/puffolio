import { describe, it, expect } from 'vitest';
import { canonicalNumber, formatDecimal, formatNumber, formatDate, changedValues } from './values';
describe('lossless dynamic values', () => {
  it('normalizes decimal comma without precision loss', () => {
    expect(canonicalNumber('9007199254740993,12', 3)).toBe('9007199254740993.120');
    expect(formatDecimal('9007199254740993.120')).toBe('9.007.199.254.740.993,120');
    expect(() => canonicalNumber('1.1234', 3)).toThrow();
  });
  it('retains false, zero, and calendar dates while omitting hidden unchanged values', () => {
    const before = {
      hidden: { type: 'text', value: 'Keep' },
      date: { type: 'date', value: '2028-02-29' },
    };
    expect(
      changedValues(before, {
        ...before,
        zero: { type: 'number', value: '0.0' },
        flag: { type: 'boolean', value: false },
        date: null,
      }),
    ).toEqual({
      zero: { type: 'number', value: '0.0' },
      flag: { type: 'boolean', value: false },
      date: null,
    });
    expect(formatDate('2028-02-29')).toBe('29.02.2028');
  });
});

it('zeigt gespeicherte Nachkommastellen beim Lesen ohne überflüssige Nullen', async () => {
  const { formatNumber } = await import('./values');
  expect(formatNumber('60.0')).toBe('60');
  expect(formatNumber('40.00')).toBe('40');
  expect(formatNumber('12.50')).toBe('12,5');
  expect(formatNumber('1000.0')).toBe('1.000');
  expect(formatNumber('100')).toBe('100');
  expect(formatNumber('0.0')).toBe('0');
});
