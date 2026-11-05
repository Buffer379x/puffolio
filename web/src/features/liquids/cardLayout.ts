import type { Field, Preferences } from '../../app/types';
/** One line of a card below its name. The rows are the same for every card in a view. */
export type CardRow =
  | { kind: 'subtitle' | 'rating' | 'tags'; field: Field }
  | { kind: 'data'; fields: Field[] }
  | { kind: 'shelf' };
export interface CardLayout {
  rows: CardRow[];
  /** Images are off or the image field is not chosen: a colour band replaces the image. */
  stripped: boolean;
}
export function shownFields(fields: Field[], preferences: Preferences, view: 'grid' | 'list') {
  const chosen = view === 'grid' ? preferences.gridFields : preferences.listFields;
  return fields.filter(
    (f) =>
      !f.archived &&
      f.visibility[view] &&
      (preferences.images || f.type !== 'image') &&
      chosen.includes(f.id),
  );
}
export function shelfEnabled(preferences: Preferences) {
  return (
    !!preferences.roles.purchase &&
    (preferences.shelfLifeMonths > 0 || !!preferences.roles.shelfOverride)
  );
}
/** What a card shows and in which order: the field order, whatever a view shows of it. */
export function cardLayout(fields: Field[], preferences: Preferences): CardLayout {
  const { image, rating, color } = preferences.roles;
  const shown = shownFields(fields, preferences, 'grid');
  const imageRole = fields.find((f) => f.id === image && !f.archived);
  const subtitle = shown.find((f) => f.type === 'text');
  const rows: CardRow[] = [];
  for (const field of shown) {
    if (field.id === image || field.id === color) continue;
    const last = rows[rows.length - 1];
    if (field === subtitle) rows.push({ kind: 'subtitle', field });
    else if (field.id === rating) rows.push({ kind: 'rating', field });
    else if (field.type === 'tags') rows.push({ kind: 'tags', field });
    else if (last?.kind === 'data') last.fields.push(field);
    else rows.push({ kind: 'data', fields: [field] });
  }
  if (shelfEnabled(preferences)) {
    const after = (kind: CardRow['kind']) => rows.findIndex((row) => row.kind === kind);
    const anchor = after('rating') >= 0 ? after('rating') : after('subtitle');
    rows.splice(anchor + 1, 0, { kind: 'shelf' });
  }
  return {
    rows,
    stripped: !preferences.images || (!!imageRole && !shown.includes(imageRole)),
  };
}
