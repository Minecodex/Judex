// Preserve a model receipt and distinguish task originals from independently
// reviewed upstream references. Never change its intent, decision or facts.
import {chromium} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {createHash} from 'node:crypto';
import {intentFromReceipt} from './intent-envelope.mjs';
const [actorID,receiptPath,producerPath]=process.argv.slice(2);
const root='.cache/real-host/20261012-closure',env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actor=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'))[actorID];
const original=fs.readFileSync(receiptPath),receipt=JSON.parse(original),producer=JSON.parse(fs.readFileSync(producerPath,'utf8'));
const intent=intentFromReceipt(receipt),basis=receipt.snapshotBasis;
const baseURL=`http://127.0.0.1:${env.port}`,target=new URL(intent.confirmUrl,baseURL),project=target.searchParams.get('project');
if(!actor||target.origin!==baseURL||producer.task?.taskId!==intent.objectId||basis?.taskStatus!=='delivered'||basis.recommendation!=='accept')throw new Error('Concrete actual reviewer and producer receipt required');
const ids=producer.materials?.map(m=>m.versionId);
if(!ids?.length||ids.some(id=>!basis.materialVersionIds?.includes(id)))throw new Error('All producer originals must already have been independently reviewed');
const browser=await chromium.launch(),context=await browser.newContext({baseURL,storageState:actor.statePath});
try{
 const get=async route=>{const response=await context.request.get(route);if(!response.ok())throw new Error('Current review read failed');return(await response.json()).data;};
 const prefix='/api/v1/projects/'+project;
 const [frozen,task,review]=await Promise.all([get(prefix+'/confirmation-intents/'+intent.id),get(prefix+'/tasks/'+intent.objectId),get(prefix+'/tasks/'+intent.objectId+'/acceptance-review')]);
 if(frozen.state!=='pending'||frozen.payload.decision!=='accept'||frozen.reviewHash!==review.reviewHash||task.status!=='delivered'||task.version!==basis.taskVersion||task.reviewerIdentityId!==basis.reviewerIdentityId||review.blockers?.length)throw new Error('Pending frozen acceptance is stale or blocked');
 const manifest=JSON.stringify(review);
 if(ids.some(id=>!manifest.includes(id)))throw new Error('Actual task originals are not in the frozen product review');
 const supplement={...receipt,snapshotBasis:{...basis,materialVersionIds:ids,reviewedMaterialVersionIds:basis.materialVersionIds,referenceMaterialVersionIds:basis.materialVersionIds.filter(id=>!ids.includes(id))},operatorSupplement:{originalReceiptPath:path.resolve(receiptPath),originalReceiptSha256:createHash('sha256').update(original).digest('hex'),reason:'Task manifest originals and upstream independently reviewed references separated; original model receipt/intent/recommendation and all evidence preserved',reviewHash:review.reviewHash,observedAt:new Date().toISOString()}};
 const output=receiptPath.replace(/\.json$/,'.operator-manifest-supplement.json');
 fs.writeFileSync(output,JSON.stringify(supplement,null,2)+'\n');
 console.log(output);
}finally{await context.close();await browser.close();}
