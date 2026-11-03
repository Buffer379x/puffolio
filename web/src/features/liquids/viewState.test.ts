import { it, expect } from 'vitest';
import { readView, writeView } from './viewState';
it('changes only view while preserving page, search, favorite, sort and filters', () => {
  const query =
    '?page=2&pageSize=48&search=Zitrone&favorite=true&sort=name&direction=asc&filters=' +
    encodeURIComponent(
      JSON.stringify({ fieldId: 'date', operator: 'before', value: '2028-01-01' }),
    );
  const a = readView(query);
  const b = readView(writeView({ ...a, view: 'list' }));
  expect(b).toEqual({ ...a, view: 'list' });
});
