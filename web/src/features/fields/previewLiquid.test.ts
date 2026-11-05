import { expect, it } from 'vitest';
import type { Field, FieldType, Preferences } from '../../app/types';
import { cardLayout } from '../liquids/cardLayout';
import { cardOrder, previewLiquid } from './previewLiquid';
const today = '2026-11-05';
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
    roles: { image: 'bild', rating: 'note', purchase: 'kauf' },
    shelfLifeMonths: 12,
    ...patch,
  }) as Preferences;
const valueOf = (f: Field, patch: Partial<Preferences> = {}) =>
  previewLiquid([f], prefs([f], patch), today).liquid.values[f.id];

it('names the sample so that nobody takes it for a real liquid', () => {
  expect(previewLiquid([], prefs([]), today).liquid.name).toBe('Beispiel-Liquid');
});
it('uses the field name as the value of text fields', () => {
  expect(valueOf(field('Hersteller', 'text'))?.value).toBe('Hersteller');
  expect(valueOf(field('Notizen', 'longtext'))?.value).toBe('Notizen');
});
it('writes numbers in the scale of their field', () => {
  expect(valueOf(field('Menge', 'number'))?.value).toBe('1');
  expect(valueOf(field('Nikotin', 'number', { config: { scale: 2 } }))?.value).toBe('1.00');
});
it('fills date, boolean and colour', () => {
  expect(valueOf(field('Kauf', 'date'))?.value).toBe(today);
  expect(valueOf(field('Offen', 'boolean'))?.value).toBe(true);
  expect(valueOf(field('Farbe', 'color'))?.value).toBe('#C5E879');
});
it('takes the first option that is not archived', () => {
  const options = [
    { id: 'alt', label: 'Alt', archived: true },
    { id: 'neu', label: 'Neu' },
  ];
  expect(valueOf(field('Status', 'select', { config: { options } }))?.value).toBe('neu');
});
it('leaves a select without a usable option empty', () => {
  const options = [{ id: 'alt', label: 'Alt', archived: true }];
  expect(valueOf(field('Status', 'select', { config: { options } }))).toBeUndefined();
  expect(valueOf(field('Leer', 'select'))).toBeUndefined();
});
it('rates four fifths of the scale', () => {
  const rating = (stars?: number) => valueOf(field('Note', 'rating', { config: { stars } }))?.value;
  expect(rating(5)).toBe(4);
  expect(rating(3)).toBe(2);
  expect(rating(10)).toBe(8);
  expect(rating()).toBe(4);
});
it('makes one lime tag named after its field', () => {
  const f = field('Aromen', 'tags');
  const { liquid, tags } = previewLiquid([f], prefs([f]), today);
  expect(tags).toHaveLength(1);
  expect(tags[0]).toMatchObject({ fieldId: 'Aromen', name: 'Aromen', paletteId: 'lime' });
  expect(liquid.values.Aromen?.value).toEqual([tags[0].id]);
});
it('leaves the image empty, so the bottle placeholder shows', () => {
  expect(valueOf(field('bild', 'image'))).toBeUndefined();
});
it('ignores archived fields', () => {
  const f = field('Alt', 'text', { archived: true });
  expect(previewLiquid([f], prefs([f]), today).liquid.values.Alt).toBeUndefined();
});
it('shows a shelf life only where one applies', () => {
  const on = previewLiquid([], prefs([]), today).liquid.shelfLife;
  expect(on).toMatchObject({ percent: 60, expired: false });
  const off = prefs([], { roles: {} });
  expect(previewLiquid([], off, today).liquid.shelfLife).toBeUndefined();
});
it('lists a card from top to bottom, one line per field', () => {
  const fields = [
    field('Hersteller', 'text'),
    field('note', 'rating', { label: 'Bewertung' }),
    field('bild', 'image'),
    field('Aromen', 'tags'),
    field('Menge', 'number'),
    field('Kauf', 'date'),
  ];
  const preferences = prefs(fields, { roles: { image: 'bild', rating: 'note', purchase: 'Kauf' } });
  expect(cardOrder(cardLayout(fields, preferences))).toEqual([
    'Bild',
    'Name',
    'Hersteller',
    'Bewertung',
    'Haltbarkeit',
    'Aromen',
    'Menge',
    'Kauf',
  ]);
});
it('names the colour band when images are off', () => {
  const fields = [field('bild', 'image')];
  const layout = cardLayout(fields, prefs(fields, { images: false }));
  expect(cardOrder(layout).slice(0, 2)).toEqual(['Farbstreifen', 'Name']);
});
