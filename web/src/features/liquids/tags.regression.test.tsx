import { useState } from 'react';
import { render, screen, fireEvent, waitFor, cleanup, within } from '@testing-library/react';
import { beforeAll, afterEach, it, expect, vi } from 'vitest';
import { AppContext } from '../../app/state';
import { TagsInput } from '../../components/FieldInput';
import { FieldValue } from '../../components/FieldValue';
import { FilterBuilder } from './FilterBuilder';
import { useTags } from './data';
import type { Field, Tag } from '../../app/types';
const field: Field = {
  id: 'tags',
  label: 'Tags',
  type: 'tags',
  position: 0,
  version: 1,
  required: false,
  archived: false,
  visibility: { form: true, detail: true, list: true, grid: true },
  config: { filterable: true },
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
it('invalidates every mounted tag catalogue after creating a pink tag in the editor', async () => {
  const catalog: Tag[] = [];
  vi.stubGlobal('fetch', async (_url: string, init: RequestInit) => {
    if (init.method === 'POST') {
      const body = JSON.parse(String(init.body));
      const tag = { id: 'new-tag', fieldId: field.id, version: 1, ...body };
      catalog.push(tag);
      return new Response(JSON.stringify(tag), { status: 201 });
    }
    return new Response(JSON.stringify({ items: catalog }));
  });
  function Consumer({ ids, label }: { ids: string[]; label: string }) {
    const tags = useTags();
    return (
      <section aria-label={label}>
        <FieldValue field={field} value={ids} tags={tags.data} />
        <FilterBuilder
          fields={[field]}
          tags={tags.data}
          value={{ fieldId: field.id, operator: 'any', value: ids }}
          onChange={() => {}}
        />
      </section>
    );
  }
  function Editor() {
    const [ids, setIds] = useState<string[]>([]);
    return (
      <>
        <TagsInput field={field} value={ids} onChange={(v) => setIds(v as string[])} />
        <Consumer ids={ids} label="Kachel" />
        <Consumer ids={ids} label="Detail" />
      </>
    );
  }
  render(
    <AppContext.Provider value={{ fields: [field] } as any}>
      <Editor />
    </AppContext.Provider>,
  );
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Tag hinzufügen' })).toBeInTheDocument(),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Tag hinzufügen' }));
  fireEvent.change(screen.getByLabelText('Tagname'), { target: { value: 'Neuer Rosatag' } });
  fireEvent.click(screen.getByRole('button', { name: 'Rosa' }));
  fireEvent.click(screen.getByRole('button', { name: 'Speichern' }));
  for (const label of ['Kachel', 'Detail']) {
    const consumer = within(screen.getByRole('region', { name: label }));
    await waitFor(() =>
      expect(consumer.queryByText('Tag nicht verfügbar')).not.toBeInTheDocument(),
    );
    await waitFor(() => expect(consumer.getAllByText('Neuer Rosatag')).toHaveLength(2));
    expect(consumer.getAllByText('Neuer Rosatag')[0]).toHaveStyle({ backgroundColor: '#D9B4D7' });
  }
});
