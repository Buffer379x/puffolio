import { test, expect, fields, addLiquid, goto } from './fixtures';
test('view switch preserves page search sort and combined favorite/tag filters', async ({
  app,
}) => {
  const defs = await fields(app);
  const tagField = defs.find((f) => f.type === 'tags');
  const tag = await app.write('/fields/' + tagField.id + '/tags', 'POST', {
    name: 'Testtag',
    paletteId: 'teal',
  });
  for (let i = 0; i < 25; i++)
    await addLiquid(
      app,
      'Treffer ' + String(i).padStart(2, '0'),
      { [tagField.id]: { type: 'tags', value: [tag.id] } },
      true,
    );
  const filters = { fieldId: tagField.id, operator: 'all', value: [tag.id] };
  await goto(
    app,
    '/liquids?view=grid&search=Treffer&favorite=true&sort=name&direction=asc&page=2&pageSize=24&filters=' +
      encodeURIComponent(JSON.stringify(filters)),
  );
  await expect(app.page.getByText('Seite 2 von 2')).toBeVisible();
  await expect(app.page.getByRole('button', { name: 'Treffer 24', exact: true })).toBeVisible();
  await app.page.getByRole('button', { name: 'Liste', exact: true }).click();
  await expect(app.page).toHaveURL(/view=list/);
  await expect(app.page.getByText('Seite 2 von 2')).toBeVisible();
  expect(new URL(app.page.url()).searchParams.get('filters')).toBe(JSON.stringify(filters));
  await app.page
    .getByRole('button', { name: 'Favorit entfernen: Treffer 24', exact: true })
    .click();
  await expect(app.page.getByRole('button', { name: 'Treffer 24', exact: true })).not.toBeVisible();
});
test('409 keeps visible draft and hidden field values survive subsequent edit', async ({ app }) => {
  const defs = await fields(app);
  const quantity = defs.find((f) => f.type === 'number');
  const text = defs.find((f) => f.type === 'text');
  const liquid = await addLiquid(app, 'Konflikt', {
    [quantity.id]: { type: 'number', value: '0.0' },
    [text.id]: { type: 'text', value: 'Versteckt' },
  });
  await app.write('/fields/' + text.id, 'PATCH', {
    ...text,
    visibility: { ...text.visibility, form: false },
  });
  await goto(app, '/liquids');
  await app.page.getByRole('button', { name: 'Bearbeiten', exact: true }).click();
  await app.page.getByLabel('Name', { exact: true }).fill('Mein Entwurf');
  await app.write('/liquids/' + liquid.id, 'PATCH', {
    version: liquid.version,
    name: 'Anderer Stand',
  });
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('alert').first()).toContainText('Entwurf bleibt erhalten');
  await expect(app.page.getByLabel('Name', { exact: true })).toHaveValue('Mein Entwurf');
  await app.page.getByRole('button', { name: 'Abbrechen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Bestätigen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Bearbeiten', exact: true }).click();
  await expect(app.page.getByLabel('Name', { exact: true })).toHaveValue('Anderer Stand');
  await app.page.getByLabel(quantity.label, { exact: true }).fill('2,5');
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  const saved = await (await app.api.get('/api/v1/liquids/' + liquid.id)).json();
  expect(saved.values[text.id].value).toBe('Versteckt');
  expect(saved.values[quantity.id].value).toBe('2.5');
});
