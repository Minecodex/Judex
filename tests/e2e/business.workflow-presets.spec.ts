import {test,expect,type BrowserContext} from '@playwright/test';
import {randomUUID} from 'node:crypto';
import {chooseValue} from './workspace-helpers';
async function register(context:BrowserContext,name:string) {
  const email=randomUUID()+'@judex.test';
  const response=await context.request.post('/api/v1/auth/register',{data:{displayName:name,email,password:'password-workflow-123'}});
  expect(response.status()).toBe(201);return {email,user:(await response.json()).data.user};
}
async function command(context:BrowserContext,path:string,data:unknown,key=randomUUID()) {
  const session=(await(await context.request.get('/api/v1/auth/session')).json()).data;
  return context.request.post('/api/v1'+path,{data,headers:{'X-CSRF-Token':session.csrfToken,'Idempotency-Key':key}});
}
async function read(context:BrowserContext,path:string) {
  const response=await context.request.get('/api/v1'+path);expect(response.ok()).toBeTruthy();return (await response.json()).data;
}

test('T04 workflow templates preserve failed selections, import real drafts, edit, publish and replay idempotently',async({browser},info)=>{
  test.setTimeout(90000);
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  try {
    await register(context,'流程模板验收');
    const project=(await(await command(context,'/projects',{title:'流程模板验收项目'})).json()).data;
    const prefix='/projects/'+project.id;
    const workflowCatalog=await read(context,'/workflow-presets'),positions=await read(context,'/position-presets');
    expect(workflowCatalog.scenarios.map((s:{id:string;name:unknown})=>({id:s.id,name:s.name}))).toEqual(positions.scenarios.map((s:{id:string;name:unknown})=>({id:s.id,name:s.name})));
    const initialIdentities=(await read(context,prefix+'/identities')).items;
    const page=await context.newPage();
    await page.route('**/api/v1/workflow-presets',route=>route.fulfill({status:503,contentType:'application/json',body:'{}'}),{times:1});
    await page.goto('/?project='+project.id+'&settings=flows');
    await page.getByTestId('open-workflow-presets').click();
    await expect(page.getByRole('alert')).toContainText('流程模板加载失败');
    await page.getByRole('button',{name:'重新加载',exact:true}).click();
    await chooseValue(page,'workflow-preset-scenario','software');
    await expect(page.locator('.judex-workflow-thumbnail svg')).toHaveCount(7);
    await page.getByTestId('workflow-preset-card-software-standard').click();
    await page.getByTestId('workflow-preset-card-software-bug').click();
    await page.getByTestId('preview-workflow-software-standard').click();
    await expect(page.getByTestId('workflow-preset-full-preview')).toBeVisible();
    await page.getByTestId('workflow-preview-back').click();
    await expect(page.getByTestId('workflow-preset-selection-count')).toHaveText('已选择 2 个流程');
    await page.route('**/workflows/import-presets',route=>route.fulfill({status:503,contentType:'application/json',body:'{}'}),{times:1});
    await page.getByTestId('import-workflow-presets').click();
    await expect(page.getByRole('alert')).toContainText('选择已保留');
    await expect(page.getByTestId('workflow-preset-selection-count')).toHaveText('已选择 2 个流程');
    let release:()=>void=()=>{};
    const hold=new Promise<void>(resolve=>{release=resolve;});
    await page.route('**/workflows/import-presets',async route=>{await hold;await route.continue();},{times:1});
    await page.getByTestId('import-workflow-presets').click();
    await expect(page.getByRole('dialog').getByRole('button',{name:'关闭',exact:true})).toBeDisabled();
    release();
    await expect(page.getByTestId('workflow-draft-editor')).toBeVisible();
    let flows=(await read(context,prefix+'/workflows')).items;
    expect(flows).toHaveLength(2);expect(flows.every((f:{publishedVersionId:string|null;hasDraft:boolean})=>!f.publishedVersionId&&f.hasDraft)).toBeTruthy();
    expect((await read(context,prefix+'/positions')).items).toHaveLength(0);
    expect((await read(context,prefix+'/identities')).items).toEqual(initialIdentities);
    const flow=flows.find((f:{presetId:string})=>f.presetId==='software-standard');
    const rejected=await command(context,prefix+'/positions',{name:'草稿绑定不可用',prompt:'职责',nodeBindings:[{workflowId:flow.id,nodeId:'step1'}]});
    expect(rejected.status()).toBe(400);
    await page.getByTestId('workflow-draft-name').fill('本项目研发流程');
    await page.getByRole('button',{name:/1\.\s*需求确认/}).click();
    await page.getByTestId('workflow-node-name-step1').fill('本项目需求');
    await page.getByTestId('workflow-node-responsibility-step1').fill('核对本项目目标与交付依据');
    await page.route('**/workflows/'+flow.id+'/draft',route=>route.fulfill({status:503,contentType:'application/json',body:'{}'}),{times:1});
    await page.getByTestId('save-workflow-draft').click();
    await expect(page.getByRole('alert')).toContainText('编辑内容已保留');
    await expect(page.getByTestId('workflow-draft-name')).toHaveValue('本项目研发流程');
    await page.getByTestId('save-workflow-draft').click();
    await expect(page.getByTestId('publish-workflow-draft')).toBeEnabled();
    const stored=(await read(context,prefix+'/workflows/'+flow.id+'/versions')).items.find((v:{state:string})=>v.state==='draft');
    expect(stored.body.name).toBe('本项目研发流程');expect(stored.body.nodes[0].name).toBe('本项目需求');
    expect(stored.body.nodes[0].responsibility).toBe('核对本项目目标与交付依据');expect(stored.body.advisoryEdges).toHaveLength(6);
    await page.reload();
    await page.getByRole('button',{name:'本项目研发流程',exact:true}).click();
    await expect(page.getByTestId('workflow-draft-name')).toHaveValue('本项目研发流程');
    await page.getByTestId('publish-workflow-draft').click();
    await expect(page.getByTestId('workflow-draft-editor')).toHaveCount(0);
    await expect(page.locator('.judex-flow-title')).toContainText('已发布');
    flows=(await read(context,prefix+'/workflows')).items;
    expect(flows.find((f:{id:string})=>f.id===flow.id).publishedVersionId).toBeTruthy();
    await page.getByTestId('open-workflow-presets').click();await chooseValue(page,'workflow-preset-scenario','software');
    await expect(page.getByTestId('workflow-preset-card-software-standard').getByRole('checkbox')).toBeDisabled();
    await page.screenshot({path:info.outputPath('workflow-templates-api.png'),fullPage:true});
    const body={catalogVersion:workflowCatalog.version,scenarioId:'software',workflowIds:['software-standard','software-bug'],locale:'en'};
    const key=randomUUID(),first=await command(context,prefix+'/workflows/import-presets',body,key),second=await command(context,prefix+'/workflows/import-presets',body,key);
    expect(first.status()).toBe(201);expect(second.status()).toBe(201);
    const replay=(await second.json()).data;expect(replay).toEqual((await first.json()).data);expect(replay.items).toEqual([]);expect(replay.skipped).toHaveLength(2);
    expect((await read(context,prefix+'/workflows')).items).toHaveLength(2);
  } finally {await context.close();}
});

test('T05 workflow template imports isolate ordinary members and other projects and reject invalid selections',async({browser})=>{
  const owner=await browser.newContext(),member=await browser.newContext(),anonymous=await browser.newContext();
  try {
    expect((await anonymous.request.get('/api/v1/workflow-presets')).status()).toBe(401);
    await register(owner,'同名流程管理者');const person=await register(member,'同名流程管理者');
    const project=(await(await command(owner,'/projects',{title:'流程权限隔离'})).json()).data,prefix='/projects/'+project.id;
    const catalog=await read(owner,'/workflow-presets'),body={catalogVersion:catalog.version,scenarioId:'software',workflowIds:['software-standard'],locale:'zh-CN'};
    expect((await command(member,prefix+'/workflows/import-presets',body)).status()).toBe(404);
    const position=(await(await command(owner,prefix+'/positions',{name:'邀请职责',prompt:'项目协作'})).json()).data;
    const invitation=(await(await command(owner,prefix+'/invitations',{targetEmail:person.email,positionIds:[position.id]})).json()).data;
    expect((await command(member,'/invitations/'+invitation.id+'/accept',{expectedVersion:1})).status()).toBe(200);
    expect((await command(member,prefix+'/workflows/import-presets',body)).status()).toBe(403);
    const page=await member.newPage();await page.goto('/?project='+project.id+'&settings=flows');
    await expect(page.getByTestId('open-workflow-presets')).toHaveCount(0);
    for(const invalid of [
      {...body,workflowIds:['software-standard','content-publish']},
      {...body,workflowIds:['software-standard','software-standard']},
      {...body,scenarioId:'missing'},
      {...body,catalogVersion:'old'},
    ])expect((await command(owner,prefix+'/workflows/import-presets',invalid)).status()).toBeGreaterThanOrEqual(400);
    expect((await read(owner,prefix+'/workflows')).items).toHaveLength(0);
  } finally {await owner.close();await member.close();await anonymous.close();}
});

test('T06 researched full workflow annotations persist through import, project editing and explicit publication',async({browser},info)=>{
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  try {
    await register(context,'完整流程验收');
    const project=(await(await command(context,'/projects',{title:'完整研发流程验收'})).json()).data,prefix='/projects/'+project.id;
    const catalog=await read(context,'/workflow-presets'),preset=catalog.workflows.find((p:{id:string})=>p.id==='software-delivery-advanced');
    const initial=(await read(context,prefix+'/identities')).items;
    const page=await context.newPage();await page.goto('/?project='+project.id+'&settings=flows');
    await page.getByTestId('open-workflow-presets').click();await chooseValue(page,'workflow-preset-scenario','software');
    await page.getByTestId('preview-workflow-software-delivery-advanced').click();
    const preview=page.getByTestId('workflow-preset-full-preview');
    await expect(preview.locator('svg').first()).toBeVisible();
    await expect(preview.locator('g.node')).toHaveCount(preset.nodes.length);
    await expect(page.getByTestId('workflow-source-links').locator('a')).toHaveCount(preset.sources.length);
    await page.getByTestId('workflow-natural-size').click();
    await expect(page.getByTestId('workflow-preview-viewport')).toHaveCSS('--diagram-scale','1');
    await page.screenshot({path:info.outputPath('full-workflow-api-preview.png'),fullPage:true});
    await page.getByTestId('workflow-preview-select').click();
    await page.getByTestId('import-workflow-presets').click();
    await expect(page.getByTestId('workflow-draft-editor')).toBeVisible();
    const flow=(await read(context,prefix+'/workflows')).items[0];
    const draft=(await read(context,prefix+'/workflows/'+flow.id+'/versions')).items[0];
    expect(draft.body.nodes).toHaveLength(preset.nodes.length);
    expect(draft.body.nodes.filter((n:{kind?:string})=>n.kind==='decision')).toHaveLength(7);
    expect(draft.body.advisoryEdges.filter((e:{kind?:string})=>e.kind==='feedback')).toHaveLength(7);
    expect(draft.body.nodes.every((n:{allowedPositionIds:string[];delegationUserIds?:string[]})=>n.allowedPositionIds.length===0&&!n.delegationUserIds?.length)).toBeTruthy();
    expect((await read(context,prefix+'/identities')).items).toEqual(initial);expect((await read(context,prefix+'/positions')).items).toHaveLength(0);
    await page.getByRole('button',{name:/1\.\s*场景与验收基线/}).click();
    await page.getByTestId('workflow-node-phase-p1_work1').fill('本项目的需求阶段');
    await page.getByTestId('save-workflow-draft').click();
    await expect(page.getByTestId('publish-workflow-draft')).toBeEnabled();
    await page.getByTestId('publish-workflow-draft').click();
    await expect(page.locator('.judex-flow-title')).toContainText('已发布');
    const published=(await read(context,prefix+'/workflows/'+flow.id+'/versions')).items.find((v:{state:string})=>v.state==='published');
    expect(published.body.nodes[0].phase).toBe('本项目的需求阶段');expect(published.body.advisoryEdges).toHaveLength(draft.body.advisoryEdges.length);
    expect((await read(context,prefix+'/workflows')).items[0].presetId).toBe(preset.id);
  } finally {await context.close();}
});
