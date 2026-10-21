/** Shared acceptance scenarios: receives Playwright's Page or the CUA tab.playwright facade. */
export function createV2Scenarios(ui, { fixture } = {}) {
 const results = [];
 const button = (name) => ui.getByRole('button', { name, exact: true });
 const dialog = (name) => ui.getByRole('dialog', { name, exact: true });
 const room = (name) => ui.getByRole('region', { name, exact: true });
 const sidebar = () => ui.locator('.judex-v2-sidebar');
 const assert = (condition, message) => { if (!condition) throw new Error(message); };
 const check = async (name, action) => { await action(); results.push({ name, result: 'passed' }); };
 const close = async (name) => dialog(name).getByRole('button', { name: '关闭', exact: true }).first().click();
 const say = async (content, isDM = false) => { await ui.getByRole('textbox', { name: isDM ? '发送消息…' : '聊聊你的想法，或 @ 一位伙伴…', exact: true }).fill(content); await button('发送消息').click(); };
 const openGroupSettings = async (name) => room(name).getByRole('button', { name, exact: true }).click();
 const aiCount = () => ui.getByTestId('ai-message').count();
 const phases = {
  async start() {
   await check('项目卡片入口与按需文件阅读', async () => {
    await button('关于这个 Demo').first().click(); await button('重置示例数据').click();
    assert(await ui.locator('.judex-v2-project-card').count() === 3, 'Expected three project cards');
    await ui.getByTestId('project-judex').click();
    assert(await ui.locator('.judex-v2-reader').count() === 0, 'Reader must be closed initially');
    await button('打开文件 · 工作区体验提案 v2').click();
    assert(await ui.locator('.judex-v2-reader .judex-v2-document').count() === 1, 'Document did not open beside chat');
    await button('版本记录').click();
    await ui.locator('.judex-v2-version-history').getByRole('button').filter({ hasText: '第一版' }).click();
    assert(!(await ui.locator('.judex-v2-reader').innerText()).includes('AI 有岗位，人来定方向'), 'Historical v1 should not include v2 additions');
   });
   await check('分享固定版本，目标群只能读取已分享版本', async () => {
    await button('分享到群聊').click();
    await dialog('分享到群聊').getByRole('button', { name: '分享到群聊', exact: true }).click();
    await button('关闭阅读区').click(); await ui.getByTestId('group-chat').click();
    await button('打开文件 · 工作区体验提案 v1').click(); await button('版本记录').click();
    assert(await ui.locator('.judex-v2-version-history').getByRole('button').count() === 1, 'Shared recipient must not see unshared versions');
    await button('版本记录').click(); await button('关闭阅读区').click();
    await room('聊天与文件').getByRole('button', { name: '群聊文件', exact: true }).first().click();
    await button('从文件列表移除 · 工作区体验提案').click();
    assert(await ui.locator('.judex-v2-file-list-row').filter({ hasText: '工作区体验提案' }).count() === 0, 'Removed file is still in library');
    await close('群聊文件'); await button('打开文件 · 工作区体验提案 v1').click();
    assert(await ui.locator('.judex-v2-reader').count() === 1, 'Historical reference must remain readable after removal');
    await button('关闭阅读区').click();
   });
  },
  async files() {
   if (!fixture) return;
   await check('显式更新文件版本，历史消息保留原版；同名上传生成独立文件', async () => {
    await ui.getByTestId('group-workspace').click();
    await room('工作区体验').getByRole('button', { name: '群聊文件', exact: true }).first().click();
    let chooser = ui.waitForEvent('filechooser'); await button('更新版本 · 工作区体验提案').click(); await (await chooser).setFiles(fixture);
    await ui.locator('.judex-v2-file-list-open').filter({ hasText: '工作区体验提案' }).click();
    assert((await ui.locator('.judex-v2-reader').innerText()).includes('V3 文件版本验收'), 'Updated content did not render');
    await button('关闭阅读区').click(); await button('打开文件 · 工作区体验提案 v2').click();
    assert((await ui.locator('.judex-v2-reader').innerText()).includes('AI 有岗位，人来定方向'), 'Original message lost v2');
    await button('关闭阅读区').click();
    await room('工作区体验').getByRole('button', { name: '群聊文件', exact: true }).first().click();
    for (let i = 0; i < 2; i++) { chooser = ui.waitForEvent('filechooser'); await dialog('群聊文件').getByRole('button', { name: '添加文件', exact: true }).click(); await (await chooser).setFiles(fixture); }
    assert(await ui.locator('.judex-v2-file-list-row').filter({ hasText: 'review-v3.md' }).count() === 2, 'Same-name uploads must be independent files');
    await close('群聊文件');
   });
  },
  async messages() {
   await check('自由建群、可选目的和零工作流；普通消息与 @ 人不触发 AI', async () => {
    await button('创建群聊').click(); await ui.getByTestId('group-name').fill('E2E · 自由讨论');
    await dialog('创建群聊').getByRole('button', { name: '创建群聊', exact: true }).click();
    assert(await room('E2E · 自由讨论').count() === 1, 'Group was not created');
    await say('普通网页消息只保存，不触发 AI。'); assert(await aiCount() === 0, 'Ordinary message triggered AI');
    await say('@陈墨 请看一下这个想法。'); assert(await aiCount() === 0, 'Human mention triggered AI');
   });
   await check('明确 @ 岗位 AI 仅唤醒指定身份', async () => {
    await button('提及成员或岗位 AI').click(); await button('陈墨 工程师 AI 陈墨 · 岗位 AI').click();
    await say('@陈墨·工程师 AI 请从实现边界看看这个想法。'); assert(await aiCount() === 1, 'Expected exactly the mentioned AI to reply');
   });
   await check('本地只记录不触发，普通推送唤醒本人所有本群岗位', async () => {
    await button('模拟本地推送').click(); await ui.getByRole('checkbox', { name: '本次只记录 开启后只保存内容，不唤醒岗位 AI。', exact: true }).check(); await button('推送到当前群').click();
    assert(await aiCount() === 1, 'Record-only push triggered AI');
    await openGroupSettings('E2E · 自由讨论'); await dialog('群聊设置').getByRole('button', { name: '成员 2', exact: true }).click();
    await ui.locator('.judex-v2-member-config-row').filter({ hasText: '林舟' }).getByRole('button', { name: '产品负责人', exact: true }).click();
    await button('保存修改').click(); await button('模拟本地推送').click(); await button('推送到当前群').click();
    assert(await aiCount() === 3, 'Local push must wake the sender’s two roles, with no other role');
   });
   await check('群公共知识记录工具来源，切换岗位保持公共背景相同', async () => {
    await room('E2E · 自由讨论').getByRole('button', { name: '群公共知识', exact: true }).first().click();
    const shared = await ui.locator('.judex-v2-prompt-layers>div').allTextContents();
    await button('林舟·产品设计师 AI 查看岗位的组合提示词').click(); await ui.getByRole('option', { name: '陈墨·工程师 AI', exact: true }).click();
    const other = await ui.locator('.judex-v2-prompt-layers>div').allTextContents();
    assert(JSON.stringify(shared.slice(0, 3)) === JSON.stringify(other.slice(0, 3)), 'Shared prompt background diverged by role');
    assert(other[4].includes('他人的个人提示词不在此处展示'), 'Private prompt leaked');
    assert((await dialog('群公共知识').innerText()).includes('工具结果'), 'Tool provenance is missing');
    await close('群公共知识');
   });
  },
  async hierarchy() {
   await check('多个同级子群与三层上限，子群独立配置', async () => {
    for (const [parent, name] of [['E2E · 自由讨论', 'E2E · 子群'], ['E2E · 子群', 'E2E · 第三层']]) {
     await openGroupSettings(parent); await button('创建子群').click(); await ui.getByTestId('group-name').fill(name); await dialog('创建群聊').getByRole('button', { name: '创建群聊', exact: true }).click();
    }
    await openGroupSettings('E2E · 第三层'); assert(!(await button('创建子群').isEnabled()), 'A fourth level must be blocked'); await button('取消').last().click();
    await sidebar().getByRole('button', { name: 'E2E · 自由讨论', exact: true }).click(); await openGroupSettings('E2E · 自由讨论'); await button('创建子群').click(); await ui.getByTestId('group-name').fill('E2E · 同级子群'); await dialog('创建群聊').getByRole('button', { name: '创建群聊', exact: true }).click();
    assert(await sidebar().getByRole('button', { name: 'E2E · 子群', exact: true }).count() === 1, 'First sibling disappeared');
    assert(await sidebar().getByRole('button', { name: 'E2E · 同级子群', exact: true }).count() === 1, 'Second sibling missing');
   });
   await check('父群级联归档全部子孙群，归档后仍可讨论', async () => {
    await sidebar().getByRole('button', { name: 'E2E · 自由讨论', exact: true }).click(); await openGroupSettings('E2E · 自由讨论'); await button('归档群聊').click(); await button('归档').click();
    for (const name of ['E2E · 自由讨论', 'E2E · 子群', 'E2E · 第三层', 'E2E · 同级子群']) assert(await sidebar().getByRole('button', { name, exact: true }).count() === 1, `Archived descendant missing: ${name}`);
    await say('归档之后仍然可以继续讨论。'); assert(await ui.getByText('归档之后仍然可以继续讨论。', { exact: true }).count() > 0, 'Archived chat became read-only');
    await button('全部').click();
   });
  },
  async loops() {
   await check('循环保护：带意见继续、空意见继续、终止与重新 @', async () => {
    await ui.getByTestId('group-agents').click();
    await ui.getByRole('textbox', { name: '可以给一点意见，也可以让它们继续…', exact: true }).fill('优先保证阅读清晰，再讨论层级展示。');
    await button('继续讨论').click(); assert(await ui.getByTestId('discussion-card').count() === 0, 'Pause card did not resolve');
    assert(await ui.getByText('已继续讨论', { exact: true }).count() === 1, 'Continue state missing');
    await button('终止本次讨论').click(); assert(await ui.getByText('本次 AI 讨论已终止', { exact: true }).count() === 1, 'Stop state missing');
    const before = await aiCount(); await say('@陈墨·工程师 AI 这是一段新的讨论。'); assert(await aiCount() === before + 1, 'Terminating a discussion permanently disabled future mentions');
    await button('关于这个 Demo').first().click(); await button('体验 10 轮讨论暂停').click(); await button('继续讨论').click();
    assert(await ui.getByText('已继续讨论', { exact: true }).count() === 1, 'Blank feedback must allow continuation');
   });
  },
  async dm() {
   await check('平台级人类私聊、附件阅读与跨项目复用', async () => {
    await ui.getByRole('navigation').getByRole('button', { name: '聊天', exact: true }).click();
    assert(await button('群公共知识').count() === 0, 'Private chat exposed group AI knowledge');
    await say('@陈墨·工程师 AI 这条私聊也不触发机器人。', true); assert(await aiCount() === 0, 'Private chat triggered AI');
    await button('打开文件 · 一点设计灵感.md v1').click(); assert(await ui.locator('.judex-v2-reader').count() === 1, 'DM file did not open'); await button('关闭阅读区').click();
    await button('发起聊天').click(); await ui.getByRole('textbox', { name: '搜索平台用户…', exact: true }).fill('唐可'); await button('开始聊天 · 唐可').click(); await say('跨项目共用这一段私聊。', true);
    await ui.getByRole('navigation').getByRole('button', { name: '项目空间', exact: true }).click(); await ui.getByTestId('project-brand').click(); await ui.getByRole('navigation').getByRole('button', { name: '聊天', exact: true }).click();
    assert((await room('唐可').innerText()).includes('跨项目共用这一段私聊。'), 'Private history changed when switching projects');
   });
  },
  async preferences() {
   await check('深浅配色、中英文与桌面布局', async () => {
    await button('切换深浅配色').click(); await button('切换语言').click();
    assert(await ui.getByRole('textbox', { name: 'Write a message…', exact: true }).count() === 1, 'English did not apply');
    const layout = await ui.evaluate(() => ({ dark: document.documentElement.dataset.theme === 'dark', overflow: document.body.scrollWidth > innerWidth }));
    assert(layout.dark && !layout.overflow, 'Dark theme or desktop layout failed');
    await button('Toggle appearance').click(); await button('Switch language').click();
   });
  },
  async settings() {
   await check('项目配置、岗位和个人提示词保存，示例邀请可用于组群', async () => {
    await ui.getByRole('navigation').getByRole('button', { name: '群聊', exact: true }).click();
    await sidebar().getByRole('button', { name: '项目设置', exact: true }).click();
    await ui.getByRole('textbox', { name: '项目描述', exact: true }).fill('E2E 配置保存后的项目背景。');
    await ui.getByRole('spinbutton', { name: 'AI 讨论闭环上限', exact: true }).fill('12');
    await dialog('项目设置').getByRole('button', { name: '职位', exact: true }).first().click();
    await ui.getByRole('textbox', { name: '职位提示词', exact: true }).fill('E2E 岗位视角。');
    await ui.getByRole('textbox', { name: '我的个人提示词', exact: true }).fill('E2E 个人偏好。');
    await dialog('项目设置').getByRole('button', { name: '成员', exact: true }).click();
    await ui.getByRole('textbox', { name: '成员姓名', exact: true }).fill('E2E 新伙伴'); await button('加入项目').click();
    await button('保存修改').click();
    await sidebar().getByRole('button', { name: '项目设置', exact: true }).click();
    assert(await ui.getByRole('spinbutton', { name: 'AI 讨论闭环上限', exact: true }).evaluate(el => el.value) === '12', 'Project loop limit was not saved');
    await dialog('项目设置').getByRole('button', { name: '职位', exact: true }).first().click();
    assert(await ui.getByRole('textbox', { name: '我的个人提示词', exact: true }).evaluate(el => el.value) === 'E2E 个人偏好。', 'Personal prompt was not saved');
    await button('保存修改').click(); await button('创建群聊').click();
    assert(await dialog('创建群聊').getByRole('checkbox', { name: 'E2E 新伙伴 E2E 新伙伴', exact: true }).count() === 1, 'Invited project member not available for group creation');
    await button('取消').last().click();
   });
  },
  async finish() { await button('关于这个 Demo').first().click(); await button('重置示例数据').click(); },
 };
 return { results, ...phases, async runAll() { for (const phase of Object.values(phases)) await phase(); return results; } };
}
