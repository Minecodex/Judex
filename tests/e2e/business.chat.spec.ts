import {expect, test, type Page} from '@playwright/test';
import {command, registerMember} from './cooperation-business-helpers';
async function enterChat(page: Page, name: string) {
  await registerMember(page, name);
  const project = await command(page, 'post', '/projects', {title: '讨论验收-' + name});
  await page.goto('/projects/' + project.id);
  await expect(page.getByTestId('hub-tab-plans')).toBeVisible();
  await page.getByRole('button', {name: '项目讨论记录', exact: true}).click();
  await expect(page.locator('.judex-collab-overview')).toBeVisible();
  await expect(page.getByTestId('workspace-launcher')).toBeVisible();
  return project;
}
test('W01 project hub and current discussion tools preserve layout and receive live topics', async ({browser}) => {
  const context = await browser.newContext(), page = await context.newPage();
  try {
    const project = await enterChat(page, 'w01');
    await expect(page.getByTestId('work-nav-plans')).toBeVisible();
    const selected = page.locator('.judex-conversation-tab[aria-selected="true"]');await expect(selected).toHaveCount(2);
    const launcher = page.getByTestId('work-nav-plans');await launcher.hover();
    await expect.poll(() => launcher.evaluate(e => getComputedStyle(e).backgroundColor)).toBe('rgb(230, 238, 230)');
    await expect(page.getByTestId('account-menu-button')).toBeVisible();
    const topic = await command(page, 'post', '/projects/' + project.id + '/topics', {title: '外部议题-w01', initialMessage: '外部提交的原始内容'});
    await expect(page.getByTestId('scoped-topic-' + topic.id)).toBeVisible();
    await page.getByTestId('work-nav-resources').click();await expect(page.locator('.judex-material-library-panel')).toBeVisible();
    await page.getByTestId('workspace-back-projects').click();await expect(page.getByTestId('hub-tab-plans')).toBeVisible();
    await expect(page.getByRole('button', {name: '开启一个计划', exact: true}).first()).toBeVisible();
  } finally {await context.close();}
});
test('W02 a real discussion sends once, closes its tab and opens project decisions', async ({browser}) => {
  const context = await browser.newContext(), page = await context.newPage();
  try {
    const project = await enterChat(page, 'w02');
    await page.getByTestId('new-discussion').click();await page.getByTestId('discussion-title').fill('接口稳定性讨论');await page.getByTestId('create-discussion').click();
    await expect(page.getByTestId('work-discussion-input')).toBeVisible();
    await page.getByTestId('work-discussion-input').fill('下单接口幂等缺失，请评估影响。');await page.getByTestId('send-work-message').click();
    await expect(page.getByTestId('chat-thread')).toContainText('下单接口幂等缺失，请评估影响。');
    const topics = (await command(page, 'get', '/projects/' + project.id + '/topics')).items;
    const topic = topics.find((v: any) => v.title === '接口稳定性讨论');
    expect((await command(page, 'get', '/projects/' + project.id + '/topics/' + topic.id + '/messages')).items.filter((m: any) => m.content === '下单接口幂等缺失，请评估影响。')).toHaveLength(1);
    await page.getByRole('button', {name: /关闭标签 · 接口稳定性讨论/}).click();await expect(page.getByTestId('tab-topic:' + topic.id)).toHaveCount(0);
    await page.getByTestId('workspace-back-projects').click();await page.getByTestId('hub-tab-decisions').click();
    await expect(page.getByTestId('hub-tab-decisions')).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByText('当前没有需要你决定的事项', {exact: true})).toBeVisible();
  } finally {await context.close();}
});
