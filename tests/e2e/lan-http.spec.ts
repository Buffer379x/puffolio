import { test, expect, goto } from './fixtures';
test.use({ lanHTTP: true });
test('direct LAN HTTP creates widgets and options and offers manual invitation copying', async ({
  app,
}) => {
  await goto(app, '/dashboard');
  expect(new URL(app.url).hostname).not.toBe('127.0.0.1');
  const capabilities = await app.page.evaluate(() => ({
    secure: isSecureContext,
    uuid: typeof crypto.randomUUID,
    clipboard: typeof navigator.clipboard,
  }));
  expect(capabilities).toEqual({
    secure: false,
    uuid: 'undefined',
    clipboard: 'undefined',
  });
  await app.page.getByRole('button', { name: 'Dashboard anpassen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Widget hinzufügen', exact: true }).click();
  let dialog = app.page.getByRole('dialog', {
    name: 'Widget hinzufügen',
    exact: true,
  });
  await dialog.getByLabel('Widgettitel', { exact: true }).fill('LAN-Anzahl');
  await dialog.getByRole('button', { name: 'Übernehmen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(
    app.page.getByRole('button', { name: 'Dashboard anpassen', exact: true }),
  ).toBeVisible();
  const layout = await (await app.api.get('/api/v1/dashboard')).json();
  expect(layout.widgets.find((w: any) => w.title === 'LAN-Anzahl')?.id).toMatch(
    /^[\da-f]{8}-[\da-f]{4}-4[\da-f]{3}-[89ab][\da-f]{3}-[\da-f]{12}$/,
  );
  await goto(app, '/settings');
  await app.page.getByRole('button', { name: 'Feld hinzufügen', exact: true }).click();
  dialog = app.page.getByRole('dialog', {
    name: 'Feld hinzufügen',
    exact: true,
  });
  await dialog.getByLabel('Feldname', { exact: true }).fill('LAN-Auswahl');
  await dialog.getByLabel('Feldtyp', { exact: true }).selectOption('select');
  await dialog.getByRole('button', { name: 'Option hinzufügen', exact: true }).click();
  await dialog.getByLabel('Option 1', { exact: true }).fill('Frucht');
  await dialog.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  const fields = await (await app.api.get('/api/v1/fields')).json();
  expect(fields.items.find((f: any) => f.label === 'LAN-Auswahl')?.config.options[0].label).toBe(
    'Frucht',
  );
  await goto(app, '/admin');
  await app.page.getByRole('button', { name: 'Nutzer einladen', exact: true }).click();
  dialog = app.page.getByRole('dialog', {
    name: 'Nutzer einladen',
    exact: true,
  });
  await dialog.getByLabel('Benutzername', { exact: true }).fill('lan-guest');
  await dialog.getByLabel('Anzeigename', { exact: true }).fill('LAN-Gast');
  await dialog.getByRole('button', { name: 'Einladung erstellen', exact: true }).click();
  dialog = app.page.getByRole('dialog', {
    name: 'Einladungslink',
    exact: true,
  });
  const link = dialog.getByRole('textbox', {
    name: 'Einladungslink',
    exact: true,
  });
  await dialog.getByRole('button', { name: 'Link kopieren', exact: true }).click();
  await expect(dialog.getByRole('status')).toHaveText('Link markiert. Bitte manuell kopieren.');
  await expect(link).toBeFocused();
  expect(
    await link.evaluate((el) => {
      const input = el as HTMLInputElement;
      return input.selectionStart === 0 && input.selectionEnd === input.value.length;
    }),
  ).toBe(true);
  await expect(dialog.getByRole('button', { name: 'Kopiert', exact: true })).toHaveCount(0);
});
