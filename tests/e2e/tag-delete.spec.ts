import { test, expect, addLiquid, fields, goto } from './fixtures';
type App = Parameters<typeof goto>[0];
const tagField = async (app: App) => (await fields(app)).find((f: { type: string }) => f.type === 'tags');
const makeTag = async (app: App, fid: string, name: string) =>
  await app.write(`/fields/${fid}/tags`, 'POST', { name, paletteId: 'teal' });
const openWarning = async (app: App, name: string) => {
  await goto(app, '/settings?section=tags');
  await app.page.getByRole('button', { name: `${name} löschen`, exact: true }).click();
  return app.page.getByRole('dialog', { name: `Tag „${name}“ löschen?`, exact: true });
};
const confirm = (dialog: ReturnType<App['page']['getByRole']>) =>
  dialog.getByRole('button', { name: 'Bestätigen', exact: true }).click();

test('deleting a used tag warns with the numbers and removes it everywhere', async ({ app }) => {
  const field = await tagField(app);
  const tag = await makeTag(app, field.id, 'Minze');
  const value = { [field.id]: { type: 'tags', value: [tag.id] } };
  await addLiquid(app, 'Eins', value);
  await addLiquid(app, 'Zwei', value);
  const trashed = await addLiquid(app, 'Drei', value);
  await app.write(`/liquids/${trashed.id}`, 'DELETE', { version: trashed.version });

  const warning = await openWarning(app, 'Minze');
  await expect(warning).toContainText('3 Liquids zugeordnet, davon 1 im Papierkorb');
  await expect(warning).toContainText('Das lässt sich nicht rückgängig machen.');
  await expect(
    warning.getByRole('button', { name: 'Vorher Sammlung exportieren', exact: true }),
  ).toBeVisible();

  await warning.getByRole('button', { name: 'Abbrechen', exact: true }).click();
  await expect(warning).not.toBeVisible();
  await expect(app.page.locator('.setting-line .tag')).toHaveText(['Minze']);

  // By keyboard, because Safari does not focus a clicked button.
  const trash = app.page.getByRole('button', { name: 'Minze löschen', exact: true });
  await trash.focus();
  await app.page.keyboard.press('Enter');
  await expect(warning).toBeVisible();
  await warning.getByRole('button', { name: 'Abbrechen', exact: true }).click();
  await expect(warning).not.toBeVisible();
  await expect(trash).toBeFocused();

  await app.page.keyboard.press('Enter');
  await confirm(warning);
  await expect(warning).not.toBeVisible();
  await expect(app.page.locator('.setting-line .tag')).toHaveCount(0);
  await expect(app.page.getByRole('button', { name: 'Tag hinzufügen', exact: true })).toBeFocused();

  await app.page.goto(app.url + '/liquids?view=grid');
  await expect(app.page.locator('.item-card')).toHaveCount(2);
  await expect(app.page.locator('.item-card .tag')).toHaveCount(0);
  await expect(app.page.locator('.filter-chips')).not.toContainText('Minze');
});
test('an unused tag gets the warning too', async ({ app }) => {
  const field = await tagField(app);
  await makeTag(app, field.id, 'Frei');
  const warning = await openWarning(app, 'Frei');
  await expect(warning).toContainText('Er ist keinem Liquid zugeordnet.');
  await expect(warning).toContainText('Das lässt sich nicht rückgängig machen.');
  await expect(warning.getByRole('button', { name: /exportieren/ })).toHaveCount(0);
  await confirm(warning);
  await expect(app.page.locator('.setting-line .tag')).toHaveCount(0);
});
test('one liquid reads in the singular', async ({ app }) => {
  const field = await tagField(app);
  const tag = await makeTag(app, field.id, 'Einzeln');
  await addLiquid(app, 'Eins', { [field.id]: { type: 'tags', value: [tag.id] } });
  const warning = await openWarning(app, 'Einzeln');
  await expect(warning).toContainText('Dieser Tag ist 1 Liquid zugeordnet. Er wird dort entfernt.');
});
test('the warning names required field, default value, widgets and views when they apply', async ({
  app,
}) => {
  const field = await tagField(app);
  const tag = await makeTag(app, field.id, 'Minze');
  await app.write(`/fields/${field.id}`, 'PATCH', {
    version: field.version,
    required: true,
    config: { ...field.config, defaultValue: [tag.id] },
  });
  await addLiquid(app, 'Eins');
  const filter = { fieldId: field.id, operator: 'any', value: [tag.id] };
  const { version } = await (await app.api.get('/api/v1/dashboard')).json();
  await app.write('/dashboard', 'PUT', {
    version,
    widgets: [{ id: 'w', type: 'count', title: 'Minze', size: 'small', filters: filter }],
  });
  for (const name of ['Erste', 'Zweite'])
    await app.write('/saved-views', 'POST', {
      name,
      search: '',
      sort: 'name',
      direction: 'asc',
      filters: filter,
    });
  const warning = await openWarning(app, 'Minze');
  await expect(warning).toContainText(
    `„${field.label}“ ist ein Pflichtfeld: 1 Liquid hätte danach keinen Tag mehr. Die Pflicht gilt dann nur noch für neu angelegte Liquids.`,
  );
  await expect(warning).toContainText('Der Tag ist Standardwert dieses Felds.');
  await expect(warning).toContainText('1 Dashboard-Widget filtert nach diesem Tag.');
  await expect(warning).toContainText('2 gespeicherte Ansichten filtern nach diesem Tag.');
});
test('a required field is not mentioned while every liquid keeps a tag', async ({ app }) => {
  const field = await tagField(app);
  const [tag, other] = [await makeTag(app, field.id, 'Minze'), await makeTag(app, field.id, 'Rest')];
  await app.write(`/fields/${field.id}`, 'PATCH', { version: field.version, required: true });
  await addLiquid(app, 'Beide', { [field.id]: { type: 'tags', value: [tag.id, other.id] } });
  const warning = await openWarning(app, 'Minze');
  await expect(warning).toContainText('1 Liquid zugeordnet');
  await expect(warning).not.toContainText('Pflichtfeld');
});
test('the warning leaves out what does not apply', async ({ app }) => {
  const field = await tagField(app);
  const tag = await makeTag(app, field.id, 'Schlicht');
  await addLiquid(app, 'Eins', { [field.id]: { type: 'tags', value: [tag.id] } });
  const warning = await openWarning(app, 'Schlicht');
  await expect(warning).not.toContainText('Pflichtfeld');
  await expect(warning).not.toContainText('Standardwert');
  await expect(warning).not.toContainText('Widget');
  await expect(warning).not.toContainText('Ansicht');
  await expect(warning).not.toContainText('Papierkorb');
});
test('a tag changed elsewhere asks to look again and can then be deleted', async ({ app }) => {
  const field = await tagField(app);
  const tag = await makeTag(app, field.id, 'Minze');
  const warning = await openWarning(app, 'Minze');
  await app.write(`/fields/${field.id}/tags/${tag.id}`, 'PATCH', {
    name: 'Neu',
    paletteId: 'blue',
    version: tag.version,
  });
  await confirm(warning);
  await expect(warning.locator('.error, [role="alert"]').first()).toBeVisible();
  await warning.getByRole('button', { name: 'Abbrechen', exact: true }).click();
  await expect(app.page.locator('.setting-line .tag')).toHaveText(['Neu']);
  await app.page.getByRole('button', { name: 'Neu löschen', exact: true }).click();
  const again = app.page.getByRole('dialog', { name: 'Tag „Neu“ löschen?', exact: true });
  await expect(again.locator('.error, [role="alert"]')).toHaveCount(0);
  await confirm(again);
  await expect(app.page.locator('.setting-line .tag')).toHaveCount(0);
});
for (const width of [1280, 360])
  test(`a tag name of 100 characters stays inside the warning at ${width} px`, async ({ app }) => {
    await app.page.setViewportSize({ width, height: 800 });
    const field = await tagField(app);
    const name = 'Waldmeister'.repeat(9) + 'x';
    await makeTag(app, field.id, name);
    await goto(app, '/settings?section=tags');
    const line = app.page.locator('.setting-line');
    expect(await app.page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBe(0);
    await expect(line.getByRole('button', { name: 'Bearbeiten', exact: true })).toBeInViewport();
    await line.getByRole('button', { name: `${name} löschen`, exact: true }).click();
    const warning = app.page.getByRole('dialog');
    await expect(warning).toBeVisible();
    const overflow = await warning.evaluate((node) => {
      const content = node.querySelector('.dialog-content')!;
      return content.scrollWidth - content.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(0);
    await expect(warning.getByRole('button', { name: 'Bestätigen', exact: true })).toBeInViewport();
  });
test('a link with a deleted tag shows the liquids and does not trap the back button', async ({
  app,
}) => {
  const field = await tagField(app);
  const tag = await makeTag(app, field.id, 'Minze');
  const gone = await makeTag(app, field.id, 'Weg');
  await addLiquid(app, 'Eins', { [field.id]: { type: 'tags', value: [tag.id] } });
  await addLiquid(app, 'Zwei');
  await app.write(`/fields/${field.id}/tags/${gone.id}`, 'DELETE', {
    version: gone.version,
    confirm: true,
  });
  await goto(app, '/settings');
  const filter = encodeURIComponent(
    JSON.stringify({ fieldId: field.id, operator: 'any', value: [gone.id] }),
  );
  await app.page.goto(app.url + `/liquids?view=grid&filters=${filter}`);
  await expect(app.page.locator('.item-card')).toHaveCount(2);
  await expect(app.page).not.toHaveURL(/filters=/);
  await expect(app.page.getByText('Gelöschter Tag aus dem Filter entfernt')).toBeVisible();

  const mixed = encodeURIComponent(
    JSON.stringify({ fieldId: field.id, operator: 'any', value: [tag.id, gone.id] }),
  );
  await app.page.goto(app.url + `/liquids?view=grid&filters=${mixed}`);
  await expect(app.page.locator('.item-card')).toHaveCount(1);
  await expect(app.page.locator('.item-card')).toContainText('Eins');
  await app.page.goBack();
  await expect(app.page.locator('.item-card')).toHaveCount(2);
  await expect(app.page).not.toHaveURL(/filters=/);
});
