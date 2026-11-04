import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { it, expect, vi, afterEach, beforeAll } from 'vitest';
import { AppContext } from '../../app/state';
import { LiquidEditor } from './LiquidEditor';
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
it('preserves the edited draft on409 and submits only changed shown values', async () => {
  let sent: any;
  vi.stubGlobal('fetch', async (_url: string, init: RequestInit) => {
    if (init.method === 'GET') return new Response(JSON.stringify(original));
    sent = JSON.parse(init.body as string);
    return new Response(JSON.stringify({ error: { code: 'conflict', message: 'Geändert' } }), {
      status: 409,
    });
  });
  const field = {
    id: 'visible',
    label: 'Menge',
    type: 'number',
    config: { scale: 1 },
    visibility: { form: true, detail: true, list: true, grid: true },
    version: 1,
    position: 0,
    required: false,
    archived: false,
  };
  const hidden = {
    ...field,
    id: 'hidden',
    label: 'Privat',
    visibility: { ...field.visibility, form: false },
  };
  const original = {
    id: 'liquid',
    name: 'Alt',
    favorite: false,
    version: 3,
    createdAt: '2026-01-01',
    updatedAt: '2026-01-01',
    values: {
      visible: { type: 'number', value: '0.0' },
      hidden: { type: 'number', value: '123.0' },
    },
  };
  render(
    <AppContext.Provider
      value={{ fields: [field, hidden], preferences: { roles: {}, shelfLifeMonths: 0 } } as any}
    >
      <LiquidEditor
        liquid={original}
        onClose={() => {}}
        onSaved={() => {
          throw new Error('must not save');
        }}
      />
    </AppContext.Provider>,
  );
  fireEvent.change(await screen.findByLabelText('Name'), { target: { value: 'Entwurf' } });
  fireEvent.change(screen.getByLabelText('Menge'), { target: { value: '2,5' } });
  fireEvent.click(screen.getByRole('button', { name: 'Speichern' }));
  await screen.findByText('Geändert');
  expect(screen.getByLabelText('Name')).toHaveValue('Entwurf');
  expect(sent).toMatchObject({
    version: 3,
    name: 'Entwurf',
    values: { visible: { type: 'number', value: '2.5' } },
  });
  expect(sent.values.hidden).toBeUndefined();
});
