import { test, expect, addLiquid } from './fixtures';
import { randomUUID } from 'node:crypto';
test('protected setup, real login and invitation isolate two collections', async ({
  server,
  browser,
  playwright,
}) => {
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
  const me = await (await context.request.get(server.url + '/api/v1/me')).json();
  const invite = await context.request.post(server.url + '/api/v1/admin/users', {
    headers: { Origin: server.url, 'X-CSRF-Token': me.csrfToken },
    data: { username: 'second', displayName: 'Zweites Konto', role: 'user' },
  });
  expect(invite.status()).toBe(201);
  const token = (await invite.json()).invitationToken;
  const other = await browser.newContext();
  const p = await other.newPage();
  await p.goto(server.url + '/invite?token=' + token);
  await p.getByLabel('Anzeigename').fill('Zweites Konto');
  await p.getByLabel('Passwort', { exact: true }).fill(password);
  await p.getByRole('button', { name: 'Einladung annehmen', exact: true }).click();
  await expect(p.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible();
  const mine = await context.request.get(server.url + '/api/v1/fields');
  const foreignId = (await mine.json()).items[0].id;
  expect(
    (await other.request.get(server.url + '/api/v1/fields/' + foreignId + '/usage')).status(),
  ).toBe(404);
  expect(
    (await (await other.request.get(server.url + '/api/v1/fields')).json()).items,
  ).toHaveLength(0);
  await other.close();
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
