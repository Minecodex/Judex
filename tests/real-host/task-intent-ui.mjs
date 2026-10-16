// A named synthetic reviewer acts through the product's fixed-review page.
// Read-only API assertions bind the click to the independently reviewed file.
import {chromium,expect} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {intentFromReceipt} from './intent-envelope.mjs';
import {scenes} from './scene-definitions.mjs';
const [actorID,receiptPath,scene='A']=process.argv.slice(2);
if(!['A','B','C'].includes(scene))throw new Error('Approved scene required');
const folder=scene==='A'?'A-MiniIssue':scenes[scene].folder;
const root='.cache/real-host/20261012-closure';
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actor=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'))[actorID];
const receipt=JSON.parse(fs.readFileSync(receiptPath,'utf8'));
const intent=intentFromReceipt(receipt);
const basis=receipt.snapshotBasis??(receipt.task?.snapshotBasis?{...receipt.task.snapshotBasis,reviewerIdentityId:receipt.session?.reviewerIdentityId,materialVersionIds:receipt.materialVersionIds,reviewDoc:receipt.reviewDoc,recommendation:receipt.recommendation?.startsWith('accept')?'accept':'reject'}:null);
if(!actor||!basis||basis.taskStatus!=='delivered'||basis.taskId!==intent.objectId)throw new Error('Named actor and exact independently reviewed task required');
const baseURL=`http://127.0.0.1:${env.port}`;
const target=new URL(intent.confirmUrl,baseURL);
const project=target.searchParams.get('project');
if(target.origin!==baseURL||!/^\/confirm\/[0-9a-f-]{36}$/.test(target.pathname)||!project)throw new Error('Owned project intent required');
const browser=await chromium.launch();
const context=await browser.newContext({baseURL,storageState:actor.statePath,viewport:{width:1440,height:1000}});
const page=await context.newPage();
const get=async route=>{
 const response=await context.request.get(route);
 if(!response.ok())throw new Error(`Review read failed ${response.status()} ${route}`);
 return (await response.json()).data;
};
try{
 const prefix=`/api/v1/projects/${project}`;
 const frozen=await get(`${prefix}/confirmation-intents/${intent.id}`);
 const decision=frozen.payload.decision;
 if(frozen.operation!=='task.acceptance'||frozen.state!=='pending'||!['accept','reject'].includes(decision)||frozen.objectId!==intent.objectId)throw new Error('Pending decision on the reviewed task required');
 if(decision==='reject'&&(!frozen.payload.reason?.trim()||basis.recommendation!=='reject'))throw new Error('Independent concrete rejection reason required');
 const task=await get(`${prefix}/tasks/${intent.objectId}`);
 const evidence=await get(`${prefix}/tasks/${intent.objectId}/acceptance-review`);
 const team=JSON.parse(fs.readFileSync(path.join(env.localRoot,folder,'team-context.json'),'utf8'));
 if((team.projectID??team.projectId)!==project)throw new Error('Reviewed intent belongs to another approved scene');
 const identities=(await get(`${prefix}/identities`)).items;
 if(!identities.some(i=>i.id===basis.reviewerIdentityId&&i.currentBinding?.userId===actor.userId))throw new Error('Actor does not hold the current reviewer identity');
 if(task.status!=='delivered'||task.version!==basis.taskVersion||task.reviewerIdentityId!==basis.reviewerIdentityId||frozen.payload.expectedVersion!==task.version)throw new Error('Current task/reviewer differs from independent review');
 // Each read has its own review record ID. The canonical manifest hash and
 // target version determine whether the intent still reviews the same facts.
 if(evidence.reviewHash!==frozen.reviewHash||evidence.targetVersion!==task.version||!frozen.payload.reviewId)throw new Error('Fixed evidence changed');
 const materialIDs=basis.materialVersionIds??[basis.materialVersionId];
 if((decision==='accept'&&(evidence.blockers??[]).length)||materialIDs.some(id=>!id||!JSON.stringify(evidence).includes(id)))throw new Error('Reviewed originals missing or acceptance blocked');
 await page.goto(target.href);
 const approve=page.getByTestId('confirm-approve');
 await expect(approve).toBeEnabled();
 await expect(page.locator('body')).toContainText(actor.name);
 await page.screenshot({path:path.join(root,`${actorID}-${intent.objectId}-accept-review.png`),fullPage:true});
 const confirming=page.waitForResponse(r=>r.url().endsWith(`/confirmation-intents/${intent.id}/confirm`)&&r.request().method()==='POST');
 await approve.click();
 const confirmed=await confirming;
 if(!confirmed.ok())throw new Error('Browser confirmation failed '+confirmed.status()+' '+await confirmed.text());
 await expect(approve).toHaveCount(0);
 const committed=await get(`${prefix}/confirmation-intents/${intent.id}`);
 const decided=await get(`${prefix}/tasks/${intent.objectId}`);
 if(committed.state!=='committed'||decided.status!==(decision==='accept'?'accepted':'rework')||(decision==='accept'&&!decided.latestAcceptanceId)||decided.version!==basis.taskVersion+1)throw new Error('Human decision postcondition failed');
 fs.writeFileSync(path.join(root,`task-${decision}-${intent.objectId}.json`),JSON.stringify({actor:actorID,projectId:project,taskId:decided.id,taskVersion:decided.version,status:decided.status,decision,reason:frozen.payload.reason??null,acceptanceId:decided.latestAcceptanceId,intentId:intent.id,reviewHash:evidence.reviewHash,materialVersionIds:materialIDs,confirmedVia:'actual browser fixed-review page',confirmedAt:new Date().toISOString()},null,2)+'\n');
 console.log(`${actorID} ${decision} ${decided.title} through the fixed-review browser page`);
}finally{await context.close();await browser.close();}
