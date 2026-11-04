#!/usr/bin/env node
// Regenerates docs/screenshots from the real app and synthetic sample data.
//   pnpm --dir web build && go build -o work/bin/puffolio ./cmd/puffolio
//   pnpm --dir web screenshots
// PUFFOLIO_CHROMIUM may name a Chromium binary if Playwright's own is not installed.
import { mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import { DEMO_LOGIN, startInstance } from './seed.mjs';

const root = resolve(import.meta.dirname, '../..');
const { chromium } = createRequire(resolve(root, 'web/package.json'))('@playwright/test');
const out = resolve(root, 'docs/screenshots');
mkdirSync(out, { recursive: true });

const DESKTOP = { width: 1440, height: 900 };
const PHONE = { width: 390, height: 844 };
/** Grows the window to the content, so the fixed sidebar reaches the bottom of the image. */
const fitHeight = async (page, width, limit) => {
  const height = await page.evaluate(() => document.documentElement.scrollHeight);
  await page.setViewportSize({ width, height: Math.min(height, limit) });
  await page.waitForTimeout(150);
};

const { url, stop } = await startInstance(process.env.PUFFOLIO_DEMO_PORT || '18799');
const browser = await chromium.launch({
  executablePath: process.env.PUFFOLIO_CHROMIUM || undefined,
});
try {
  const session = async (options, signIn = true) => {
    const context = await browser.newContext({
      locale: 'de-DE',
      timezoneId: 'Europe/Berlin',
      ...options,
    });
    const page = await context.newPage();
    page.on('pageerror', (error) => {
      throw error;
    });
    if (signIn) {
      await page.goto(url + '/login');
      await page.getByLabel('Benutzername').fill(DEMO_LOGIN.username);
      await page.getByLabel('Passwort').fill(DEMO_LOGIN.password);
      await page.getByRole('button', { name: 'Anmelden', exact: true }).click();
      await page.waitForSelector('main h1');
    }
    return { context, page };
  };
  const open = async (page, route) => {
    await page.goto(url + route);
    await page.waitForSelector('main h1');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(250);
  };
  const shot = (page, name) => page.screenshot({ path: `${out}/${name}.png` });

  for (const scheme of ['dark', 'light']) {
    const suffix = scheme === 'dark' ? '' : '-hell';
    const { context, page } = await session({ viewport: DESKTOP, colorScheme: scheme });
    await open(page, '/dashboard');
    await fitHeight(page, DESKTOP.width, 1200);
    await shot(page, 'dashboard' + suffix);
    await page.setViewportSize(DESKTOP);
    await open(page, '/liquids?view=grid');
    await fitHeight(page, DESKTOP.width, 1200);
    await shot(page, 'liquids-kacheln' + suffix);
    await context.close();
  }

  const { context, page } = await session({ viewport: DESKTOP, colorScheme: 'dark' });
  await open(page, '/liquids?view=list&sort=shelf&direction=asc');
  await shot(page, 'liquids-liste');
  await open(page, '/liquids?view=grid');
  await page.getByRole('button', { name: 'Citrus Mint', exact: true }).click();
  await page.getByRole('dialog').waitFor();
  await page.waitForTimeout(300);
  await shot(page, 'liquid-detail');
  await open(page, '/settings?section=fields');
  await fitHeight(page, DESKTOP.width, 1560);
  await shot(page, 'einstellungen-felder');
  await page.setViewportSize(DESKTOP);
  await open(page, '/tools/nicotine');
  await page.getByLabel('Flaschenmenge').fill('120');
  await page.getByLabel('Aroma').fill('10');
  await page.getByLabel('Nikotinshots').fill('20');
  await page.getByLabel('Stärke der Shots').fill('20');
  await page.getByRole('button', { name: 'Berechnen', exact: true }).click();
  await page.getByText('3,333 mg/ml').waitFor();
  await shot(page, 'nikotinrechner');
  await context.close();

  const visitor = await session({ viewport: DESKTOP, colorScheme: 'dark' }, false);
  await open(visitor.page, '/liquids?view=grid');
  await shot(visitor.page, 'besucher');
  await visitor.context.close();

  const phone = await session({
    viewport: PHONE,
    deviceScaleFactor: 2,
    isMobile: true,
    hasTouch: true,
    colorScheme: 'dark',
  });
  await open(phone.page, '/liquids?view=grid');
  await shot(phone.page, 'mobil-liquids');
  await open(phone.page, '/dashboard');
  await shot(phone.page, 'mobil-dashboard');
  await phone.context.close();
} finally {
  await browser.close();
  stop();
}
console.log('Screenshots:', out);
