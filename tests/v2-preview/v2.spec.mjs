import { test, expect } from '@playwright/test';
import { fileURLToPath } from 'node:url';
import { createV2Scenarios } from './scenarios.mjs';

test('V2 interactive demo journeys', async ({ page }) => {
 await page.goto('/');
 const suite = createV2Scenarios(page, { fixture: fileURLToPath(new URL('./fixtures/review-v3.md', import.meta.url)) });
 const results = await suite.runAll();
 expect(results.length).toBe(13);
 expect(results.every(result => result.result === 'passed')).toBe(true);
});
