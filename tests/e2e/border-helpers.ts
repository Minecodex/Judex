import {expect, type Locator, type Page} from '@playwright/test';

export async function assertVisibleSurfaceBorders(page: Page) {
  const failures = await page.evaluate(() => {
    const failures: string[] = [];
    const activeModal=Array.from(document.querySelectorAll('[data-slot="modal-dialog"]')).filter(e=>e.getClientRects().length).at(-1);
    for (const element of document.querySelectorAll('.judex-ui-card,.judex-position-card,.judex-task-card,.judex-project-card,.judex-checkbox-card .checkbox__content,.judex-ui-select .select__trigger,.button--outline,.judex-input')) {
      const rect = element.getBoundingClientRect(), css = getComputedStyle(element);
      if(element.closest('[inert],[aria-hidden="true"]')||activeModal&&!activeModal.contains(element))continue;
      if (rect.width < 30 || rect.height < 24 || css.visibility === 'hidden' || css.pointerEvents === 'none' || css.borderTopStyle !== 'solid' || parseFloat(css.borderTopWidth) < 1) continue;
      let contained = rect.top >= 0 && rect.left >= 0 && rect.right <= innerWidth && rect.bottom <= innerHeight;
      for (let parent = element.parentElement; parent && contained; parent = parent.parentElement) {
        const style = getComputedStyle(parent);
        if (style.overflowX !== 'visible' || style.overflowY !== 'visible') {
          const clip = parent.getBoundingClientRect();
          contained = rect.top >= clip.top && rect.bottom <= clip.bottom && rect.left >= clip.left && rect.right <= clip.right;
        }
      }
      if (!contained) continue;
      for (const [x,y] of [[rect.left + rect.width / 2, rect.top + .5], [rect.left + rect.width / 2, rect.bottom - .5], [rect.left + .5, rect.top + rect.height / 2], [rect.right - .5, rect.top + rect.height / 2]]) {
        const top = document.elementFromPoint(x,y);
        if (top && !element.contains(top)) failures.push(element.className + ' covered by ' + top.className);
      }
    }
    return failures;
  });
  expect(failures).toEqual([]);
}

export async function waitForBorderColor(elements: Locator, color: string) {
  const count = await elements.count();
  expect(count).toBeGreaterThan(0);
  // Screenshots finish CSS transitions. Measure every box after hover/focus
  // colors settle so geometry and pixels describe the same visual state.
  await expect.poll(() => elements.evaluateAll(boxes =>
    boxes.map(box => getComputedStyle(box).borderColor),
  )).toEqual(Array(count).fill(color));
}
