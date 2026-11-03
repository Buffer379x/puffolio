import { test, expect, goto } from './fixtures';
test('native dialog Escape restores focus and mobile drawer is inert while closed', async ({
  app,
}) => {
  await app.page.setViewportSize({ width: 360, height: 800 });
  await goto(app, '/liquids');
  const opener = app.page.getByRole('button', { name: 'Liquid hinzufügen', exact: true }).first();
  await opener.click();
  await expect(
    app.page.getByRole('dialog', { name: 'Liquid hinzufügen', exact: true }),
  ).toBeVisible();
  await app.page.keyboard.press('Escape');
  await expect(app.page.getByRole('dialog')).not.toBeVisible();
  await expect(opener).toBeFocused();
  const menu = app.page.getByRole('button', { name: 'Menü öffnen', exact: true });
  await menu.click();
  const drawer = app.page.getByRole('dialog', { name: 'Hauptnavigation' });
  await expect(drawer).toBeVisible();
  await app.page.keyboard.press('Shift+Tab');
  await expect(drawer.getByRole('button', { name: 'Abmelden', exact: true })).toBeFocused();
  await app.page.keyboard.press('Escape');
  await expect(drawer).not.toBeVisible();
  await expect(menu).toBeFocused();
  expect(await app.page.locator('.sidebar').evaluate((el) => (el as HTMLElement).inert)).toBe(true);
  await app.page.setViewportSize({ width: 320, height: 800 });
  expect(await app.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
    true,
  );
});
