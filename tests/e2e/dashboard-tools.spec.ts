import { test, expect, addField, addLiquid, fields, goto, type App } from './fixtures';
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
    await d.getByLabel('Widgetzeitraum').selectOption('custom');
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
  const dates = (widgets: any[]) => widgets.filter((w) => w.type === 'dates').length;
  expect(dates(saved.widgets)).toBe(dates(before) + 2);
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
test('tool contains only image/title and calculator invalidates old results on any change', async ({
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
  await expect(app.page.getByText('3,333 mg/ml', { exact: true })).toBeVisible();
  await expect(app.page.getByLabel('Einheit der Shotstärke')).toHaveCount(0);
  await expect(app.page.getByText(/mg\/l\b/)).toHaveCount(0);
  await app.page.getByLabel('Stärke der Shots', { exact: true }).fill('18');
  await expect(app.page.getByText('3,333 mg/ml', { exact: true })).not.toBeVisible();
  await app.page.getByRole('button', { name: 'Berechnen', exact: true }).click();
  await expect(app.page.getByText('3,000 mg/ml', { exact: true })).toBeVisible();
  await app.page.getByLabel('Nikotinshots', { exact: true }).fill('110');
  await app.page.getByRole('button', { name: 'Berechnen', exact: true }).click();
  await expect(app.page.getByText('0,000 ml', { exact: true })).toBeVisible();
  await app.page.getByLabel('Nikotinshots', { exact: true }).fill('111');
  await app.page.getByRole('button', { name: 'Berechnen', exact: true }).click();
  await expect(app.page.getByRole('alert')).toContainText('überschreiten');
  expect((await (await app.api.get('/api/v1/liquids')).json()).total).toBe(0);
});
test('key-figure widgets look like every other widget', async ({ app }) => {
  await goto(app, '/dashboard');
  const look = (name: string) =>
    app.page.getByRole('heading', { name, exact: true }).evaluate((heading) => {
      const text = getComputedStyle(heading);
      return {
        size: text.fontSize,
        weight: text.fontWeight,
        color: text.color,
        surface: getComputedStyle(heading.closest('.panel')!).backgroundColor,
      };
    });
  const reference = await look('Zuletzt hinzugefügt');
  expect(await look('Liquids')).toEqual(reference);
  expect(await look('Favoriten')).toEqual(reference);
});
async function taggedLiquids(app: App, count: number) {
  const tagField = (await fields(app)).find((f) => f.type === 'tags');
  const tag = await app.write(`/fields/${tagField.id}/tags`, 'POST', {
    name: 'Testtag',
    paletteId: 'teal',
  });
  for (let i = 0; i < count; i++)
    await addLiquid(app, 'Liquid ' + i, { [tagField.id]: { type: 'tags', value: [tag.id] } });
  return tagField;
}
/** The ring is drawn in a 100-unit viewBox, so its size follows from the SVG's on-screen scale. */
const donutGeometry = (app: App, title: string) =>
  app.page
    .locator('.panel')
    .filter({ has: app.page.getByRole('heading', { name: title, exact: true }) })
    .evaluate((panel) => {
      const body = panel.querySelector('.panel-body')!;
      const chart = panel.querySelector('.donut-chart')!.getBoundingClientRect();
      const slot = panel.querySelector('.donut')!.getBoundingClientRect();
      const scale = panel.querySelector<SVGSVGElement>('.donut svg')!.getScreenCTM()!;
      return {
        contentBottom:
          body.getBoundingClientRect().bottom - parseFloat(getComputedStyle(body).paddingBottom),
        chartBottom: chart.bottom,
        chartMiddle: chart.top + chart.height / 2,
        slotMiddle: slot.top + slot.height / 2,
        slotSmallerSide: Math.min(slot.width, slot.height),
        ring: 100 * Math.hypot(scale.a, scale.b),
      };
    });
test('donut uses the height a taller neighbour gives its row', async ({ app }) => {
  await taggedLiquids(app, 6);
  await app.page.setViewportSize({ width: 1440, height: 1000 });
  await goto(app, '/dashboard');
  await expect(app.page.locator('.donut')).toBeVisible();
  const g = await donutGeometry(app, 'Tags');
  expect(Math.abs(g.chartBottom - g.contentBottom)).toBeLessThanOrEqual(1);
  expect(Math.abs(g.slotMiddle - g.chartMiddle)).toBeLessThanOrEqual(1);
  expect(g.ring).toBeGreaterThan(150);
  expect(Math.abs(g.ring - g.slotSmallerSide)).toBeLessThanOrEqual(2);
});
test('donut keeps its minimum size alone in a row and on a phone', async ({ app }) => {
  const tagField = await taggedLiquids(app, 2);
  const { version } = await (await app.api.get('/api/v1/dashboard')).json();
  await app.write('/dashboard', 'PUT', {
    version,
    widgets: [
      { id: 'solo', type: 'donut', title: 'Solo', size: 'wide', fieldId: tagField.id, limit: 5 },
    ],
  });
  await app.page.setViewportSize({ width: 1440, height: 1000 });
  await goto(app, '/dashboard');
  await expect(app.page.locator('.donut')).toBeVisible();
  const wide = await donutGeometry(app, 'Solo');
  expect(wide.ring).toBeGreaterThanOrEqual(139);
  expect(wide.ring).toBeLessThan(160);
  await app.page.setViewportSize({ width: 360, height: 800 });
  const phone = await donutGeometry(app, 'Solo');
  expect(phone.ring).toBeGreaterThanOrEqual(139);
  const overflow = await app.page.evaluate(
    () => document.documentElement.scrollWidth - innerWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
});
test('recently added keeps its rows aligned for any name length', async ({ app }) => {
  const longName = 'Sehr langer Name '.repeat(7).trim();
  await addLiquid(app, 'Kurz');
  await addLiquid(app, longName);
  await addLiquid(app, 'Donaudampfschifffahrtsgesellschaft'.repeat(3));
  await app.page.setViewportSize({ width: 1440, height: 1000 });
  await goto(app, '/dashboard');
  await expect(app.page.locator('.mini-card')).toHaveCount(3);
  const cards = await app.page.locator('.mini-card').evaluateAll((list) =>
    list.map((card) => {
      const name = card.querySelector<HTMLElement>('.name-button span')!;
      return {
        subtitleTop: card.querySelector('.card-subtitle')!.getBoundingClientRect().top,
        ratingTop: card.querySelector('.card-rating-row')!.getBoundingClientRect().top,
        nameHeight: name.getBoundingClientRect().height,
        lineHeight: parseFloat(getComputedStyle(name).lineHeight),
        width: name.getBoundingClientRect().right <= card.getBoundingClientRect().right + 0.5,
      };
    }),
  );
  for (const card of cards) {
    expect(Math.abs(card.subtitleTop - cards[0].subtitleTop)).toBeLessThan(1);
    expect(Math.abs(card.ratingTop - cards[0].ratingTop)).toBeLessThan(1);
    expect(Math.abs(card.nameHeight - 2 * card.lineHeight)).toBeLessThan(1);
    expect(card.width).toBe(true);
  }
  await expect(
    app.page.getByRole('button', { name: longName, exact: true }).locator('span'),
  ).toHaveAttribute('title', longName);
  const empty = app.page.locator('.mini-card').filter({ hasText: 'Kurz' });
  await expect(empty.locator('.card-subtitle')).toHaveText('—');
  await expect(empty.locator('.card-rating-row')).toHaveText('—');
});
