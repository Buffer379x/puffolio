import { test, expect, addLiquid, goto } from './fixtures';
import { readFile } from 'node:fs/promises';
test('real ZIP export/import preview and commit; trash restores and permanently deletes', async ({
  app,
}) => {
  let liquid = await addLiquid(app, 'Transfer');
  await goto(app, '/settings?section=data');
  const pending = app.page.waitForEvent('download');
  await app.page.getByRole('button', { name: 'Gesamte Sammlung exportieren', exact: true }).click();
  const download = await pending;
  const bytes = await readFile((await download.path())!);
  expect(bytes.subarray(0, 2).toString()).toBe('PK');
  await app.page
    .getByLabel('Importdatei')
    .setInputFiles({ name: 'collection.zip', mimeType: 'application/zip', buffer: bytes });
  const d = app.page.getByRole('dialog', { name: 'Import prüfen', exact: true });
  await expect(d).toBeVisible();
  await d.getByRole('button', { name: 'Importieren', exact: true }).click();
  await expect(d).not.toBeVisible();
  expect((await (await app.api.get('/api/v1/liquids')).json()).total).toBe(2);
  await app.write('/liquids/' + liquid.id, 'DELETE', { version: liquid.version });
  await app.page.reload();
  await app.page.getByRole('button', { name: 'Wiederherstellen', exact: true }).click();
  await expect(app.page.getByText('Der Papierkorb ist leer.', { exact: true })).toBeVisible();
  liquid = await (await app.api.get('/api/v1/liquids/' + liquid.id)).json();
  await app.write('/liquids/' + liquid.id, 'DELETE', { version: liquid.version });
  await app.page.reload();
  await app.page.getByRole('button', { name: 'Endgültig löschen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Bestätigen', exact: true }).click();
  await expect(app.page.getByText('Der Papierkorb ist leer.', { exact: true })).toBeVisible();
  expect((await app.api.get('/api/v1/liquids/' + liquid.id)).status()).toBe(404);
});
test('admin invitation link is real and backup requires the administrator password', async ({
  app,
}) => {
  await goto(app, '/admin');
  await app.page.getByRole('button', { name: 'Nutzer einladen', exact: true }).click();
  const d = app.page.getByRole('dialog');
  await d.getByLabel('Benutzername', { exact: true }).fill('guest');
  await d.getByLabel('Anzeigename', { exact: true }).fill('Gast');
  await d.getByRole('button', { name: 'Einladung erstellen', exact: true }).click();
  const link = await d.getByLabel('Einladungslink').inputValue();
  expect(new URL(link).pathname).toBe('/invite');
  expect(new URL(link).searchParams.get('token')).toBeTruthy();
  await d.getByRole('button', { name: 'Schließen', exact: true }).last().click();
  await expect(app.page.getByRole('cell', { name: 'Gast', exact: true })).toBeVisible();
  await app.page.getByRole('button', { name: 'Vollständige Sicherung', exact: true }).click();
  await d.getByLabel('Administratorpasswort').fill(app.password);
  const pending = app.page.waitForEvent('download');
  await d.getByRole('button', { name: 'Sicherung herunterladen', exact: true }).click();
  const bytes = await readFile((await (await pending).path())!);
  expect(bytes.subarray(0, 2).toString()).toBe('PK');
});
