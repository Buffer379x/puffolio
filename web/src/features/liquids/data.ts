import { useTagRevision } from '../../app/tagEvents';
import { useApp, useLoad } from '../../app/state';
import { api } from '../../app/api';
import type { Tag, Filter, Field } from '../../app/types';
import { canonicalNumber } from '../fields/values';
export function useTags() {
  const { fields } = useApp();
  const revision = useTagRevision();
  return useLoad(
    async (s) =>
      (
        await Promise.all(
          fields
            .filter((f) => f.type === 'tags' && !f.archived)
            .map((f) => api<{ items: Tag[] }>('/fields/' + f.id + '/tags', 'GET', undefined, s)),
        )
      ).flatMap((r) => r.items || []),
    [fields, revision],
  );
}
export function normalizeFilter(filter: Filter | undefined, fields: Field[]): Filter | undefined {
  if (!filter) return undefined;
  if (filter.op) {
    if (!filter.conditions?.length) return undefined;
    return {
      ...filter,
      conditions: filter.conditions.map((c) => normalizeFilter(c, fields)!).filter(Boolean),
    };
  }
  const f = fields.find((f) => f.id === filter.fieldId);
  const c = { ...filter };
  if (f?.type === 'number')
    for (const key of ['value', 'value2'] as const)
      if (c[key] !== undefined && c[key] !== null)
        c[key] = canonicalNumber(String(c[key]), f.config.scale || 0);
  return c;
}
