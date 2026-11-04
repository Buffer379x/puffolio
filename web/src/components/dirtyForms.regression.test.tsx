import { render, screen, fireEvent, waitFor, cleanup, within } from '@testing-library/react';
import { beforeAll, afterEach, it, expect, vi } from 'vitest';
import { AppContext } from '../app/state';
import { TagDialog } from './FieldInput';
import { DashboardPage } from '../features/dashboard/DashboardPage';
import { FieldsSettings } from '../features/fields/FieldsSettings';
import { LiquidsPage } from '../features/liquids/LiquidsPage';
const field = {
  id: 'field',
  label: 'Notiz',
  type: 'text',
  position: 0,
  version: 1,
  required: false,
  archived: false,
  visibility: { form: true, detail: true, list: true, grid: true },
  config: { defaultValue: 'Bisher' },
};
const preferences = {
  version: 1,
  theme: 'light',
  timezone: 'Europe/Berlin',
  view: 'grid',
  images: true,
  listFields: ['field'],
  gridFields: ['field'],
  roles: {},
};
const user = { id: 'u', username: 'test', displayName: 'Test', role: 'admin', version: 1 };
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
const cases = [
  {
    name: 'tag',
    title: 'Tag bearbeiten',
    component: () => (
      <TagDialog
        fieldId="tags"
        tag={{ id: 'tag', fieldId: 'tags', name: 'Alt', paletteId: 'lime', version: 1 }}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />
    ),
    open: async () => {},
    edit: () =>
      fireEvent.change(screen.getByLabelText('Tagname'), { target: { value: 'Entwurf' } }),
    draft: () => expect(screen.getByLabelText('Tagname')).toHaveValue('Entwurf'),
  },
  {
    name: 'widget',
    title: 'Widget bearbeiten',
    component: () => <DashboardPage />,
    open: async () => {
      fireEvent.click(await screen.findByRole('button', { name: 'Dashboard anpassen' }));
      fireEvent.click(screen.getByRole('button', { name: /bearbeiten$/ }));
    },
    edit: () =>
      fireEvent.change(screen.getByLabelText('Widgettitel'), { target: { value: 'Entwurf' } }),
    draft: () => expect(screen.getByLabelText('Widgettitel')).toHaveValue('Entwurf'),
  },
  {
    name: 'view',
    title: 'Ansicht',
    component: () => <LiquidsPage query="?view=grid" />,
    open: async () => fireEvent.click(await screen.findByRole('button', { name: 'Ansicht' })),
    edit: () => fireEvent.click(screen.getByLabelText('Bilder anzeigen')),
    draft: () => expect(screen.getByLabelText('Bilder anzeigen')).not.toBeChecked(),
  },
  {
    name: 'field default',
    title: 'Feld bearbeiten',
    component: () => <FieldsSettings />,
    open: async () => fireEvent.click(await screen.findByRole('button', { name: /bearbeiten$/ })),
    edit: () => fireEvent.click(screen.getByLabelText('Standardwert für neue Liquids')),
    draft: () => expect(screen.getByLabelText('Standardwert für neue Liquids')).not.toBeChecked(),
  },
];
it.each(cases)(
  'guards dirty $name drafts on navigation, unload, Escape and close until discard is confirmed',
  async (c) => {
    vi.stubGlobal('fetch', async (url: string) => {
      let data: unknown = { items: [] };
      if (url.endsWith('/dashboard'))
        data = {
          version: 1,
          widgets: [{ id: 'widget', type: 'count', title: 'Anzahl', size: 'small' }],
        };
      else if (url.endsWith('/dashboard/query')) data = { items: [{ id: 'widget', count: 0 }] };
      else if (url.endsWith('/admin/users')) data = { items: [user] };
      else if (url.endsWith('/admin/system')) data = { version: 'test' };
      else if (url.includes('/liquids?')) data = { items: [], total: 0, page: 1, pageSize: 24 };
      return new Response(JSON.stringify(data));
    });
    render(
      <AppContext.Provider
        value={
          {
            me: { user },
            fields: [field],
            preferences,
            schemaVersion: 1,
            savePreferences: async () => {},
            navigate: () => {},
            refresh: async () => {},
          } as any
        }
      >
        {c.component()}
      </AppContext.Provider>,
    );
    await c.open();
    c.edit();
    const navigation = new Event('puffolio-before-navigate', { cancelable: true });
    expect(window.dispatchEvent(navigation)).toBe(false);
    const unload = new Event('beforeunload', { cancelable: true });
    expect(window.dispatchEvent(unload)).toBe(false);
    const dialog = screen.getByRole('dialog', { name: c.title });
    fireEvent(dialog, new Event('cancel', { cancelable: true }));
    const confirmation = await screen.findByRole('dialog', { name: 'Änderungen verwerfen?' });
    c.draft();
    fireEvent.click(within(confirmation).getByRole('button', { name: 'Abbrechen' }));
    expect(screen.queryByRole('dialog', { name: 'Änderungen verwerfen?' })).not.toBeInTheDocument();
    c.draft();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Schließen' }));
    await screen.findByRole('dialog', { name: 'Änderungen verwerfen?' });
    fireEvent.click(screen.getByRole('button', { name: 'Bestätigen' }));
    await waitFor(() =>
      expect(
        screen.queryByRole('dialog', { name: 'Änderungen verwerfen?' }),
      ).not.toBeInTheDocument(),
    );
  },
);
