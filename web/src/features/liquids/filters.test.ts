import { describe, expect, it } from 'vitest';
import type { Field, Tag } from '../../app/types';
import {
  countConditions,
  quickTags,
  withQuickTags,
  withoutQuickTags,
  withoutUnknownTags,
} from './filters';
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
describe('Gelöschte Tags im Link-Filter', () => {
  const fields = [
    { id: 'tags', type: 'tags', archived: false },
    { id: 'old', type: 'tags', archived: true },
    { id: 'status', type: 'select', archived: false },
  ] as Field[];
  const tags = [{ id: 'a', fieldId: 'tags' }] as Tag[];
  const clean = (filter: never) => withoutUnknownTags(filter, tags, fields);
  const on = (value: string[], operator = 'any', fieldId = 'tags') => ({
    fieldId,
    operator,
    value,
  });
  it.each(['any', 'all', 'none'])('nimmt die gelöschte ID aus „%s“', (operator) => {
    expect(clean(on(['a', 'gone'], operator) as never)).toEqual(on(['a'], operator));
  });
  it('entfernt eine leere Bedingung und eine leere Gruppe', () => {
    expect(clean(on(['gone']) as never)).toBeUndefined();
    expect(clean({ op: 'and', conditions: [on(['gone'])] } as never)).toBeUndefined();
    expect(clean({ op: 'and', conditions: [other, on(['gone'])] } as never)).toEqual({
      op: 'and',
      conditions: [other],
    });
    expect(
      clean({ op: 'and', conditions: [{ op: 'or', conditions: [on(['gone']), other] }] } as never),
    ).toEqual({ op: 'and', conditions: [{ op: 'or', conditions: [other] }] });
  });
  it.each([
    ['bekannte Tags', on(['a'])],
    ['ein Tag aus einem anderen Feld als Auswahloption', on(['gone'], 'any', 'status')],
    ['ein archiviertes Tagfeld, dessen Tags nicht geladen sind', on(['gone'], 'any', 'old')],
    ['eine Bedingung ohne IDs', { fieldId: 'tags', operator: 'notEmpty' }],
    ['ein ungültiger Wert', { fieldId: 'tags', operator: 'any', value: 'gone' }],
    ['ein unbekanntes Feld', on(['gone'], 'any', 'unknown')],
    ['eine ungültige Filterstruktur', [other]],
    ['eine Gruppe ohne gelöschte Tags', { op: 'and', conditions: [on(['a']), other] }],
  ])('lässt %s unverändert', (_, filter) => {
    expect(clean(filter as never)).toBe(filter);
  });
  it('lässt keinen Filter ohne Filter', () => {
    expect(clean(undefined as never)).toBeUndefined();
  });
  it('behandelt einen Tag aus einem anderen Feld als unbekannt', () => {
    const two = [{ id: 'tags', type: 'tags', archived: false }, { id: 'more', type: 'tags', archived: false }] as Field[];
    expect(withoutUnknownTags(on(['a'], 'any', 'more') as never, tags, two)).toBeUndefined();
  });
});
