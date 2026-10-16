// Each real model proposal adds one fixed accepted software reference.
import fs from 'node:fs';
import path from 'node:path';
import {chromium,expect} from '@playwright/test';
import {intentFromReceipt} from './intent-envelope.mjs';
const [phase,value,code]=process.argv.slice(2),root='.cache/real-host/20261012-closure';
const references={A24:['8b355144-610b-4349-8031-662cb3174ee0','1c27d436-d46b-492f-90d1-7e87181ffedd'],A32:['558dea8f-1473-4618-b6c4-fc7e64d76869','a82fd017-a411-47e1-975e-ff4565070273'],A33:['f118d0a6-cb9c-42fa-8a78-60947a1bf739','a82fd017-a411-47e1-975e-ff4565070273'],A43:['ab8a25ab-e64a-4be0-9dd7-36202c018b82','37e495cf-7b96-49aa-848e-5ea92ae38660']};
if(!['submit','approve'].includes(phase)||!references[code])throw new Error('Approved reference phase/source required');
const environment=JSON.parse(fs.readFileSync(root+'/environment.json','utf8')),actors=JSON.parse(fs.readFileSync(root+'/actors/actors.json','utf8'));
const work=JSON.parse(fs.readFileSync(root+'/A-P5-approved.json','utf8'));
const project='21a13532-7929-4c6c-99a7-01e044eee64e',plan=work.plan.id,[task,owningPlan]=references[code];
if(work.projectId!==project||!work.plan.title.startsWith('A-P5 ')||work.tasks.length!==4||work.tasks.some(t=>t.planId!==plan))throw new Error('Actual approved four-task plan required');
const baseURL=`http://127.0.0.1:${environment.port}`,prefix='/api/v1/projects/'+project,browser=await chromium.launch();
const contextFor=id=>browser.newContext({baseURL,storageState:actors[id].statePath,viewport:{width:1440,height:1000}});
const get=async(context,suffix)=>{const response=await context.request.get(prefix+suffix).catch(()=>{throw new Error('Reference read transport failed; private details omitted');});if(!response.ok())throw new Error('Reference read HTTP '+response.status());return(await response.json()).data;};
const check=review=>{
 const change=review.changes[0];
 if(review.changes.length!==1||change.operation!=='reference_task'||change.targetType!=='plan'||change.targetId!==plan||change.expectedVersion<1||change.fields.taskId!==task||Object.keys(change.fields).some(k=>k!=='taskId'))throw new Error('Exact one-reference model draft required');return change;
};
const before=async(context,review)=>{
 const change=check(review),target=await get(context,'/plans/'+plan),source=await get(context,'/tasks/'+task),owner=await get(context,'/plans/'+owningPlan);
 if(target.status!=='active'||target.version!==change.expectedVersion||(target.referenceTaskIds??[]).includes(task)||source.status!=='accepted'||source.planId!==owningPlan||owner.status!=='accepted'||(target.referenceTaskIds??[]).some(id=>!Object.values(references).some(r=>r[0]===id)))throw new Error('Current formal source/plan version differs');return target;
};
const sorted=ids=>JSON.stringify([...ids].sort());
try{
 if(phase==='submit'){
  const intent=intentFromReceipt(JSON.parse(fs.readFileSync(value,'utf8'))),url=new URL(intent.confirmUrl,baseURL);
  if(url.origin!==baseURL||url.searchParams.get('project')!==project)throw new Error('Owned exact intent required');
  const context=await contextFor('U0');try{
   const frozen=await get(context,'/confirmation-intents/'+intent.id),review=await get(context,'/proposals/'+frozen.objectId+'/review');
   if(frozen.operation!=='proposal.submit'||frozen.state!=='pending'||review.status!=='draft'||frozen.reviewHash!==review.reviewHash)throw new Error('Pending frozen submission required');await before(context,review);
   const page=await context.newPage();await page.goto(url.href);const approve=page.getByTestId('confirm-approve');await expect(approve).toBeEnabled();await expect(page.locator('body')).toContainText(actors.U0.name);
   await page.screenshot({path:root+`/A-P5-reference-${code}-submit-${intent.id}.png`,fullPage:true});
   const pending=page.waitForResponse(r=>r.url().endsWith(`/confirmation-intents/${intent.id}/confirm`)&&r.request().method()==='POST');await approve.click();if(!(await pending).ok())throw new Error('Fixed reference submission failed');await expect(approve).toHaveCount(0);
   console.log('U0 submitted exact '+code+' reference through fixed browser');
  }finally{await context.close();}
 }else{
  if(!/^[a-f0-9-]{36}$/.test(value??''))throw new Error('Observed proposal UUID required');const decisions=[];
  const initialContext=await contextFor('U0');let initial;try{initial=await before(initialContext,await get(initialContext,'/proposals/'+value+'/review'));}finally{await initialContext.close();}
  for(const actor of ['U0','U1','U2','U3','U4','U5']){
   const context=await contextFor(actor);try{
    const review=await get(context,'/proposals/'+value+'/review');check(review);if(review.status==='approved')break;if(review.status!=='pending')throw new Error('Pending reference review required');await before(context,review);
    const slots=review.slots.filter(s=>s.canDecide&&s.state==='pending');if(!slots.length)continue;
    const page=await context.newPage();await page.goto(`/projects/${project}?tab=decisions&view=proposal&item=${value}`);const approve=page.getByTestId('approve-proposal-'+value);await expect(approve).toBeEnabled();
    await page.screenshot({path:root+`/A-P5-reference-${code}-${value}-${actor}.png`,fullPage:true});
    const pending=page.waitForResponse(r=>r.url().endsWith(`/proposals/${value}/decisions`)&&r.request().method()==='POST');await approve.click();if(!(await pending).ok())throw new Error('Reference actor decision failed');
    decisions.push({actor,reviewHash:review.reviewHash,slotIds:slots.map(s=>s.id),via:'actual named fixed browser'});
    fs.writeFileSync(root+`/A-P5-reference-${code}-${value}-decisions.json`,JSON.stringify(decisions,null,2)+'\n');
   }finally{await context.close();}
  }
  const context=await contextFor('U0');try{
   const review=await get(context,'/proposals/'+value+'/review'),change=check(review),target=await get(context,'/plans/'+plan),source=await get(context,'/tasks/'+task);
   if(review.status!=='approved'||target.status!=='active'||target.version!==change.expectedVersion+1||sorted(target.referenceTaskIds??[])!==sorted([...(initial.referenceTaskIds??[]),task])||source.status!=='accepted'||source.planId!==owningPlan)throw new Error('Reference postcondition failed');
   fs.writeFileSync(root+`/A-P5-reference-${code}-approved.json`,JSON.stringify({proposalId:value,planId:plan,version:target.version,referenceTaskIds:target.referenceTaskIds,sourceTaskId:task,owningPlanUnchanged:source.planId,decisions,confirmedAt:new Date().toISOString()},null,2)+'\n');
   console.log(code+' formally referenced; original task ownership and acceptance preserved');
  }finally{await context.close();}
 }
}finally{await browser.close();}
