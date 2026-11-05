import { expect, it } from 'vitest';
import type { Field, FieldType, Preferences } from '../../app/types';
import { cardLayout, shelfEnabled, shownFields } from './cardLayout';
const field = (id: string, type: FieldType, patch: Partial<Field> = {}): Field => ({
  id,
  label: id,
  type,
  position: 0,
  version: 1,
  required: false,
  archived: false,
  visibility: { form: true, detail: true, list: true, grid: true },
  config: {},
  ...patch,
});
const prefs = (fields: Field[], patch: Partial<Preferences> = {}): Preferences =>
  ({
    images: true,
    gridFields: fields.map((f) => f.id),
    listFields: [],
    roles: { image: 'bild', rating: 'note', color: 'farbe', purchase: 'kauf' },
    shelfLifeMonths: 12,
    ...patch,
  }) as Preferences;
/** Rows as compact strings, so a test reads like the card from top to bottom. */
const rows = (fields: Field[], patch: Partial<Preferences> = {}) =>
  cardLayout(fields, prefs(fields, patch)).rows.map((row) =>
    row.kind === 'shelf'
      ? 'shelf'
      : row.kind === 'data'
        ? `data[${row.fields.map((f) => f.id).join(',')}]`
        : `${row.kind}[${row.field.id}]`,
  );
const hersteller = field('hersteller', 'text');
const note = field('note', 'rating');
const bild = field('bild', 'image');
const tags = field('tags', 'tags');
const farbe = field('farbe', 'color');
const nikotin = field('nikotin', 'number');
it('keeps the familiar card for the starter order', () => {
  expect(rows([hersteller, note, bild, tags, farbe, nikotin])).toEqual([
    'subtitle[hersteller]',
    'rating[note]',
    'shelf',
    'tags[tags]',
    'data[nikotin]',
  ]);
});
it('follows the field order', () => {
  expect(rows([nikotin, tags, note, hersteller])).toEqual([
    'data[nikotin]',
    'tags[tags]',
    'rating[note]',
    'shelf',
    'subtitle[hersteller]',
  ]);
});
it('treats only the first text field as the subtitle', () => {
  const second = field('zweit', 'text');
  expect(rows([hersteller, second], { shelfLifeMonths: 0, roles: {} })).toEqual([
    'subtitle[hersteller]',
    'data[zweit]',
  ]);
});
it('groups neighbouring data fields and separates them at other rows', () => {
  const a = field('a', 'number');
  const b = field('b', 'date');
  const c = field('c', 'boolean');
  expect(rows([a, b, tags, c], { shelfLifeMonths: 0, roles: {} })).toEqual([
    'data[a,b]',
    'tags[tags]',
    'data[c]',
  ]);
});
it('places the shelf bar behind the subtitle when no rating is visible', () => {
  expect(rows([hersteller, tags])).toEqual(['subtitle[hersteller]', 'shelf', 'tags[tags]']);
});
it('leads with the shelf bar when neither rating nor subtitle is visible', () => {
  expect(rows([tags, nikotin])).toEqual(['shelf', 'tags[tags]', 'data[nikotin]']);
});
it('shows no shelf row when shelf life is off', () => {
  expect(rows([note], { shelfLifeMonths: 0 })).toEqual(['rating[note]']);
  expect(rows([note], { roles: { rating: 'note' } })).toEqual(['rating[note]']);
});
it('treats further rating fields as data', () => {
  expect(rows([note, field('zweite', 'rating')], { shelfLifeMonths: 0 })).toEqual([
    'rating[note]',
    'data[zweite]',
  ]);
});
it('gives image and colour roles no row', () => {
  expect(rows([bild, farbe], { shelfLifeMonths: 0 })).toEqual([]);
});
it('leaves out archived, unchosen and not visible fields', () => {
  const fields = [
    field('archiviert', 'number', { archived: true }),
    field('nicht-gewaehlt', 'number'),
    field('verborgen', 'number', {
      visibility: { form: true, detail: true, list: true, grid: false },
    }),
  ];
  const chosen = { gridFields: ['archiviert', 'verborgen'], shelfLifeMonths: 0 };
  expect(rows(fields, chosen)).toEqual([]);
});
it('strips the image when images are off or the image field is not shown', () => {
  expect(cardLayout([bild], prefs([bild])).stripped).toBe(false);
  expect(cardLayout([bild], prefs([bild], { images: false })).stripped).toBe(true);
  expect(cardLayout([bild], prefs([bild], { gridFields: [] })).stripped).toBe(true);
  expect(cardLayout([], prefs([])).stripped).toBe(false);
});
it('selects fields per view', () => {
  const list = field('nur-liste', 'number', {
    visibility: { form: true, detail: true, list: true, grid: false },
  });
  const p = prefs([list], { listFields: ['nur-liste'], gridFields: ['nur-liste'] });
  expect(shownFields([list], p, 'list')).toEqual([list]);
  expect(shownFields([list], p, 'grid')).toEqual([]);
});
it('needs a purchase field and a storage time for the shelf bar', () => {
  expect(shelfEnabled(prefs([]))).toBe(true);
  expect(shelfEnabled(prefs([], { roles: {} }))).toBe(false);
  expect(shelfEnabled(prefs([], { shelfLifeMonths: 0 }))).toBe(false);
  expect(
    shelfEnabled(prefs([], { shelfLifeMonths: 0, roles: { purchase: 'k', shelfOverride: 'o' } })),
  ).toBe(true);
});
