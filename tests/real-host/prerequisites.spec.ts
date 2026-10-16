import {test,expect} from '@playwright/test';
import {randomUUID} from 'node:crypto';
import fs from 'node:fs';
import {chooseValue} from '../e2e/workspace-helpers';

// Controlled UI contract fixture, distinct from the Kimi/model business scenes.
test('role multi-bind editing retains every binding and the selected real catalog model',async({page,context},info)=>{
  const env=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/environment.json','utf8'));
  const registered=await context.request.post('/api/v1/auth/register',{data:{displayName:'Contract owner',email:randomUUID()+'@contract.judex.test',password:'contract-password-123'}});
  expect(registered.status()).toBe(201);
  const session=(await(await context.request.get('/api/v1/auth/session')).json()).data;
  const write=async(path:string,data:unknown)=>{
    const response=await context.request.post('/api/v1'+path,{data,headers:{'X-CSRF-Token':session.csrfToken,'Idempotency-Key':randomUUID()}});
    expect(response.ok(),path+' '+await response.text()).toBeTruthy();return (await response.json()).data;
  };
  const read=async(path:string)=>(await(await context.request.get('/api/v1'+path)).json()).data;
  const project=await write('/projects',{title:'[contract] role binding UI',defaultModelId:env.models.primary});
  const root='/projects/'+project.id;
  await page.goto('/?project='+project.id+'&settings=team');
  await page.getByTestId('new-position').click();
  await page.getByTestId('position-name').fill('Contract backend');
  await page.getByTestId('position-prompt').fill('Implementation and performance verification');
  await chooseValue(page,'position-model',env.models.position);
  await page.getByTestId('save-position').click();await expect(page.getByRole('dialog')).toHaveCount(0);
  const initialPosition=(await read(root+'/positions')).items[0];expect(initialPosition.nodeBindings).toHaveLength(0);
  const flows:any[]=[];
  for(const title of ['Contract software','Contract performance']){
    const flow=await write(root+'/workflows',{name:title,instructions:'Controlled UI contract',nodes:[{id:'dev',name:'Develop',responsibility:'Implement',allowedPositionIds:[],defaultApprovalPolicy:'all'},{id:'qa',name:'Review',responsibility:'Verify',allowedPositionIds:[],defaultApprovalPolicy:'all'}],advisoryEdges:[{from:'dev',to:'qa'}]});
    const draft=(await read(root+'/workflows/'+flow.id+'/versions')).items.find((v:any)=>v.state==='draft');
    await write(root+'/workflows/'+flow.id+'/publish',{expectedVersion:flow.version,draftHash:draft.draftHash});flows.push(flow);
  }
  await page.goto('/?project='+project.id+'&settings=team');
  await page.getByTestId('edit-position-'+initialPosition.id).click();
  await page.getByTestId('position-binding-add').click();
  await chooseValue(page,'position-flow',flows[0].id);await chooseValue(page,'position-node','dev');
  await page.getByTestId('position-binding-add').click();
  await chooseValue(page,'position-flow-1',flows[1].id);await chooseValue(page,'position-node-1','qa');
  await page.getByTestId('save-position').click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  const position=(await read(root+'/positions')).items[0];
  expect(position.modelId).toBe(env.models.position);
  expect(position.nodeBindings).toHaveLength(2);
  await page.getByTestId('edit-position-'+position.id).click();
  await expect(page.getByTestId('position-binding-0')).toBeVisible();await expect(page.getByTestId('position-binding-1')).toBeVisible();
  await page.getByTestId('position-prompt').fill('Updated responsibility, preserving every binding');
  await page.getByTestId('save-position').click();await expect(page.getByRole('dialog')).toHaveCount(0);
  const saved=(await read(root+'/positions')).items[0];
  expect(saved.nodeBindings).toEqual(position.nodeBindings);expect(saved.modelId).toBe(position.modelId);
  for(const [locale,theme] of [['zh-CN','light'],['en','light'],['zh-CN','dark'],['en','dark']]){
    await page.evaluate(({locale,theme})=>{localStorage.setItem('argus.locale',locale);localStorage.setItem('judex.theme',theme);},{locale,theme});
    await page.reload();await page.getByTestId('edit-position-'+position.id).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme',theme);
    await expect(page.locator('html')).toHaveAttribute('lang',locale);
    await expect(page.getByTestId('position-binding-1')).toBeVisible();
    await page.screenshot({path:info.outputPath(`role-bindings-${locale}-${theme}.png`),fullPage:true});
    await page.getByTestId('save-position').click();await expect(page.getByRole('dialog')).toHaveCount(0);
  }
});
