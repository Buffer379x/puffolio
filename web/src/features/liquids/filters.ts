import type { Field, Filter, Tag } from '../../app/types';
import { isFilterStructure } from './FilterBuilder';
/**
 * The tag chips on the Liquids page are shorthand for one "any of" condition in
 * the same filter tree the API and the filter dialog already use, so they
 * combine with everything else and survive reloads and saved views.
 *
 * A filter from a hand-edited link may be malformed. It passes through untouched,
 * stays visible as invalid, and is never quietly widened into showing more.
 */
const isQuick = (c: Filter, fieldId: string) =>
  !c.op && c.fieldId === fieldId && c.operator === 'any' && Array.isArray(c.value);
const parts = (filter?: Filter): Filter[] =>
  !filter ? [] : filter.op === 'and' ? filter.conditions || [] : [filter];
export function quickTags(filter: Filter | undefined, fieldId: string | undefined): string[] {
  if (!fieldId || !isFilterStructure(filter)) return [];
  const found = parts(filter).find((c) => isQuick(c, fieldId));
  return (found?.value as string[] | undefined) || [];
}
export function withoutQuickTags(filter: Filter | undefined, fieldId: string | undefined) {
  if (!fieldId || !isFilterStructure(filter)) return filter;
  const all = parts(filter);
  const rest = all.filter((c) => !isQuick(c, fieldId));
  if (rest.length === all.length) return filter;
  return rest.length ? { op: 'and' as const, conditions: rest } : undefined;
}
export function withQuickTags(
  filter: Filter | undefined,
  fieldId: string | undefined,
  ids: string[],
): Filter | undefined {
  if (filter !== undefined && !isFilterStructure(filter)) return filter;
  const rest = parts(withoutQuickTags(filter, fieldId));
  if (!ids.length || !fieldId) return rest.length ? withoutQuickTags(filter, fieldId) : undefined;
  return { op: 'and', conditions: [...rest, { fieldId, operator: 'any', value: ids }] };
}
export function countConditions(filter?: Filter): number {
  if (!filter) return 0;
  if (!isFilterStructure(filter)) return 1;
  return filter.op ? (filter.conditions || []).reduce((sum, c) => sum + countConditions(c), 0) : 1;
}
/**
 * A link may still name a tag deleted since. Its condition would show nothing, with no chip to
 * remove on the quick tag field, so the id goes. Only tag fields whose tags are loaded count; a
 * malformed filter stays untouched as above. The same object comes back if nothing changed.
 */
export function withoutUnknownTags(
  filter: Filter | undefined,
  tags: Tag[],
  fields: Field[],
): Filter | undefined {
  if (!isFilterStructure(filter)) return filter;
  const known = new Set(fields.filter((f) => f.type === 'tags' && !f.archived).map((f) => f.id));
  const clean = (node: Filter): Filter | undefined => {
    if (node.op) {
      const conditions = node.conditions || [];
      const kept = conditions.map(clean);
      if (kept.every((c, i) => c === conditions[i])) return node;
      const rest = kept.filter((c) => c !== undefined);
      return rest.length ? { ...node, conditions: rest } : undefined;
    }
    if (
      !known.has(node.fieldId as string) ||
      !['any', 'all', 'none'].includes(node.operator as string) ||
      !Array.isArray(node.value)
    )
      return node;
    const ids = node.value as string[];
    const value = ids.filter((id) => tags.some((t) => t.id === id && t.fieldId === node.fieldId));
    if (value.length === ids.length) return node;
    return value.length ? { ...node, value } : undefined;
  };
  return clean(filter);
}
