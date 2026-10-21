import assert from 'node:assert/strict';
import {spawn,execFileSync} from 'node:child_process';
import {readEnvironment,writeEvidence} from './environment.mjs';

const state=readEnvironment();
const kube=(...args)=>execFileSync('kubectl',args,{encoding:'utf8',timeout:60000});
const namespace=JSON.parse(kube('get','namespace',state.namespace,'-o','json'));
assert.equal(namespace.metadata.uid,state.namespaceUID);assert.equal(namespace.metadata.labels['judex.run'],state.token);assert.equal(namespace.metadata.labels['judex.owner'],state.owner);
process.env.JUDEX_V2_URL='http://127.0.0.1:18102';
const {client,base}=await import('./api.mjs');let forward,stopping=false;
function connect(){if(stopping)return;const current=spawn('kubectl',['-n',state.namespace,'port-forward','service/judex','18102:8080'],{stdio:'ignore'});forward=current;current.on('exit',()=>{if(!stopping&&forward===current)setTimeout(connect,500);});}
async function wait(fn,timeout=240000){const end=Date.now()+timeout;while(Date.now()<end){try{const result=await fn();if(result)return result;}catch{}await new Promise(resolve=>setTimeout(resolve,200));}throw new Error('Nonzero closure restart deadline');}
const report={status:'running',version:state.version,checks:[]};const save=()=>writeEvidence('restart-closures.json',report);save();
try{
 connect();await wait(async()=>(await fetch(base+'/readyz')).ok,30000);
 const a=client(),b=client(),u=await a.register('Closure restart owner'),v=await b.register('Closure restart peer');
 const p=await a.call('POST','/projects',{name:'Nonzero closure process restart',defaultModelId:state.models['restart-loop'],loopLimit:10});
 await a.call('POST',`/projects/${p.id}/members`,{userId:v,role:'member'});
 const position=await a.call('POST',`/projects/${p.id}/positions`,{name:'Loop role',prompt:'Controlled nonzero closure restart.'});
 const group=await a.call('POST',`/projects/${p.id}/groups`,{name:'Preserve causal history through process loss',members:[{userId:u,positionIds:[position.id]},{userId:v,positionIds:[position.id]}]});
 const role=group.members.find(m=>m.userId===u).roles[0].id;
 await a.call('POST',`/conversations/${group.id}/messages`,{body:'Continue until the shared ten-loop limit',mentions:[{kind:'role_agent',targetId:role}]});
 const before=await wait(async()=>{const d=(await a.call('GET',`/groups/${group.id}/discussions`)).items[0];if(d?.loopCount!==2)return false;const r=(await a.call('GET',`/groups/${group.id}/runs`)).items.find(r=>r.state==='running'&&r.modelCalls===1);return r?{discussion:d,run:r}:false;});
 report.loopsAtRestart=before.discussion.loopCount;report.interruptedRunId=before.run.id;
 const deployment=JSON.parse(kube('-n',state.namespace,'get','deployment','judex','-o','json'));
 const selector=Object.entries(deployment.spec.selector.matchLabels).map(([k,v])=>k+'='+v).join(',');
 const pods=JSON.parse(kube('-n',state.namespace,'get','pods','-l',selector,'-o','json')).items.filter(p=>!p.metadata.deletionTimestamp&&p.metadata.ownerReferences?.some(o=>o.kind==='ReplicaSet'));
 assert.equal(pods.length,1);report.oldPodUID=pods[0].metadata.uid;save();
 kube('-n',state.namespace,'delete','pod',pods[0].metadata.name,'--grace-period=0','--force','--wait=false');forward.kill();
 await wait(async()=>(await fetch(base+'/readyz')).ok,90000);
 const paused=await wait(async()=>{const d=(await a.call('GET',`/groups/${group.id}/discussions`)).items[0];return d?.state==='paused'?d:false;});
 assert.equal(paused.id,before.discussion.id);assert.equal(paused.loopCount,10);assert.equal(paused.allowance,10);assert.equal(paused.pauseReason,'loop_limit');
 const messages=(await a.call('GET',`/conversations/${group.id}/messages`)).items.filter(m=>m.roleAgentId);
 assert.equal(messages.length,20,'Public causal effects were repeated or lost during restart');
 const restored=(await a.call('GET',`/groups/${group.id}/runs`)).items.find(r=>r.id===before.run.id);assert.ok(restored?.modelCalls>=3,'Unknown interrupted call was not retained');
 await a.call('POST',`/groups/${group.id}/discussions/${paused.id}/control`,{expectedVersion:paused.version,action:'terminate'});
 report.groupId=group.id;report.loopsAfterRestart=paused.loopCount;report.publicAgentMessages=messages.length;report.recoveredModelCalls=restored.modelCalls;
 report.checks.push('restart-with-two-existing-closures','same-discussion-after-restart','unknown-call-retained','exactly-twenty-causal-publications','atomic-ten-loop-pause-after-recovery');
 report.status='passed';report.completedAt=new Date().toISOString();save();console.log(JSON.stringify(report));
}catch(error){report.status='failed';report.error=String(error);save();throw error;}finally{stopping=true;forward?.kill();}
