// Review the actual model draft, then use named actors' product UI decisions.
import {chromium,expect} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {intentFromReceipt} from './intent-envelope.mjs';
const [phase,value,kind='creation',scene='A-P2']=process.argv.slice(2);
if(!['submit','approve'].includes(phase)||!['creation','dependencies'].includes(kind)||!['A-P2','A-P3','A-P4','A-P5'].includes(scene))throw new Error('Known scene review phase required');
const root='.cache/real-host/20261012-closure',env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'));
const project='21a13532-7929-4c6c-99a7-01e044eee64e',baseURL=`http://127.0.0.1:${env.port}`,prefix=`/api/v1/projects/${project}`;
const product='2a2888fd-4c5b-457b-a622-33b358ca640c',qaA='af739ffe-a283-4caf-b0ab-67e903e55704',qaB='846abe01-c033-40a1-8b80-63102eacf4ca',backend='4b02a3ee-4c71-488a-a985-827d641558f7',ops='b861062d-ef8c-48ea-a591-0445000bb32d';
const frontend='ad102867-50a6-4399-8567-f3d6a8db71c4';
const contracts={
 'A-P2':{workflow:'217ef1ef-c862-5dd1-9bb1-83e5633d7fea',expected:{A21:[qaA,product,'aw2-repro'],A22:[backend,qaB,'aw2-fix'],A23:[qaB,product,'aw2-reg'],A24:[ops,product,'aw2-patch']},dependency:'6e009085-d73c-4fb5-bc03-8328915abf7b'},
 'A-P3':{workflow:'a56f66a0-ef11-5797-b244-34a18d43c322',expected:{A31:[qaA,product,'aw3-base'],A32:[backend,qaB,'aw3-opt'],A33:[qaB,product,'aw3-compare']},dependency:'8b355144-610b-4349-8031-662cb3174ee0'},
 'A-P4':{workflow:'4b1c2af6-6945-59ce-a04f-43c852313146',expected:{A41:[ops,product,'aw4-diag'],A42:[backend,qaB,'aw4-restore'],A43:[qaA,product,'aw4-review']},dependency:'f118d0a6-cb9c-42fa-8a78-60947a1bf739'},
 'A-P5':{workflow:'c56402f6-f6b8-51ba-9604-288809f935c2',expected:{A51:[frontend,qaB,'aw1-fe'],A52:[product,qaA,'aw1-int'],A53:[[qaA,qaB],product,'aw1-test-a'],A54:[ops,product,'aw1-build']},dependency:'8b355144-610b-4349-8031-662cb3174ee0'},
};
const {workflow,expected,dependency:initialDependency}=contracts[scene];
const codes=Object.keys(expected),initial=codes[0];
function assertReview(review){
 if(kind==='creation'){
  const plans=review.changes.filter(c=>c.operation==='create_plan'),tasks=review.changes.filter(c=>c.operation==='create_task');
  if(review.changes.length!==codes.length+1||plans.length!==1||tasks.length!==codes.length)throw new Error('Exact scene plan and task count required');
  const plan=plans[0];if(!plan.fields.title.startsWith(scene+' ')||plan.fields.ownerIdentityId!==product||plan.fields.workflowId!==workflow||!plan.clientRef||!plan.fields.goal?.trim()||!plan.fields.acceptanceCriteria?.trim())throw new Error('Scene owner/workflow/goal differs');
  for(const [code,[participant,reviewer,node]] of Object.entries(expected)){
   const found=tasks.filter(c=>c.fields.title.startsWith(code+' '));if(found.length!==1)throw new Error('One actual task per scene code required '+code);
   const f=found[0].fields;
   const content=f.expectedOutput+' '+f.acceptanceCriteria;
   const relevant=scene==='A-P2'?/页码|page/i:scene==='A-P3'?/性能|p95|基准/i:scene==='A-P4'?/故障|恢复|诊断|隔离/:/综合|集成|发布|交付|回归|质量/;
   const participants=Array.isArray(participant)?participant:[participant];
   if(f.planId!==plan.clientRef||f.workflowId!==workflow||f.nodeId!==node||f.reviewerIdentityId!==reviewer||JSON.stringify([...f.participantIdentityIds].sort())!==JSON.stringify([...participants].sort())||!f.expectedOutput?.trim()||!f.acceptanceCriteria?.trim()||!relevant.test(content))throw new Error('Reviewed ownership, role, source or scope differs '+code);
   if(scene==='A-P5'&&code==='A53'&&(!/独立|两份|两名/.test(content)||!new RegExp(qaA).test(JSON.stringify(f))||!new RegExp(qaB).test(JSON.stringify(f))))throw new Error('Two separate required QA contributions required');
   const planScope=plan.fields.goal+' '+plan.fields.acceptanceCriteria;
   if(scene==='A-P4'&&code==='A41'&&(!/127\.0\.0\.1:18482/.test(planScope)||!/fault\//.test(planScope)||!/不.*(?:正常|生产|其他任何实例)/.test(planScope)))throw new Error('Exact inherited plan fault-service and exclusion boundary required');
   if(scene==='A-P3'&&code==='A31'&&(!/100000|100,000|10万/.test(content)||!/20\s*%/.test(content)||!/10\s*%/.test(content)))throw new Error('Predeclared baseline and performance gates missing');
   if(code===initial){
    const r=f.requirements;if(r?.length!==1||r[0].kind!=='task_acceptance'||r[0].phase!=='start'||r[0].hard!==true||r[0].targetId!==initialDependency)throw new Error('Initial task must require the exact accepted release source');
   }else if(f.requirements?.length)throw new Error('Later dependencies require actual generated task IDs');
  }
 }else{
  const work=JSON.parse(fs.readFileSync(path.join(root,scene+'-approved.json'),'utf8'));
  const byCode=code=>work.tasks.find(t=>t.title.startsWith(code+' '));
  if(review.changes.length!==codes.length-1)throw new Error('Exact existing-task dependencies required');
  for(const [target,source] of codes.slice(1).map((code,i)=>[code,codes[i]])){
   const task=byCode(target),c=review.changes.find(c=>c.targetId===task.id),r=c?.fields.requirements;
   if(c?.operation!=='set_requirements'||c.targetType!=='task'||Object.keys(c.fields).some(k=>k!=='requirements')||r?.length!==1||r[0].targetId!==byCode(source).id||r[0].kind!=='task_acceptance'||r[0].phase!=='start'||r[0].hard!==true||!r[0].label?.trim())throw new Error('Actual A-P2 task dependency differs '+target);
  }
 }
}
const browser=await chromium.launch();
const contextFor=id=>browser.newContext({baseURL,storageState:actors[id].statePath,viewport:{width:1440,height:1000}});
const read=async(context,suffix)=>{const response=await context.request.get(prefix+suffix);if(!response.ok())throw new Error('Scene review read failed '+response.status());return(await response.json()).data;};
try{
 if(phase==='submit'){
  const file=JSON.parse(fs.readFileSync(value,'utf8')),intent=intentFromReceipt(file);
  const url=new URL(intent.confirmUrl,baseURL);if(url.origin!==baseURL||url.searchParams.get('project')!==project)throw new Error('Owned scene intent required');
  const context=await contextFor('U0');try{
   const frozen=await read(context,`/confirmation-intents/${intent.id}`),review=await read(context,`/proposals/${frozen.objectId}/review`);
   if(frozen.operation!=='proposal.submit'||frozen.state!=='pending'||review.status!=='draft'||frozen.reviewHash!==review.reviewHash)throw new Error('Pending frozen submission required');assertReview(review);
   const page=await context.newPage();await page.goto(url.href);const approve=page.getByTestId('confirm-approve');await expect(approve).toBeEnabled();await expect(page.locator('body')).toContainText(actors.U0.name);
   await page.screenshot({path:path.join(root,`${scene}-${kind}-submit-${intent.id}.png`),fullPage:true});
   const waiting=page.waitForResponse(r=>r.url().endsWith(`/confirmation-intents/${intent.id}/confirm`)&&r.request().method()==='POST');await approve.click();if(!(await waiting).ok())throw new Error('Scene submission confirmation failed');await expect(approve).toHaveCount(0);
   if((await read(context,`/confirmation-intents/${intent.id}`)).state!=='committed')throw new Error('Submission did not commit');
   console.log('U0 submitted the exact model '+scene+' '+kind+' draft via frozen browser confirmation');
  }finally{await context.close();}
 }else{
  if(!/^[0-9a-f-]{36}$/.test(value??''))throw new Error('Observed proposal UUID required');const decisions=[];
  for(const actorID of ['U0','U1','U2','U3','U4']){
   const context=await contextFor(actorID);try{
    const review=await read(context,`/proposals/${value}/review`);if(review.status==='approved')break;if(review.status!=='pending')throw new Error('Pending actual A-P2 review required');assertReview(review);
    const slots=review.slots.filter(s=>s.canDecide&&s.state==='pending');if(!slots.length)continue;
    const page=await context.newPage();await page.goto(`/projects/${project}?tab=decisions&view=proposal&item=${value}`);const approve=page.getByTestId('approve-proposal-'+value);await expect(approve).toBeEnabled();
    await page.screenshot({path:path.join(root,`${scene}-${kind}-${value}-${actorID}.png`),fullPage:true});
    const waiting=page.waitForResponse(r=>r.url().endsWith(`/proposals/${value}/decisions`)&&r.request().method()==='POST');await approve.click();if(!(await waiting).ok())throw new Error('Actor scene approval failed');
    decisions.push({actor:actorID,reviewId:review.reviewId,reviewHash:review.reviewHash,slots:slots.map(s=>s.id),confirmedVia:'actual actor fixed proposal browser'});fs.writeFileSync(path.join(root,`${scene}-${kind}-${value}-decisions.json`),JSON.stringify(decisions,null,2)+'\n');console.log(actorID+' confirmed its '+slots.length+' own '+scene+' '+kind+' approval slots');
   }finally{await context.close();}
  }
  const context=await contextFor('U0');try{
   const review=await read(context,`/proposals/${value}/review`);if(review.status!=='approved')throw new Error('Scene approvals remain incomplete');assertReview(review);
   const plans=(await read(context,'/plans')).items,tasks=(await read(context,'/tasks')).items,plan=plans.find(p=>p.title.startsWith(scene+' ')),owned=tasks.filter(t=>t.planId===plan?.id);
   if(!plan||owned.length!==codes.length)throw new Error('Actual scene formal work count differs');
   if(kind==='dependencies')for(const c of review.changes){const t=owned.find(t=>t.id===c.targetId);if(!t?.requirements.some(r=>r.targetId===c.fields.requirements[0].targetId&&r.hard&&r.phase==='start'))throw new Error('Approved dependency not persisted');}
   fs.writeFileSync(path.join(root,scene+'-approved.json'),JSON.stringify({projectId:project,proposalID:value,reviewHash:review.reviewHash,decisions,plan,tasks:owned},null,2)+'\n');
   fs.writeFileSync(path.join(env.localRoot,'A-MiniIssue/work-context.json'),JSON.stringify({projectId:project,plans:plans.filter(p=>p.title.startsWith('A-P')),tasks:tasks.filter(t=>plans.some(p=>p.id===t.planId&&p.title.startsWith('A-P'))),source:'actual browser-authenticated product reads'},null,2)+'\n');
   console.log(scene+' '+kind+' became formal through all required named actors');
  }finally{await context.close();}
 }
}finally{await browser.close();}
