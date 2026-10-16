// Fixed review of the approved B/C matrix; the platform model authors the draft.
import {chromium,expect} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {scenes} from './scene-definitions.mjs';
import {intentFromReceipt} from './intent-envelope.mjs';
const [phase,value,kind='creation',scene]=process.argv.slice(2);
if(!['submit','approve'].includes(phase)||!['creation','dependencies'].includes(kind)||!scenes[scene])throw new Error('Approved artifact scene review required');
const root='.cache/real-host/20261012-closure',env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'));
const team=JSON.parse(fs.readFileSync(path.join(root,scene+'-team.json'),'utf8'));
const published=JSON.parse(fs.readFileSync(path.join(root,scene+'-workflows-reviewed.json'),'utf8'));
if(published.projectID!==team.projectID)throw new Error('Workflow project differs');
const project=team.projectID,baseURL=`http://127.0.0.1:${env.port}`,prefix=`/api/v1/projects/${project}`;
const contracts=scene==='B'?{
 B11:['owner','brand','B-P1','B-W1',/约束|需求/],B12:['design','brand','B-P1','B-W1',/PNG/],B13:['brand','owner','B-P1','B-W1',/审阅|品牌/],
 B21:['design','brand','B-P2','B-W2',/桌面|尺寸|适配/],B22:['design','brand','B-P2','B-W2',/透明/],
}:{
 C11:['data','owner','C-P1','C-W1',/XLSX|工作表|公式/],C12:['office','owner','C-P1','C-W1',/DOCX|文档|报告/],C13:['owner','data','C-P1','C-W1',/一致性|数值|来源/],
 C21:['office','owner','C-P2','C-W2',/PPTX|演示|汇报/],C22:['owner','data','C-P2','C-W2',/版面|图表|数值/],
};
const codes=Object.keys(contracts),ownerActor='U0',actorIDs=[ownerActor,...scenes[scene].members.map(([actor])=>actor)];
const identity=role=>{
 const holders=team.identities.filter(i=>i.templateId===team.positions[role]&&i.currentBinding?.userId);
 if(holders.length!==1)throw new Error('Exact active scene role required '+role);return holders[0].id;
};
const flowFor=code=>{
 const found=published.workflows.filter(f=>f.originalName.startsWith(code+' ')&&f.published);
 if(found.length!==1)throw new Error('Exact reviewed published workflow required '+code);return found[0];
};
const browser=await chromium.launch();
const contextFor=id=>browser.newContext({baseURL,storageState:actors[id].statePath,viewport:{width:1440,height:1000}});
const read=async(context,suffix)=>{const response=await context.request.get(prefix+suffix);if(!response.ok())throw new Error('Scene read failed '+response.status());return(await response.json()).data;};
async function assertReview(context,review){
 if(kind==='creation'){
  const plans=review.changes.filter(c=>c.operation==='create_plan'),tasks=review.changes.filter(c=>c.operation==='create_task');
  if(review.changes.length!==7||plans.length!==2||tasks.length!==5)throw new Error('Exactly two plans and five approved artifact tasks required');
  for(const planCode of [scene+'-P1',scene+'-P2']){
   const found=plans.filter(p=>p.fields.title.startsWith(planCode+' '));if(found.length!==1)throw new Error('Exact owning scene plan required');
   const plan=found[0],flow=flowFor(scene+'-W'+planCode.slice(-1));
   if(!plan.clientRef||plan.fields.ownerIdentityId!==identity('owner')||plan.fields.workflowId!==flow.id||!plan.fields.goal?.trim()||!plan.fields.acceptanceCriteria?.trim())throw new Error('Reviewed owner/workflow/goal differs');
  }
  for(const [code,[maker,reviewer,planCode,flowCode,relevant]]of Object.entries(contracts)){
   const matches=tasks.filter(t=>t.fields.title.startsWith(code+' '));if(matches.length!==1)throw new Error('One formal task per approved code required '+code);
   const f=matches[0].fields,plan=plans.find(p=>p.fields.title.startsWith(planCode+' ')),flow=flowFor(flowCode);
   const version=(await read(context,'/workflows/'+flow.id+'/versions')).items.find(v=>v.id===flow.publishedVersionId&&v.state==='published');
   const node=version?.body.nodes.find(n=>n.id===f.nodeId),content=f.expectedOutput+' '+f.acceptanceCriteria;
   if(f.planId!==plan.clientRef||f.workflowId!==flow.id||!node?.allowedPositionIds.includes(team.positions[maker])||f.reviewerIdentityId!==identity(reviewer)||JSON.stringify(f.participantIdentityIds)!==JSON.stringify([identity(maker)])||!f.expectedOutput?.trim()||!f.acceptanceCriteria?.trim()||!relevant.test(content)||f.requirements?.length)throw new Error('Scene role, current published node, original output or ownership differs '+code);
   if(code==='B12'&&!/SVG/.test(content))throw new Error('Editable SVG source required');
   if(code==='C11'&&!/重算/.test(content))throw new Error('Actual formula recalculation required');
   if(['C12','C21'].includes(code)&&!/PDF/.test(content))throw new Error('Original Office file and actual PDF export required');
  }
 }else{
  const work=JSON.parse(fs.readFileSync(path.join(root,scene+'-current-work.json'),'utf8'));
  const task=code=>work.tasks.find(t=>t.title.startsWith(code+' '));
  if(review.changes.length!==4)throw new Error('Exact four start dependencies required');
  for(let i=1;i<codes.length;i++){
   const current=task(codes[i]),previous=task(codes[i-1]),c=review.changes.find(c=>c.targetId===current?.id),r=c?.fields.requirements;
   if(c?.operation!=='set_requirements'||c.targetType!=='task'||Object.keys(c.fields).some(k=>k!=='requirements')||r?.length!==1||r[0].targetId!==previous?.id||r[0].kind!=='task_acceptance'||r[0].phase!=='start'||r[0].hard!==true||!r[0].label?.trim())throw new Error('Actual accepted-artifact predecessor differs '+codes[i]);
  }
 }
}
try{
 if(phase==='submit'){
  const intent=intentFromReceipt(JSON.parse(fs.readFileSync(value,'utf8'))),url=new URL(intent.confirmUrl,baseURL);
  if(url.origin!==baseURL||url.searchParams.get('project')!==project)throw new Error('Owned scene confirmation required');
  const context=await contextFor(ownerActor);try{
   const frozen=await read(context,'/confirmation-intents/'+intent.id),review=await read(context,'/proposals/'+frozen.objectId+'/review');
   if(frozen.operation!=='proposal.submit'||frozen.state!=='pending'||review.status!=='draft'||frozen.reviewHash!==review.reviewHash)throw new Error('Pending fixed draft required');await assertReview(context,review);
   const page=await context.newPage();await page.goto(url.href);const approve=page.getByTestId('confirm-approve');await expect(approve).toBeEnabled();await expect(page.locator('body')).toContainText(actors.U0.name);
   await page.screenshot({path:path.join(root,`${scene}-${kind}-submit-${intent.id}.png`),fullPage:true});
   const response=page.waitForResponse(r=>r.url().endsWith('/confirmation-intents/'+intent.id+'/confirm')&&r.request().method()==='POST');await approve.click();if(!(await response).ok())throw new Error('Fixed scene submit failed');await expect(approve).toHaveCount(0);
   if((await read(context,'/confirmation-intents/'+intent.id)).state!=='committed')throw new Error('Scene submit not committed');console.log('U0 submitted exact '+scene+' '+kind+' draft via fixed browser');
  }finally{await context.close();}
 }else{
  if(!/^[0-9a-f-]{36}$/.test(value??''))throw new Error('Actual proposal UUID required');const decisions=[];
  for(const actorID of actorIDs){
   const context=await contextFor(actorID);try{
    const review=await read(context,'/proposals/'+value+'/review');if(review.status==='approved')break;if(review.status!=='pending')throw new Error('Pending actor review required');await assertReview(context,review);
    const slots=review.slots.filter(s=>s.canDecide&&s.state==='pending');if(!slots.length)continue;
    const page=await context.newPage();await page.goto(`/projects/${project}?tab=decisions&view=proposal&item=${value}`);const approve=page.getByTestId('approve-proposal-'+value);await expect(approve).toBeEnabled();
    await page.screenshot({path:path.join(root,`${scene}-${kind}-${value}-${actorID}.png`),fullPage:true});const response=page.waitForResponse(r=>r.url().endsWith('/proposals/'+value+'/decisions')&&r.request().method()==='POST');await approve.click();if(!(await response).ok())throw new Error('Actor fixed proposal decision failed');
    decisions.push({actor:actorID,reviewId:review.reviewId,reviewHash:review.reviewHash,slots:slots.map(s=>s.id),confirmedVia:'actual actor fixed proposal browser'});fs.writeFileSync(path.join(root,`${scene}-${kind}-${value}-decisions.json`),JSON.stringify(decisions,null,2)+'\n');console.log(actorID+' approved its '+slots.length+' own '+scene+' '+kind+' slots');
   }finally{await context.close();}
  }
  const context=await contextFor(ownerActor);try{
   const review=await read(context,'/proposals/'+value+'/review');if(review.status!=='approved')throw new Error('Actor approvals incomplete');await assertReview(context,review);
   const plans=(await read(context,'/plans')).items.filter(p=>p.title.startsWith(scene+'-P')),tasks=(await read(context,'/tasks')).items.filter(t=>plans.some(p=>p.id===t.planId));
   if(plans.length!==2||tasks.length!==5)throw new Error('Formal approved scene count differs');
   const snapshot={projectId:project,observedAt:new Date().toISOString(),plans,tasks,source:'actual named browser authenticated product reads'};
   fs.writeFileSync(path.join(root,scene+'-current-work.json'),JSON.stringify(snapshot,null,2)+'\n');fs.writeFileSync(path.join(env.localRoot,scenes[scene].folder,'work-context.json'),JSON.stringify(snapshot,null,2)+'\n');
   for(const plan of plans)fs.writeFileSync(path.join(root,plan.title.split(' ')[0]+'-approved.json'),JSON.stringify({projectId:project,proposalID:value,reviewHash:review.reviewHash,decisions,plan,tasks:tasks.filter(t=>t.planId===plan.id)},null,2)+'\n');
   console.log(scene+' '+kind+' became formal through named actors');
  }finally{await context.close();}
 }
}finally{await browser.close();}
