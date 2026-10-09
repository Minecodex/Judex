import {expect, test, type BrowserContext, type Page} from '@playwright/test';
import {randomUUID} from 'node:crypto';
import {chooseValue} from './workspace-helpers';
import {assertPreferenceGeometry} from './ui-consistency-helpers';

async function register(context: BrowserContext) {
  const email = randomUUID() + '@judex.test';
  const response = await context.request.post('/api/v1/auth/register', {
    data: {displayName: '多职位成员', email, password: 'password-prefs-123'},
  });
  expect(response.status(), await response.text()).toBe(201);
  return {email, user: (await response.json()).data.user};
}
async function read(context: BrowserContext, path: string) {
  const response = await context.request.get('/api/v1' + path);
  expect(response.ok(), await response.text()).toBeTruthy();
  return (await response.json()).data;
}
async function write(context: BrowserContext, path: string, data: unknown, method: 'POST'|'PUT' = 'POST', key = randomUUID()) {
  const session = await read(context, '/auth/session');
  return context.request.fetch('/api/v1' + path, {method, data,
    headers: {'X-CSRF-Token': session.csrfToken, 'Idempotency-Key': key}});
}
async function command(context: BrowserContext, path: string, data: unknown) {
  const response = await write(context, path, data);
  expect(response.ok(), await response.text()).toBeTruthy();
  return (await response.json()).data;
}
async function save(page: Page, prompt: string, status = 200) {
  await page.getByTestId('next-personal-prompt').fill(prompt);
  const response = page.waitForResponse(response => response.url().endsWith('/me/preferences') && response.request().method() === 'PUT');
  await page.getByTestId('next-save-preference').click();
  expect((await response).status()).toBe(status);
  await expect(page.getByTestId('next-save-preference')).toBeEnabled();
}

test('P05 separate position prompts persist, preserve drafts and reject unassigned or stale writes', async ({browser}, info) => {
  test.setTimeout(120000);
  const ownerContext = await browser.newContext({viewport:{width:1440,height:1000}});
  const otherContext = await browser.newContext({viewport:{width:1440,height:1000}});
  try {
    const owner = await register(ownerContext), other = await register(otherContext);
    const project = await command(ownerContext,'/projects',{title:'按职位设置工作提示词'});
    const prefix = '/projects/' + project.id;
    const page = await ownerContext.newPage();
    await page.goto('/?project=' + project.id + '&settings=preferences');
    await expect(page.locator('[data-testid^=position-preferences-]')).toHaveCount(0);
    await expect(page.getByTestId('next-save-preference')).toHaveCount(0);
    const dev = await command(ownerContext,prefix+'/positions',{name:'开发工程师',prompt:'开发公开职责'});
    const review = await command(ownerContext,prefix+'/positions',{name:'评审工程师',prompt:'评审公开职责'});
    const third = await command(ownerContext,prefix+'/positions',{name:'他人的职位',prompt:'他人公开职责'});
    const identity = await command(ownerContext,prefix+'/identities',{positionId:dev.id,userId:owner.user.id});
    const reviewIdentity = await command(ownerContext,prefix+'/identities',{positionId:review.id,userId:owner.user.id});
    await command(ownerContext,prefix+'/identities',{positionId:review.id,userId:owner.user.id});
    const invite = await command(ownerContext,prefix+'/invitations',{targetEmail:other.email,positionIds:[third.id]});
    await command(otherContext,'/invitations/'+invite.id+'/accept',{expectedVersion:1,token:invite.token});
    await page.reload();
    await page.getByTestId("position-preferences-"+dev.id).click();
    await assertPreferenceGeometry(page);
    await page.getByTestId('personal-prompt-position').click();
    await expect(page.getByRole('option')).toHaveCount(2);
    await expect(page.getByRole('option',{name:'他人的职位'})).toHaveCount(0);
    await page.keyboard.press('Escape');
    await chooseValue(page,'personal-prompt-position',dev.id);
    await page.getByTestId('next-personal-prompt').fill('开发草稿');
    await chooseValue(page,'personal-prompt-position',review.id);
    await expect(page.getByTestId('next-personal-prompt')).toHaveValue('');
    await page.getByTestId('next-personal-prompt').fill('评审草稿');
    await chooseValue(page,'personal-prompt-position',dev.id);
    await expect(page.getByTestId('next-personal-prompt')).toHaveValue('开发草稿');
    await page.route('**/me/preferences', route => route.request().method()==='PUT'
      ? route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{code:'DEPENDENCY_UNAVAILABLE',message:'retry',retryable:true}})})
      : route.continue(), {times:1});
    await save(page,'PRIVATE-DEVELOPMENT',503);
    await expect(page.getByTestId('next-personal-prompt')).toHaveValue('PRIVATE-DEVELOPMENT');
    await save(page,'PRIVATE-DEVELOPMENT');
    await chooseValue(page,'personal-prompt-position',review.id);
    await expect(page.getByTestId('next-personal-prompt')).toHaveValue('评审草稿');
    await save(page,'PRIVATE-REVIEW');
    await page.reload();
    await expect(page.getByTestId('personal-prompt-position')).toHaveAttribute('data-value',review.id);
    await expect(page.getByTestId('next-personal-prompt')).toHaveValue('PRIVATE-REVIEW');
    await chooseValue(page,'personal-prompt-position',dev.id);
    await expect(page.getByTestId('next-personal-prompt')).toHaveValue('PRIVATE-DEVELOPMENT');
    const prefs = await read(ownerContext,prefix+'/me/preferences');
    expect(prefs.items).toHaveLength(2);
    expect(prefs.items.find((p:any)=>p.positionId===dev.id)).toMatchObject({revision:1,prompt:'PRIVATE-DEVELOPMENT'});
    expect(prefs.items.find((p:any)=>p.positionId===review.id)).toMatchObject({revision:1,prompt:'PRIVATE-REVIEW'});
    const forbidden = await write(ownerContext,prefix+'/me/preferences',{positionId:third.id,expectedRevision:0,prompt:'deny'},'PUT');
    expect(forbidden.status()).toBe(403);
    expect((await read(otherContext,prefix+'/me/preferences')).items).toEqual([{positionId:third.id,revision:0,prompt:''}]);
    const otherPage = await otherContext.newPage();
    await otherPage.goto('/projects/'+project.id+'/settings/team?settingsItem=preference:'+third.id);
    await expect(otherPage.getByTestId('personal-prompt-position')).toHaveAttribute('data-value',third.id);
    await expect(otherPage.getByTestId('next-personal-prompt')).toHaveValue('');
    // The shared prompt preview must follow the viewed identity's position.
    await page.goto('/?project='+project.id+'&settings=team');
    for (const [id,prompt,excluded] of [[identity.id,'PRIVATE-DEVELOPMENT','PRIVATE-REVIEW'],[reviewIdentity.id,'PRIVATE-REVIEW','PRIVATE-DEVELOPMENT']]) {
      await page.getByTestId('seat-'+id).getByRole('button',{name:'查看组合提示词',exact:true}).click();
      await expect(page.getByRole('dialog')).toContainText(prompt);
      await expect(page.getByRole('dialog')).not.toContainText(excluded);
      await page.getByRole('dialog').getByRole('button',{name:'关闭',exact:true}).click();
    }
    await page.goto('/projects/'+project.id+'/settings/team?settingsItem=preference:'+dev.id);
    // Verify the four desktop language/theme combinations on the real page.
    for (const locale of ['zh-CN','en']) for (const theme of ['light','dark']) {
      await page.evaluate(({locale,theme})=>{localStorage.setItem('argus.locale',locale);localStorage.setItem('judex.theme',theme);},{locale,theme});
      await page.reload();
      await expect(page.getByTestId('personal-prompt-position')).toHaveAccessibleName(new RegExp(locale==='en'?'My position':'我承担的职位'));
      await expect(page.getByTestId('next-personal-prompt')).toHaveValue('PRIVATE-DEVELOPMENT');
      await expect(page.locator('html')).toHaveAttribute('data-theme',theme);
      await assertPreferenceGeometry(page);
      expect(await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth)).toBe(true);
      await page.screenshot({path:info.outputPath(`position-prompts-${locale}-${theme}.png`),fullPage:true});
    }
    // A stale UI must not fetch a newer revision and silently overwrite it.
    const remote = await write(ownerContext,prefix+'/me/preferences',{positionId:dev.id,expectedRevision:1,prompt:'PRIVATE-REMOTE'},'PUT');
    expect(remote.status()).toBe(200);
    await save(page,'PRIVATE-STALE-DRAFT',409);
    await expect(page.getByTestId('next-personal-prompt')).toHaveValue('PRIVATE-STALE-DRAFT');
    expect((await read(ownerContext,prefix+'/me/preferences')).items.find((p:any)=>p.positionId===dev.id).prompt).toBe('PRIVATE-REMOTE');
    // Clearing one position leaves the other intact; replacement uses the successor's configuration.
    await page.reload();
    await save(page,'');
    expect((await read(ownerContext,prefix+'/me/preferences')).items.find((p:any)=>p.positionId===review.id).prompt).toBe('PRIVATE-REVIEW');
    await command(ownerContext,prefix+'/identities/'+identity.id+'/replace',{expectedBindingVersion:1,newUserId:other.user.id,reason:'任职替换验证'});
    await otherPage.reload();
    await chooseValue(otherPage,'personal-prompt-position',dev.id);
    await expect(otherPage.getByTestId('next-personal-prompt')).toHaveValue('');
    expect((await read(otherContext,prefix+'/me/preferences')).items.find((p:any)=>p.positionId===dev.id)).toMatchObject({revision:0,prompt:''});
    const lost = await write(ownerContext,prefix+'/me/preferences',{positionId:dev.id,expectedRevision:3,prompt:'deny'},'PUT');
    expect(lost.status()).toBe(403);
  } finally { await ownerContext.close(); await otherContext.close(); }
});
