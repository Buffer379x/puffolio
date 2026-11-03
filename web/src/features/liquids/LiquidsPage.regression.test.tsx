import { render, screen, waitFor, cleanup } from '@testing-library/react';
import { beforeAll, afterEach, it, expect, vi } from 'vitest';
import { AppContext } from '../../app/state';
import type { Field, Preferences } from '../../app/types';
import { LiquidsPage } from './LiquidsPage';
const field = (id: string, type: Field['type']): Field => ({
  id,
  label: id,
  type,
  position: 0,
  version: 1,
  required: false,
  archived: false,
  visibility: { form: true, detail: true, list: true, grid: true },
  config: { stars: 5, scale: 1, unit: type === 'number' ? 'ml' : undefined },
});
const fields = [
  field('rating', 'rating'),
  field('quantity', 'number'),
  field('primary', 'image'),
  field('second', 'image'),
  field('color', 'color'),
];
const prefs: Preferences = {
  version: 1,
  theme: 'light',
  timezone: 'Europe/Berlin',
  view: 'grid',
  images: true,
  listFields: fields.map((f) => f.id),
  gridFields: fields.map((f) => f.id),
  roles: { rating: 'rating', quantity: 'quantity', image: 'primary', color: 'color' },
};
const liquid = {
  id: 'liquid',
  name: 'Testliquid',
  version: 1,
  favorite: false,
  values: {
    rating: { type: 'rating', value: 4 },
    quantity: { type: 'number', value: '12.0' },
    primary: { type: 'image', value: 'primary-media' },
    second: { type: 'image', value: 'second-media' },
    color: { type: 'color', value: '#123456' },
  },
};
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
function mount(preferences: Preferences, definitions = fields, view = 'grid') {
  vi.stubGlobal(
    'fetch',
    async (url: string) =>
      new Response(
        JSON.stringify(
          url.includes('/liquids?')
            ? { items: [liquid], total: 1, page: 1, pageSize: 24 }
            : { items: [] },
        ),
      ),
  );
  return render(
    <AppContext.Provider
      value={
        {
          fields: definitions,
          preferences,
          savePreferences: async () => {},
          navigate: () => {},
        } as any
      }
    >
      <LiquidsPage query={'?view=' + view} />
    </AppContext.Provider>,
  );
}
it('renders additional selected card image fields using their saved crop URL', async () => {
  mount(prefs);
  expect(await screen.findByRole('img', { name: 'second' })).toHaveAttribute(
    'src',
    '/api/v1/media/second-media',
  );
});
it.each(['deselected', 'field-hidden', 'archived'])(
  'hides primary role values and images when %s for the grid',
  async (mode) => {
    const definitions = fields.map((f) =>
      mode === 'field-hidden'
        ? { ...f, visibility: { ...f.visibility, grid: false } }
        : mode === 'archived'
          ? { ...f, archived: true }
          : f,
    );
    mount({ ...prefs, gridFields: mode === 'deselected' ? [] : prefs.gridFields }, definitions);
    await screen.findByRole('button', { name: 'Testliquid' });
    expect(screen.queryByLabelText('4 von 5')).not.toBeInTheDocument();
    expect(screen.queryByText('12,0 ml')).not.toBeInTheDocument();
    expect(document.querySelector('img[src="/api/v1/media/primary-media"]')).toBeNull();
    expect(document.querySelector('img[src="/api/v1/media/second-media"]')).toBeNull();
    expect(document.querySelector('[style*="rgb(18, 52, 86)"]')).toBeNull();
  },
);
it.each(['grid', 'list'])('honors images-off for every selected image in %s', async (view) => {
  mount({ ...prefs, images: false }, fields, view);
  await screen.findByRole('button', { name: 'Testliquid' });
  expect(document.querySelector('img[src^="/api/v1/media/"]')).toBeNull();
});
