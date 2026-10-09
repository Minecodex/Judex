import { expect, test, type Page } from '@playwright/test';
import fs from 'node:fs';
import { createHash } from 'node:crypto';

async function command(page:Page,path:string,body:unknown) {
  const session=(await (await page.request.get('/api/v1/auth/session')).json()).data;
  const response=await page.request.post('/api/v1'+path,{data:body,headers:{'X-CSRF-Token':session.csrfToken,'Idempotency-Key':crypto.randomUUID()}});
  expect(response.ok(),await response.text()).toBeTruthy();return (await response.json()).data;
}

test('P01 project cards use real summaries, server search and recent discussion deep links',async({page},info)=>{
  const stamp=Date.now().toString(36);
  await page.goto('/register');
  await page.getByLabel(/名称|Name/).fill('林晓');
  await page.getByLabel(/邮箱|Email/).fill(`portal-${stamp}@judex.test`);
  await page.getByLabel(/密码|Password/).first().fill('password-portal-11');
  await page.getByLabel(/确认密码|Confirm/).fill('password-portal-11');
  await page.getByRole('button',{name:/创建账号|Create account/}).click();
  await expect(page.getByTestId('workspace-new-project')).toBeVisible();
  const project=await command(page,'/projects',{title:'Judex 产品体验',description:'把讨论、决定和本地工作连成一个清晰的协作体验。'});
  await command(page,'/projects',{title:'品牌与设计系统',description:'建立共同的设计准则。'});
  await command(page,'/projects',{title:'基础设施升级',description:'改善团队基础设施。'});
  const topic=await command(page,`/projects/${project.id}/topics`,{title:'V1 产品体验优化'});
  await page.reload();
  await expect(page.locator('.judex-co-project-card')).toHaveCount(3);
  const card=page.locator('.judex-co-project-card').filter({hasText:'Judex 产品体验'});
  expect(await card.evaluate(element=>getComputedStyle(element).paddingTop)).toBe(await page.evaluate(()=>getComputedStyle(document.documentElement).getPropertyValue('--s5').trim()));
  await expect(card.locator('.judex-co-meta')).toHaveText('1·1');
  const alignment=await page.evaluate(()=>{
    const center=(element:Element)=>{const r=element.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}};
    const search=document.querySelector('.judex-co-search')!;
    const badge=document.querySelector('.judex-co-project-card .judex-ui-status')!;
    const range=document.createRange();range.selectNodeContents(badge.querySelector('.chip__label')!);
    const text=range.getBoundingClientRect(),rect=badge.getBoundingClientRect();
    return {searchIcon:center(search.querySelector('svg')!),searchInput:center(search.querySelector('input')!),badge:center(badge),badgeLabel:center(badge.querySelector('.chip__label')!),labelInside:text.width>0&&text.left>=rect.left&&text.right<=rect.right&&text.top>=rect.top&&text.bottom<=rect.bottom};
  });
  expect(Math.abs(alignment.searchIcon.y-alignment.searchInput.y)).toBeLessThanOrEqual(1);
  expect(Math.abs(alignment.badge.x-alignment.badgeLabel.x)).toBeLessThanOrEqual(1);
  expect(Math.abs(alignment.badge.y-alignment.badgeLabel.y)).toBeLessThanOrEqual(1);
  expect(alignment.labelInside).toBe(true);
  await page.getByRole('button',{name:'我参与的',exact:true}).click();
  await expect(page.getByText('没有找到相关项目',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:/全部项目/}).click();
  await page.getByRole('textbox',{name:'搜索项目…'}).fill('品牌');
  await expect(page.locator('.judex-co-project-card')).toHaveCount(1);
  await expect(page.locator('.judex-co-project-card')).toContainText('品牌与设计系统');
  await page.getByRole('textbox',{name:'搜索项目…'}).fill('');
  await expect(page.locator('.judex-co-project-card')).toHaveCount(3);
  await page.screenshot({path:info.outputPath('projects-light.png'),fullPage:true});
  await page.locator('.judex-recent-list').getByRole('button',{name:/V1 产品体验优化/}).click();
  await expect(page).toHaveURL(new RegExp(topic.id));
  await expect(page.getByTestId('tab-topic:'+topic.id)).toHaveAttribute('aria-selected','true');
  await expect(page.getByTestId('work-discussion-input')).toBeVisible();
  await page.getByTestId('work-discussion-input').fill('保留这段真实草稿');
  await page.getByTestId('workspace-add-tool').click();
  await page.getByRole('menuitem',{name:'共享资料',exact:true}).click();
  await expect(page.getByTestId('work-discussion-input')).toHaveValue('保留这段真实草稿');
  const response=await page.request.get('/api/v1/projects?include=summary&limit=1');
  expect(response.ok()).toBeTruthy();const data=(await response.json()).data;
  expect(data.totalCount).toBe(3);expect(data.items).toHaveLength(1);expect(data.nextCursor).toBeTruthy();
  expect((await page.request.get('/api/v1/projects?ownership=invalid')).status()).toBe(400);
  expect((await page.request.get('/api/v1/projects?include=invalid')).status()).toBe(400);
  expect((await page.request.get('/api/v1/me/recent-topics?limit=100')).status()).toBe(200);
  expect((await page.request.get('/api/v1/me/recent-topics?limit=101')).status()).toBe(400);
});

test('P03 local AI connection downloads real Windows/macOS CLI and Skills archives',async({page},info)=>{
  test.setTimeout(60000);const stamp=Date.now().toString(36);
  await page.goto('/register');await page.getByLabel(/名称|Name/).fill('下载验收');await page.getByLabel(/邮箱|Email/).fill(`downloads-${stamp}@judex.test`);
  await page.getByLabel(/密码|Password/).first().fill('password-download-11');await page.getByLabel(/确认密码|Confirm/).fill('password-download-11');
  await page.getByRole('button',{name:/创建账号|Create account/}).click();await expect(page.getByTestId('workspace-new-project')).toBeVisible();
  const project=await command(page,'/projects',{title:'CLI 下载验收'});
  await page.route('**/downloads/manifest.json',route=>route.fulfill({status:503,contentType:'application/json',body:'{}'}),{times:1});
  await page.goto('/?project='+project.id+'&settings=local');await expect(page.getByTestId('local-ai-connection')).toBeVisible();
  await expect(page.getByTestId('local-ai-connection').getByRole('alert')).toContainText(/下载暂不可用|Downloads are temporarily unavailable/);
  for(const id of ['download-cli-windows','download-cli-macos','download-skills'])await expect(page.getByTestId(id)).toBeDisabled();
  await page.getByRole('button',{name:/重新加载|Reload/}).click();
  await expect(page.getByTestId('download-cli-windows')).toBeEnabled();
  const response=await page.request.get('/downloads/manifest.json');expect(response.ok()).toBeTruthy();const manifest=await response.json();
  for(const [kind,id]of [['windows','download-cli-windows'],['macos','download-cli-macos'],['skills','download-skills']]){
    const [download]=await Promise.all([page.waitForEvent('download'),page.getByTestId(id).click()]);
    expect(download.suggestedFilename()).toBe(manifest.artifacts[kind].filename);
    const target=info.outputPath(download.suggestedFilename());await download.saveAs(target);
    const bytes=fs.readFileSync(target);expect(bytes.length).toBe(manifest.artifacts[kind].size);expect(createHash('sha256').update(bytes).digest('hex')).toBe(manifest.artifacts[kind].sha256);
  }
  await expect(page.getByText('此界面尚未接入真实 CLI／Skill',{exact:false})).toHaveCount(0);
  await page.screenshot({path:info.outputPath('local-ai-downloads.png'),fullPage:true});
});

test('P02 real authentication and project pages share desktop tokens in both languages and themes',async({browser},info)=>{
  const context=await browser.newContext();const page=await context.newPage();
  const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
  await page.goto('/login');
  for(const locale of ['zh-CN','en']) for(const theme of ['light','dark']) {
    await page.evaluate(({locale,theme})=>{localStorage.setItem('argus.locale',locale);localStorage.setItem('judex.theme',theme);},{locale,theme});
    for(const width of [1120,1280,1440,1920,3840]) {
      await page.setViewportSize({width,height:width===3840?1926:1000});
      await page.goto('/login');await expect(page.locator('.judex-entry-split')).toBeVisible();
      await expect(page.getByRole('button',{name:locale==='en'?'Sign in':'登录',exact:true})).toBeVisible();
      const metrics=await page.evaluate(()=>({width:document.documentElement.clientWidth,scrollWidth:document.documentElement.scrollWidth,font:getComputedStyle(document.body).fontSize,accent:getComputedStyle(document.querySelector('button[type=submit]')!).backgroundColor}));
      expect(metrics.scrollWidth).toBeLessThanOrEqual(metrics.width);expect(metrics.font).toBe('14px');
      expect(metrics.accent).toBe(theme==='light'?'rgb(45, 102, 81)':'rgb(159, 197, 170)');
      if(width===1440) await page.screenshot({path:info.outputPath(`login-${locale}-${theme}.png`)});
    }
  }
  expect(errors).toEqual([]);await context.close();
});

test('P04 top project navigation preserves conversation tabs, text and attachments across projects',async({page},info)=>{
 test.setTimeout(120000);const stamp=Date.now().toString(36),errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto('/register');await page.getByLabel(/名称|Name/).fill('导航验收');await page.getByLabel(/邮箱|Email/).fill('navigation-'+stamp+'@judex.test');await page.getByLabel(/密码|Password/).first().fill('password-navigation-11');await page.getByLabel(/确认密码|Confirm/).fill('password-navigation-11');await page.getByRole('button',{name:/创建账号|Create account/}).click();await expect(page.getByTestId("workspace-new-project")).toBeVisible();
 const project=await command(page,'/projects',{title:'返回入口验收'}),other=await command(page,'/projects',{title:'另一个项目'});
 const topic=await command(page,'/projects/'+project.id+'/topics',{title:'带附件的讨论'}),second=await command(page,'/projects/'+project.id+'/topics',{title:'另一场讨论'});
 for(let i=0;i<18;i++)await command(page,'/projects/'+project.id+'/topics',{title:'列表滚动 '+i});
 await page.goto('/projects/'+project.id+'/chat/'+topic.id);await expect(page.getByTestId('tab-topic:'+topic.id)).toHaveAttribute('aria-selected','true');
 await page.getByTestId('work-discussion-input').fill('保留项目一的草稿与附件');await page.getByRole('button',{name:'添加资料',exact:true}).click();await page.locator('input[data-testid="work-files"]').setInputFiles({name:'返回后继续.txt',mimeType:'text/plain',buffer:Buffer.from('只在用户发送时提交的附件')});await expect(page.locator('.judex-work-upload')).toContainText('返回后继续.txt');
 await page.getByTestId('workspace-add-tool').click();await page.getByRole('menuitem',{name:'共享资料',exact:true}).click();await page.getByTestId('scoped-topic-'+second.id).click();await page.getByTestId('work-discussion-input').fill('另一场讨论的独立草稿');await page.getByTestId('tab-topic:'+topic.id).click();await expect(page.getByTestId('work-discussion-input')).toHaveValue('保留项目一的草稿与附件');
 await page.evaluate(()=>{(window as any).judexNavigationProbe='same-document';});await page.getByTestId('account-menu-button').click();await page.getByTestId('account-projects').click();await expect(page.getByTestId('workspace-new-project')).toBeVisible();expect(await page.evaluate(()=>(window as any).judexNavigationProbe)).toBe('same-document');
 await page.getByTestId('project-enter-'+other.id).click();await expect(page.getByTestId('hub-tab-plans')).toBeVisible();await page.getByRole('button',{name:'项目讨论记录',exact:true}).click();await expect(page.getByTestId('work-discussion-input')).toHaveCount(0);await expect(page.locator('.judex-work-upload')).toHaveCount(0);await expect(page.getByTestId('tab-topic:'+topic.id)).toHaveCount(0);
 await page.getByTestId('account-menu-button').click();await page.getByTestId('account-projects').click();await page.getByTestId('project-enter-'+project.id).click();await page.getByRole('button',{name:'项目讨论记录',exact:true}).click();await page.getByTestId('scoped-topic-'+topic.id).click();await expect(page.getByTestId('work-discussion-input')).toHaveValue('保留项目一的草稿与附件');await expect(page.locator('.judex-work-upload')).toContainText('返回后继续.txt');await expect(page.getByTestId('tab-topic:'+second.id)).toBeVisible();
 await page.screenshot({path:info.outputPath('top-project-return.png'),fullPage:true});expect(errors).toEqual([]);
});
