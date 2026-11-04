import { test, expect } from './fixtures';
import { randomUUID } from 'node:crypto';
test('setup is protected by the one-time code', async ({ server, browser }) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const password = 'Test-' + randomUUID();
  await page.goto(server.url);
  await page.getByLabel('Einrichtungscode').fill('wrong');
  await page.getByLabel('Benutzername').fill('admin');
  await page.getByLabel('Anzeigename').fill('Testkonto');
  await page.getByLabel('Passwort', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Einrichten', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('ungültig');
  await page.getByLabel('Einrichtungscode').fill(server.code);
  await page.getByRole('button', { name: 'Einrichten', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible();
  await context.close();
});
test('expired cookie login preserves a liquid draft', async ({ app }) => {
  await app.page.goto(app.url + '/liquids');
  await app.page.getByRole('button', { name: 'Liquid hinzufügen', exact: true }).first().click();
  await app.page.getByLabel('Name', { exact: true }).fill('Erhaltener Entwurf');
  await app.context.clearCookies();
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  const session = app.page.getByRole('dialog', { name: 'Sitzung abgelaufen', exact: true });
  await expect(session).toBeVisible();
  await session.getByLabel('Benutzername').fill(app.username);
  await session.getByLabel('Passwort', { exact: true }).fill(app.password);
  await session.getByRole('button', { name: 'Anmelden', exact: true }).click();
  await expect(session).not.toBeVisible();
  await expect(app.page.getByLabel('Name', { exact: true })).toHaveValue('Erhaltener Entwurf');
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(
    app.page.getByRole('button', { name: 'Erhaltener Entwurf', exact: true }),
  ).toBeVisible();
});
