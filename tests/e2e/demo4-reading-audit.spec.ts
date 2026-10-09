import {test, expect} from '@playwright/test';
import fs from 'node:fs';
import {demo4Fixture} from './demo4-fixture';

const output = process.env.JUDEX_READING_AUDIT_OUTPUT ?? '.cache/demo4-review3';
fs.mkdirSync(output, {recursive:true});
test.beforeEach(async ({page}) => {
  const state = demo4Fixture(), original = state.taskActivities![0];
  state.taskActivities!.push(...Array.from({length:40}, (_, i) => ({...original,
    id:'history-' + i, text:'第 ' + i + ' 项原始进展，保留材料、职责和核对依据。'.repeat(18),
    createdAt:new Date(Date.UTC(2026, 9, 8, 8, i)).toISOString(),
  })));
  await page.addInitScript(state => localStorage.setItem('judex.web.preview.v1', JSON.stringify(state)), state);
});

for (const surface of ['drawer', 'discussion'] as const) test(`D4-R7 ${surface} retains loaded history and reading position when returning to records`, async ({page}) => {
  if (surface === 'drawer') {
    await page.goto('/projects/leaf/plans/leaf-first/route?task=demo4-review&section=records');
  } else {
    await page.goto('/projects/leaf/plans/leaf-first/chat/main%3Aleaf-first');
    await page.locator('.judex-co-context-task').filter({hasText:'交互评审'}).click();
    await page.getByTestId('workspace-add-tool').click();
    await page.getByRole('menuitem', {name:'任务记录', exact:true}).click();
  }
  const records = page.getByTestId('task-section-records'), scroll = page.locator('.judex-task-scroll');
  await records.getByRole('button', {name:'加载更早记录', exact:true}).click();
  await expect(records.locator('[data-activity]')).toHaveCount(41);
  const original = records.locator('[data-activity="history-39"]').getByRole('button', {name:'展开原始报告', exact:true});
  await original.click();await expect(original).toHaveAttribute('aria-expanded', 'true');
  await original.evaluate(el=>Promise.all(el.closest('.judex-ui-disclosure')!.getAnimations({subtree:true}).map(a=>a.finished.catch(()=>{}))));
  await scroll.evaluate(el => {el.scrollTop = 900;el.dispatchEvent(new Event('scroll'));});
  await expect.poll(() => scroll.evaluate(el => el.scrollTop)).toBe(900);
  const before = await scroll.evaluate(el => ({top:el.scrollTop,height:el.scrollHeight,rows:el.querySelectorAll('[data-activity]').length}));
  await page.screenshot({path:output + '/' + surface + '-history-before.png', fullPage:true});
  if (surface === 'drawer') {
    await page.getByRole('tab', {name:'概览', exact:true}).click();
    await page.getByRole('tab', {name:'任务记录', exact:true}).click();
  } else {
    await page.getByTestId('workspace-tab-task:demo4-review:overview').click();
    await page.getByTestId('workspace-tab-task:demo4-review:records').click();
  }
  await expect(records).toBeVisible();
  const after = await scroll.evaluate(el => ({top:el.scrollTop,height:el.scrollHeight,rows:el.querySelectorAll('[data-activity]').length}));
  fs.writeFileSync(output + '/' + surface + '-history.json', JSON.stringify({before,after}, null, 2));
  await page.screenshot({path:output + '/' + surface + '-history-after.png', fullPage:true});
  await expect(records.locator('[data-activity]')).toHaveCount(41, {timeout:2000});
  await expect(original).toHaveAttribute('aria-expanded', 'true');
  await expect.poll(() => scroll.evaluate(el => el.scrollTop), {timeout:2000}).toBe(900);
});

test('D4-R7 a viewer copies reading state and preserves the original after exit', async ({page}) => {
  await page.goto('/projects/leaf/plans/leaf-first/route?task=demo4-review&section=records');
  const records=page.getByTestId('task-section-records'),scroll=page.locator('.judex-task-scroll');
  await records.getByRole('button', {name:'加载更早记录', exact:true}).click();
  await expect(records.locator('[data-activity]')).toHaveCount(41);
  await scroll.evaluate(el=>{el.scrollTop=900;el.dispatchEvent(new Event('scroll'));});
  await page.getByRole('button', {name:'全屏查看', exact:true}).click();
  const viewer=page.getByTestId('route-fullscreen');await expect(viewer).toBeVisible();
  await expect(records.locator('[data-activity]')).toHaveCount(41);
  await expect.poll(()=>scroll.evaluate(el=>el.scrollTop)).toBe(900);
  await page.getByTestId('task-record-source').click();await page.locator('[data-option-value="web"]').click();
  await expect(records.locator('[data-activity]')).toHaveCount(0);
  await viewer.getByTestId('exit-route-fullscreen').click();
  await expect(page.getByTestId('task-record-source')).toHaveAttribute('data-value', 'all');
  await expect(records.locator('[data-activity]')).toHaveCount(41);
  await expect.poll(()=>scroll.evaluate(el=>el.scrollTop)).toBe(900);
});

test('D4-R7 another task keeps its own history expansion and source filter', async ({page}) => {
  await page.goto('/projects/leaf/plans/leaf-first/route?task=demo4-review&section=records');
  const records=page.getByTestId('task-section-records');
  await records.getByRole('button', {name:'加载更早记录', exact:true}).click();
  await page.getByTestId('task-record-source').click();await page.locator('[data-option-value="cli"]').click();
  await page.getByTestId('close-task-drawer').click();
  await page.getByTestId('execution-task-build').locator('.judex-co-card-hit').click();
  await page.getByRole('tab', {name:'任务记录', exact:true}).click();
  await expect(page.getByTestId('task-record-source')).toHaveAttribute('data-value','all');
  await expect(records.locator('[data-activity]')).toHaveCount(0);
  await page.getByTestId('close-task-drawer').click();
  await page.getByTestId('execution-task-demo4-review').locator('.judex-co-card-hit').click();
  await page.getByRole('tab', {name:'任务记录', exact:true}).click();
  await expect(page.getByTestId('task-record-source')).toHaveAttribute('data-value','cli');
  await expect(records.locator('[data-activity]')).toHaveCount(41);
});
