import {test, expect} from '@playwright/test';
import fs from 'node:fs';
import {execFileSync} from 'node:child_process';

test('both selected tab borders remain opaque and complete at display scales', async ({browser}, info) => {
  test.setTimeout(120000);
  for (const theme of ['light', 'dark']) for (const scale of [1, 1.25, 1.75, 2]) {
    const context = await browser.newContext({viewport:{width:1440,height:1000},deviceScaleFactor:scale});
    try {
      const page = await context.newPage(); await page.addInitScript(theme => localStorage.setItem('judex.theme',theme),theme);
      await page.goto('/projects/leaf/chat/labels?view=workspace'); await expect(page.getByTestId('workspace-launcher')).toBeVisible(); await page.mouse.move(0,0);
      const tabs = page.locator('.judex-conversation-tab[aria-selected="true"]'); await expect(tabs).toHaveCount(2);
      const boxes = await tabs.evaluateAll(elements => elements.map(element => {
        const rect=element.getBoundingClientRect(),css=getComputedStyle(element),parent=element.parentElement!.getBoundingClientRect();
        return {x:rect.x,y:rect.y,width:rect.width,height:rect.height,border:css.borderColor,background:css.backgroundColor,
          parentTop:parent.top,parentBottom:parent.bottom,bottomWidth:css.borderBottomWidth};
      }));
      for (const box of boxes) {
        expect(box.border).toBe(theme==='light'?'rgb(45, 102, 81)':'rgb(159, 197, 170)'); expect(box.bottomWidth).toBe('1px');
        expect(box.y-box.parentTop).toBeGreaterThanOrEqual(2); expect(box.parentBottom-box.y-box.height).toBeGreaterThanOrEqual(2);
      }
      const screenshot=info.outputPath(`tabs-${theme}-${scale}.png`),metadata=info.outputPath(`tabs-${theme}-${scale}.json`);
      await page.screenshot({path:screenshot,animations:'disabled'});fs.writeFileSync(metadata,JSON.stringify({scale,boxes}));
      execFileSync(process.env.JUDEX_PYTHON??'python',['tests/e2e/border-pixels.py',screenshot,metadata],{windowsHide:true});
    } finally {await context.close();}
  }
});

test('workspace feature buttons visibly respond to hover, press and keyboard focus', async ({page},info) => {
  for(const theme of ['light','dark']) {
    await page.goto('/projects/leaf/chat/labels?view=workspace');await page.evaluate(theme=>localStorage.setItem('judex.theme',theme),theme);await page.reload();
    await expect(page.getByTestId('workspace-launcher')).toBeVisible();
    const accent=theme==='light'?'rgb(45, 102, 81)':'rgb(159, 197, 170)';
    for(const id of ['plans','resources']) {
      const item=page.getByTestId('workspace-launcher').getByTestId('work-nav-'+id);await page.mouse.move(0,0);
      await expect.poll(()=>item.evaluate(element=>getComputedStyle(element).borderColor)).toBe(theme==='light'?'rgb(204, 214, 203)':'rgb(74, 93, 78)');
      await item.hover();await expect.poll(()=>item.evaluate(element=>getComputedStyle(element).borderColor)).toBe(accent);
      await expect.poll(()=>item.evaluate(element=>getComputedStyle(element).backgroundColor)).toBe(theme==='light'?'rgb(230, 238, 230)':'rgb(44, 64, 50)');
    }
    await page.screenshot({path:info.outputPath('launcher-hover-'+theme+'.png'),animations:'disabled'});
    const item=page.getByTestId('workspace-launcher').getByTestId('work-nav-plans');await item.hover();await page.mouse.down();
    await expect.poll(()=>item.evaluate(element=>getComputedStyle(element).borderColor)).toBe(theme==='light'?'rgb(61, 107, 81)':'rgb(176, 206, 184)');
    await page.mouse.up();await expect(page.getByTestId('workspace-tab-plans')).toBeVisible();
    await page.goto('/projects/leaf/chat/labels?view=workspace');await expect(page.getByTestId('workspace-launcher')).toBeVisible();await page.getByTestId('workspace-launcher').getByTestId('work-nav-plans').focus();await page.keyboard.press('Tab');
    await expect(page.getByTestId('workspace-launcher').getByTestId('work-nav-resources')).toBeFocused();
    expect(await page.getByTestId('workspace-launcher').getByTestId('work-nav-resources').evaluate(element=>getComputedStyle(element).outlineStyle)).toBe('solid');
    await page.keyboard.press('Enter');await expect(page.getByTestId('workspace-tab-resources')).toBeVisible();
  }
});
