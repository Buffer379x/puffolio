import { test, expect, addField, goto } from './fixtures';
test('duplicate date widgets save as whole layout and persist across reload; cancel discards draft', async ({
  app,
}) => {
  const dateA = await addField(app, 'Datum A', 'date'),
    dateB = await addField(app, 'Datum B', 'date');
  await goto(app, '/dashboard');
  await app.page.getByRole('button', { name: 'Dashboard anpassen', exact: true }).click();
  const before = (await (await app.api.get('/api/v1/dashboard')).json()).widgets;
  for (const [field, title] of [
    [dateA, 'Erste Termine'],
    [dateB, 'Zweite Termine'],
  ] as const) {
    await app.page.getByRole('button', { name: 'Widget hinzufügen', exact: true }).click();
    const d = app.page.getByRole('dialog');
    await d.getByLabel('Widgettyp').selectOption('dates');
    await d.getByLabel('Widgettitel').fill(title);
    await d.getByLabel('Widgetfeld').selectOption(field.id);
    await d.getByLabel('Widget von').fill('2026-01-01');
    await d.getByLabel('Widget bis').fill('2028-12-31');
    await d.getByRole('button', { name: 'Übernehmen', exact: true }).click();
  }
  await app.page.getByRole('button', { name: 'Zweite Termine nach oben', exact: true }).click();
  await app.page.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(
    app.page.getByRole('button', { name: 'Dashboard anpassen', exact: true }),
  ).toBeVisible();
  const saved = await (await app.api.get('/api/v1/dashboard')).json();
  expect(saved.widgets.filter((w: any) => w.type === 'dates')).toHaveLength(2);
  expect(saved.widgets.length).toBe(before.length + 2);
  await app.page.reload();
  await expect(app.page.getByRole('heading', { name: 'Erste Termine', exact: true })).toBeVisible();
  await expect(
    app.page.getByRole('heading', { name: 'Zweite Termine', exact: true }),
  ).toBeVisible();
  await app.page.getByRole('button', { name: 'Dashboard anpassen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Zweite Termine entfernen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Abbrechen', exact: true }).click();
  await app.page.getByRole('button', { name: 'Bestätigen', exact: true }).click();
  expect((await (await app.api.get('/api/v1/dashboard')).json()).widgets).toEqual(saved.widgets);
});
test('tool contains only image/title and calculator invalidates old results for unit changes', async ({
  app,
}) => {
  await goto(app, '/tools');
  const card = app.page.locator('.item-card');
  await expect(card).toHaveCount(1);
  await expect(card.getByRole('img')).toHaveCount(0);
  await app.page.getByRole('link', { name: 'Nikotinrechner', exact: true }).click();
  await app.page.getByLabel('Flaschenmenge', { exact: true }).fill('120');
  await app.page.getByLabel('Aroma', { exact: true }).fill('10');
  await app.page.getByLabel('Nikotinshots', { exact: true }).fill('20');
  await app.page.getByLabel('Stärke der Shots', { exact: true }).fill('20');
  await app.page.getByRole('button', { name: 'Berechnen', exact: true }).click();
  await expect(app.page.getByText('0,003 mg/ml', { exact: true })).toBeVisible();
  await app.page.getByLabel('Einheit der Shotstärke').selectOption('mg/ml');
  await expect(app.page.getByText('0,003 mg/ml', { exact: true })).not.toBeVisible();
  await app.page.getByRole('button', { name: 'Berechnen', exact: true }).click();
  await expect(app.page.getByText('3,333 mg/ml', { exact: true })).toBeVisible();
  await app.page.getByLabel('Nikotinshots', { exact: true }).fill('110');
  await app.page.getByRole('button', { name: 'Berechnen', exact: true }).click();
  await expect(app.page.getByText('0,000 ml', { exact: true })).toBeVisible();
  await app.page.getByLabel('Nikotinshots', { exact: true }).fill('111');
  await app.page.getByRole('button', { name: 'Berechnen', exact: true }).click();
  await expect(app.page.getByRole('alert')).toContainText('überschreiten');
  expect((await (await app.api.get('/api/v1/liquids')).json()).total).toBe(0);
});
