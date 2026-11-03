import { render, screen, cleanup, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, it, expect, vi } from 'vitest';
import { FormField } from './ui';
import { FieldInput } from './FieldInput';
import type { Field } from '../app/types';
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
it('binds the visible single-control label and both hint and error to the actual input', async () => {
  render(
    <FormField label="Menge" hint="In Millilitern" error="Wert prüfen">
      {(bindings: any) => (
        <div className="input-unit">
          <input {...bindings} />
          <span>ml</span>
        </div>
      )}
    </FormField>,
  );
  const input = screen.getByRole('textbox', { name: 'Menge' });
  await userEvent.click(screen.getByText('Menge'));
  expect(input).toHaveFocus();
  expect(input).toHaveAccessibleDescription('In Millilitern Wert prüfen');
  expect(input).toHaveAttribute('aria-invalid', 'true');
});
const field = (type: Field['type']): Field => ({
  id: type,
  label: 'Prüffeld',
  type,
  position: 0,
  version: 1,
  required: false,
  archived: false,
  visibility: { form: true, detail: true, list: true, grid: true },
  config: { stars: 5, scale: 1, options: [{ id: 'option', label: 'Option' }] },
});
it.each(['rating', 'tags', 'image', 'color'] as const)(
  'names the %s group and associates its error without nested or dangling labels',
  async (type) => {
    vi.stubGlobal(
      'fetch',
      async () =>
        new Response(
          '{"items":[{"id":"tag","name":"Testtag","paletteId":"pink","fieldId":"tags","version":1}]}',
        ),
    );
    render(
      <FieldInput field={field(type)} value={null} error="Eingabe prüfen" onChange={() => {}} />,
    );
    const group = screen.getByRole('group', { name: 'Prüffeld' });
    expect(group.tagName).toBe('FIELDSET');
    expect(group).toHaveAccessibleDescription('Eingabe prüfen');
    expect(group).toHaveAttribute('aria-invalid', 'true');
    expect(document.querySelector('label label')).toBeNull();
    for (const label of document.querySelectorAll<HTMLLabelElement>('label[for]')) {
      expect(label.control).not.toBeNull();
      expect(['INPUT', 'SELECT', 'TEXTAREA', 'BUTTON']).toContain(label.control!.tagName);
    }
    if (type === 'rating') {
      const radio = screen.getByRole('radio', { name: 'Prüffeld: 3 von 5' });
      expect(radio).toHaveAccessibleDescription('Eingabe prüfen');
      await userEvent.click(radio);
    }
    if (type === 'tags') {
      await screen.findByRole('checkbox', { name: 'Testtag' });
      expect(screen.getByRole('checkbox', { name: 'Testtag' })).toHaveAccessibleDescription(
        'Eingabe prüfen',
      );
    }
    if (type === 'image')
      expect(screen.getByLabelText('Prüffeld hochladen')).toHaveAccessibleDescription(
        'Eingabe prüfen',
      );
    if (type === 'color') {
      expect(screen.getByRole('textbox', { name: 'Prüffeld' })).toHaveAccessibleDescription(
        'Eingabe prüfen',
      );
      expect(screen.getByLabelText('Prüffeld auswählen')).toHaveAccessibleDescription(
        'Eingabe prüfen',
      );
    }
  },
);
it.each(['text', 'longtext', 'number', 'date', 'select', 'boolean'] as const)(
  'connects the visible %s label to one actual native control',
  async (type) => {
    render(
      <FieldInput field={field(type)} value={null} error="Eingabe prüfen" onChange={() => {}} />,
    );
    const label = screen.getByText('Prüffeld', { selector: 'label' }) as HTMLLabelElement;
    expect(label.control).not.toBeNull();
    await userEvent.click(label);
    expect(label.control).toHaveFocus();
    expect(label.control).toHaveAccessibleDescription('Eingabe prüfen');
  },
);
it('announces the server-resolved today hint through the new date input description', () => {
  render(
    <FieldInput
      field={{ ...field('date'), config: { defaultToday: true } }}
      value={null}
      isNew
      onChange={() => {}}
    />,
  );
  expect(screen.getByLabelText('Prüffeld')).toHaveAccessibleDescription('Heute beim Anlegen');
});
