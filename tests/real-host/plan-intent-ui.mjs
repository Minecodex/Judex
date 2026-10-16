// A named synthetic plan owner separately confirms the reviewed business plan.
import {chromium,expect} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {intentFromReceipt} from './intent-envelope.mjs';
import {scenes} from './scene-definitions.mjs';
const [actorID,receiptPath,scene='A']=process.argv.slice(2),root='.cache/real-host/20261012-closure';
if(!['A','B','C'].includes(scene))throw new Error('Approved scene required');
const folder=scene==='A'?'A-MiniIssue':scenes[scene].folder;
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actor=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'))[actorID];
const receipt=JSON.parse(fs.readFileSync(receiptPath,'utf8'));
const intent=intentFromReceipt(receipt);
const basis=receipt.snapshotBasis;
// Exact approved scene contracts, including the formally referenced patch.
// The reference remains owned by A-P2 and is not another business sample.
const patchTask='8b355144-610b-4349-8031-662cb3174ee0';
const contracts={
 'c6308a0a-32c1-4f4b-9717-3755aaa81d00':{snapshot:'A-P1-approved.json',owned:6,references:[patchTask]},
 '1c27d436-d46b-492f-90d1-7e87181ffedd':{snapshot:'A-P2-approved.json',owned:4,references:[]},
 'a82fd017-a411-47e1-975e-ff4565070273':{snapshot:'A-P3-approved.json',owned:3,references:[]},
 '37e495cf-7b96-49aa-848e-5ea92ae38660':{snapshot:'A-P4-approved.json',owned:3,references:[]},
};
const contractsByCode={'B-P1':3,'B-P2':2,'C-P1':3,'C-P2':2};
if(scene==='A'&&fs.existsSync(path.join(root,'A-P5-approved.json'))){
 const approved=JSON.parse(fs.readFileSync(path.join(root,'A-P5-approved.json'),'utf8'));
 if(!approved.plan.title.startsWith('A-P5 ')||approved.tasks.length!==4||approved.tasks.some(t=>t.planId!==approved.plan.id))throw new Error('Exact approved combined plan required');
 contracts[approved.plan.id]={snapshot:'A-P5-approved.json',owned:4,references:[patchTask,'558dea8f-1473-4618-b6c4-fc7e64d76869','f118d0a6-cb9c-42fa-8a78-60947a1bf739','ab8a25ab-e64a-4be0-9dd7-36202c018b82']};
}
if(scene!=='A')for(const [code,owned]of Object.entries(contractsByCode).filter(([code])=>code.startsWith(scene+'-'))){
 const snapshot=code+'-approved.json',file=path.join(root,snapshot);
 if(fs.existsSync(file)){
  const approved=JSON.parse(fs.readFileSync(file,'utf8'));
  if(!approved.plan.title.startsWith(code+' ')||approved.tasks.length!==owned||approved.tasks.some(t=>t.planId!==approved.plan.id))throw new Error('Formal scene plan differs from its approved task contract');
  contracts[approved.plan.id]={snapshot,owned,references:[]};
 }
}
const contract=contracts[intent.objectId];
if(!actor||!contract||basis?.recommendation!=='accept'||!basis.productChecksPassed||!Array.isArray(basis.tasks))throw new Error('Concrete approved scene product and owner review required');
const work=JSON.parse(fs.readFileSync(path.join(root,contract.snapshot),'utf8'));
const expectedIds=[...work.tasks.filter(t=>t.planId===intent.objectId).map(t=>t.id),...contract.references];
const sorted=ids=>JSON.stringify([...ids].sort());
if(work.plan.id!==intent.objectId||work.tasks.length!==contract.owned||new Set(expectedIds).size!==contract.owned+contract.references.length||sorted(basis.tasks.map(t=>t.taskId))!==sorted(expectedIds))throw new Error('The concrete plan must review every owned task and exact formal reference');
const baseURL=`http://127.0.0.1:${env.port}`,target=new URL(intent.confirmUrl,baseURL),project=target.searchParams.get('project');
if(target.origin!==baseURL||!/^\/confirm\/[0-9a-f-]{36}$/.test(target.pathname)||!project)throw new Error('Owned project intent required');
const browser=await chromium.launch(),context=await browser.newContext({baseURL,storageState:actor.statePath,viewport:{width:1440,height:1000}}),page=await context.newPage();
const get=async route=>{const response=await context.request.get(route);if(!response.ok())throw new Error('Owner review read failed '+response.status());return(await response.json()).data;};
try{
 const prefix=`/api/v1/projects/${project}`,frozen=await get(`${prefix}/confirmation-intents/${intent.id}`),plan=await get(`${prefix}/plans/${intent.objectId}`),review=await get(`${prefix}/plans/${intent.objectId}/acceptance-review`);
 const team=JSON.parse(fs.readFileSync(path.join(env.localRoot,folder,'team-context.json'),'utf8'));
 const identities=(await get(`${prefix}/identities`)).items;
 if((team.projectID??team.projectId)!==project||work.projectId!==project)throw new Error('Plan intent belongs to another approved scene');
 if(frozen.operation!=='plan.acceptance'||frozen.state!=='pending'||frozen.payload.decision!=='accept'||plan.status!=='active'||plan.version!==basis.planVersion||plan.ownerIdentityId!==basis.ownerIdentityId||!identities.some(i=>i.id===basis.ownerIdentityId&&i.currentBinding?.userId===actor.userId))throw new Error('Current owner or plan differs from the independent review');
 if(review.reviewHash!==frozen.reviewHash||review.targetVersion!==basis.planVersion||frozen.payload.expectedVersion!==basis.planVersion||(review.blockers??[]).length)throw new Error('Fixed plan review changed or blocked');
 const manifest=typeof review.manifest==='string'?JSON.parse(review.manifest):review.manifest;
 if(sorted(plan.referenceTaskIds??[])!==sorted(contract.references)||sorted(manifest.tasks?.map(t=>t.taskId)??[])!==sorted(expectedIds)||review.taskAcceptanceIds?.length!==expectedIds.length)throw new Error('All exact owned/referenced tasks must have separate acceptances');
 for(const task of manifest.tasks){
  const checked=basis.tasks.find(t=>t.taskId===task.taskId);
  if(task.status!=='accepted'||task.executionException||!task.acceptanceId||!checked||checked.version!==task.version||checked.acceptanceId!==task.acceptanceId)throw new Error('Task acceptance changed since the concrete product review');
 }
 await page.goto(target.href);const approve=page.getByTestId('confirm-approve');await expect(approve).toBeEnabled();await expect(page.locator('body')).toContainText(actor.name);
 await page.screenshot({path:path.join(root,`${actorID}-${intent.objectId}-plan-review.png`),fullPage:true});
 const confirming=page.waitForResponse(r=>r.url().endsWith(`/confirmation-intents/${intent.id}/confirm`)&&r.request().method()==='POST');await approve.click();
 const response=await confirming;if(!response.ok())throw new Error('Owner browser confirmation failed '+response.status());await expect(approve).toHaveCount(0);
 const committed=await get(`${prefix}/confirmation-intents/${intent.id}`),decided=await get(`${prefix}/plans/${intent.objectId}`);
 if(committed.state!=='committed'||decided.status!=='accepted'||decided.version!==basis.planVersion+1||!decided.latestAcceptanceId)throw new Error('Separate plan acceptance postcondition failed');
 fs.writeFileSync(path.join(root,`plan-accept-${decided.id}.json`),JSON.stringify({actor:actorID,projectId:project,planId:decided.id,status:decided.status,version:decided.version,acceptanceId:decided.latestAcceptanceId,intentId:intent.id,taskAcceptanceIds:review.taskAcceptanceIds,reviewHash:review.reviewHash,productSources:basis.materialVersionIds,confirmedVia:'actual plan owner browser fixed review',confirmedAt:new Date().toISOString()},null,2)+'\n');
 console.log(actorID+' separately accepted '+decided.title+' through the fixed plan review');
}finally{await context.close();await browser.close();}
