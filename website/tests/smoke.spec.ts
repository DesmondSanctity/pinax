import { test, expect } from '@playwright/test';

test.describe('marketing site smoke', () => {
  test('home page renders with H1, OG meta and JSON-LD', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/Pinax/);
    await expect(page.locator('h1').first()).toContainText(/Any docs site|local MCP/i);

    const ogImage = await page.locator('meta[property="og:image"]').getAttribute('content');
    expect(ogImage).toContain('/og.png');

    const ld = await page.locator('script[type="application/ld+json"]').textContent();
    expect(ld).toBeTruthy();
    const data = JSON.parse(ld!);
    expect(data['@type']).toBe('SoftwareApplication');
    expect(data.name).toBe('Pinax');
  });

  test('catalog page lists entries with copy buttons', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await page.goto('/catalog');
    await expect(page.locator('h1').first()).toContainText(/Catalog/i);

    const copyButtons = page.locator('button.copy-btn');
    await expect(copyButtons.first()).toBeVisible();
    expect(await copyButtons.count()).toBeGreaterThan(0);

    await copyButtons.first().click();
    await expect(copyButtons.first()).toHaveAttribute('aria-label', /Copied/i);
    const clip = await page.evaluate(() => navigator.clipboard.readText());
    expect(clip.length).toBeGreaterThan(0);
  });

  test('quick-start docs page renders with sidebar', async ({ page }) => {
    await page.goto('/docs/quick-start');
    await expect(page.locator('h1').first()).toBeVisible();
    await expect(page.getByRole('navigation', { name: /Docs navigation/i })).toBeVisible();
  });

  test('404 page renders helpful links', async ({ page }) => {
    const res = await page.goto('/does-not-exist');
    expect(res?.status()).toBe(404);
    await expect(page.locator('h1').first()).toContainText(/Nothing here/i);
    await expect(page.getByRole('link', { name: /Quick start/ })).toBeVisible();
  });
});
