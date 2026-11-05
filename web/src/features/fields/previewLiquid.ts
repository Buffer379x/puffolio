import type { Field, Liquid, Preferences, Scalar, Tag, Values } from '../../app/types';
import { shelfEnabled, type CardLayout } from '../liquids/cardLayout';
import { canonicalNumber } from './values';
const SAMPLE_COLOR = '#C5E879';
function sampleValue(field: Field, today: string): Scalar | undefined {
  switch (field.type) {
    case 'text':
    case 'longtext':
      return field.label;
    case 'number':
      return canonicalNumber('1', field.config.scale);
    case 'date':
      return today;
    case 'boolean':
      return true;
    case 'select':
      return field.config.options?.find((option) => !option.archived)?.id;
    case 'rating':
      return Math.round((field.config.stars || 5) * 0.8);
    case 'color':
      return SAMPLE_COLOR;
    case 'tags':
      return [`preview-${field.id}`];
    case 'image':
      // No value: the card falls back to its bottle, and nothing asks the server for a file.
      return undefined;
  }
}
/** A liquid that exists only for the preview, with a value in every field it can show. */
export function previewLiquid(
  fields: Field[],
  preferences: Preferences,
  today: string,
): { liquid: Liquid; tags: Tag[] } {
  const values: Values = {};
  const tags: Tag[] = [];
  for (const field of fields) {
    if (field.archived) continue;
    const value = sampleValue(field, today);
    if (value === undefined) continue;
    values[field.id] = { type: field.type, value };
    if (field.type === 'tags')
      tags.push({
        id: `preview-${field.id}`,
        fieldId: field.id,
        name: field.label,
        paletteId: 'lime',
        version: 0,
      });
  }
  return {
    liquid: {
      id: 'preview',
      name: 'Beispiel-Liquid',
      favorite: false,
      version: 0,
      createdAt: today,
      updatedAt: today,
      values,
      shelfLife: shelfEnabled(preferences)
        ? { percent: 60, expiresOn: today, expired: false }
        : undefined,
    },
    tags,
  };
}
/** The card from top to bottom in words, for the list beside the preview. */
export function cardOrder(layout: CardLayout): string[] {
  return [
    layout.stripped ? 'Farbstreifen' : 'Bild',
    'Name',
    ...layout.rows.flatMap((row) => {
      if (row.kind === 'shelf') return ['Haltbarkeit'];
      if (row.kind === 'data') return row.fields.map((field) => field.label);
      return [row.field.label];
    }),
  ];
}
