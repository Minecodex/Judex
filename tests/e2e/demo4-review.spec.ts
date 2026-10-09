import {test, expect} from '@playwright/test';
import fs from 'node:fs';
import {demo4Fixture} from './demo4-fixture';
const output = '.cache/demo4-remediation';
fs.mkdirSync(output, {recursive: true});
const material = '11111111-1111-4111-8111-111111111111', source = '22222222-2222-4222-8222-222222222222';

test.beforeEach(async ({page}) => {
  const s = demo4Fixture(), task = s.tasks.find(t => t.id === 'demo4-review')!, flow = s.flows.find(v => v.id === task.flowId)!;
  flow.body = {name: '实际发布流程', instructions: '全局规则必须与材料及交接依据共同核对', nodes: [{id: task.nodeId, name: '评审环节', responsibility: '核对当前事实', allowedPositionIds: [], defaultApprovalPolicy: 'all'}], advisoryEdges: [], approvalPolicies: {}, hardRules: [{nodeId: task.nodeId, kind: 'task_acceptance', phase: 'start', targetId: 'demo4-needs'}, {nodeId: task.nodeId, kind: 'material_ready', phase: 'accept', targetId: material}, {nodeId: task.nodeId, kind: 'handoff_receipt', phase: 'both', targetId: source}, {nodeId: task.nodeId, kind: 'material_ready', phase: 'start', targetId: 'missing-fixed-version'}]};
  flow.nodes = [{id: task.nodeId, label: {zh: '评审环节', en: 'Review step'}, responsibility: {zh: '核对当前事实', en: 'Verify current facts'}}];
  task.files = [{id: material, versionId: material, name: '评审材料.md', text: '固定依据', author: '顾言', at: Date.parse('2026-10-08T10:00:00+08:00')}];
  s.handoffs.push({id: 'review-handoff', projectId: 'leaf', title: {zh: '评审依据交接', en: 'Review evidence handoff'}, taskId: task.id, receiverSeatId: task.reviewerSeatId, flowId: task.flowId, flowVersion: flow.version, stale: false, kind: 'dependency', sources: [{id: source, taskId: 'demo4-needs', senderSeatId: task.seatIds[0], revision: 7, summary: {zh: '已经核对的需求', en: 'Verified requirements'}, files: [], status: 'accepted'}], history: []});
  const delivered = s.tasks.find(t => t.id === 'build')!;
  delivered.status = delivered.businessStatus = 'delivered';
  const topic = {id: 'review-origin', projectId: 'leaf', title: {zh: '核对来源讨论', en: 'Source discussion'}, kind: 'discussion' as const, planIds: ['leaf-first'], taskIds: [task.id], messages: []};
  s.topics.push(topic); s.taskActivities![0].topicIds = [topic.id];
  await page.addInitScript(s => localStorage.setItem('judex.web.preview.v1', JSON.stringify(s)), s);
});

test('R1: published rules retain each target type, phase and global instructions', async ({page}) => {
  await page.goto('/projects/leaf/plans/leaf-first/route?task=demo4-review&section=flow');
  const body = page.getByTestId('task-section-flow');
  await expect(body).toContainText('评审环节'); await expect(body).toContainText('核对当前事实');
  await expect(body).toContainText('全局规则必须与材料及交接依据共同核对');
  await expect(body.locator('[data-rule-kind="task_acceptance"]')).toContainText('需求确认');
  await expect(body.locator('[data-rule-kind="task_acceptance"]')).toContainText('开始前置');
  await expect(body.locator('[data-rule-target="' + material + '"]')).toContainText('评审材料.md');
  await expect(body.locator('[data-rule-target="' + material + '"]')).toContainText('固定材料版本');
  await expect(body.locator('[data-rule-target="' + material + '"]')).toContainText('验收前');
  await expect(body.locator('[data-rule-kind="handoff_receipt"]')).toContainText('评审依据交接');
  await expect(body.locator('[data-rule-kind="handoff_receipt"]')).toContainText('交付 v7');
  await expect(body.locator('[data-rule-kind="handoff_receipt"]')).toContainText('开始与验收前置');
  await expect(body.locator('[data-rule-target="missing-fixed-version"]')).toContainText('依据暂不可用');
  await expect(body.locator('[data-rule-target="missing-fixed-version"]')).toContainText('missing-fixed-version');
  await expect(body.getByTestId('locate-task-plan')).toBeEnabled();
  fs.writeFileSync(output + '/flow-facts.json', JSON.stringify(await body.locator('[data-rule-kind]').allTextContents(), null, 2));
  await page.screenshot({path: output + '/flow-complete.png', fullPage: true});
  await page.locator('.judex-task-scroll').evaluate(el => {el.scrollTop = el.scrollHeight;});
  await page.screenshot({path: output + '/flow-rules-complete.png', fullPage: true});
});

test('R1/R2: locating a plan from a task opens the shared read-only viewer and returns to the task', async ({page}) => {
  await page.goto('/projects/leaf?tab=deliveries&view=task&item=build&section=flow');
  const button = page.getByTestId('locate-task-plan'); await button.click();
  const viewer = page.getByTestId('route-fullscreen'); await expect(viewer).toBeVisible();
  await expect(viewer.getByTestId('task-section-overview')).toBeVisible();
  await expect(viewer.getByTestId('execution-task-build')).toHaveClass(/judex-task-card-selected/);
  await expect(viewer.getByRole('button', {name: '进入计划', exact: true})).toBeVisible();
  await expect(viewer.getByTestId('accept-task')).toHaveCount(0);
  await viewer.getByTestId('exit-route-fullscreen').click(); await expect(viewer).toHaveCount(0);
  await expect(page.getByTestId('task-section-flow')).toBeVisible(); await expect(button).toBeFocused();
});

test('R2: preview source navigation exits the viewer and preserves the original discussion draft', async ({page}) => {
  await page.goto('/projects/leaf/plans/leaf-first/chat/main%3Aleaf-first');
  const input = page.locator('.judex-chat-input-wrap textarea'); await input.fill('保留当前讨论未发送内容');
  await page.getByRole('button', {name: '预览路线', exact: true}).click();
  await expect(page.getByTestId('route-fullscreen')).toBeVisible();
  await page.getByTestId('execution-task-demo4-review').locator('.judex-co-card-hit').click();
  await page.getByRole('tab', {name: '任务记录', exact: true}).click();
  await page.getByTestId('task-section-records').getByRole('button', {name: '核对来源讨论', exact: true}).click();
  await expect(page).toHaveURL(/review-origin/); await expect(page.getByTestId('route-fullscreen')).toHaveCount(0);
  await expect(page.locator('.judex-co-conversation-title').getByRole('heading', {name: '核对来源讨论', exact: true})).toBeVisible();
  await expect.poll(() => page.evaluate(() => !!document.fullscreenElement)).toBe(false);
  await page.screenshot({path: output + '/preview-source-visible.png', fullPage: true});
  fs.writeFileSync(output + '/preview-source.json', JSON.stringify({url: page.url(), previewStillOpen: false}));
  await page.goBack(); await expect(input).toHaveValue('保留当前讨论未发送内容');
});

test('R2: workflow settings from a preview exit before showing the source', async ({page}) => {
  await page.goto('/projects/leaf/plans/leaf-first/chat/main%3Aleaf-first');
  await page.getByRole('button', {name: '预览路线', exact: true}).click();
  await page.getByTestId('execution-task-demo4-review').locator('.judex-co-card-hit').click();
  await page.getByRole('tab', {name: '流程参考', exact: true}).click();
  await page.getByTestId('task-section-flow').getByRole('button', {name: '查看流程设置'}).click();
  await expect(page.getByTestId('route-fullscreen')).toHaveCount(0); await expect(page).toHaveURL(/settings\/flows/);
});

test('R2: a task source in the discussion opens the shared inspector without changing the conversation or draft', async ({page}) => {
  await page.goto('/projects/leaf/plans/leaf-first/chat/main%3Aleaf-first');
  await page.getByTestId('work-discussion-input').fill('保留本次发言范围和草稿');
  await page.locator('.judex-co-context-task').filter({hasText: '交互评审'}).click();
  await page.getByTestId('workspace-add-tool').click();
  await page.getByRole('menuitem', {name: '流程参考', exact: true}).click();
  await page.getByTestId('task-section-flow').locator('[data-rule-kind="task_acceptance"]').getByRole('button').click();
  await expect(page.getByTestId('task-section-overview')).toBeVisible();
  await expect(page.getByTestId('task-inspector')).toContainText('需求确认');
  await expect(page).toHaveURL(/chat\/main%3Aleaf-first/);
  await expect(page.getByTestId('work-discussion-input')).toHaveValue('保留本次发言范围和草稿');
});

test('R3: route drawer close and keyboard Escape restore the task trigger', async ({page}) => {
  await page.goto('/projects/leaf/plans/leaf-first/route');
  const hit = page.getByTestId('execution-task-demo4-review').locator('.judex-co-card-hit');
  await hit.click(); await page.getByTestId('close-task-drawer').click();
  await expect(page.getByTestId('task-details-drawer')).toHaveCount(0); await expect(hit).toBeFocused();
  await page.keyboard.press('Enter'); await expect(page.getByTestId('task-details-drawer')).toBeVisible();
  await page.keyboard.press('Escape'); await expect(page.getByTestId('task-details-drawer')).toHaveCount(0); await expect(hit).toBeFocused();
  fs.writeFileSync(output + '/drawer-focus.json', JSON.stringify(await hit.evaluate(el => ({focused: document.activeElement === el, key: el.getAttribute('data-focus-key')}))));
});

test('R3: fullscreen reparenting retains the drawer opener, then restores the fullscreen trigger', async ({page}) => {
  await page.goto('/projects/leaf/plans/leaf-first/route');
  const hit = page.getByTestId('execution-task-demo4-review').locator('.judex-co-card-hit'); await hit.click();
  await page.getByRole('button', {name: '全屏查看', exact: true}).click(); await expect(page.getByTestId('route-fullscreen')).toBeVisible();
  await page.getByTestId('close-task-drawer').click(); await expect(hit).toBeFocused();
  await page.getByTestId('exit-route-fullscreen').click(); await expect(page.getByTestId('route-fullscreen')).toHaveCount(0);
  await expect(page.getByRole('button', {name: '全屏查看', exact: true})).toBeFocused();
  await expect(page.getByTestId('task-details-drawer')).toBeVisible();
});

test('R3: delivery drawer close and Escape restore the delivery trigger', async ({page}) => {
  await page.goto('/projects/leaf?tab=deliveries');
  const trigger = page.getByTestId('delivery-card-build').getByRole('button', {name: '查看原记录'});
  await trigger.click(); await expect(page.getByTestId('task-details-drawer')).toBeVisible();
  await page.getByTestId('close-task-drawer').click(); await expect(trigger).toBeFocused();
  await page.keyboard.press('Enter'); await expect(page.getByTestId('task-details-drawer')).toBeVisible();
  await page.keyboard.press('Escape'); await expect(page.getByTestId('task-details-drawer')).toHaveCount(0); await expect(trigger).toBeFocused();
});

for (const width of [1120, 1440, 1920]) for (const locale of ['zh-CN', 'en']) for (const theme of ['light', 'dark']) test(`R1 visual rules ${width} ${locale} ${theme}`, async ({page}) => {
  await page.setViewportSize({width, height: 1000});
  await page.addInitScript(({locale, theme}) => {localStorage.setItem('argus.locale', locale); localStorage.setItem('judex.theme', theme);}, {locale, theme});
  await page.goto('/projects/leaf/plans/leaf-first/route?task=demo4-review&section=flow');
  await expect(page.getByTestId('task-section-flow')).toBeVisible();
  const rules = page.locator('[data-rule-kind="handoff_receipt"]');
  await expect(rules).toContainText(locale === 'en' ? 'Before start and acceptance' : '开始与验收前置');
  await page.locator('.judex-task-scroll').evaluate(el => {el.scrollTop = el.scrollHeight;});
  await expect(page.getByTestId('locate-task-plan')).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const suffix = width + '-' + locale + '-' + theme;
  await page.screenshot({path: output + '/flow-rules-' + suffix + '.png', fullPage: true, animations: 'disabled'});
  await page.getByRole('button', {name: locale === 'en' ? 'View fullscreen' : '全屏查看', exact: true}).click();
  await expect(page.getByTestId('route-fullscreen')).toBeVisible();
  await page.locator('.judex-task-scroll').evaluate(el => {el.scrollTop = el.scrollHeight;});
  await page.screenshot({path: output + '/flow-fullscreen-' + suffix + '.png', fullPage: true, animations: 'disabled'});
  await page.keyboard.press('Escape'); await expect(page.getByTestId('route-fullscreen')).toHaveCount(0);
});
