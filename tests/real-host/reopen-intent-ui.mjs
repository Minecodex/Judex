import fs from 'node:fs';
import path from 'node:path';
import {chromium,expect} from '@playwright/test';
const [actorID,receiptPath]=process.argv.slice(2),root='.cache/real-host/20261012-closure';
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actor=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'))[actorID];
const receipt=JSON.parse(fs.readFileSync(receiptPath,'utf8')),intent=receipt.intent?.data??receipt.intent;
if(actorID!=='U4'||!intent||receipt.sourceMaterialVersion!=='26d7f3b9-8a68-4128-b7aa-734b9d46ebfb')throw new Error('Concrete A15-triggered A13 reviewer decision required');
const baseURL=`http://127.0.0.1:${env.port}`,url=new URL(intent.confirmUrl),project=url.searchParams.get('project');
if(url.origin!==baseURL||project!=='21a13532-7929-4c6c-99a7-01e044eee64e'||intent.objectId!=='3309af7c-36cf-4b38-8292-a3d3b232fdf7')throw new Error('Owned scene only');
const browser=await chromium.launch(),context=await browser.newContext({baseURL,storageState:actor.statePath}),page=await context.newPage();
const prefix=`/api/v1/projects/${project}`,read=async suffix=>(await(await context.request.get(prefix+suffix)).json()).data;
try{
 const pending=await read('/confirmation-intents/'+intent.id),task=await read('/tasks/'+intent.objectId);
 if(pending.state!=='pending'||pending.operation!=='task.reopen'||task.status!=='accepted'||task.version!==pending.payload.expectedVersion||task.latestAcceptanceId!==pending.payload.acceptanceId||pending.reviewHash!==task.latestAcceptanceId||pending.payload.reason!==receipt.reason)throw new Error('Accepted-source snapshot changed');
 await page.goto(url.href);await expect(page.getByTestId('confirm-approve')).toBeEnabled();
 const waiting=page.waitForResponse(r=>r.url().endsWith(`/confirmation-intents/${intent.id}/confirm`)&&r.request().method()==='POST');
 await page.getByTestId('confirm-approve').click();const response=await waiting;if(!response.ok())throw new Error('Browser reopen failed');
 await expect(page.getByTestId('confirm-approve')).toHaveCount(0);
 const reopened=await read('/tasks/'+intent.objectId);
 if(reopened.status!=='rework'||reopened.version!==task.version+1)throw new Error('Reopen postcondition failed');
 fs.writeFileSync(path.join(root,'A13-reopened.json'),JSON.stringify({actor:actorID,taskId:task.id,previousVersion:task.version,previousAcceptanceId:task.latestAcceptanceId,currentVersion:reopened.version,status:reopened.status,reason:receipt.reason,confirmedVia:'actual browser frozen reopen page'},null,2)+'\n');
 console.log('U4 reopened A13 through its actual accepted-source review page');
}finally{await context.close();await browser.close();}
