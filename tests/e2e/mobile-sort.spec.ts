import { test, expect, addField, goto } from './fixtures';
test('mobile sorting keeps ordinary labels readable and long field names available', async ({
  app,
}) => {
  const label =
    'Sehr langer eigener Feldname für die Sortierung mit vollständig lesbarer Bezeichnung';
  const field = await addField(app, label, 'text');
  for (const width of [320, 360, 390]) {
    await app.page.setViewportSize({ width, height: 800 });
    await goto(app, '/liquids');
    for (const [button, direction] of [
      ['Absteigend sortiert', 'asc'],
      ['Aufsteigend sortiert', 'desc'],
    ]) {
      await app.page.getByRole('button', { name: button, exact: true }).click();
      await expect
        .poll(() => new URL(app.page.url()).searchParams.get('direction'))
        .toBe(direction);
      const geometry = await app.page.locator('.toolbar-end').evaluate((toolbar) => {
        const selects = Array.from(toolbar.querySelectorAll('select'));
        const rects = selects.map((select) => {
          const style = getComputedStyle(select),
            rect = select.getBoundingClientRect();
          const canvas = document.createElement('canvas'),
            context = canvas.getContext('2d')!;
          context.font = style.font;
          const textWidth = context.measureText(select.selectedOptions[0].textContent || '').width;
          return {
            width: rect.width,
            height: rect.height,
            left: rect.left,
            right: rect.right,
            bottom: rect.bottom,
            required:
              textWidth +
              parseFloat(style.paddingLeft) +
              parseFloat(style.paddingRight) +
              parseFloat(style.borderLeftWidth) +
              parseFloat(style.borderRightWidth),
          };
        });
        const segment = toolbar.querySelector('[role="group"]')!.getBoundingClientRect();
        return {
          rects,
          segment: {
            top: segment.top,
            left: segment.left,
            right: segment.right,
          },
          viewport: innerWidth,
          scroll: document.documentElement.scrollWidth,
        };
      });
      for (const rect of geometry.rects) {
        expect(rect.height).toBeGreaterThanOrEqual(44);
        expect(rect.width + 1).toBeGreaterThanOrEqual(rect.required);
        expect(rect.left).toBeGreaterThanOrEqual(0);
        expect(rect.right).toBeLessThanOrEqual(width);
      }
      expect(geometry.segment.right).toBeLessThanOrEqual(width);
      expect(geometry.scroll).toBeLessThanOrEqual(geometry.viewport);
    }
    await app.page
      .getByRole('combobox', { name: 'Sortieren nach', exact: true })
      .selectOption('field:' + field.id);
    await expect
      .poll(() => new URL(app.page.url()).searchParams.get('sort'))
      .toBe('field:' + field.id);
    expect(await app.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
  }
});
