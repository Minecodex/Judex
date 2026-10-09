import {test, expect} from '@playwright/test';
import {chooseValue, openSettings, selectPerson} from './workspace-helpers';

test('position preset modal selects a scenario, imports only selected cards and keeps project edits', async ({page}, info) => {
  await page.goto('/?project=leaf');
  await selectPerson(page, '林然');
  await openSettings(page, 'team');
  await page.getByTestId('open-position-presets').click();
  const dialog = page.getByRole('dialog', {name: '添加职位模板'});
  await expect(dialog).toBeVisible();
  await expect(page.getByTestId('import-position-presets')).toBeDisabled();
  await chooseValue(page, 'position-preset-scenario', 'software');
  await expect(dialog.locator('[data-testid^="preset-card-"]')).toHaveCount(8);
  await page.getByTestId('preset-card-frontend-developer').click();
  await page.getByTestId('preset-card-devops-engineer').click();
  await expect(page.getByTestId('preset-selection-count')).toHaveText('已选择 2 个职位');
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await page.getByTestId('open-position-presets').click();
  await chooseValue(page, 'position-preset-scenario', 'software');
  await page.getByTestId('preset-card-frontend-developer').click();
  await page.getByTestId('preset-card-devops-engineer').click();
  await page.screenshot({path: info.outputPath('preset-selection.png'), fullPage: true});
  await page.getByTestId('import-position-presets').click();
  await expect(dialog).toBeHidden();
  const positions = await page.evaluate(() => JSON.parse(localStorage.getItem('judex.web.preview.v1')!).positions);
  const added = positions.filter((position: {projectId: string; presetId?: string}) => position.projectId === 'leaf' && position.presetId);
  expect(added.map((position: {presetId: string}) => position.presetId).sort()).toEqual(['devops-engineer', 'frontend-developer']);
  await page.reload();
  await page.getByTestId('edit-position-' + added[0].id).click();
  await page.getByTestId('position-name').fill('本项目前端职责');
  await page.getByTestId('position-prompt').fill('独立职责内容');
  await page.getByTestId('save-position').click();
  await page.getByTestId('open-position-presets').click();
  await chooseValue(page, 'position-preset-scenario', 'software');
  await expect(page.getByTestId('preset-card-frontend-developer').getByRole('checkbox')).toBeDisabled();
  await page.getByTestId('preset-card-qa-engineer').click();
  await chooseValue(page, 'position-preset-scenario', 'publishing');
  await expect(page.getByTestId('preset-selection-count')).toHaveText('已选择 0 个职位');
  await page.getByTestId('preset-select-all').click();
  await expect(page.getByTestId('preset-selection-count')).toHaveText('已选择 7 个职位');
  await page.getByTestId('preset-select-all').click();
  await expect(page.getByTestId('import-position-presets')).toBeDisabled();
});

test('position preset cards and sticky modal actions fit desktop widths in both themes and languages', async ({page}, info) => {
  test.setTimeout(120000);
  await page.goto('/?project=leaf');
  await selectPerson(page, '林然');
  for (const locale of ['zh-CN', 'en']) for (const theme of ['light', 'dark']) for (const width of [1120, 1280, 1440, 1920, 3840]) {
    await page.evaluate(({locale, theme}) => {localStorage.setItem('argus.locale', locale); localStorage.setItem('judex.theme', theme);}, {locale, theme});
    await page.setViewportSize({width, height: width === 3840 ? 1926 : 900});
    await page.goto('/?project=leaf&settings=team');
    await page.getByTestId('open-position-presets').click();
    await chooseValue(page, 'position-preset-scenario', 'software');
    const geometry = await page.getByRole('dialog').evaluate(element => {
      const rect = element.getBoundingClientRect();
      const footer = element.querySelector('.judex-dialog-footer')!.getBoundingClientRect();
      const overflowing = [...element.querySelectorAll('*')].filter(child => child.getBoundingClientRect().right > rect.right + 1).map(child => ({tag: child.tagName, class: child.className, width: child.getBoundingClientRect().width, right: child.getBoundingClientRect().right, position: getComputedStyle(child).position})).slice(0, 6);
      return {left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom, width: element.clientWidth, scrollWidth: element.scrollWidth, pageWidth: document.documentElement.clientWidth, pageScrollWidth: document.documentElement.scrollWidth, footerBottom: footer.bottom, height: innerHeight, overflowing};
    });
    if (geometry.scrollWidth > geometry.width) console.log(JSON.stringify(geometry));
    expect(geometry.left).toBeGreaterThanOrEqual(0); expect(geometry.right).toBeLessThanOrEqual(width);
    expect(geometry.width).toBeGreaterThanOrEqual(700);
    expect(geometry.top).toBeGreaterThanOrEqual(0); expect(geometry.bottom).toBeLessThanOrEqual(geometry.height);
    expect(geometry.scrollWidth).toBeLessThanOrEqual(geometry.width);
    expect(geometry.pageScrollWidth).toBeLessThanOrEqual(geometry.pageWidth);
    expect(geometry.footerBottom).toBeLessThanOrEqual(geometry.height);
    await expect(page.getByTestId('import-position-presets')).toBeVisible();
    if (width === 1440) await page.screenshot({path: info.outputPath(`presets-${locale}-${theme}.png`), fullPage: true});
    await page.keyboard.press('Escape');
  }
});
