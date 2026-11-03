import type { Filter } from '../../app/types';
export interface ViewState {
  view: 'grid' | 'list';
  search: string;
  favorite: boolean;
  sort: string;
  direction: string;
  page: number;
  pageSize: number;
  filters?: Filter;
}
export function readView(query: string): ViewState {
  const p = new URLSearchParams(query);
  let filters: Filter | undefined;
  try {
    filters = JSON.parse(p.get('filters') || 'null') || undefined;
  } catch {
    /* Invalid external URL is not sent as a query. */
  }
  return {
    view: p.get('view') === 'list' ? 'list' : 'grid',
    search: p.get('search') || '',
    favorite: p.get('favorite') === 'true',
    sort: p.get('sort') || 'createdAt',
    direction: p.get('direction') === 'asc' ? 'asc' : 'desc',
    page: Math.max(1, Math.min(1000000, Number(p.get('page')) || 1)),
    pageSize: [24, 48, 96].includes(Number(p.get('pageSize'))) ? Number(p.get('pageSize')) : 24,
    filters,
  };
}
export function writeView(s: ViewState): string {
  const p = new URLSearchParams({
    view: s.view,
    search: s.search,
    favorite: String(s.favorite),
    sort: s.sort,
    direction: s.direction,
    page: String(s.page),
    pageSize: String(s.pageSize),
  });
  if (s.filters) p.set('filters', JSON.stringify(s.filters));
  return '?' + p.toString();
}
