import { test, expect, fields, addField, addLiquid, goto, syntheticPNG } from './fixtures';
test('search retains focus after each debounce and accepts another term', async ({ app }) => {
  await goto(app, '/liquids');
  const search = app.page.getByRole('searchbox', { name: 'Liquids suchen' });
  await search.fill('Zitrone');
  await expect.poll(() => new URL(app.page.url()).searchParams.get('search')).toBe('Zitrone');
  await expect(search).toBeFocused();
  await app.page.keyboard.type(' frisch');
  await expect
    .poll(() => new URL(app.page.url()).searchParams.get('search'))
    .toBe('Zitrone frisch');
  await expect(search).toBeFocused();
});
test('canceling an in-flight crop never applies its later revision to the parent draft', async ({
  app,
}) => {
  const image = (await fields(app)).find((f) => f.type === 'image');
  const uploaded = await app.api.post('/api/v1/media', {
    headers: { 'X-CSRF-Token': app.csrf },
    multipart: { file: { name: 'synthetic.png', mimeType: 'image/png', buffer: syntheticPNG() } },
  });
  expect(uploaded.ok()).toBeTruthy();
  const media = await uploaded.json();
  const liquid = await addLiquid(app, 'Crop-Abbruch', {
    [image.id]: { type: 'image', value: media.id },
  });
  let release!: () => void;
  let started!: () => void;
  let finished!: () => void;
  const gate = new Promise<void>((r) => {
      release = r;
    }),
    pending = new Promise<void>((r) => {
      started = r;
    }),
    settled = new Promise<void>((r) => {
      finished = r;
    });
  await app.page.route('**/api/v1/media/*/crop', async (route) => {
    const response = await route.fetch();
    started();
    await gate;
    try {
      await route.fulfill({ response });
    } catch {
      /* The canceled browser request may already be aborted. */
    } finally {
      finished();
    }
  });
  await goto(app, '/liquids');
  await app.page.getByRole('button', { name: 'Bearbeiten', exact: true }).click();
  await app.page.getByRole('button', { name: 'Ausschnitt ändern', exact: true }).click();
  const crop = app.page.getByRole('dialog', { name: 'Bildausschnitt', exact: true });
  await crop.getByLabel('Zoom', { exact: true }).fill('2');
  await crop.getByRole('button', { name: 'Ausschnitt übernehmen', exact: true }).click();
  await pending;
  await app.page.keyboard.press('Escape');
  await expect(crop).not.toBeVisible();
  release();
  await settled;
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  expect(
    (await (await app.api.get('/api/v1/liquids/' + liquid.id)).json()).values[image.id].value,
  ).toBe(media.id);
});
test('selected card images and primary roles obey view choices and images-off', async ({ app }) => {
  const defs = await fields(app),
    image = defs.find((f) => f.type === 'image'),
    rating = defs.find((f) => f.type === 'rating'),
    quantity = defs.find((f) => f.label === 'Menge');
  const second = await addField(app, 'Zusatzbild', 'image');
  const uploaded = await app.api.post('/api/v1/media', {
    headers: { 'X-CSRF-Token': app.csrf },
    multipart: { file: { name: 'synthetic.png', mimeType: 'image/png', buffer: syntheticPNG() } },
  });
  const media = await uploaded.json();
  await addLiquid(app, 'Sichtbarkeit', {
    [image.id]: { type: 'image', value: media.id },
    [second.id]: { type: 'image', value: media.id },
    [rating.id]: { type: 'rating', value: 4 },
    [quantity.id]: { type: 'number', value: '12.0' },
  });
  const prefs = await (await app.api.get('/api/v1/preferences')).json();
  await app.write('/preferences', 'PUT', {
    ...prefs,
    gridFields: [image.id, second.id, rating.id, quantity.id],
  });
  await goto(app, '/liquids');
  await expect(app.page.locator('main img[src^="/api/v1/media/"]')).toHaveCount(2);
  await app.page.getByRole('button', { name: 'Ansicht', exact: true }).click();
  const view = app.page.getByRole('dialog', { name: 'Ansicht', exact: true });
  const grid = view.getByRole('group', { name: 'Kachelfelder' });
  for (const f of [image, rating, quantity])
    await grid.getByLabel(f.label, { exact: true }).uncheck();
  await view.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('img', { name: 'Zusatzbild', exact: true })).toBeVisible();
  await expect(app.page.locator('main img[src^="/api/v1/media/"]')).toHaveCount(1);
  await expect(app.page.getByText('12,0 ml', { exact: true })).not.toBeVisible();
  await expect(app.page.getByLabel('4 von 5', { exact: true })).not.toBeVisible();
  await app.page.getByRole('button', { name: 'Ansicht', exact: true }).click();
  await view.getByLabel('Bilder anzeigen', { exact: true }).uncheck();
  await view.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.locator('main img[src^="/api/v1/media/"]')).toHaveCount(0);
});
test('new pink tags reach cards detail and filter options without reload', async ({ app }) => {
  const tagField = (await fields(app)).find((f) => f.type === 'tags');
  const prefs = await (await app.api.get('/api/v1/preferences')).json();
  await app.write('/preferences', 'PUT', { ...prefs, gridFields: [tagField.id] });
  await goto(app, '/liquids');
  await app.page.getByRole('button', { name: 'Liquid hinzufügen', exact: true }).first().click();
  await app.page.getByLabel('Name', { exact: true }).fill('Tagtest');
  await app.page.getByRole('button', { name: 'Tag hinzufügen', exact: true }).click();
  const tag = app.page.getByRole('dialog', { name: 'Tag hinzufügen', exact: true });
  await tag.getByLabel('Tagname', { exact: true }).fill('Rosatag');
  await tag.getByRole('button', { name: 'Rosa', exact: true }).click();
  await tag.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(tag).not.toBeVisible();
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  const badge = app.page.locator('.item-card .tag').filter({ hasText: 'Rosatag' });
  await expect(badge).toBeVisible();
  await expect(badge).toHaveCSS('background-color', 'rgb(217, 180, 215)');
  await app.page.getByRole('button', { name: 'Tagtest', exact: true }).click();
  await expect(app.page.getByRole('dialog').getByText('Rosatag', { exact: true })).toBeVisible();
  await app.page
    .getByRole('dialog')
    .getByRole('button', { name: 'Schließen', exact: true })
    .click();
  await app.page.getByRole('button', { name: 'Filter', exact: true }).click();
  const filter = app.page.getByRole('dialog', { name: 'Filter', exact: true });
  await filter.getByRole('button', { name: 'Bedingung hinzufügen', exact: true }).click();
  await filter.getByLabel('Filterfeld 1', { exact: true }).selectOption(tagField.id);
  await filter.getByLabel('Operator 1', { exact: true }).selectOption('any');
  await expect(filter.getByLabel('Rosatag', { exact: true })).toBeVisible();
});
test('expired profile writes reopen login and preserve the edited display name', async ({
  app,
}) => {
  await goto(app, '/settings?section=profile');
  await app.page.getByLabel('Anzeigename', { exact: true }).fill('Profilentwurf');
  await app.context.clearCookies();
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  const login = app.page.getByRole('dialog', { name: 'Sitzung abgelaufen', exact: true });
  await expect(login).toBeVisible();
  await login.getByLabel('Benutzername', { exact: true }).fill(app.username);
  await login.getByLabel('Passwort', { exact: true }).fill(app.password);
  await login.getByRole('button', { name: 'Anmelden', exact: true }).click();
  await expect(login).not.toBeVisible();
  await expect(app.page.getByLabel('Anzeigename', { exact: true })).toHaveValue('Profilentwurf');
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByText('Profil gespeichert.', { exact: true })).toBeVisible();
});
test('dirty widget and existing default close only after explicit discard confirmation', async ({
  app,
}) => {
  const field = await addField(app, 'Standardtest', 'text', { defaultValue: 'Vorlage' });
  await goto(app, '/dashboard');
  await app.page.getByRole('button', { name: 'Dashboard anpassen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Bearbeiten', exact: true }).first().click();
  const widget = app.page.getByRole('dialog', { name: 'Widget bearbeiten', exact: true });
  await widget.getByLabel('Widgettitel', { exact: true }).fill('Ungespeichert');
  await app.page.keyboard.press('Escape');
  const confirm = app.page.getByRole('dialog', { name: 'Änderungen verwerfen?', exact: true });
  await expect(confirm).toBeVisible();
  await confirm.getByRole('button', { name: 'Abbrechen', exact: true }).click();
  await expect(widget.getByLabel('Widgettitel', { exact: true })).toHaveValue('Ungespeichert');
  await widget.getByRole('button', { name: 'Schließen', exact: true }).click();
  await confirm.getByRole('button', { name: 'Bestätigen', exact: true }).click();
  await expect(widget).not.toBeVisible();
  await app.page.getByRole('button', { name: 'Abbrechen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Bestätigen', exact: true }).click();
  await goto(app, '/settings');
  await app.page
    .locator('.setting-line')
    .filter({ hasText: 'Standardtest' })
    .getByRole('button', { name: 'Bearbeiten', exact: true })
    .click();
  const editor = app.page.getByRole('dialog', { name: 'Feld bearbeiten', exact: true });
  await editor.getByLabel('Standardwert für neue Liquids', { exact: true }).uncheck();
  await app.page.keyboard.press('Escape');
  await expect(confirm).toBeVisible();
  await confirm.getByRole('button', { name: 'Bestätigen', exact: true }).click();
  await expect(editor).not.toBeVisible();
  expect((await fields(app)).find((f) => f.id === field.id).config.defaultValue).toBe('Vorlage');
});
