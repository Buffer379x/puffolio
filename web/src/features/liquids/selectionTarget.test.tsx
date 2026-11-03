import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, it, expect, vi } from 'vitest';
import { AppContext } from '../../app/state';
import { LiquidsPage } from './LiquidsPage';
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
it('selects the current page from the clickable header label and retains native Space toggling', async () => {
  vi.stubGlobal(
    'fetch',
    async (url: string) =>
      new Response(
        JSON.stringify(
          url.includes('/liquids?')
            ? {
                items: [
                  { id: 'one', name: 'Eins', favorite: false, version: 1, values: {} },
                  { id: 'two', name: 'Zwei', favorite: false, version: 1, values: {} },
                ],
                total: 2,
                page: 1,
                pageSize: 24,
              }
            : { items: [] },
        ),
      ),
  );
  render(
    <AppContext.Provider
      value={
        {
          fields: [],
          preferences: { view: 'list', images: false, listFields: [], gridFields: [], roles: {} },
          navigate: () => {},
          savePreferences: async () => {},
        } as any
      }
    >
      <LiquidsPage query="?view=list" />
    </AppContext.Provider>,
  );
  await screen.findByRole('button', { name: 'Eins' });
  const checkbox = screen.getByRole('checkbox', { name: 'Alle auf dieser Seite auswählen' });
  const label = (checkbox as HTMLInputElement).labels?.[0];
  expect(label).toBeDefined();
  await userEvent.click(label!);
  expect(screen.getByRole('checkbox', { name: 'Eins auswählen' })).toBeChecked();
  expect(screen.getByRole('checkbox', { name: 'Zwei auswählen' })).toBeChecked();
  checkbox.focus();
  await userEvent.keyboard(' ');
  expect(screen.getByRole('checkbox', { name: 'Eins auswählen' })).not.toBeChecked();
  expect(screen.getByRole('checkbox', { name: 'Zwei auswählen' })).not.toBeChecked();
});
