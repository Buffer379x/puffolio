import type { Filter } from '../../app/types';
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
