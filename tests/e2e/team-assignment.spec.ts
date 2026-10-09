import {test, expect} from '@playwright/test';
import {chooseValue, openSettings, selectPerson} from './workspace-helpers';

test('positions own member assignment and card actions remain compact', async ({page}, info) => {
  await page.goto('/?project=leaf'); await selectPerson(page, '林然'); await openSettings(page, 'team');
  await expect(page.locator('.judex-project-assignment')).toHaveCount(0);
  const card = page.getByTestId('position-card-build-role');
  await expect(card).toBeVisible();
  for (const button of await card.locator('.judex-card-actions button').all()) {
    const size = await button.evaluate(element => ({height: element.getBoundingClientRect().height, font: getComputedStyle(element).fontSize}));
    expect(size.height).toBeGreaterThanOrEqual(32); expect(size.height).toBeLessThanOrEqual(34); expect(size.font).toBe('13px');
  }
  const before = await page.evaluate(() => JSON.parse(localStorage.getItem('judex.web.preview.v1')!));
  await card.getByTestId('assign-position-build-role').click();
  await expect(page.getByRole('dialog').getByRole('checkbox')).toHaveCount(0);
  await expect(page.getByTestId('assign-existing-position')).toBeDisabled();
  await chooseValue(page, 'position-assignment-member', '林然');
  await page.getByTestId('assign-existing-position').click();
  await expect(page.getByRole('dialog')).toBeHidden();
  const after = await page.evaluate(() => JSON.parse(localStorage.getItem('judex.web.preview.v1')!));
  expect(after.seats).toHaveLength(before.seats.length + 1);
  expect(after.seats.slice(0, before.seats.length)).toEqual(before.seats);
  expect(after.seats.at(-1)).toMatchObject({positionId: 'build-role', person: '林然'});
  expect(after.tasks).toEqual(before.tasks); expect(after.flows).toEqual(before.flows);
  await expect(card.locator('.judex-position-holders')).toContainText('林然');
  await page.reload(); await expect(page.getByTestId('position-card-build-role').locator('.judex-position-holders')).toContainText('林然');
  await page.screenshot({path: info.outputPath('position-assignment.png'), fullPage: true, animations: 'disabled'});
});
