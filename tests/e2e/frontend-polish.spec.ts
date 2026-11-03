import { test, expect, fields, addField, addLiquid, goto } from './fixtures';
test('visible labels and grouped field descriptions work with native controls', async ({ app }) => {
  await addField(app, 'Heute-Datum', 'date', { defaultToday: true });
  await goto(app, '/liquids');
  await app.page.getByRole('button', { name: 'Liquid hinzufügen', exact: true }).first().click();
  const dialog = app.page.getByRole('dialog', { name: 'Liquid hinzufügen', exact: true });
  await dialog
    .locator('label')
    .filter({ hasText: /^Name$/ })
    .click();
  await expect(dialog.getByLabel('Name', { exact: true })).toBeFocused();
  await dialog.getByLabel('Name', { exact: true }).fill('Beschriftungstest');
  for (const name of ['Bewertung', 'Tags', 'Bild', 'Farbe'])
    await expect(dialog.getByRole('group', { name, exact: true })).toBeVisible();
  await dialog.getByRole('radio', { name: 'Bewertung: 3 von 5', exact: true }).focus();
  await app.page.keyboard.press('Space');
  await expect(
    dialog.getByRole('radio', { name: 'Bewertung: 3 von 5', exact: true }),
  ).toBeChecked();
  await expect(dialog.getByLabel('Heute-Datum', { exact: true })).toHaveAccessibleDescription(
    'Heute beim Anlegen',
  );
  await dialog
    .locator('label')
    .filter({ hasText: /^Menge$/ })
    .click();
  const quantity = dialog.getByLabel('Menge', { exact: true });
  await expect(quantity).toBeFocused();
  await quantity.fill('keine Zahl');
  await dialog.getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(quantity).toHaveAccessibleDescription('Zahl eingeben.');
  await expect(quantity).toHaveAttribute('aria-invalid', 'true');
  await expect(dialog.locator('label label')).toHaveCount(0);
  expect(
    await dialog.evaluate((el) =>
      Array.from(el.querySelectorAll<HTMLLabelElement>('label[for]')).every(
        (label) =>
          !!label.control &&
          ['INPUT', 'SELECT', 'TEXTAREA', 'BUTTON'].includes(label.control.tagName),
      ),
    ),
  ).toBe(true);
});
test('typed table values stay unbroken, actions align and bounded scrolling reaches every column', async ({
  app,
}) => {
  const definitions = await fields(app);
  const values: Record<string, unknown> = {};
  for (const f of definitions) {
    let value: unknown;
    if (f.type === 'rating') value = 4;
    else if (f.type === 'number') value = f.label === 'Menge' ? '120.0' : '3.0';
    else if (f.type === 'date') value = '2026-11-03';
    else if (f.type === 'color') value = '#A1B2C3';
    else if (f.type === 'text') value = 'Beispielhersteller';
    else if (f.type === 'longtext')
      value =
        'Ein langer echter Text darf sinnvoll umbrechen und bleibt im Detail vollständig lesbar. '.repeat(
          8,
        );
    if (value !== undefined) values[f.id] = { type: f.type, value };
  }
  await addLiquid(app, 'Geometrie A', values);
  await addLiquid(app, 'Geometrie B', values);
  const prefs = await (await app.api.get('/api/v1/preferences')).json();
  await app.write('/preferences', 'PUT', { ...prefs, listFields: definitions.map((f) => f.id) });
  for (const width of [1512, 360]) {
    await app.page.setViewportSize({ width, height: 900 });
    await goto(app, '/liquids?view=list');
    await expect(app.page.getByRole('button', { name: 'Geometrie A', exact: true })).toBeVisible();
    const table = app.page.getByRole('table');
    const geometry = await table.evaluate((el) => {
      const lines = (cell: Element) => {
        const tops: number[] = [];
        const walker = document.createTreeWalker(cell, NodeFilter.SHOW_TEXT);
        let node: Node | null;
        while ((node = walker.nextNode())) {
          if (!node.textContent?.trim()) continue;
          const range = document.createRange();
          range.selectNodeContents(node);
          for (const rect of range.getClientRects())
            if (rect.width > 0.5 && rect.height > 0.5) tops.push(Math.round(rect.top * 2) / 2);
        }
        return new Set(tops).size;
      };
      return {
        headers: Array.from(el.querySelectorAll('thead th')).map(lines),
        typed: Array.from(
          el.querySelectorAll(
            'tbody tr:first-child .cell-date,tbody tr:first-child .cell-rating,tbody tr:first-child .cell-number,tbody tr:first-child .cell-color',
          ),
        ).map(lines),
        textHeight:
          el
            .querySelector('tbody tr:first-child .cell-longtext .text-value')
            ?.getBoundingClientRect().height || 0,
      };
    });
    expect(geometry.headers.every((count) => count <= 1)).toBe(true);
    expect(geometry.typed.length).toBeGreaterThan(3);
    expect(geometry.typed.every((count) => count === 1)).toBe(true);
    expect(geometry.textHeight).toBeGreaterThan(20);
    expect(geometry.textHeight).toBeLessThanOrEqual(61);
    const scroll = app.page.getByLabel('Liquidtabelle', { exact: true });
    expect(await scroll.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true);
    expect(await app.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    const selectAll = app.page.getByRole('checkbox', {
      name: 'Alle auf dieser Seite auswählen',
      exact: true,
    });
    const label = selectAll.locator('xpath=..');
    const hitbox = await label.boundingBox();
    expect(hitbox!.width).toBeGreaterThanOrEqual(44);
    expect(hitbox!.height).toBeGreaterThanOrEqual(44);
    await label.click({ position: { x: 2, y: 2 } });
    await expect(
      app.page.getByRole('checkbox', { name: 'Geometrie A auswählen', exact: true }),
    ).toBeChecked();
    await expect(
      app.page.getByRole('checkbox', { name: 'Geometrie B auswählen', exact: true }),
    ).toBeChecked();
    await selectAll.focus();
    await app.page.keyboard.press('Space');
    await expect(
      app.page.getByRole('checkbox', { name: 'Geometrie A auswählen', exact: true }),
    ).not.toBeChecked();
    await scroll.evaluate((el) => {
      el.scrollLeft = el.scrollWidth;
    });
    const buttons = table.locator('tbody tr').first().locator('.table-actions button');
    const a = await buttons.nth(0).boundingBox(),
      b = await buttons.nth(1).boundingBox(),
      bounds = await scroll.boundingBox();
    expect(Math.abs(a!.y - b!.y)).toBeLessThan(1);
    expect(a!.width).toBeGreaterThanOrEqual(44);
    expect(b!.width).toBeGreaterThanOrEqual(44);
    expect(a!.x).toBeGreaterThanOrEqual(bounds!.x);
    expect(b!.x + b!.width).toBeLessThanOrEqual(bounds!.x + bounds!.width);
    if (width > 900) {
      const pinned = await table.locator('tbody tr').first().locator('.name-column').boundingBox();
      expect(Math.abs(pinned!.x - bounds!.x)).toBeLessThan(2);
    }
  }
});
