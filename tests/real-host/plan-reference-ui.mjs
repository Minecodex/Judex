// Exact one-reference review; business data is changed only by actor UI votes.
import fs from 'node:fs';
import path from 'node:path';
import {chromium,expect} from '@playwright/test';
import {intentFromReceipt} from './intent-envelope.mjs';
const [phase,value]=process.argv.slice(2),root='.cache/real-host/20261012-closure';
if(!['submit','approve'].includes(phase))throw new Error('Known phase required');
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'));
const project='21a13532-7929-4c6c-99a7-01e044eee64e',planID='c6308a0a-32c1-4f4b-9717-3755aaa81d00',patch='8b355144-610b-4349-8031-662cb3174ee0',patchPlan='1c27d436-d46b-492f-90d1-7e87181ffedd';
const baseURL=`http://127.0.0.1:${env.port}`,prefix=`/api/v1/projects/${project}`,browser=await chromium.launch();
const contextFor=id=>browser.newContext({baseURL,storageState:actors[id].statePath,viewport:{width:1440,height:1000}});
const get=async(context,suffix)=>{const r=await context.request.get(prefix+suffix);if(!r.ok())throw new Error('Reference read failed '+r.status());return(await r.json()).data;};
function checkedChange(review){
 const c=review.changes[0];
 if(review.changes.length!==1||c.operation!=='reference_task'||c.targetType!=='plan'||c.targetId!==planID||c.expectedVersion<1||c.fields.taskId!==patch||Object.keys(c.fields).some(k=>k!=='taskId'))throw new Error('Exact formal patch reference required');
 return c;
}
async function before(context,review){
 const c=checkedChange(review),p=await get(context,'/plans/'+planID),t=await get(context,'/tasks/'+patch),source=await get(context,'/plans/'+patchPlan);
 if(p.status!=='active'||p.version!==c.expectedVersion||p.referenceTaskIds?.length||t.status!=='accepted'||t.planId!==patchPlan||source.status!=='accepted')throw new Error('Concrete plan/accepted patch ownership changed');
}
try{
 if(phase==='submit'){
  const raw=JSON.parse(fs.readFileSync(value,'utf8')),intent=intentFromReceipt(raw),url=new URL(intent.confirmUrl,baseURL);
  if(url.origin!==baseURL||url.searchParams.get('project')!==project)throw new Error('Owned intent required');
  const context=await contextFor('U0');try{
   const frozen=await get(context,'/confirmation-intents/'+intent.id),review=await get(context,'/proposals/'+frozen.objectId+'/review');
   if(frozen.operation!=='proposal.submit'||frozen.state!=='pending'||review.status!=='draft'||frozen.reviewHash!==review.reviewHash)throw new Error(`Pending frozen submit required: intent=${frozen.state}, proposal=${review.status}, hashMatches=${frozen.reviewHash===review.reviewHash}`);await before(context,review);
   const page=await context.newPage();await page.goto(url.href);const approve=page.getByTestId('confirm-approve');await expect(approve).toBeEnabled();await expect(page.locator('body')).toContainText(actors.U0.name);
   await page.screenshot({path:path.join(root,'A-P1-reference-submit-'+intent.id+'.png'),fullPage:true});
   const waiting=page.waitForResponse(r=>r.url().endsWith(`/confirmation-intents/${intent.id}/confirm`)&&r.request().method()==='POST');await approve.click();if(!(await waiting).ok())throw new Error('Reference submit failed');await expect(approve).toHaveCount(0);console.log('U0 submitted the exact one-reference model draft through the fixed browser');
  }finally{await context.close();}
 }else{
  if(!/^[0-9a-f-]{36}$/.test(value??''))throw new Error('Observed proposal UUID required');
  const file=path.join(root,'A-P1-reference-'+value+'-decisions.json'),decisions=fs.existsSync(file)?JSON.parse(fs.readFileSync(file,'utf8')):[];
  for(const id of ['U0','U1','U2','U3','U4']){
   const context=await contextFor(id);try{
    const review=await get(context,'/proposals/'+value+'/review');checkedChange(review);if(review.status==='approved')break;if(review.status!=='pending')throw new Error('Pending reference review required');await before(context,review);
    const slots=review.slots.filter(s=>s.canDecide&&s.state==='pending');if(!slots.length)continue;
    const page=await context.newPage();await page.goto(`/projects/${project}?tab=decisions&view=proposal&item=${value}`);const approve=page.getByTestId('approve-proposal-'+value);await expect(approve).toBeEnabled();
    await page.screenshot({path:path.join(root,`A-P1-reference-${value}-${id}.png`),fullPage:true});const waiting=page.waitForResponse(r=>r.url().endsWith(`/proposals/${value}/decisions`)&&r.request().method()==='POST');await approve.click();if(!(await waiting).ok())throw new Error('Reference actor decision failed');
    decisions.push({actor:id,reviewHash:review.reviewHash,slotIds:slots.map(s=>s.id),via:'actual fixed actor browser'});fs.writeFileSync(file,JSON.stringify(decisions,null,2)+'\n');console.log(id+' approved its own reference slots');
   }finally{await context.close();}
  }
  const context=await contextFor('U0');try{
   const review=await get(context,'/proposals/'+value+'/review'),c=checkedChange(review),plan=await get(context,'/plans/'+planID),task=await get(context,'/tasks/'+patch);
   if(review.status!=='approved'||plan.status!=='active'||plan.version!==c.expectedVersion+1||JSON.stringify(plan.referenceTaskIds)!==JSON.stringify([patch])||task.planId!==patchPlan||task.status!=='accepted')throw new Error('Reference postcondition failed');
   fs.writeFileSync(path.join(root,'A-P1-reference-approved.json'),JSON.stringify({proposalId:value,planId:planID,version:plan.version,referenceTaskIds:plan.referenceTaskIds,owningPlanUnchanged:task.planId,decisions,confirmedAt:new Date().toISOString()},null,2)+'\n');console.log('Accepted A24 is formally referenced by active A-P1 and remains owned by accepted A-P2');
  }finally{await context.close();}
 }
}finally{await browser.close();}
