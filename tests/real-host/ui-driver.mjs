// Browser-side actions by the explicitly named test actor. No direct decisions
// via API/SQL; CLI receives the result of the same reviewed browser intent.
import {chromium} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {assertAP1Requirements} from './review-contract.mjs';
import {scenes} from './scene-definitions.mjs';
import {intentFromReceipt} from './intent-envelope.mjs';
const [mode,actorID,value,scene='A']=process.argv.slice(2);
if(!['A','B','C'].includes(scene))throw new Error('Approved scene required');
const root='.cache/real-host/20261012-closure';
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actor=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'))[actorID];
if(!actor)throw new Error('Unknown explicitly selected test actor');
const baseURL=`http://127.0.0.1:${env.port}`;
const browser=await chromium.launch();
const context=await browser.newContext({baseURL,storageState:actor.statePath,viewport:{width:1440,height:1000}});
const page=await context.newPage();
try{
 if(mode==='device'){
  if(!/^[A-Z0-9_-]{4,24}$/i.test(value??''))throw new Error('Observed device code required');
  await page.goto('/device?userCode='+encodeURIComponent(value));
  await page.getByRole('button',{name:'查看授权请求',exact:true}).click();
  await page.getByRole('button',{name:'确认授权',exact:true}).waitFor({state:'visible'});
  const body=await page.locator('body').innerText();
  if(!body.includes(actor.email)||!body.includes('judex-cli@windows'))throw new Error('Device review account/name mismatch');
  const permissions=page.getByRole('checkbox');
  for(let index=0;index<await permissions.count();index++){
    if(!await permissions.nth(index).isChecked())await permissions.nth(index).press('Space');
    if(!await permissions.nth(index).isChecked())throw new Error('Reviewed permission did not become selected');
  }
  await page.getByRole('button',{name:'确认授权',exact:true}).click();
  await page.getByText('操作已完成',{exact:true}).waitFor();
  console.log(actorID+' authorized the observed CLI device through its actual review page');
 }else if(mode==='intent'||mode==='proposal-submit'||mode==='requirements-submit'){
  const file=JSON.parse(fs.readFileSync(value,'utf8'));
  const intent=intentFromReceipt(file);
  const target=new URL(intent.confirmUrl,baseURL);
  if(target.origin!==baseURL||!/^\/confirm\/[0-9a-f-]{36}$/.test(target.pathname))throw new Error('Owned intent link required');
  const project=target.searchParams.get('project');
  const intentPrefix=mode==='intent'?'/api/v1/confirmation-intents':`/api/v1/projects/${project}/confirmation-intents`;
  const response=await context.request.get(intentPrefix+'/'+intent.id);
  const review=(await response.json()).data;
  if(review?.state==='expired')throw new Error('Intent expired; Kimi must request a fresh review of the same project body');
  const expectedTitle=scene==='A'?'MiniIssue A 软件闭环':scenes[scene].title;
  if(mode==='intent'&&(actorID!=='U0'||review.operation!=='project.create'||review.state!=='pending'||review.payload?.title!==expectedTitle||review.payload?.defaultModelId!==env.models.primary))throw new Error('Concrete project creation review differs from approved scene');
  if(mode==='proposal-submit'||mode==='requirements-submit'){
   if(review.operation!=='proposal.submit'||review.state!=='pending'||!project)throw new Error('Current project proposal submit intent required');
   const proposal=(await(await context.request.get(`/api/v1/projects/${project}/proposals/${review.objectId}/review`)).json()).data;
   if(mode==='requirements-submit'){
    assertAP1Requirements(proposal);
    if(proposal.reviewHash!==review.reviewHash)throw new Error('Fixed dependency review changed');
   }else{
   const plans=proposal.changes.filter(c=>c.operation==='create_plan'),newTasks=proposal.changes.filter(c=>c.operation==='create_task'),cancelled=proposal.changes.filter(c=>c.operation==='cancel_task');
   if(proposal.reviewHash!==review.reviewHash||newTasks.length!==6||proposal.changes.length!==(plans.length?7:12)||(!plans.length&&cancelled.length!==6))throw new Error('Fixed six-task proposal review changed');
   if(newTasks.some(c=>!c.fields.planId||!(plans.some(p=>p.clientRef===c.fields.planId)||/^[0-9a-f-]{36}$/.test(c.fields.planId))))throw new Error('Reviewed task owning plan is missing');
   }
  }
  await page.goto(target.href);
  await page.getByTestId('confirm-approve').waitFor({state:'visible'});
  await page.screenshot({path:path.join(root,`${actorID}-${intent.id}-review.png`),fullPage:true});
  const confirming=page.waitForResponse(r=>r.url().endsWith('/confirmation-intents/'+intent.id+'/confirm')&&r.request().method()==='POST');
  await page.getByTestId('confirm-approve').click();
  const confirmed=await confirming;
  if(!confirmed.ok())throw new Error('Reviewed browser confirm failed '+confirmed.status()+' '+await confirmed.text());
  await page.getByTestId('confirm-approve').waitFor({state:'detached'});
  const finished=(await(await context.request.get(intentPrefix+'/'+intent.id)).json()).data;
  if(finished.state!=='committed')throw new Error('Browser did not commit the reviewed project intent');
  const ownerSession=scene==='A'?'KA01':scenes[scene].ownerSession;
  fs.writeFileSync(path.join(root,mode==='intent'?ownerSession+'-project-result.json':`${mode}-${intent.id}-result.json`),JSON.stringify({actor:actorID,intentId:intent.id,state:finished.state,resultRef:finished.resultRef,confirmedVia:'browser'},null,2)+'\n');
  console.log(actorID+' confirmed the exact Kimi-created '+mode+' through the browser');
 }else throw new Error('Use device or intent');
 await context.storageState({path:actor.statePath});
}finally{await context.close();await browser.close();}
