import {test, expect} from '@playwright/test';
import {execFileSync} from 'node:child_process';
import fs from 'node:fs';
import {chooseValue, openSettings, selectPerson} from './workspace-helpers';

test('card border pixels stay continuous at fractional display scales in both themes', async ({browser}, info) => {
  test.setTimeout(180000);
  for (const scale of [1, 1.25, 1.5, 1.75, 2]) for (const theme of ['light', 'dark']) {
    const context = await browser.newContext({viewport: {width:1440, height:1100}, deviceScaleFactor: scale});
    try {
      const page = await context.newPage();
      await page.addInitScript(theme => localStorage.setItem('judex.theme', theme), theme);
      await page.goto('/?project=leaf'); await selectPerson(page, '林然'); await openSettings(page, 'team');
      await page.getByTestId('open-position-presets').click(); await chooseValue(page, 'position-preset-scenario', 'software');
      await page.mouse.move(0, 0);
      const card = page.getByTestId('preset-card-product-manager');
      await expect.poll(() => card.evaluate(element => getComputedStyle(element).borderColor)).toBe(theme === 'light' ? 'rgb(204, 214, 203)' : 'rgb(74, 93, 78)');
      const metadata = await page.locator('.judex-preset-grid .checkbox__content').evaluateAll(elements => elements.map(element => {
        const rect = element.getBoundingClientRect(), css = getComputedStyle(element);
        return {x:rect.x, y:rect.y, width:rect.width, height:rect.height, border:css.borderColor, background:css.backgroundColor};
      }));
      const screenshot = info.outputPath(`border-${theme}-${scale}.png`), geometry = info.outputPath(`border-${theme}-${scale}.json`);
      await page.screenshot({path:screenshot, animations:'disabled'}); fs.writeFileSync(geometry, JSON.stringify({scale, boxes:metadata}));
      execFileSync(process.env.JUDEX_PYTHON ?? 'python', ['tests/e2e/border-pixels.py', screenshot, geometry], {windowsHide:true});
      const original = await card.evaluate(element => getComputedStyle(element).borderColor);
      await card.hover();
      await expect.poll(() => card.evaluate(element => getComputedStyle(element).borderColor)).toBe(theme === 'light' ? 'rgb(45, 102, 81)' : 'rgb(159, 197, 170)');
      expect(original).not.toBe(await card.evaluate(element => getComputedStyle(element).borderColor));
      // The whole visible card, including padding, must toggle the checkbox.
      await card.click({position: {x:8, y:25}}); await expect(card.getByRole('checkbox')).toBeChecked();
      await page.mouse.move(0,0); await expect.poll(() => card.evaluate(element => getComputedStyle(element).borderColor)).toBe(theme === 'light' ? 'rgb(45, 102, 81)' : 'rgb(159, 197, 170)');
    } finally {await context.close();}
  }
});
