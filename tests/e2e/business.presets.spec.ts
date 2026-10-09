import {test, expect, type BrowserContext} from '@playwright/test';
import {randomUUID} from 'node:crypto';
import {chooseValue} from './workspace-helpers';

async function register(context: BrowserContext, displayName: string) {
  const email = randomUUID() + '@judex.test';
  const response = await context.request.post('/api/v1/auth/register', {data: {displayName, email, password: 'password-preset-123'}});
  expect(response.status()).toBe(201);
  return {email, user: (await response.json()).data.user};
}
async function command(context: BrowserContext, path: string, data: unknown, key = randomUUID()) {
  const session = (await (await context.request.get('/api/v1/auth/session')).json()).data;
  return context.request.post('/api/v1' + path, {data, headers: {'X-CSRF-Token': session.csrfToken, 'Idempotency-Key': key}});
}
async function read(context: BrowserContext, path: string) {
  const response = await context.request.get('/api/v1' + path);
  expect(response.ok()).toBeTruthy(); return (await response.json()).data;
}

test('T01 position preset modal adds selected real roles, handles retry and persists independent edits', async ({browser}, info) => {
  const context = await browser.newContext({viewport: {width: 1440, height: 1000}});
  try {
    await register(context, '职位模板验收');
    const response = await command(context, '/projects', {title: '职位模板验收项目'});
    expect(response.status()).toBe(201); const project = (await response.json()).data;
    const prefix = `/projects/${project.id}`;
    const page = await context.newPage();
    await page.route('**/api/v1/position-presets', route => route.fulfill({status: 503, contentType: 'application/json', body: '{}'}), {times: 1});
    await page.goto(`/?project=${project.id}&settings=team`);
    await page.getByTestId('open-position-presets').click();
    const dialog = page.getByRole('dialog', {name: '添加职位模板'});
    await expect(dialog.getByRole('alert')).toContainText('职位模板加载失败');
    await expect(page.getByTestId('import-position-presets')).toBeDisabled();
    await dialog.getByRole('button', {name: '重新加载'}).click();
    await chooseValue(page, 'position-preset-scenario', 'software');
    await page.getByTestId('preset-card-project-manager').click();
    await page.getByTestId('preset-card-frontend-developer').click();
    await expect(page.getByTestId('preset-selection-count')).toHaveText('已选择 2 个职位');
    await dialog.locator('.judex-dialog-body').evaluate(element => {element.scrollTop = 0;});
    await page.screenshot({path: info.outputPath('real-preset-selection.png'), fullPage: true, animations: 'disabled'});
    // A rejected write must preserve the selection for retry.
    await page.route('**/positions/import-presets', route => route.fulfill({status: 503, contentType: 'application/json', body: JSON.stringify({error: {code: 'INTERNAL', message: 'retry'}})}), {times: 1});
    await page.getByTestId('import-position-presets').click();
    await expect(dialog.getByRole('alert')).toContainText('添加失败');
    await expect(page.getByTestId('preset-selection-count')).toHaveText('已选择 2 个职位');
    let releaseWrite!: () => void;
    const writeGate = new Promise<void>(resolve => {releaseWrite = resolve;});
    await page.route('**/positions/import-presets', async route => {await writeGate; await route.continue();}, {times: 1});
    const post = page.waitForRequest(request => request.url().endsWith('/positions/import-presets') && request.method() === 'POST');
    try {
      await page.getByTestId('import-position-presets').click();
      await expect(page.getByTestId('import-position-presets')).toBeDisabled();
      await expect(dialog.getByRole('button', {name: '关闭', exact: true})).toBeDisabled();
      await page.keyboard.press('Escape'); await expect(dialog).toBeVisible();
    } finally {releaseWrite();}
    expect((await post).postDataJSON().roleIds.sort()).toEqual(['frontend-developer', 'project-manager']);
    await expect(dialog).toBeHidden();
    const positions = (await read(context, prefix + '/positions')).items;
    expect(positions).toHaveLength(2);
    expect(positions.map((position: {presetId: string}) => position.presetId).sort()).toEqual(['frontend-developer', 'project-manager']);
    for (const position of positions) {expect(position.nodeBindings).toEqual([]); expect(position.prompt).toBeTruthy(); expect(position.publicSummary).toBeTruthy();}
    await page.reload();
    const frontend = positions.find((position: {presetId: string}) => position.presetId === 'frontend-developer');
    await page.getByTestId('edit-position-' + frontend.id).click();
    await page.getByTestId('position-name').fill('定制前端职位');
    await page.getByTestId('position-prompt').fill('本项目的独立前端职责');
    await page.getByTestId('position-summary').fill('本项目的职责简介');
    await page.getByTestId('save-position').click();
    await expect(page.getByTestId('position-name')).toBeHidden();
    const edited = (await read(context, prefix + '/positions')).items.find((position: {id: string}) => position.id === frontend.id);
    expect(edited.presetId).toBe(frontend.presetId); expect(edited.prompt).toBe('本项目的独立前端职责');
    await page.getByTestId('open-position-presets').click();
    await chooseValue(page, 'position-preset-scenario', 'software');
    await expect(page.getByTestId('preset-card-frontend-developer').getByRole('checkbox')).toBeDisabled();
    await page.keyboard.press('Escape');
    const catalog = await read(context, '/position-presets');
    expect(catalog.scenarios).toHaveLength(20);
    expect(catalog.roles.find((role: {id: string}) => role.id === frontend.presetId).prompt.zh).not.toBe(edited.prompt);
    const body = {catalogVersion: catalog.version, scenarioId: 'software', roleIds: ['qa-engineer', 'devops-engineer'], locale: 'en'};
    const key = randomUUID();
    const first = await command(context, prefix + '/positions/import-presets', body, key);
    const second = await command(context, prefix + '/positions/import-presets', body, key);
    expect(first.status()).toBe(201); expect(second.status()).toBe(201);
    expect((await second.json()).data).toEqual((await first.json()).data);
    const duplicate = await command(context, prefix + '/positions/import-presets', body);
    const skipped = (await duplicate.json()).data;
    expect(skipped.items).toEqual([]); expect(skipped.skipped).toHaveLength(2);
    expect((await read(context, prefix + '/positions')).items).toHaveLength(4);
  } finally {await context.close();}
});

test('T02 position preset import rejects ordinary members, other projects and invalid selections', async ({browser}) => {
  const owner = await browser.newContext(), member = await browser.newContext(), anonymous = await browser.newContext();
  try {
    expect((await anonymous.request.get('/api/v1/position-presets')).status()).toBe(401);
    await register(owner, '模板管理者'); const person = await register(member, '模板管理者');
    const response = await command(owner, '/projects', {title: '模板权限隔离'}); const project = (await response.json()).data;
    const prefix = `/projects/${project.id}`;
    const catalog = await read(owner, '/position-presets');
    const body = {catalogVersion: catalog.version, scenarioId: 'software', roleIds: ['frontend-developer', 'qa-engineer'], locale: 'zh-CN'};
    expect((await command(member, prefix + '/positions/import-presets', body)).status()).toBe(404);
    const position = (await (await command(owner, prefix + '/positions', {name: '邀请职责', prompt: '项目协作'})).json()).data;
    const invitation = (await (await command(owner, prefix + '/invitations', {targetEmail: person.email, positionIds: [position.id]})).json()).data;
    expect((await command(member, `/invitations/${invitation.id}/accept`, {expectedVersion: 1})).status()).toBe(200);
    expect((await command(member, prefix + '/positions/import-presets', body)).status()).toBe(403);
    const page = await member.newPage(); await page.goto(`/?project=${project.id}&settings=team`);
    await expect(page.getByTestId('settings-page')).toBeVisible();
    await expect(page.getByTestId('open-position-presets')).toHaveCount(0);
    const invalid = await command(owner, prefix + '/positions/import-presets', {...body, roleIds: ['frontend-developer', 'screenwriter']});
    expect(invalid.status()).toBe(400);
    expect((await read(owner, prefix + '/positions')).items).toHaveLength(1);
    expect((await command(owner, prefix + '/positions/import-presets', {...body, catalogVersion: 'old'})).status()).toBe(409);
    expect((await command(owner, prefix + '/positions/import-presets', {...body, roleIds: ['qa-engineer', 'qa-engineer']})).status()).toBe(400);
  } finally {await owner.close(); await member.close(); await anonymous.close();}
});


test('T03 position cards assign specific members by ID, preserve other identities and support shared roles', async ({browser}, info) => {
  const owner = await browser.newContext({viewport: {width:1440,height:1000}}), first = await browser.newContext(), second = await browser.newContext();
  try {
    await register(owner,'职位分配管理员'); const a = await register(first,'同名成员'), b = await register(second,'同名成员');
    const projectResponse = await command(owner,'/projects',{title:'按职位分配验收'}); expect(projectResponse.status()).toBe(201);
    const project = (await projectResponse.json()).data, prefix = `/projects/${project.id}`;
    const starter = (await (await command(owner,prefix+'/positions',{name:'初始协作',prompt:'项目协作'})).json()).data;
    const target = (await (await command(owner,prefix+'/positions',{name:'后端开发',prompt:'实现后端并提交材料'})).json()).data;
    for (const [context,person] of [[first,a],[second,b]] as const) {
      const invitation = (await (await command(owner,prefix+'/invitations',{targetEmail:person.email,positionIds:[starter.id]})).json()).data;
      expect((await command(context,`/invitations/${invitation.id}/accept`,{expectedVersion:1})).status()).toBe(200);
    }
    const before = (await read(owner,prefix+'/identities')).items;
    const page = await owner.newPage(); await page.goto(`/?project=${project.id}&settings=team`);
    await expect(page.locator('.judex-project-assignment')).toHaveCount(0);
    const card = page.getByTestId('position-card-'+target.id);
    await card.getByTestId('assign-position-'+target.id).click();
    await chooseValue(page,'position-assignment-member',b.user.id);
    const sent = page.waitForRequest(request=>request.url().endsWith('/identities')&&request.method()==='POST');
    await page.getByTestId('assign-existing-position').click();
    expect((await sent).postDataJSON()).toMatchObject({positionId:target.id,userId:b.user.id});
    await expect(page.getByRole('dialog')).toBeHidden();
    let identities = (await read(owner,prefix+'/identities')).items;
    const added = identities.filter((identity:{templateId?:string})=>identity.templateId===target.id);
    expect(added).toHaveLength(1); expect(added[0].currentBinding.userId).toBe(b.user.id);
    expect(identities.filter((identity:{id:string})=>before.some((item:{id:string})=>item.id===identity.id))).toEqual(before);
    await page.reload(); await card.getByTestId('assign-position-'+target.id).click();
    await chooseValue(page,'position-assignment-member',a.user.id);
    await page.getByTestId('assign-existing-position').click(); await expect(page.getByRole('dialog')).toBeHidden();
    identities = (await read(owner,prefix+'/identities')).items;
    const holders = identities.filter((identity:{templateId?:string})=>identity.templateId===target.id);
    expect(holders).toHaveLength(2); expect(holders.map((identity:{currentBinding:{userId:string}})=>identity.currentBinding.userId).sort()).toEqual([a.user.id,b.user.id].sort());
    for (const button of await card.locator('.judex-card-actions button').all()) expect((await button.boundingBox())!.height).toBeLessThanOrEqual(34);
    await page.screenshot({path:info.outputPath('position-first-assignment.png'),fullPage:true,animations:'disabled'});
    const memberPage = await first.newPage(); await memberPage.goto(`/?project=${project.id}&settings=team`);
    await expect(memberPage.getByTestId('settings-page')).toBeVisible(); await expect(memberPage.getByTestId('assign-position-'+target.id)).toHaveCount(0);
    expect((await command(first,prefix+'/identities',{positionId:target.id,userId:a.user.id})).status()).toBe(403);
  } finally {await owner.close();await first.close();await second.close();}
});
