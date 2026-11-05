import { test, expect, addField, fields, goto } from './fixtures';
type App = Parameters<typeof goto>[0];
const preview = (app: App) => app.page.getByRole('region', { name: 'Kachelvorschau', exact: true });
const table = (app: App) => app.page.locator('.table-scroll[aria-label="Felder"]');
const openSettings = async (app: App, width: number, height: number) => {
  await app.page.setViewportSize({ width, height });
  await goto(app, '/settings');
  await expect(preview(app).locator('.item-card')).toBeVisible();
};
const geometry = (app: App) =>
  app.page.evaluate(() => {
    const scroll = document.querySelector('.table-scroll[aria-label="Felder"]')!;
    const section = document.querySelector('section[aria-labelledby="card-preview-title"]')!;
    const rows = section.querySelectorAll('.card-data');
    const lastRow = rows[rows.length - 1].getBoundingClientRect();
    const a = scroll.getBoundingClientRect();
    const b = section.getBoundingClientRect();
    return {
      tableRight: a.right,
      tableTop: a.top,
      tableBottom: a.bottom,
      previewLeft: b.left,
      previewTop: b.top,
      lastRowBottom: lastRow.bottom,
      tableOverflow: scroll.scrollWidth - scroll.clientWidth,
      pageOverflow: document.documentElement.scrollWidth - innerWidth,
      height: innerHeight,
    };
  });
for (const [width, height] of [
  [1440, 900],
  [1280, 800],
  [1024, 768],
  [768, 1024],
  [820, 1180],
])
  test(`table and preview sit side by side at ${width} x ${height}`, async ({ app }) => {
    await openSettings(app, width, height);
    const g = await geometry(app);
    expect(g.previewLeft).toBeGreaterThanOrEqual(g.tableRight);
    expect(Math.abs(g.previewTop - g.tableTop)).toBeLessThanOrEqual(1);
    // Every row of the card is there at first sight, without scrolling. Below the last one is
    // only the edit icon, which may reach past the edge with another browser's font metrics.
    expect(g.lastRowBottom).toBeLessThanOrEqual(g.height);
    expect(g.tableOverflow).toBeLessThanOrEqual(0);
    expect(g.pageOverflow).toBeLessThanOrEqual(0);
  });
for (const [width, height] of [
  [901, 768],
  [600, 800],
  [360, 800],
  [640, 400], // 1280 x 800 at 200 % zoom
])
  test(`table and preview stack at ${width} x ${height}`, async ({ app }) => {
    await openSettings(app, width, height);
    const g = await geometry(app);
    expect(g.previewTop).toBeGreaterThanOrEqual(g.tableBottom);
    expect(g.pageOverflow).toBeLessThanOrEqual(0);
  });
for (const [width, height] of [
  [1280, 800],
  [768, 1024],
])
  test(`the preview stays in view while a long field list scrolls at ${width}`, async ({ app }) => {
    // Hidden on the card, so only the table grows.
    for (let i = 0; i < 20; i++)
      await app.write('/fields', 'POST', {
        label: `Zusatz ${i}`,
        type: 'text',
        visibility: { form: true, detail: true, list: true, grid: false },
        config: { filterable: true },
      });
    await openSettings(app, width, height);
    const before = await geometry(app);
    expect(before.tableBottom).toBeGreaterThan(height + 500);
    await app.page.evaluate(() => scrollTo(0, 800));
    await expect.poll(() => app.page.evaluate(() => scrollY)).toBeGreaterThan(700);
    const g = await geometry(app);
    expect(g.previewTop).toBeGreaterThanOrEqual(0);
    expect(g.lastRowBottom).toBeLessThanOrEqual(g.height);
    // Not hidden behind the header bar of the tablet layout.
    const onTop = await app.page.evaluate(() => {
      const section = document.querySelector('section[aria-labelledby="card-preview-title"]')!;
      const h2 = section.querySelector('h2')!.getBoundingClientRect();
      const hit = document.elementFromPoint(h2.left + 4, h2.top + h2.height / 2);
      return !!hit && section.contains(hit);
    });
    expect(onTop).toBe(true);
  });
test('a card with many fields scrolls inside the preview', async ({ app }) => {
  for (let i = 0; i < 12; i++) await addField(app, `Kachelzeile ${i}`, 'text', {});
  await openSettings(app, 1280, 800);
  await app.page.evaluate(() => scrollTo(0, 400));
  await expect.poll(() => app.page.evaluate(() => scrollY)).toBeGreaterThan(300);
  const box = await preview(app).boundingBox();
  expect(box!.y + box!.height).toBeLessThanOrEqual(800);
  await preview(app).evaluate((node) => (node.scrollTop = node.scrollHeight));
  await expect(preview(app).locator('summary')).toBeInViewport();
});
test('a preview taller than the window scrolls by itself', async ({ app }) => {
  await openSettings(app, 1280, 500);
  const inner = await preview(app).evaluate((node) => ({
    overflow: node.scrollHeight - node.clientHeight,
    bottom: node.getBoundingClientRect().height,
  }));
  expect(inner.overflow).toBeGreaterThan(0);
  expect(inner.bottom).toBeLessThanOrEqual(500);
  // The page brings it fully into the window, then it scrolls by itself.
  await app.page.evaluate(() => scrollTo(0, 300));
  await expect.poll(() => app.page.evaluate(() => scrollY)).toBeGreaterThan(200);
  await preview(app).evaluate((node) => (node.scrollTop = node.scrollHeight));
  await expect(preview(app).locator('summary')).toBeInViewport();
});
test('a window of low height keeps the preview in the page flow', async ({ app }) => {
  await openSettings(app, 1280, 460);
  const position = await preview(app).evaluate((node) => getComputedStyle(node).position);
  expect(position).toBe('static');
});
test('a long field name wraps instead of pushing the table or page wider', async ({ app }) => {
  await addField(app, 'Sehrlangerfeldname'.repeat(5), 'text', {});
  await openSettings(app, 1024, 768);
  const g = await geometry(app);
  expect(g.tableOverflow).toBeLessThanOrEqual(0);
  expect(g.pageOverflow).toBeLessThanOrEqual(0);
  expect(g.previewLeft).toBeGreaterThanOrEqual(g.tableRight);
});
test('a box ticked in the table shows in the preview without moving the page', async ({ app }) => {
  await openSettings(app, 1280, 800);
  const rows = preview(app).locator('.card-data');
  const before = await rows.count();
  await app.page.evaluate(() => scrollTo(0, 100));
  const y = await app.page.evaluate(() => scrollY);
  await app.page.getByRole('checkbox', { name: 'Nikotin: Kachel', exact: true }).click();
  await expect(rows).toHaveCount(before - 1);
  expect(await app.page.evaluate(() => scrollY)).toBe(y);
});
test('the order is folded under the card and opens by keyboard', async ({ app }) => {
  await openSettings(app, 1280, 800);
  const list = preview(app).getByRole('list', { name: 'Reihenfolge', exact: true });
  await expect(list).toHaveCount(0);
  const summary = preview(app).locator('summary');
  await expect(summary).toHaveText('Reihenfolge');
  const box = await summary.boundingBox();
  expect(box!.height).toBeGreaterThanOrEqual(44);
  await summary.focus();
  await app.page.keyboard.press('Enter');
  await expect(list).toBeVisible();
  await expect(list.getByRole('listitem').first()).toHaveText('Bild');
  // Stays open when a change redraws the preview.
  await app.page.getByRole('checkbox', { name: 'Nikotin: Kachel', exact: true }).click();
  await expect(list.getByRole('listitem').last()).toHaveText('Tags');
  await expect(list).toBeVisible();
});

const row = (app: App, label: string) =>
  app.page.locator('.field-table tbody tr').filter({ hasText: label });
const openDialog = async (app: App, label: string) => {
  await goto(app, '/settings');
  await app.page.getByRole('button', { name: `${label} bearbeiten`, exact: true }).click();
  return app.page.getByRole('dialog', { name: 'Feld bearbeiten', exact: true });
};
test('the field table offers only move and edit', async ({ app }) => {
  await goto(app, '/settings');
  await expect(row(app, 'Nikotin').locator('.actions-column button')).toHaveCount(3);
  await expect(app.page.getByRole('button', { name: 'Nikotin löschen', exact: true })).toHaveCount(0);
});
test('a field is archived and activated again from its dialog', async ({ app }) => {
  const dialog = await openDialog(app, 'Nikotin');
  await dialog.getByRole('button', { name: 'Archivieren', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await expect(row(app, 'Nikotin')).toContainText('Archiviert');
  await expect(row(app, 'Nikotin').getByRole('checkbox').first()).toBeDisabled();

  const again = await openDialog(app, 'Nikotin');
  await expect(again.getByRole('button', { name: 'Archivieren', exact: true })).toHaveCount(0);
  await again.getByRole('button', { name: 'Aktivieren', exact: true }).click();
  await expect(again).not.toBeVisible();
  await expect(row(app, 'Nikotin')).not.toContainText('Archiviert');
});
test('a field is deleted from its dialog after the warning', async ({ app }) => {
  const dialog = await openDialog(app, 'Nikotin');
  await dialog.getByRole('button', { name: 'Löschen', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  const warning = app.page.getByRole('dialog', { name: 'Feld endgültig löschen?', exact: true });
  await expect(warning).toContainText('Nikotin: 0 Liquids mit Werten.');

  await warning.getByRole('button', { name: 'Abbrechen', exact: true }).click();
  await expect(warning).not.toBeVisible();
  await expect(row(app, 'Nikotin')).toHaveCount(1);

  await app.page.getByRole('button', { name: 'Nikotin bearbeiten', exact: true }).click();
  await dialog.getByRole('button', { name: 'Löschen', exact: true }).click();
  await warning.getByRole('button', { name: 'Bestätigen', exact: true }).click();
  await expect(warning).not.toBeVisible();
  await expect(row(app, 'Nikotin')).toHaveCount(0);
  await expect(app.page.getByRole('button', { name: 'Feld hinzufügen', exact: true })).toBeFocused();
  expect((await fields(app)).some((f: { label: string }) => f.label === 'Nikotin')).toBe(false);
});
test('archive and delete wait while the dialog has unsaved changes', async ({ app }) => {
  const dialog = await openDialog(app, 'Nikotin');
  const archive = dialog.getByRole('button', { name: 'Archivieren', exact: true });
  const remove = dialog.getByRole('button', { name: 'Löschen', exact: true });
  await expect(archive).toBeEnabled();
  await dialog.getByLabel('Feldname', { exact: true }).fill('Stärke');
  await expect(archive).toBeDisabled();
  await expect(remove).toBeDisabled();
  await expect(dialog).toContainText('Erst speichern oder verwerfen');
  await dialog.getByLabel('Feldname', { exact: true }).fill('Nikotin');
  await expect(archive).toBeEnabled();
});
test('archive and delete share a row in the dialog on a phone', async ({ app }) => {
  await app.page.setViewportSize({ width: 360, height: 800 });
  const dialog = await openDialog(app, 'Nikotin');
  const archive = await dialog.getByRole('button', { name: 'Archivieren', exact: true }).boundingBox();
  const remove = await dialog.getByRole('button', { name: 'Löschen', exact: true }).boundingBox();
  expect(archive!.y).toBe(remove!.y);
  expect(archive!.height).toBeGreaterThanOrEqual(44);
  expect(remove!.x).toBeGreaterThanOrEqual(archive!.x + archive!.width);
  const footer = await dialog.locator('.modal-footer').boundingBox();
  expect(remove!.x + remove!.width).toBeLessThanOrEqual(footer!.x + footer!.width);
});
test('a new field has neither archive nor delete', async ({ app }) => {
  await goto(app, '/settings');
  await app.page.getByRole('button', { name: 'Feld hinzufügen', exact: true }).click();
  const dialog = app.page.getByRole('dialog', { name: 'Feld hinzufügen', exact: true });
  await expect(dialog.getByRole('button', { name: 'Archivieren', exact: true })).toHaveCount(0);
  await expect(dialog.getByRole('button', { name: 'Löschen', exact: true })).toHaveCount(0);
});
