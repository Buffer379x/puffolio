import { render, screen, fireEvent, waitFor, cleanup, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeAll, afterEach, it, expect, vi } from 'vitest';
import { App } from './App';
const user = { id: 'user', username: 'tester', displayName: 'Original', role: 'admin', version: 1 };
const me = { user, collectionId: 'collection', csrfToken: 'csrf' };
const preferences = {
  version: 1,
  theme: 'light',
  timezone: 'Europe/Berlin',
  view: 'grid',
  images: true,
  listFields: [],
  gridFields: [],
  roles: {},
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
  history.replaceState(null, '', '/');
});
function setup(route: string, expireProfile = false) {
  history.replaceState(null, '', route);
  vi.stubGlobal('matchMedia', () => ({
    matches: false,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
  let failed = false;
  vi.stubGlobal('fetch', async (url: string, init: RequestInit) => {
    let result: unknown = {};
    if (url.endsWith('/setup/status')) result = { setupRequired: false };
    else if (url.endsWith('/auth/login')) result = { ...me, csrfToken: 'renewed' };
    else if (url.endsWith('/me')) {
      if (init.method === 'PATCH' && expireProfile && !failed) {
        failed = true;
        return new Response(
          JSON.stringify({ error: { code: 'unauthorized', message: 'Anmelden' } }),
          { status: 401 },
        );
      }
      result = me;
    } else if (url.endsWith('/preferences')) result = preferences;
    else if (url.endsWith('/fields')) result = { items: [], version: 1 };
    else if (url.includes('/liquids?')) result = { items: [], total: 0, page: 1, pageSize: 24 };
    else if (url.endsWith('/saved-views')) result = { items: [] };
    return new Response(JSON.stringify(result));
  });
}
it('keeps search focus across debounce pauses and appends the next term to the query', async () => {
  setup('/liquids');
  render(<App />);
  const search = await screen.findByRole('searchbox', { name: 'Liquids suchen' });
  const actor = userEvent.setup();
  await actor.type(search, 'Zitrone');
  await waitFor(() => expect(new URLSearchParams(location.search).get('search')).toBe('Zitrone'));
  await new Promise((r) => setTimeout(r, 20));
  expect(search).toHaveFocus();
  await actor.type(search, ' frisch');
  await waitFor(() =>
    expect(new URLSearchParams(location.search).get('search')).toBe('Zitrone frisch'),
  );
  expect(search).toHaveFocus();
});
it('reauthenticates an expired profile PATCH while retaining the edited display name', async () => {
  setup('/settings?section=profile', true);
  render(<App />);
  const display = await screen.findByRole('textbox', { name: 'Anzeigename' });
  fireEvent.change(display, { target: { value: 'Erhaltener Profilentwurf' } });
  fireEvent.click(screen.getByRole('button', { name: 'Speichern' }));
  const session = await screen.findByRole('dialog', { name: 'Sitzung abgelaufen' });
  fireEvent.change(within(session).getByLabelText('Benutzername'), { target: { value: 'tester' } });
  fireEvent.change(within(session).getByLabelText('Passwort'), {
    target: { value: 'SyntheticPassword' },
  });
  fireEvent.click(within(session).getByRole('button', { name: 'Anmelden' }));
  await waitFor(() =>
    expect(screen.queryByRole('dialog', { name: 'Sitzung abgelaufen' })).not.toBeInTheDocument(),
  );
  expect(display).toHaveValue('Erhaltener Profilentwurf');
});
