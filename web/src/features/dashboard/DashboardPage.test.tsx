import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { beforeAll, afterEach, it, expect, vi } from 'vitest';
import { AppContext } from '../../app/state';
import { DashboardPage } from './DashboardPage';
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
it('cancels a whole layout draft and saves duplicate widgets together with expected version', async () => {
  const puts: any[] = [];
  vi.stubGlobal('fetch', async (url: string, init: RequestInit) => {
    let body: any;
    if (init.body) body = JSON.parse(String(init.body));
    if (url.endsWith('/fields')) return new Response('{"items":[]}');
    if (url.endsWith('/dashboard/query'))
      return new Response(
        JSON.stringify({ items: body.widgets.map((w: any) => ({ id: w.id, count: 0 })) }),
      );
    if (init.method === 'PUT') {
      puts.push(body);
      return new Response(JSON.stringify({ ...body, version: 2 }));
    }
    return new Response('{"version":1,"widgets":[]}');
  });
  render(
    <AppContext.Provider
      value={
        { fields: [], preferences: { roles: {}, shelfLifeMonths: 0 }, navigate: () => {} } as any
      }
    >
      <DashboardPage />
    </AppContext.Provider>,
  );
  await screen.findByRole('button', { name: 'Dashboard anpassen' });
  fireEvent.click(screen.getByRole('button', { name: 'Dashboard anpassen' }));
  fireEvent.click(screen.getByRole('button', { name: 'Widget hinzufügen' }));
  fireEvent.click(screen.getByRole('button', { name: 'Übernehmen' }));
  fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));
  fireEvent.click(screen.getByRole('button', { name: 'Bestätigen' }));
  expect(puts).toHaveLength(0);
  fireEvent.click(screen.getByRole('button', { name: 'Dashboard anpassen' }));
  for (let i = 0; i < 2; i++) {
    fireEvent.click(screen.getByRole('button', { name: 'Widget hinzufügen' }));
    fireEvent.click(screen.getByRole('button', { name: 'Übernehmen' }));
  }
  fireEvent.click(screen.getByRole('button', { name: 'Speichern' }));
  await screen.findByRole('button', { name: 'Dashboard anpassen' });
  expect(puts[0].version).toBe(1);
  expect(puts[0].widgets).toHaveLength(2);
  expect(puts[0].widgets[0].id).not.toBe(puts[0].widgets[1].id);
});
it('takes the entry count as a typed number from 1 to 10 and refuses anything else', async () => {
  const puts: any[] = [];
  vi.stubGlobal('fetch', async (url: string, init: RequestInit) => {
    const body = init.body ? JSON.parse(String(init.body)) : undefined;
    if (url.endsWith('/dashboard/query'))
      return new Response(
        JSON.stringify({ items: body.widgets.map((w: any) => ({ id: w.id, count: 0 })) }),
      );
    if (url.endsWith('/fields')) return new Response('{"items":[]}');
    if (init.method === 'PUT') {
      puts.push(body);
      return new Response(JSON.stringify({ ...body, version: 2 }));
    }
    return new Response('{"version":1,"widgets":[]}');
  });
  render(
    <AppContext.Provider
      value={
        { fields: [], preferences: { roles: {}, shelfLifeMonths: 0 }, navigate: () => {} } as any
      }
    >
      <DashboardPage />
    </AppContext.Provider>,
  );
  fireEvent.click(await screen.findByRole('button', { name: 'Dashboard anpassen' }));
  fireEvent.click(screen.getByRole('button', { name: 'Widget hinzufügen' }));
  fireEvent.change(screen.getByLabelText('Widgettyp'), { target: { value: 'recent' } });
  const count = screen.getByLabelText('Widgeteinträge') as HTMLInputElement;
  expect(count.tagName).toBe('INPUT');
  expect(count.value).toBe('5');
  fireEvent.change(count, { target: { value: '11' } });
  fireEvent.click(screen.getByRole('button', { name: 'Übernehmen' }));
  expect(screen.getByText('Einträge: ganze Zahl von 1 bis 10 eingeben.')).toBeTruthy();
  expect(count.getAttribute('aria-invalid')).toBe('true');
  fireEvent.change(screen.getByLabelText('Widgeteinträge'), { target: { value: '7' } });
  fireEvent.click(screen.getByRole('button', { name: 'Übernehmen' }));
  fireEvent.click(screen.getByRole('button', { name: 'Speichern' }));
  await screen.findByRole('button', { name: 'Dashboard anpassen' });
  expect(puts[0].widgets[0].limit).toBe(7);
});
