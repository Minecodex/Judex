// The approved synthetic U0/U5 actors preserve one QA identity across handoff.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
import {chromium,expect} from '@playwright/test';
import {intentFromReceipt} from './intent-envelope.mjs';
const [phase,receiptFile]=process.argv.slice(2);
if(!['invite','replace','verify-old-intent'].includes(phase))throw new Error('Approved handoff phase required');
const root='.cache/real-host/20261012-closure',environment=JSON.parse(fs.readFileSync(root+'/environment.json','utf8'));
const ns=JSON.parse(execFileSync('kubectl',['--context','docker-desktop','get','namespace',environment.namespace,'-o','json'],{encoding:'utf8',windowsHide:true})).metadata;
if(ns.uid!==environment.uid||ns.labels?.['judex.dev/test-run']!==environment.runId)throw new Error('Owned namespace required');
const actors=JSON.parse(fs.readFileSync(root+'/actors/actors.json','utf8'));
const project='21a13532-7929-4c6c-99a7-01e044eee64e',identity='af739ffe-a283-4caf-b0ab-67e903e55704',taskId='f3309487-a97d-4880-95ab-6eb1d290e69c';
const baseURL=`http://127.0.0.1:${environment.port}`,prefix=`/api/v1/projects/${project}`,file=root+'/QA-alpha-handoff.json';
const evidence=fs.existsSync(file)?JSON.parse(fs.readFileSync(file,'utf8')):{projectId:project,identityId:identity};
const browser=await chromium.launch(),owner=await browser.newContext({baseURL,storageState:actors.U0.statePath,viewport:{width:1440,height:1000}});
const read=async(context,suffix)=>{const r=await context.request.get(prefix+suffix).catch(()=>{throw new Error('Handoff read transport failed; private metadata omitted');});if(!r.ok())throw new Error('Handoff read HTTP '+r.status());return(await r.json()).data;};
const persist=()=>fs.writeFileSync(file,JSON.stringify(evidence,null,2)+'\n');
try{
 const identities=(await read(owner,'/identities')).items;
 const seat=identities.find(i=>i.id===identity);
 if(!seat||seat.kind!=='position')throw new Error('Original stable QA identity required');
 const tasks=(await read(owner,'/tasks')).items;
 const backlog=tasks.find(t=>t.title.startsWith('A53 '));
 if(!backlog||!['ready','working'].includes(backlog.status)||!backlog.participants.some(p=>p.identityId===identity))throw new Error('Concrete original-identity A53 backlog required before handoff');
 if(phase==='invite'){
  if(seat.currentBindingVersion!==1||seat.currentBinding?.userId!==actors.U3.userId)throw new Error('Original holder required');
  const members=(await read(owner,'/members')).items;
  if(!members.some(m=>m.userId===actors.U5.userId)){
   if(!evidence.invitation){
    const page=await owner.newPage();await page.goto('/?project='+project+'&settings=team');await page.waitForTimeout(1500);
    const invitationButton=page.getByTestId('next-invite');await expect(invitationButton).toBeVisible();await expect(invitationButton).toBeEnabled();await invitationButton.click({force:true});await page.getByTestId('next-invite-name').fill(actors.U5.email);
    // Membership only. Assigning another identity would violate the scenario.
    const waiting=page.waitForResponse(r=>r.url().endsWith(prefix+'/invitations')&&r.request().method()==='POST');
    await page.getByTestId('next-create-invite').click();const response=await waiting;if(!response.ok()){const body=await response.json();throw new Error('UI invitation failed '+response.status()+' '+JSON.stringify(body.error));}
    const invitation=(await response.json()).data,target=new URL(invitation.inviteUrl,baseURL);
    if(target.origin!==baseURL)throw new Error('Invitation origin changed');
    evidence.invitation={id:invitation.id,url:target.href,accepted:false};persist();
   }
   const member=await browser.newContext({baseURL,storageState:actors.U5.statePath});
   try{const page=await member.newPage();await page.goto(evidence.invitation.url);await page.getByRole('button',{name:'接受邀请',exact:true}).click();await page.waitForURL(url=>url.pathname==='/');await member.storageState({path:actors.U5.statePath});evidence.invitation.accepted=true;persist();}finally{await member.close();}
  }
  const current=(await read(owner,'/identities')).items;
  if(current.length!==identities.length||current.some(i=>i.currentBinding?.userId===actors.U5.userId))throw new Error('Invitation created an unintended role identity');
  evidence.memberPreparedAt=new Date().toISOString();evidence.identityCount=current.length;persist();console.log('U5 joined through actual invitation UI without a new identity');
 }else{
  if(!receiptFile)throw new Error('Actual old-binding CLI intent required');
  const receipt=JSON.parse(fs.readFileSync(receiptFile,'utf8')),intent=intentFromReceipt(receipt);
  const url=new URL(intent.confirmUrl,baseURL);
  if(url.origin!==baseURL||url.searchParams.get('project')!==project||intent.objectId!==taskId)throw new Error('Exact A12 old-binding intent required');
  const old=await browser.newContext({baseURL,storageState:actors.U3.statePath});
  try{
   const frozen=await read(old,'/confirmation-intents/'+intent.id);
   if(frozen.operation!=='task.reopen')throw new Error('Read-only old acceptance reopening probe required');
   const task=await read(owner,'/tasks/'+taskId);
   if(task.status!=='accepted'||task.version!==7||!task.latestAcceptanceId)throw new Error('Original accepted task changed');
   if(phase==='replace'){
    if(frozen.state!=='pending'||new Date(frozen.expiresAt).getTime()-Date.now()<30000||seat.currentBindingVersion!==1||seat.currentBinding?.userId!==actors.U3.userId)throw new Error('Fresh pending old-holder intent and original binding required');
    const bindingsBefore=tasks.filter(t=>t.reviewerIdentityId===identity||t.participants.some(p=>p.identityId===identity)).map(t=>({id:t.id,version:t.version,status:t.status,reviewer:t.reviewerIdentityId,participants:t.participants.map(p=>p.identityId)}));
    const page=await owner.newPage();await page.goto('/?project='+project+'&settings=team');
    await page.getByTestId('seat-'+identity).getByRole('button',{name:'替换关联人',exact:true}).click();
    await page.getByTestId('replace-seat-person').click();await page.locator('[data-option-value='+JSON.stringify(actors.U5.userId)+']').click();
    const changed=page.waitForResponse(r=>r.url().endsWith('/identities/'+identity+'/replace')&&r.request().method()==='POST');
    await page.getByTestId('confirm-seat-replace').click();if(!(await changed).ok())throw new Error('Actual binding replacement failed');await page.getByRole('dialog',{name:'替换关联人',exact:true}).waitFor({state:'detached'});
    const after=(await read(owner,'/identities')).items,updated=after.find(i=>i.id===identity);
    if(after.length!==identities.length||updated.currentBindingVersion!==2||updated.currentBinding?.userId!==actors.U5.userId)throw new Error('Stable identity handoff failed');
    const currentTasks=(await read(owner,'/tasks')).items;
    const bindingsAfter=currentTasks.filter(t=>t.reviewerIdentityId===identity||t.participants.some(p=>p.identityId===identity)).map(t=>({id:t.id,version:t.version,status:t.status,reviewer:t.reviewerIdentityId,participants:t.participants.map(p=>p.identityId)}));
    if(JSON.stringify(bindingsBefore)!==JSON.stringify(bindingsAfter))throw new Error('Historical work or backlog changed during binding replacement');
    evidence.beforeBinding={userId:actors.U3.userId,version:1};evidence.afterBinding={userId:actors.U5.userId,version:2};evidence.oldIntentId=intent.id;evidence.taskAcceptanceId=task.latestAcceptanceId;evidence.preservedWorkIds=bindingsAfter.map(t=>t.id);evidence.replacedAt=new Date().toISOString();persist();
    await page.screenshot({path:root+'/QA-alpha-replaced.png',fullPage:true});console.log('U0 replaced the holder through actual UI; identity and work remained');
   }else{
    if(seat.currentBindingVersion!==2||seat.currentBinding?.userId!==actors.U5.userId||evidence.oldIntentId!==intent.id)throw new Error('Recorded actual handoff required');
    const page=await old.newPage();await page.goto(url.href);const approve=page.getByTestId('confirm-approve');
    if(await approve.count()&&await approve.isEnabled()){
     const rejected=page.waitForResponse(r=>r.url().endsWith('/confirmation-intents/'+intent.id+'/confirm')&&r.request().method()==='POST');await approve.click();const response=await rejected;if(response.ok())throw new Error('Old binding unexpectedly confirmed');evidence.oldDecisionHttpStatus=response.status();
    }else evidence.oldDecisionBlockedInUI=true;
    const after=await read(owner,'/tasks/'+taskId),current=await read(old,'/confirmation-intents/'+intent.id);
    if(current.state==='committed'||after.status!=='accepted'||after.version!==7||after.latestAcceptanceId!==evidence.taskAcceptanceId)throw new Error('Old intent changed formal facts');
    evidence.oldIntentState=current.state;evidence.oldBindingBlockedAt=new Date().toISOString();persist();await page.screenshot({path:root+'/QA-alpha-old-intent-blocked.png',fullPage:true});console.log('Old U3 fixed-review page could not change the accepted task');
   }
  }finally{await old.close();}
 }
}finally{await owner.close();await browser.close();}
