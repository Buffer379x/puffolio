import { test, expect, fields, addField, addLiquid, goto, syntheticPNG } from './fixtures';
test('dynamic dates, false and zero work; required scalar backfill is explicit', async ({
  app,
}) => {
  await addLiquid(app, 'Vorhanden');
  await goto(app, '/settings');
  await app.page.getByRole('button', { name: 'Feld hinzufügen', exact: true }).click();
  const d = app.page.getByRole('dialog');
  await d.getByLabel('Feldname').fill('Prüfdatum');
  await d.getByLabel('Feldtyp').selectOption('date');
  await d.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(d).not.toBeVisible();
  await app.page.getByRole('button', { name: 'Feld hinzufügen', exact: true }).click();
  await d.getByLabel('Feldname').fill('Geprüft');
  await d.getByLabel('Feldtyp').selectOption('boolean');
  await d.getByLabel('Pflichtfeld', { exact: true }).check();
  await d.getByLabel('Pflichtfeld für vorhandene Liquids').selectOption('backfill');
  await d.getByRole('combobox', { name: 'Auffüllwert', exact: true }).selectOption('false');
  await d.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(d).not.toBeVisible();
  const defs = await fields(app);
  const flag = defs.find((f) => f.label === 'Geprüft');
  const initial = (await (await app.api.get('/api/v1/liquids')).json()).items[0];
  expect(initial.values[flag.id].value).toBe(false);
  await goto(app, '/liquids');
  await app.page.getByRole('button', { name: 'Liquid hinzufügen', exact: true }).first().click();
  await app.page.getByLabel('Name', { exact: true }).fill('Datum und Null');
  await app.page.getByLabel('Prüfdatum', { exact: true }).fill('2028-02-29');
  await app.page.getByRole('combobox', { name: 'Geprüft', exact: true }).selectOption('false');
  await app.page.getByLabel('Nikotin', { exact: true }).fill('0');
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  const saved = (await (await app.api.get('/api/v1/liquids?search=Datum')).json()).items[0];
  expect(saved.values[flag.id].value).toBe(false);
  expect(saved.values[defs.find((f) => f.label === 'Prüfdatum').id].value).toBe('2028-02-29');
  expect(saved.values[defs.find((f) => f.label === 'Nikotin').id].value).toBe('0.0');
});
test('new upload crops immediately; canceling recrop retains old immutable image reference', async ({
  app,
}) => {
  await goto(app, '/liquids');
  await app.page.getByRole('button', { name: 'Liquid hinzufügen', exact: true }).first().click();
  await app.page.getByLabel('Name', { exact: true }).fill('Bildtest');
  await app.page
    .getByLabel('Bild hochladen', { exact: true })
    .setInputFiles({ name: 'synthetic.png', mimeType: 'image/png', buffer: syntheticPNG() });
  const crop = app.page.getByRole('dialog', { name: 'Bildausschnitt', exact: true });
  await expect(crop).toBeVisible();
  await crop.getByLabel('Zoom', { exact: true }).fill('2');
  await crop.getByLabel('Horizontal', { exact: true }).fill('0');
  await crop.getByRole('button', { name: 'Ausschnitt übernehmen', exact: true }).click();
  await expect(crop).not.toBeVisible();
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  const defs = await fields(app);
  const image = defs.find((f) => f.type === 'image');
  const liquid = (await (await app.api.get('/api/v1/liquids')).json()).items[0];
  const original = liquid.values[image.id].value;
  await app.page.getByRole('button', { name: /bearbeiten$/ }).click();
  await app.page.getByLabel('Name', { exact: true }).fill('Bildtest Entwurf');
  const cropOpener = app.page.getByRole('button', { name: 'Ausschnitt ändern', exact: true });
  await cropOpener.focus();
  await cropOpener.click();
  await crop.getByLabel('Zoom', { exact: true }).fill('3');
  await app.page.keyboard.press('Escape');
  await expect(crop).not.toBeVisible();
  await expect(
    app.page.getByRole('dialog', { name: 'Liquid bearbeiten', exact: true }),
  ).toBeVisible();
  await expect(app.page.getByLabel('Name', { exact: true })).toHaveValue('Bildtest Entwurf');
  await expect(cropOpener).toBeFocused();
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  expect(
    (await (await app.api.get('/api/v1/liquids/' + liquid.id)).json()).values[image.id].value,
  ).toBe(original);
  await app.page.getByRole('button', { name: /bearbeiten$/ }).click();
  await app.page.getByRole('button', { name: 'Ausschnitt ändern', exact: true }).click();
  await crop.getByLabel('Zoom', { exact: true }).fill('3');
  await crop.getByRole('button', { name: 'Ausschnitt übernehmen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  expect(
    (await (await app.api.get('/api/v1/liquids/' + liquid.id)).json()).values[image.id].value,
  ).not.toBe(original);
});
const preview = (app: { page: import('@playwright/test').Page }) =>
  app.page.getByRole('region', { name: 'Kachelvorschau', exact: true });
// The list is folded away; fields-layout.spec.ts opens it. Here only its content matters.
const previewOrder = (app: { page: import('@playwright/test').Page }) =>
  preview(app).locator('ol[aria-labelledby="card-preview-order"] li');
test('preview shows the card in field order', async ({ app, browser, server }) => {
  const defs = await fields(app);
  const tags = defs.find((f) => f.type === 'tags');
  await addLiquid(app, 'Echt', {
    [tags.id]: {
      type: 'tags',
      value: [
        (await app.write(`/fields/${tags.id}/tags`, 'POST', { name: 'Minze', paletteId: 'teal' }))
          .id,
      ],
    },
    [defs.find((f) => f.label === 'Nikotin').id]: { type: 'number', value: '3.0' },
  });
  await goto(app, '/settings');
  await expect(preview(app).locator('.item-card .name-button')).toHaveText('Beispiel-Liquid');
  await expect(preview(app).locator('.item-stage .bottle')).toBeVisible();
  await expect(previewOrder(app)).toHaveText([
    'Bild',
    'Name',
    'Hersteller',
    'Bewertung',
    'Haltbarkeit',
    'Tags',
    'Nikotin',
  ]);
  // Nikotin moves past Farbe and then Tags.
  const up = app.page.getByRole('button', { name: 'Nikotin nach oben', exact: true });
  await up.click();
  await expect(app.page.locator('.field-table tbody tr').nth(4)).toContainText('Nikotin');
  await up.click();
  await expect(previewOrder(app)).toHaveText([
    'Bild',
    'Name',
    'Hersteller',
    'Bewertung',
    'Haltbarkeit',
    'Nikotin',
    'Tags',
  ]);
  const rowOrder = (scope: import('@playwright/test').Locator) =>
    scope
      .locator('.item-card-content > *')
      .evaluateAll((rows) => rows.map((row) => row.className.split(' ')[0]));
  const inPreview = await rowOrder(preview(app));
  expect(inPreview.indexOf('card-data')).toBeLessThan(inPreview.indexOf('card-tags'));
  await app.page.goto(app.url + '/liquids?view=grid');
  await expect(app.page.locator('.item-card')).toHaveCount(1);
  expect(await rowOrder(app.page.locator('main'))).toEqual(inPreview);

  await goto(app, '/settings');
  await app.page.getByRole('checkbox', { name: 'Nikotin: Kachel', exact: true }).click();
  await expect(previewOrder(app)).toHaveText([
    'Bild',
    'Name',
    'Hersteller',
    'Bewertung',
    'Haltbarkeit',
    'Tags',
  ]);
  const prefs = await (await app.api.get('/api/v1/preferences')).json();
  await app.write('/preferences', 'PUT', { ...prefs, images: false });
  await goto(app, '/settings');
  await expect(preview(app).locator('.item-stage.stripped')).toBeVisible();
  await expect(previewOrder(app).first()).toHaveText('Farbstreifen');

  const context = await browser.newContext();
  const visitor = await context.newPage();
  await visitor.goto(server.url + '/settings');
  await expect(visitor.getByRole('heading', { name: 'Anmelden', exact: true })).toBeVisible();
  await expect(visitor.getByText('Kachelvorschau', { exact: true })).toHaveCount(0);
  await context.close();
});
test('preview card is for looking only', async ({ app }) => {
  await goto(app, '/settings');
  const card = preview(app).locator('.item-card');
  await expect(card.locator('.fav-button')).toBeVisible();
  expect(await card.evaluate((node) => !!node.closest('[inert]'))).toBe(true);
});
test('ticking a view box shows the field; a choice made in the view dialog is named', async ({
  app,
}) => {
  await goto(app, '/settings');
  const hint = preview(app).getByText(/In „Ansicht“ ausgeblendet/);
  await expect(hint).toHaveCount(0);
  await app.page.getByRole('checkbox', { name: 'Kaufdatum: Kachel', exact: true }).click();
  await expect(previewOrder(app).filter({ hasText: 'Kaufdatum' })).toHaveCount(1);
  await addLiquid(app, 'Echt');
  await app.page.goto(app.url + '/liquids?view=grid');
  await expect(app.page.locator('.item-card dt', { hasText: 'Kaufdatum' })).toBeVisible();

  await app.page.getByRole('button', { name: 'Ansicht', exact: true }).click();
  const view = app.page.getByRole('dialog', { name: 'Ansicht', exact: true });
  await view
    .getByRole('group', { name: 'Kachelfelder' })
    .getByLabel('Kaufdatum', { exact: true })
    .uncheck();
  await view.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(view).not.toBeVisible();
  await goto(app, '/settings');
  await expect(previewOrder(app).filter({ hasText: 'Kaufdatum' })).toHaveCount(0);
  await expect(hint).toHaveText('In „Ansicht“ ausgeblendet: Kaufdatum');

  const list = app.page.getByRole('checkbox', { name: 'Kaufdatum: Liste', exact: true });
  await list.click();
  await expect(list).toBeChecked();
  await app.page.goto(app.url + '/liquids');
  await app.page.getByRole('button', { name: 'Ansicht', exact: true }).click();
  await expect(
    view.getByRole('group', { name: 'Listenspalten' }).getByLabel('Kaufdatum', { exact: true }),
  ).toBeChecked();
});
test('a new palette colour is kept and reaches card and chart', async ({ app }) => {
  const mint = 'rgb(189, 235, 203)';
  await goto(app, '/liquids');
  await app.page.getByRole('button', { name: 'Liquid hinzufügen', exact: true }).first().click();
  await app.page.getByLabel('Name', { exact: true }).fill('Frisch');
  await app.page.getByRole('button', { name: 'Tag hinzufügen', exact: true }).click();
  const tag = app.page.getByRole('dialog', { name: 'Tag hinzufügen', exact: true });
  await tag.getByLabel('Tagname', { exact: true }).fill('Frischetag');
  await tag.getByRole('button', { name: 'Minze', exact: true }).click();
  await tag.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(tag).not.toBeVisible();
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  const badge = app.page.locator('.item-card .tag').filter({ hasText: 'Frischetag' });
  await expect(badge).toHaveCSS('background-color', mint);

  await goto(app, '/settings?section=tags');
  await app.page.getByRole('button', { name: 'Bearbeiten', exact: true }).click();
  const edit = app.page.getByRole('dialog', { name: 'Tag bearbeiten', exact: true });
  await expect(edit.getByRole('button', { name: 'Minze', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await edit.getByRole('button', { name: 'Abbrechen', exact: true }).click();

  const tagField = (await fields(app)).find((f) => f.type === 'tags');
  const { version } = await (await app.api.get('/api/v1/dashboard')).json();
  await app.write('/dashboard', 'PUT', {
    version,
    widgets: [
      { id: 'd', type: 'donut', title: 'Aromen', size: 'wide', fieldId: tagField.id, limit: 5 },
    ],
  });
  await goto(app, '/dashboard');
  await expect(app.page.locator('.donut circle')).toHaveAttribute('stroke', '#BDEBCB');
  await expect(app.page.locator('.chart-legend .legend-swatch')).toHaveCSS(
    'background-color',
    mint,
  );
});
for (const width of [1440, 620, 360, 320])
  test(`all colours fit the tag dialog at ${width} px and each stays a full target`, async ({
    app,
  }) => {
    await app.page.setViewportSize({ width, height: 800 });
    await goto(app, '/settings?section=tags');
    await app.page.getByRole('button', { name: 'Tag hinzufügen', exact: true }).click();
    const dialog = app.page.getByRole('dialog', { name: 'Tag hinzufügen', exact: true });
    const buttons = dialog.locator('.palette-picker button');
    await expect(buttons).toHaveCount(32);
    const geometry = await dialog.evaluate((node) => {
      const picker = node.querySelector('.palette-picker')!;
      const boxes = [...picker.querySelectorAll('button')].map((b) => b.getBoundingClientRect());
      return {
        dialogRight: node.getBoundingClientRect().right,
        overflow: picker.scrollWidth - picker.clientWidth,
        pageOverflow: document.documentElement.scrollWidth - innerWidth,
        right: Math.max(...boxes.map((box) => box.right)),
        columns: new Set(boxes.map((box) => box.left)).size,
        smallest: Math.min(...boxes.flatMap((box) => [box.width, box.height])),
      };
    });
    expect(geometry.columns).toBe(width <= 600 ? 4 : 8);
    // The spectrum reads row by row, the same on every size.
    const top = async (name: string) =>
      (await dialog.getByRole('button', { name, exact: true }).boundingBox())!.y;
    const firstRow = await top('Rot');
    expect(await top('Mandarine')).toBe(firstRow);
    if (width <= 600) expect(await top('Orange')).toBeGreaterThan(firstRow);
    else expect(await top('Gelb')).toBe(firstRow);
    expect(await top('Grau')).toBeGreaterThan(await top('Immergrün'));
    expect(geometry.smallest).toBeGreaterThanOrEqual(44);
    expect(geometry.right).toBeLessThanOrEqual(geometry.dialogRight);
    expect(geometry.overflow).toBeLessThanOrEqual(0);
    expect(geometry.pageOverflow).toBeLessThanOrEqual(0);
  });
