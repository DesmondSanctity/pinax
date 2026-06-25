// Render scripts/og.html → public/og.png at 1200×630 with Playwright.
// Run via: pnpm run og:render
import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const htmlPath = resolve(here, 'og.html');
const outPath = resolve(here, '..', 'public', 'og.png');

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1200, height: 630 } });
await page.goto(`file://${htmlPath}`, { waitUntil: 'networkidle' });
await page.evaluate(() => document.fonts.ready);
await page.screenshot({ path: outPath, fullPage: false, omitBackground: false });
await browser.close();
console.log(`wrote ${outPath}`);
