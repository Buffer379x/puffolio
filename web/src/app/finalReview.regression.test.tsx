import { render, screen, fireEvent, cleanup, waitFor, within } from '@testing-library/react';
import { beforeAll, afterEach, it, expect, vi } from 'vitest';
import type { ReactNode } from 'react';
import { AppContext } from './state';
import type { Field, Filter } from './types';
import { DashboardPage } from '../features/dashboard/DashboardPage';
import { FieldsSettings } from '../features/fields/FieldsSettings';
import { AdminPage } from '../features/settings/AdminPage';
import { LiquidsPage } from '../features/liquids/LiquidsPage';
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
  vi.restoreAllMocks();
});
const numberField: Field = {
  id: 'amount',
  label: 'Vorrat',
  type: 'number',
  position: 0,
  version: 1,
  required: false,
  archived: false,
  visibility: { form: true, detail: true, list: true, grid: true },
  config: { scale: 1, unit: 'ml', filterable: true },
};
function mount(child: ReactNode, fields: Field[] = [], navigate = (path: string) => {}) {
  return render(
    <AppContext.Provider
      value={
        {
          fields,
          navigate,
          refresh: async () => {},
          schemaVersion: 1,
          me: { user: { role: 'admin' } },
          preferences: {
            view: 'grid',
            images: false,
            listFields: [],
            gridFields: [],
            roles: {},
          },
          savePreferences: async () => {},
        } as any
      }
    >
      {child}
    </AppContext.Provider>,
  );
}
function withoutUUID() {
  const getRandomValues = crypto.getRandomValues.bind(crypto);
  vi.stubGlobal('crypto', { getRandomValues });
}
it('creates distinct dashboard widgets when LAN HTTP has no randomUUID', async () => {
  withoutUUID();
  let saved: any;
  vi.stubGlobal('fetch', async (url: string, init: RequestInit) => {
    const body = init.body ? JSON.parse(String(init.body)) : undefined;
    if (url.endsWith('/dashboard/query'))
      return new Response(
        JSON.stringify({
          items: body.widgets.map((w: any) => ({ id: w.id, count: 0 })),
        }),
      );
    if (init.method === 'PUT') {
      saved = body;
      return new Response(JSON.stringify({ ...body, version: 2 }));
    }
    return new Response('{"version":1,"widgets":[]}');
  });
  mount(<DashboardPage />);
  await screen.findByRole('button', { name: 'Dashboard anpassen' });
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Dashboard anpassen' })).toBeEnabled(),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Dashboard anpassen' }));
  for (let i = 0; i < 2; i++) {
    fireEvent.click(screen.getByRole('button', { name: 'Widget hinzufügen' }));
    fireEvent.click(screen.getByRole('button', { name: 'Übernehmen' }));
  }
  fireEvent.click(screen.getByRole('button', { name: 'Speichern' }));
  await waitFor(() => expect(saved?.widgets).toHaveLength(2));
  for (const w of saved.widgets)
    expect(w.id).toMatch(/^[\da-f]{8}-[\da-f]{4}-4[\da-f]{3}-[89ab][\da-f]{3}-[\da-f]{12}$/);
  expect(saved.widgets[0].id).not.toBe(saved.widgets[1].id);
});
it('adds and saves select options when randomUUID is unavailable', async () => {
  withoutUUID();
  let saved: any;
  vi.stubGlobal('fetch', async (_url: string, init: RequestInit) => {
    saved = JSON.parse(String(init.body));
    return new Response(JSON.stringify(saved));
  });
  mount(<FieldsSettings />);
  fireEvent.click(screen.getByRole('button', { name: 'Feld hinzufügen' }));
  fireEvent.change(screen.getByLabelText('Feldname'), {
    target: { value: 'Geschmack' },
  });
  fireEvent.change(screen.getByLabelText('Feldtyp'), {
    target: { value: 'select' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Option hinzufügen' }));
  fireEvent.change(screen.getByLabelText('Option 1'), {
    target: { value: 'Frucht' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Speichern' }));
  await waitFor(() => expect(saved?.config.options[0].label).toBe('Frucht'));
  expect(saved.config.options[0].id).toMatch(
    /^[\da-f]{8}-[\da-f]{4}-4[\da-f]{3}-[89ab][\da-f]{3}-[\da-f]{12}$/,
  );
});
it('selects the invitation link and instructs manual copying without Clipboard API', async () => {
  vi.stubGlobal('navigator', { clipboard: undefined });
  vi.stubGlobal(
    'fetch',
    async (url: string, init: RequestInit) =>
      new Response(
        JSON.stringify(
          init.method === 'POST'
            ? { invitationToken: 'one-time-token' }
            : url.endsWith('/users')
              ? { items: [] }
              : { version: 'test' },
        ),
      ),
  );
  mount(<AdminPage />);
  fireEvent.click(screen.getByRole('button', { name: 'Nutzer einladen' }));
  fireEvent.change(screen.getByLabelText('Benutzername'), {
    target: { value: 'guest' },
  });
  fireEvent.change(screen.getByLabelText('Anzeigename'), {
    target: { value: 'Gast' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Einladung erstellen' }));
  const input = (await screen.findByRole('textbox', {
    name: 'Einladungslink',
  })) as HTMLInputElement;
  fireEvent.click(screen.getByRole('button', { name: 'Link kopieren' }));
  await screen.findByText('Link markiert. Bitte manuell kopieren.');
  expect(input).toHaveFocus();
  expect(input.selectionStart).toBe(0);
  expect(input.selectionEnd).toBe(input.value.length);
  expect(screen.queryByRole('button', { name: 'Kopiert' })).not.toBeInTheDocument();
});
it.each([
  { count: 0, value: undefined, expected: 'Keine Daten' },
  { count: 1, value: '0.0', expected: '0,0' },
  { count: 2, value: '0.0', expected: '0,0' },
])(
  'shows sum $count with separate missing data and measured zero',
  async ({ count, value, expected }) => {
    vi.stubGlobal(
      'fetch',
      async (url: string) =>
        new Response(
          JSON.stringify(
            url.endsWith('/dashboard/query')
              ? { items: [{ id: 'sum', count, value }] }
              : {
                  version: 1,
                  widgets: [
                    {
                      id: 'sum',
                      type: 'sum',
                      title: 'Bestand',
                      size: 'small',
                      fieldId: 'amount',
                    },
                  ],
                },
          ),
        ),
    );
    mount(<DashboardPage />, [numberField]);
    const text = await screen.findByText(expected, { exact: true });
    expect(text).toBeVisible();
    const widget = text.closest('.widget')!;
    if (count === 0)
      expect(within(widget as HTMLElement).queryByText('ml')).not.toBeInTheDocument();
    else expect(within(widget as HTMLElement).getByText('ml')).toBeVisible();
  },
);
it('summarizes nested filters and removes one condition without flattening AND/OR groups', async () => {
  const filters: Filter = {
    op: 'and',
    conditions: [
      {
        op: 'or',
        conditions: [
          { fieldId: 'amount', operator: 'gte', value: '2.0' },
          { fieldId: 'amount', operator: 'lte', value: '8.0' },
        ],
      },
      { fieldId: 'favorite', operator: 'eq', value: true },
    ],
  };
  let route = '';
  vi.stubGlobal('fetch', async () => new Response('{"items":[],"total":0,"page":1,"pageSize":24}'));
  mount(
    <LiquidsPage query={'?filters=' + encodeURIComponent(JSON.stringify(filters))} />,
    [numberField],
    (p) => {
      route = p;
    },
  );
  expect(screen.getByText('Alle Bedingungen (UND)')).toBeVisible();
  expect(screen.getByText('Eine Bedingung (ODER)')).toBeVisible();
  fireEvent.click(
    screen.getByRole('button', {
      name: 'Filter entfernen: Vorrat Mindestens 2,0 ml',
    }),
  );
  expect(JSON.parse(new URLSearchParams(route.split('?')[1]).get('filters')!)).toEqual({
    op: 'and',
    conditions: [
      {
        op: 'or',
        conditions: [{ fieldId: 'amount', operator: 'lte', value: '8.0' }],
      },
      { fieldId: 'favorite', operator: 'eq', value: true },
    ],
  });
  expect(screen.getByRole('button', { name: 'Feldfilter entfernen' })).toBeVisible();
});
it('exports only selected liquid IDs from the selection bar', async () => {
  let selection: any;
  vi.stubGlobal('fetch', async (url: string, init: RequestInit) => {
    if (url.endsWith('/exports')) {
      selection = JSON.parse(String(init.body));
      return new Response('zip');
    }
    return new Response(
      JSON.stringify(
        url.includes('/liquids?')
          ? {
              items: [
                {
                  id: 'one',
                  name: 'Eins',
                  version: 1,
                  favorite: false,
                  values: {},
                },
                {
                  id: 'two',
                  name: 'Zwei',
                  version: 1,
                  favorite: false,
                  values: {},
                },
              ],
              total: 2,
              page: 1,
              pageSize: 24,
            }
          : { items: [] },
      ),
    );
  });
  vi.stubGlobal(
    'URL',
    Object.assign(URL, {
      createObjectURL: () => 'blob:export',
      revokeObjectURL: () => {},
    }),
  );
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  mount(<LiquidsPage query="" />);
  await screen.findByRole('button', { name: 'Eins' });
  fireEvent.click(screen.getByRole('checkbox', { name: 'Eins auswählen' }));
  fireEvent.click(screen.getByRole('button', { name: 'Auswahl exportieren' }));
  await waitFor(() => expect(selection).toEqual({ ids: ['one'] }));
  expect(screen.getByRole('button', { name: 'Gesamte Sammlung exportieren' })).toBeVisible();
});

it.each(
  [
    { op: 'and', conditions: [null] },
    { op: 'and', conditions: {} },
    { op: 'and', conditions: [{ op: 'or', conditions: [null] }] },
    { op: 'and', conditions: [false] },
    [{ fieldId: 'favorite', operator: 'eq', value: true }],
    true,
  ].map((filters) => ({ filters })),
)(
  'keeps malformed URL filter $filters visible and removable without broadening its API query',
  async ({ filters }) => {
    let route = '';
    let requested: unknown;
    vi.stubGlobal('fetch', async (url: string) => {
      if (url.includes('/liquids?')) {
        requested = JSON.parse(new URL(url, 'http://localhost').searchParams.get('filters')!);
        return new Response(
          JSON.stringify({ error: { code: 'validation', message: 'Ungültiger Filter' } }),
          { status: 422 },
        );
      }
      return new Response('{"items":[]}');
    });
    mount(
      <LiquidsPage query={'?filters=' + encodeURIComponent(JSON.stringify(filters))} />,
      [numberField],
      (path) => {
        route = path;
      },
    );
    expect(await screen.findByText('Ungültiger Filter')).toBeVisible();
    expect(requested).toEqual(filters);
    expect(route).toBe('');
    expect(screen.getByRole('button', { name: 'Ungültigen Feldfilter entfernen' })).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Filter aktiv' }));
    const dialog = screen.getByRole('dialog', { name: 'Filter' });
    expect(
      within(dialog).getByText('Ungültiger Feldfilter. Bitte entfernen und neu anlegen.'),
    ).toBeVisible();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    expect(route).toBe('');
    fireEvent.click(screen.getByRole('button', { name: 'Ungültigen Feldfilter entfernen' }));
    expect(new URLSearchParams(route.split('?')[1]).has('filters')).toBe(false);
  },
);
