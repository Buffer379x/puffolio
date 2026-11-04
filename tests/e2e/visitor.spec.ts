import { test, expect, addLiquid, fields, goto } from './fixtures';
const today = new Date();
const yearsAgo = (years: number) =>
  `${today.getFullYear() - years}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;
test('visitors read, search, sort and switch views but cannot change anything', async ({
  app,
  browser,
  server,
}) => {
  const liquid = await addLiquid(app, 'Öffentlich', {}, true);
  await addLiquid(app, 'Zweites');
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto(server.url + '/liquids');
  await expect(page.getByRole('button', { name: 'Öffentlich', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: /hinzufügen/ })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /bearbeiten$/i })).toHaveCount(0);
  await expect(
    page.getByRole('button', { name: /Favorit (entfernen|markieren)|Als Favorit/ }),
  ).toHaveCount(0);
  await expect(page.getByRole('link', { name: 'Einstellungen' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Liste', exact: true }).click();
  await expect(page.getByRole('table')).toBeVisible();
  await expect(page.getByRole('checkbox')).toHaveCount(0);
  await page.getByRole('button', { name: 'Name', exact: true }).click();
  await expect(page.locator('tbody tr').first()).toContainText('Öffentlich');
  await page.getByRole('button', { name: 'Name', exact: true }).click();
  await expect(page.locator('tbody tr').first()).toContainText('Zweites');
  await page.getByLabel('Liquids suchen').fill('Öffentl');
  await expect(page.locator('tbody tr')).toHaveCount(1);
  await page.getByRole('button', { name: 'Öffentlich', exact: true }).click();
  const detail = page.getByRole('dialog', { name: 'Öffentlich' });
  await expect(detail).toBeVisible();
  await expect(detail.getByRole('button', { name: 'Löschen' })).toHaveCount(0);
  await expect(detail.getByRole('button', { name: 'Bearbeiten' })).toHaveCount(0);
  await detail.getByRole('button', { name: 'Schließen' }).click();
  // A visitor's display choice is theirs alone and does not touch the owner's.
  const ownerView = (await (await app.api.get('/api/v1/preferences')).json()).view;
  await page.reload();
  await expect(page.getByRole('table')).toBeVisible();
  expect((await (await app.api.get('/api/v1/preferences')).json()).view).toBe(ownerView);
  await page.goto(server.url + '/dashboard');
  await expect(page.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Dashboard anpassen' })).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'Zuletzt hinzugefügt' })).toBeVisible();
  const denied = await context.request.patch(server.url + '/api/v1/liquids/' + liquid.id, {
    headers: { Origin: server.url },
    data: { version: liquid.version, name: 'Angriff' },
  });
  expect(denied.status()).toBe(401);
  await context.close();
});
test('visitors sign in on request and settings stay behind the login', async ({
  app,
  browser,
  server,
}) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto(server.url + '/settings');
  await expect(page.getByRole('heading', { name: 'Anmelden', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Ohne Anmeldung ansehen' }).click();
  await expect(page.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible();
  await page.goto(server.url + '/login');
  await page.getByLabel('Benutzername').fill(app.username);
  await page.getByLabel('Passwort', { exact: true }).fill(app.password);
  await page.getByRole('button', { name: 'Anmelden', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Dashboard anpassen' })).toBeVisible();
  await page.goto(server.url + '/settings?section=data');
  await expect(page.getByRole('heading', { name: 'Instanzsicherung' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Papierkorb' })).toBeVisible();
  await context.close();
});
test('shelf life drains from the purchase date and follows settings, overrides and status', async ({
  app,
}) => {
  const defs = await fields(app);
  const purchase = defs.find((f) => f.label === 'Kaufdatum');
  const override = defs.find((f) => f.label === 'Eigene Haltedauer');
  const status = defs.find((f) => f.label === 'Status');
  const used = status.config.options.find((o: any) => o.label === 'Aufgebraucht').id;
  const bought = (years: number) => ({ [purchase.id]: { type: 'date', value: yearsAgo(years) } });
  await addLiquid(app, 'Ein Jahr alt', bought(1));
  await addLiquid(app, 'Überlagert', bought(6));
  await addLiquid(app, 'Ohne Kaufdatum');
  await addLiquid(app, 'Kurz haltbar', {
    ...bought(1),
    [override.id]: { type: 'number', value: '24' },
  });
  await addLiquid(app, 'Leer', { ...bought(9), [status.id]: { type: 'select', value: used } });
  await goto(app, '/liquids?view=grid');
  const card = (name: string) => app.page.locator('.item-card').filter({ hasText: name });
  const meter = (name: string) => card(name).getByRole('meter', { name: 'Haltbarkeit' });
  await expect(meter('Ein Jahr alt')).toHaveAttribute('aria-valuenow', '80');
  await expect(meter('Kurz haltbar')).toHaveAttribute('aria-valuenow', '50');
  await expect(meter('Überlagert')).toHaveAttribute('aria-valuenow', '0');
  await expect(card('Überlagert').getByText('Abgelaufen')).toBeVisible();
  await expect(meter('Ohne Kaufdatum')).toHaveCount(0);
  await expect(meter('Leer')).toHaveCount(0);
  const width = await meter('Ein Jahr alt')
    .locator('.shelf-fill')
    .evaluate((el) => el.style.width);
  expect(width).toBe('80%');
  // The form explains what an empty own value means, and a value overrides the default.
  await app.page.getByRole('button', { name: 'Ein Jahr alt bearbeiten', exact: true }).click();
  const form = app.page.getByRole('dialog', { name: 'Liquid bearbeiten' });
  await expect(form.getByLabel('Eigene Haltedauer', { exact: true })).toHaveAccessibleDescription(
    'Leer: Standard (60 Monate)',
  );
  await form.getByLabel('Eigene Haltedauer', { exact: true }).fill('24');
  await form.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(meter('Ein Jahr alt')).toHaveAttribute('aria-valuenow', '50');
  await goto(app, '/settings');
  await app.page.getByLabel('Standardhaltedauer', { exact: true }).fill('2');
  await app.page.getByRole('button', { name: 'Haltedauer speichern', exact: true }).click();
  await expect(app.page.getByRole('status').first()).toContainText('Gespeichert');
  await goto(app, '/liquids?view=list&sort=shelf&direction=asc');
  const rows = app.page.locator('tbody tr');
  await expect(rows.first()).toContainText('Überlagert');
  await expect(rows.last().getByRole('meter')).toHaveCount(0);
  await goto(app, '/dashboard');
  const expired = app.page.locator('.stat-card').filter({ hasText: 'Abgelaufen' });
  await expect(expired.locator('.stat-value')).toHaveText('1');
  const soon = app.page.locator('.panel').filter({ hasText: 'Läuft bald ab' });
  await expect(soon.getByText('Leer', { exact: true })).toHaveCount(0);
  // Without a status assignment the used-up liquid counts again.
  await goto(app, '/settings');
  await app.page.getByLabel('Status für aufgebraucht').selectOption('');
  await goto(app, '/dashboard');
  await expect(expired.locator('.stat-value')).toHaveText('2');
});
test('new fields appear immediately and the whole card opens the detail', async ({ app }) => {
  await addLiquid(app, 'Kartenklick');
  await goto(app, '/settings');
  await app.page.getByRole('button', { name: 'Feld hinzufügen', exact: true }).click();
  const d = app.page.getByRole('dialog');
  await d.getByLabel('Feldname').fill('Bezugsquelle');
  await d.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(d).not.toBeVisible();
  await goto(app, '/liquids?view=list');
  await expect(app.page.getByRole('button', { name: 'Bezugsquelle', exact: true })).toBeVisible();
  await goto(app, '/liquids?view=grid');
  const box = (await app.page.locator('.item-stage').first().boundingBox())!;
  expect(Math.abs(box.width - box.height)).toBeLessThan(1);
  await app.page.mouse.click(box.x + box.width / 4, box.y + box.height / 2);
  await expect(app.page.getByRole('dialog', { name: 'Kartenklick' })).toBeVisible();
});
test('moving a liquid to the trash can be undone from the confirmation', async ({ app }) => {
  await addLiquid(app, 'Testflasche');
  await goto(app, '/liquids?view=grid');
  await app.page.getByRole('button', { name: 'Testflasche', exact: true }).click();
  await app.page.getByRole('dialog').getByRole('button', { name: 'Löschen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Bestätigen', exact: true }).click();
  await expect(app.page.getByText('Noch keine Liquids.')).toBeVisible();
  await app.page.locator('.toast').getByRole('button', { name: 'Rückgängig' }).click();
  await expect(app.page.locator('.item-card')).toHaveCount(1);
});
test('closing a detail leaves no focus ring around the list', async ({ app }) => {
  await addLiquid(app, 'Fokus');
  await goto(app, '/liquids?view=list');
  await app.page.getByRole('button', { name: 'Fokus', exact: true }).click();
  const detail = app.page.getByRole('dialog', { name: 'Fokus' });
  await detail.getByRole('button', { name: 'Schließen', exact: true }).click();
  await expect(detail).not.toBeVisible();
  expect(
    await app.page.evaluate(() => document.activeElement?.classList.contains('table-scroll')),
  ).toBe(false);
});
test('every dashboard row fills the width, whatever the widget sizes', async ({ app }) => {
  const size = app.page.viewportSize()!;
  test.skip(size.width < 1200, 'Rows only mix sizes on the desktop layout');
  const layout = await (await app.api.get('/api/v1/dashboard')).json();
  const widget = (id: string, type: string, sizing: string) => ({
    id,
    type,
    title: id,
    size: sizing,
  });
  await app.write('/dashboard', 'PUT', {
    version: layout.version,
    widgets: [
      widget('breit', 'recent', 'wide'),
      widget('klein', 'count', 'small'),
      widget('k1', 'count', 'small'),
      widget('k2', 'count', 'small'),
      widget('k3', 'count', 'small'),
      widget('k4', 'count', 'small'),
    ],
  });
  await goto(app, '/dashboard');
  await expect(app.page.getByRole('heading', { name: 'k4', exact: true })).toBeVisible();
  const boxes = await app.page
    .locator('.widget-slot')
    .evaluateAll((slots) => slots.map((s) => s.getBoundingClientRect().toJSON()));
  const grid = await app.page.locator('.dashboard-grid').boundingBox();
  const rows = new Map<number, any[]>();
  for (const box of boxes) rows.set(box.top, [...(rows.get(box.top) || []), box]);
  expect([...rows.values()].map((r) => r.length)).toEqual([2, 4]);
  for (const row of rows.values())
    expect(Math.abs(row.at(-1).right - (grid!.x + grid!.width))).toBeLessThan(1);
});
test('statistic widgets show donut, columns and shelf status', async ({ app }) => {
  const defs = await fields(app);
  const rating = defs.find((f) => f.type === 'rating');
  const purchase = defs.find((f) => f.label === 'Kaufdatum');
  const tags = defs.find((f) => f.type === 'tags');
  const tag = await app.write('/fields/' + tags.id + '/tags', 'POST', {
    name: 'Frucht',
    paletteId: 'orange',
  });
  await addLiquid(app, 'Statistik', {
    [rating.id]: { type: 'rating', value: 4 },
    [purchase.id]: { type: 'date', value: yearsAgo(0) },
    [tags.id]: { type: 'tags', value: [tag.id] },
  });
  const layout = await (await app.api.get('/api/v1/dashboard')).json();
  await app.write('/dashboard', 'PUT', {
    version: layout.version,
    widgets: [
      ...layout.widgets,
      { id: 'noten', type: 'ratings', title: 'Noten', size: 'medium', fieldId: rating.id },
      { id: 'beste', type: 'top', title: 'Beste', size: 'medium', fieldId: rating.id, limit: 3 },
    ],
  });
  await goto(app, '/dashboard');
  const panel = (name: string) =>
    app.page
      .locator('.panel')
      .filter({ has: app.page.getByRole('heading', { name, exact: true }) });
  await expect(panel('Tags').locator('.donut circle')).toHaveCount(1);
  await expect(panel('Tags').locator('.chart-legend')).toContainText('Frucht');
  await expect(panel('Käufe pro Monat').locator('.column')).toHaveCount(12);
  await expect(panel('Noten').locator('.column')).toHaveCount(5);
  await expect(panel('Haltbarkeit').locator('.status-bar div')).toHaveCount(1);
  await expect(panel('Beste')).toContainText('Statistik');
  // Hidden chart tables must not stretch the page.
  expect(
    await app.page.evaluate(
      () =>
        document.documentElement.scrollHeight -
        Math.max(
          innerHeight,
          document.querySelector('main')!.getBoundingClientRect().bottom + scrollY,
        ),
    ),
  ).toBeLessThanOrEqual(1);
  await panel('Tags')
    .getByRole('button', { name: /Frucht/ })
    .click();
  await expect(app.page.getByRole('heading', { name: 'Liquids', exact: true })).toBeVisible();
  await expect(app.page.getByRole('button', { name: 'Frucht' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
});
