import {chromium} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {assertAP1Requirements} from './review-contract.mjs';
const [proposalID,mode='tasks']=process.argv.slice(2);if(!/^[0-9a-f-]{36}$/.test(proposalID??'')||!['tasks','requirements'].includes(mode))throw new Error('Observed proposal ID and known review mode required');
const root='.cache/real-host/20261012-closure',env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8')),team=JSON.parse(fs.readFileSync(path.join(root,'A-team.json'),'utf8'));
const browser=await chromium.launch(),baseURL=`http://127.0.0.1:${env.port}`,prefix='/api/v1/projects/'+team.projectID;
const decisions=[];
try{
 for(const actorID of ['U0','U1','U2','U3','U4']){
  const context=await browser.newContext({baseURL,storageState:actors[actorID].statePath,viewport:{width:1440,height:1000}});
  try{
   const read=async suffix=>(await(await context.request.get(prefix+suffix)).json()).data;
   const review=await read('/proposals/'+proposalID+'/review');
   if(review.status==='approved')break;
   if(mode==='requirements'){
    assertAP1Requirements(review);
    if(review.status!=='pending')throw new Error('Pending fixed requirement review required');
   }else{
   const newTasks=review.changes.filter(c=>c.operation==='create_task'),newPlans=review.changes.filter(c=>c.operation==='create_plan'),cancelTasks=review.changes.filter(c=>c.operation==='cancel_task');
   const expectedCount=newPlans.length?7:12;
   if(review.status!=='pending'||review.changes.length!==expectedCount||newTasks.length!==6||(!newPlans.length&&cancelTasks.length!==6))throw new Error('Current fixed scene proposal required');
   if(newTasks.some(c=>!c.fields.planId||!(newPlans.some(p=>p.clientRef===c.fields.planId)||/^[0-9a-f-]{36}$/.test(c.fields.planId))))throw new Error('Every new task must name its actual owning plan; change ordering is not ownership');
   }
   const slots=review.slots.filter(s=>s.canDecide&&s.state==='pending');if(!slots.length)continue;
   const page=await context.newPage();await page.goto('/projects/'+team.projectID+'?tab=decisions&view=proposal&item='+proposalID);
   await page.getByTestId('approve-proposal-'+proposalID).waitFor({state:'visible'});
   await page.screenshot({path:path.join(root,`${proposalID}-review-${actorID}.png`),fullPage:true});
   const decided=page.waitForResponse(r=>r.url().endsWith('/proposals/'+proposalID+'/decisions')&&r.request().method()==='POST');
   await page.getByTestId('approve-proposal-'+proposalID).click();const response=await decided;if(!response.ok())throw new Error('Actor '+actorID+' approval failed: '+await response.text());
   decisions.push({actor:actorID,reviewId:review.reviewId,reviewHash:review.reviewHash,slots:slots.map(s=>s.id),decidedVia:'frozen_proposal_review_ui'});
   fs.writeFileSync(path.join(root,`${proposalID}-decision-log.json`),JSON.stringify(decisions,null,2)+'\n');
   console.log(actorID+' confirmed its own '+slots.length+' approval slots');
  }finally{await context.close();}
 }
 const owner=await browser.newContext({baseURL,storageState:actors.U0.statePath});
 try{
  const final=(await(await owner.request.get(prefix+'/proposals/'+proposalID+'/review')).json()).data;
  if(final.status!=='approved')throw new Error('Not all required approval slots completed');
  const plans=(await(await owner.request.get(prefix+'/plans')).json()).data.items,tasks=(await(await owner.request.get(prefix+'/tasks')).json()).data.items;
  const plan=plans.find(p=>p.title.startsWith('A-P1'));if(!plan||tasks.filter(t=>t.planId===plan.id).length!==6)throw new Error('Approved formal work count mismatch');
  fs.writeFileSync(path.join(root,'A-P1-approved.json'),JSON.stringify({proposalID,projectId:team.projectID,reviewId:final.reviewId,reviewHash:final.reviewHash,decisions,plan,tasks:tasks.filter(t=>t.planId===plan.id)},null,2)+'\n');
  fs.writeFileSync(path.join(env.localRoot,'A-MiniIssue','work-context.json'),JSON.stringify({projectId:team.projectID,plan,tasks:tasks.filter(t=>t.planId===plan.id)},null,2)+'\n');
  if(mode==='requirements'){
   for(const c of final.changes){const task=tasks.find(t=>t.id===c.targetId);if(JSON.stringify(task.requirements.map(r=>r.targetId).sort())!==JSON.stringify(c.fields.requirements.map(r=>r.targetId).sort()))throw new Error('Approved requirements did not persist');}
  }
  console.log(mode==='requirements'?'Five existing task dependencies became formal through named browser approvals':'One plan and six tasks became formal through named browser approvals');
 }finally{await owner.close();}
}finally{await browser.close();}
