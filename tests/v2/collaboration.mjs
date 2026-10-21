import assert from 'node:assert/strict';
import {client} from './api.mjs';
import {readEnvironment,writeEvidence} from './environment.mjs';
import {withNativeCLI} from './native-session.mjs';

const state=readEnvironment(),a=client(),b=client(),u=await a.register('Collaboration owner'),v=await b.register('Collaboration peer');
const report={status:'running',version:state.version,checks:[]};const save=()=>writeEvidence('collaboration.json',report);save();
async function fixture(model,name,positions=['Engineer']){
 const p=await a.call('POST','/projects',{name,defaultModelId:state.models[model],loopLimit:10});
 await a.call('POST',`/projects/${p.id}/members`,{userId:v,role:'member'});
 const roles=[];for(const name of positions)roles.push(await a.call('POST',`/projects/${p.id}/positions`,{name,prompt:'Use explicit structured mentions and public group evidence.'}));
 const g=await a.call('POST',`/projects/${p.id}/groups`,{name,members:[{userId:u,positionIds:roles.filter((_,i)=>i!==1).map(r=>r.id)},{userId:v,positionIds:roles.filter((_,i)=>i===1).map(r=>r.id)}]});
 return {p,g,role:g.members.find(m=>m.userId===u).roles.find(r=>r.positionId===roles[0].id).id};
}
async function wait(group,predicate){const end=Date.now()+180000;let d;while(Date.now()<end){d=(await a.call('GET',`/groups/${group}/discussions`)).items[0];if(d&&await predicate(d))return d;await new Promise(r=>setTimeout(r,150));}throw new Error('Discussion deadline: '+JSON.stringify(d));}
try{
 const human=await fixture('human','Human response branches');
 await a.call('POST',`/conversations/${human.g.id}/messages`,{body:'HUMAN_WAIT_TEST',mentions:[{kind:'role_agent',targetId:human.role}]});
 const waiting=await wait(human.g.id,async d=>d.waitingFor.length===2&&(await a.call('GET',`/groups/${human.g.id}/runs`)).items.every(r=>r.state==='completed'));
 const question=waiting.waitingFor[0].messageId;
 await a.call('POST',`/conversations/${human.g.id}/messages`,{body:'Ordinary extra information'});
 assert.equal((await a.call('GET',`/groups/${human.g.id}/runs`)).items.length,1);
 await b.call('POST',`/conversations/${human.g.id}/messages`,{body:'One person has replied',replyTo:question});
 const partial=(await a.call('GET',`/groups/${human.g.id}/discussions`)).items[0];assert.equal(partial.state,'active');assert.equal(partial.waitingFor.length,1);assert.equal(partial.waitingFor[0].userId,u);
 await withNativeCLI(a,[human.p.id],async cli=>{
  cli('group','use',human.g.id);
  assert.equal(cli('message','send','--text','HUMAN_REPLY_CONFIRMED','--reply-to',question,'--mention-ai',human.role).ok,true);
 });
 const settled=await wait(human.g.id,d=>d.state==='settled');assert.equal(settled.waitingFor.length,0);
 const answer=(await a.call('GET',`/conversations/${human.g.id}/messages`)).items.find(m=>m.body==='HUMAN_REPLY_CONFIRMED');assert.equal(answer.replyTo,question);assert.equal(answer.source,'cli');
 report.checks.push('human-wait-branch-metadata','ordinary-context-zero-trigger','partial-human-response','native-reply-to-and-explicit-role','answered-discussion-settles');report.humanGroupId=human.g.id;save();

 const branch=await fixture('branch-loop','Concurrent causal branches',['Branch start','Branch reviewer','Branch analyst']);
 await a.call('POST',`/conversations/${branch.g.id}/messages`,{body:'Start two peer branches',mentions:[{kind:'role_agent',targetId:branch.role}]});
 const paused=await wait(branch.g.id,d=>d.state==='paused');assert.equal(paused.loopCount,10);assert.equal(paused.allowance,10);assert.equal(paused.pauseReason,'loop_limit');
 const concurrent=await Promise.allSettled([a,b].map(who=>who.call('POST',`/groups/${branch.g.id}/discussions/${paused.id}/control`,{expectedVersion:paused.version,action:'continue',feedback:'Continue the same discussion'})));
 assert.equal(concurrent.filter(r=>r.status==='fulfilled').length,1);assert.equal(concurrent.filter(r=>r.status==='rejected'&&String(r.reason).includes('409')).length,1);
 const again=await wait(branch.g.id,d=>d.state==='paused'&&d.loopCount===20);assert.equal(again.id,paused.id);assert.equal(again.allowance,20);
 await a.call('POST',`/groups/${branch.g.id}/discussions/${again.id}/control`,{expectedVersion:again.version,action:'terminate'});
 report.checks.push('parallel-branches-shared-limit','atomic-final-allowance','simultaneous-human-continue-one-winner','continued-branches-same-discussion');report.branchGroupId=branch.g.id;report.status='passed';report.completedAt=new Date().toISOString();save();console.log(JSON.stringify(report));
}catch(error){report.status='failed';report.error=String(error);save();throw error;}
