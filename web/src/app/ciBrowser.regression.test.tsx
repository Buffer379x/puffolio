import { cleanup, render, screen, fireEvent } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { AppContext } from './state';
import { AppShell } from './AppShell';
import { ToolsPage } from '../features/tools/ToolsPage';
import { FieldInput } from '../components/FieldInput';
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});
it('returns drawer focus only after the content ancestor stops being inert', () => {
  vi.stubGlobal('matchMedia', () => ({
    matches: true,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
  render(
    <AppContext.Provider
      value={
        {
          me: { user: { displayName: 'Test', role: 'admin' } },
          preferences: { theme: 'system' },
          savePreferences: async () => {},
          navigate: () => {},
          logout: async () => {},
        } as any
      }
    >
      <AppShell path="/liquids">
        <p>Collection</p>
      </AppShell>
    </AppContext.Provider>,
  );
  const toggle = screen.getByRole('button', { name: 'Menü öffnen' });
  toggle.focus();
  const focus = toggle.focus.bind(toggle),
    inertAtFocus: boolean[] = [];
  vi.spyOn(toggle, 'focus').mockImplementation(() => {
    inertAtFocus.push(!!(toggle.closest('.app-shell') as HTMLElement).inert);
    focus();
  });
  fireEvent.click(toggle);
  fireEvent.keyDown(screen.getByRole('dialog', { name: 'Hauptnavigation' }), { key: 'Escape' });
  expect(inertAtFocus.length).toBeGreaterThan(0);
  expect(inertAtFocus.every((value) => value === false)).toBe(true);
  expect(toggle).toHaveFocus();
});
it('tools card exposes an exact calculator link name', () => {
  render(
    <AppContext.Provider value={{ navigate: () => {} } as any}>
      <ToolsPage />
    </AppContext.Provider>,
  );
  expect(screen.getByRole('link', { name: 'Nikotinrechner' })).toBeVisible();
});
it('required backfill has an exact accessible combobox name', () => {
  render(
    <FieldInput
      field={{
        id: 'flag',
        label: 'Auffüllwert',
        type: 'boolean',
        required: true,
        version: 1,
        position: 0,
        archived: false,
        visibility: { form: true, list: true, grid: true, detail: true },
        config: {},
      }}
      value={false}
      onChange={() => {}}
    />,
  );
  expect(screen.getByRole('combobox', { name: 'Auffüllwert' })).toBeVisible();
});
