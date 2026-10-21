// Second-review regressions through real HTTP, device grants, native CLI and workers.
import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';
import {randomUUID} from 'node:crypto';
import {client,base} from './api.mjs';
import {readEnvironment,writeEvidence} from './environment.mjs';
import {withNativeCLI} from './native-session.mjs';
const env=readEnvironment(),a=client(),b=client(),u=await a.register('Followup owner'),v=await b.register('Followup peer');
const report={status:'running',version:env.version,checks:[]};const save=()=>writeEvidence('followup.json',report);save();
async function fixture(model,name){
 const p=await a.call('POST','/projects',{name,defaultModelId:env.models[model]});
 await a.call('POST',`/projects/${p.id}/members`,{userId:v,role:'member'});
 const position=await a.call('POST',`/projects/${p.id}/positions`,{name:'Engineer',prompt:'Read shared evidence, preserve unresolved questions.'});
 const g=await a.call('POST',`/projects/${p.id}/groups`,{name,members:[{userId:u,positionIds:[position.id]},{userId:v,positionIds:[]}]});
 return {p,position,g,role:g.members.find(m=>m.userId===u).roles[0].id};
}
async function wait(group,predicate){
 const end=Date.now()+180000;let d;
 while(Date.now()<end){d=(await a.call('GET',`/groups/${group}/discussions`)).items[0];if(d&&await predicate(d))return d;await new Promise(r=>setTimeout(r,100));}
 throw new Error('Followup discussion deadline '+JSON.stringify(d));
}
const countRuns=async group=>(await a.call('GET',`/groups/${group}/runs`)).items.length;
function forbidden(work){let rejected=false;try{work()}catch(error){rejected=true;assert.match(String(error.stdout||error),/grant lacks scope: agent:request/)}assert.ok(rejected,'CLI request unexpectedly authorized');}
try{
 const human=await fixture('human','Narrow grant and role revocation');
 const key=randomUUID(),text='Explicit authorized AI request';
 let original;
 await withNativeCLI(a,[human.p.id],async cli=>{
  cli('group','use',human.g.id);
  original=cli('message','send','--text',text,'--mention-ai',human.role,'--request-id',key);
  assert.deepEqual(cli('message','send','--text',text,'--mention-ai',human.role,'--request-id',key),original);
 });
 const waiting=await wait(human.g.id,async d=>d.waitingFor.length===2&&(await a.call('GET',`/groups/${human.g.id}/runs`)).items.every(r=>r.state==='completed'));
 const narrowed={call(method,url,body,...rest){if(url==='/auth/device/confirm')body={...body,scopes:['projects:read','context:read','messages:write']};return a.call(method,url,body,...rest)}};
 await withNativeCLI(narrowed,[human.p.id],async(cli,directory)=>{
  cli('group','use',human.g.id);
  const before=await countRuns(human.g.id);
  forbidden(()=>cli('message','send','--text','Denied role mention','--mention-ai',human.role));
  forbidden(()=>cli('message','send','--text',text,'--mention-ai',human.role,'--request-id',key));
  forbidden(()=>cli('push','--text','Denied AI push'));
  forbidden(()=>cli('push','--record-only','--text','Push endpoint still requires its declared scope'));
  const credentials=JSON.parse(fs.readFileSync(path.join(directory,'credentials.json'))),token=Object.entries(credentials).find(([key])=>key.startsWith('access:'))?.[1];assert.ok(token);
  const denied=await fetch(base+`/api/v1/conversations/${human.g.id}/messages`,{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json','Idempotency-Key':randomUUID()},body:JSON.stringify({body:'recordOnly is not a bypass on message send',recordOnly:true,mentions:[{kind:'role_agent',targetId:human.role}]})});assert.equal(denied.status,403);
  cli('message','send','--text','Ordinary @Engineer text');
  cli('message','send','--text','Human notification only','--mention-human',v);
  cli('message','send','--text','Explicit human response without AI','--reply-to',waiting.waitingFor[0].messageId);
  assert.equal(await countRuns(human.g.id),before);
 });
 report.checks.push('narrow-device-grant','native-AI-mention-forbidden','replay-rechecks-current-scope','push-endpoint-scope','recordOnly-not-message-bypass','ordinary-human-reply-zero-trigger');save();

 const changed=await a.call('PATCH',`/projects/${human.p.id}/groups/${human.g.id}`,{name:human.g.name,purpose:human.g.purpose,workflowIds:human.g.workflowIds,expectedVersion:human.g.version,members:[{userId:u,positionIds:[]},{userId:v,positionIds:[]}]});
 const ended=(await a.call('GET',`/groups/${human.g.id}/discussions`)).items[0];assert.equal(ended.state,'settled');assert.equal(ended.id,waiting.id);
 assert.equal((await a.call('GET',`/groups/${human.g.id}`)).members.length,2);
 const restored=await a.call('PATCH',`/projects/${human.p.id}/groups/${human.g.id}`,{name:human.g.name,purpose:human.g.purpose,workflowIds:human.g.workflowIds,expectedVersion:changed.version,members:[{userId:u,positionIds:[human.position.id]},{userId:v,positionIds:[]}]});
 assert.equal(restored.members.find(m=>m.userId===u).roles[0].id,human.role);
 await a.call('POST',`/conversations/${human.g.id}/messages`,{body:'New work after unassignment',mentions:[{kind:'role_agent',targetId:human.role}]});
 const fresh=await wait(human.g.id,d=>d.id!==ended.id);assert.equal(fresh.loopCount,0);
 await a.call('POST',`/groups/${human.g.id}/discussions/${fresh.id}/control`,{expectedVersion:fresh.version,action:'terminate'});
 report.checks.push('unassigned-waiting-role-settles','human-membership-retained','reassigned-role-stable-identity','next-trigger-new-discussion');save();

 const quota=await fixture('quota','Quota fallback with shared compression');
 for(let i=0;i<3;i++)await a.call('POST',`/conversations/${quota.g.id}/messages`,{body:`Question ${i}: disagreement is unresolved. `+'Shared evidence by original reference. '.repeat(600)});
 await a.call('POST',`/conversations/${quota.g.id}/messages`,{body:'Explicit fallback request',mentions:[{kind:'role_agent',targetId:quota.role}]});
 const done=await wait(quota.g.id,d=>d.state==='settled'||d.state==='paused');assert.equal(done.state,'settled');assert.equal(done.loopCount,0);
 const runs=(await a.call('GET',`/groups/${quota.g.id}/runs`)).items;assert.ok(runs[0].modelCalls>=4);assert.ok(runs[0].tokens>0);
 const snapshots=(await a.call('GET',`/groups/${quota.g.id}/snapshots`)).items;assert.ok(snapshots.some(s=>s.coveredThrough>0&&s.summary.includes('unresolved')));
 const originalEntry=(await a.call('GET',`/groups/${quota.g.id}/context?limit=100`)).items.find(k=>k.kind==='message');
 const reference=await a.call('GET',`/groups/${quota.g.id}/context/reference?kind=message&ref=${originalEntry.refId}&offset=0&limit=8000`);assert.match(reference.content,/Question 0/);
 report.checks.push('configured-quota-fallback-executes','fallback-compression-counted-as-real-calls','smaller-fallback-shared-budget','fallback-preserves-originals-and-loop-count');save();

 const rate=await fixture('rate-limit','Rate limit does not switch models');
 await a.call('POST',`/conversations/${rate.g.id}/messages`,{body:'Ordinary rate limit must pause',mentions:[{kind:'role_agent',targetId:rate.role}]});
 const paused=await wait(rate.g.id,d=>d.state==='paused');assert.equal(paused.pauseReason,'model_failed');assert.equal((await a.call('GET',`/groups/${rate.g.id}/runs`)).items[0].modelCalls,1);
 await a.call('POST',`/groups/${rate.g.id}/discussions/${paused.id}/control`,{expectedVersion:paused.version,action:'terminate'});
 report.checks.push('ordinary-429-no-fallback');report.status='passed';report.completedAt=new Date().toISOString();save();console.log(JSON.stringify(report));
}catch(error){report.status='failed';report.error=String(error);save();throw error;}
