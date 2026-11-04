import { describe, expect, it } from 'vitest';
import { countConditions, quickTags, withQuickTags, withoutQuickTags } from './filters';
const other = { fieldId: 'rating', operator: 'gte', value: 4 };
describe('Tag-Schnellfilter', () => {
  it('fügt sich als Bedingung neben vorhandene Filter ein und lässt sich wieder entfernen', () => {
    const on = withQuickTags(other, 'tags', ['a', 'b']);
    expect(on).toEqual({
      op: 'and',
      conditions: [other, { fieldId: 'tags', operator: 'any', value: ['a', 'b'] }],
    });
    expect(quickTags(on, 'tags')).toEqual(['a', 'b']);
    expect(countConditions(withoutQuickTags(on, 'tags'))).toBe(1);
    expect(withQuickTags(on, 'tags', [])).toEqual({ op: 'and', conditions: [other] });
    expect(withQuickTags(undefined, 'tags', [])).toBeUndefined();
  });
  it('lässt ODER-Gruppen unverändert und verknüpft sie mit UND', () => {
    const group = { op: 'or' as const, conditions: [other, other] };
    expect(withoutQuickTags(group, 'tags')).toBe(group);
    expect(withQuickTags(group, 'tags', ['a'])).toEqual({
      op: 'and',
      conditions: [group, { fieldId: 'tags', operator: 'any', value: ['a'] }],
    });
  });
  it.each([true, { op: 'and', conditions: {} }, { op: 'and', conditions: [null] }, [other]])(
    'erweitert einen ungültigen Link-Filter %j nicht stillschweigend',
    (broken) => {
      const filter = broken as never;
      expect(quickTags(filter, 'tags')).toEqual([]);
      expect(withoutQuickTags(filter, 'tags')).toBe(broken);
      expect(withQuickTags(filter, 'tags', ['a'])).toBe(broken);
      expect(countConditions(filter)).toBe(1);
    },
  );
});
