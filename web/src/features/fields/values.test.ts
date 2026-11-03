import { describe, it, expect } from 'vitest';
import { canonicalNumber, formatDecimal, formatDate, changedValues } from './values';
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
