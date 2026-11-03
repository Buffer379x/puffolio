import { render, screen, fireEvent, waitFor, cleanup, within } from '@testing-library/react';
import { beforeAll, afterEach, it, expect, vi } from 'vitest';
import { AppContext } from '../../app/state';
import { LiquidsPage } from './LiquidsPage';
import { LiquidEditor } from './LiquidEditor';
const preferences = {
  version: 1,
  theme: 'light',
  timezone: 'Europe/Berlin',
  view: 'grid',
  images: false,
  listFields: [],
  gridFields: [],
  roles: {},
};
const record = {
  id: 'liquid',
  name: 'Original',
  favorite: false,
  version: 1,
  createdAt: '2026-01-01',
  updatedAt: '2026-01-01',
  values: {},
};
beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function () {
    this.open = false;
  };
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
it.each(['grid', 'list'])(
  'deliberately discards a409 draft and reopens the fresh version in %s without page refresh',
  async (view) => {
    let current = { ...record };
    const writes: number[] = [];
    vi.stubGlobal('fetch', async (url: string, init: RequestInit) => {
      if (url === '/api/v1/liquids/liquid') {
        if (init.method === 'PATCH') {
          const body = JSON.parse(String(init.body));
          writes.push(body.version);
          if (body.version !== current.version)
            return new Response(
              JSON.stringify({ error: { code: 'conflict', message: 'Geändert' } }),
              { status: 409 },
            );
          current = { ...current, name: body.name, version: current.version + 1 };
        }
        return new Response(JSON.stringify(current));
      }
      return new Response(
        JSON.stringify(
          url.includes('/liquids?')
            ? { items: [current], total: 1, page: 1, pageSize: 24 }
            : { items: [] },
        ),
      );
    });
    render(
      <AppContext.Provider
        value={
          { fields: [], preferences, navigate: () => {}, savePreferences: async () => {} } as any
        }
      >
        <LiquidsPage query={'?view=' + view} />
      </AppContext.Provider>,
    );
    const edit = () =>
      view === 'grid'
        ? screen.getByRole('button', { name: 'Bearbeiten' })
        : screen.getByRole('button', { name: /bearbeiten$/ });
    await screen.findByRole('button', { name: 'Original' });
    fireEvent.click(edit());
    fireEvent.change(await screen.findByLabelText('Name'), { target: { value: 'Mein Entwurf' } });
    current = { ...current, name: 'Aktueller Stand', version: 2 };
    fireEvent.click(screen.getByRole('button', { name: 'Speichern' }));
    await screen.findByText('Geändert');
    fireEvent.click(screen.getByRole('button', { name: 'Aktuellen Stand vergleichen' }));
    await screen.findByText(/Aktueller Stand · Version 2/);
    expect(screen.getByLabelText('Name')).toHaveValue('Mein Entwurf');
    fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));
    fireEvent.click(
      within(screen.getByRole('dialog', { name: 'Änderungen verwerfen?' })).getByRole('button', {
        name: 'Abbrechen',
      }),
    );
    expect(screen.getByLabelText('Name')).toHaveValue('Mein Entwurf');
    fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));
    fireEvent.click(screen.getByRole('button', { name: 'Bestätigen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    fireEvent.click(edit());
    expect(await screen.findByLabelText('Name')).toHaveValue('Aktueller Stand');
    fireEvent.change(screen.getByLabelText('Name'), {
      target: { value: 'Bewusst neu bearbeitet' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(writes).toEqual([1, 2]);
    expect(current.name).toBe('Bewusst neu bearbeitet');
  },
);
it('does not expose an editable stale fallback when the fresh-record load fails', async () => {
  vi.stubGlobal(
    'fetch',
    async () =>
      new Response('{"error":{"code":"unavailable","message":"Neu laden fehlgeschlagen"}}', {
        status: 503,
      }),
  );
  render(
    <AppContext.Provider value={{ fields: [] } as any}>
      <LiquidEditor liquid={record} onClose={() => {}} onSaved={() => {}} />
    </AppContext.Provider>,
  );
  await screen.findByText('Neu laden fehlgeschlagen');
  expect(screen.queryByLabelText('Name')).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: 'Speichern' })).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Erneut versuchen' })).toBeInTheDocument();
});
