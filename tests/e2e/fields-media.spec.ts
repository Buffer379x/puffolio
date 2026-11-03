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
  await d.getByLabel('Auffüllwert', { exact: true }).selectOption('false');
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
  await app.page.getByLabel('Geprüft', { exact: true }).selectOption('false');
  await app.page.getByLabel('Menge', { exact: true }).fill('0');
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  const saved = (await (await app.api.get('/api/v1/liquids?search=Datum')).json()).items[0];
  expect(saved.values[flag.id].value).toBe(false);
  expect(saved.values[defs.find((f) => f.label === 'Prüfdatum').id].value).toBe('2028-02-29');
  expect(saved.values[defs.find((f) => f.label === 'Menge').id].value).toBe('0.0');
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
  await app.page.getByRole('button', { name: 'Bearbeiten', exact: true }).click();
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
  await app.page.getByRole('button', { name: 'Bearbeiten', exact: true }).click();
  await app.page.getByRole('button', { name: 'Ausschnitt ändern', exact: true }).click();
  await crop.getByLabel('Zoom', { exact: true }).fill('3');
  await crop.getByRole('button', { name: 'Ausschnitt übernehmen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  expect(
    (await (await app.api.get('/api/v1/liquids/' + liquid.id)).json()).values[image.id].value,
  ).not.toBe(original);
});
