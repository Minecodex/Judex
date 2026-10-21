import assert from 'node:assert/strict';
import {randomUUID} from 'node:crypto';
import {client,base} from './api.mjs';
import {readEnvironment} from './environment.mjs';

export function browserControls(peer,{email,password,proxy,apiBase=base,modelIds=readEnvironment().models}){
 const owner=client(apiBase),third=client(apiBase);let self,peerId,thirdId,projectId,groupId;
 async function wait(fn){const end=Date.now()+90000;while(Date.now()<end){const out=await fn();if(out)return out;await new Promise(resolve=>setTimeout(resolve,150));}throw new Error('Browser fixture deadline');}
 return {
  async setup(rootURL){
   const params=new URL(rootURL).searchParams;projectId=params.get('project');groupId=params.get('group');
   const login=await owner.call('POST','/auth/login',{email,password});self=login.user.id;await owner.call('GET','/auth/session');peerId=(await peer.call('GET','/auth/session')).user.id;
   thirdId=await third.register('Third colleague');
   await owner.call('POST',`/projects/${projectId}/members`,{userId:thirdId,role:'member'});
   const members=(await owner.call('GET',`/projects/${projectId}/members`)).items;
   await owner.call('POST',`/projects/${projectId}/members`,{userId:peerId,role:'admin',expectedVersion:members.find(m=>m.userId===peerId).version});
   const group=await owner.call('GET',`/groups/${groupId}`);
   await owner.call('PATCH',`/projects/${projectId}/groups/${groupId}`,{name:group.name,purpose:group.purpose,workflowIds:group.workflowIds,expectedVersion:group.version,members:[...group.members.map(m=>({userId:m.userId,positionIds:m.roles.map(r=>r.positionId)})),{userId:thirdId,positionIds:[]}]});
   return {projectId,groupId,thirdName:'Third colleague'};
  },
  async groupStayedRemote(){const g=await owner.call('GET',`/groups/${groupId}`);assert.equal(g.purpose,'Remote group purpose');assert.ok(!g.members.some(m=>m.userId===thirdId));},
  async projectStayedRemote(){const p=await owner.call('GET',`/projects/${projectId}`);assert.equal(p.description,'Remote project description');},
  hideConfiguration:value=>proxy.hideConfiguration(value),
  async configurationHidden(before){await wait(()=>proxy.hiddenCount()>before);},
  hiddenCount:()=>proxy.hiddenCount(),
  async projectDescription(expected){await wait(async()=>(await owner.call('GET',`/projects/${projectId}`)).description===expected);},
  corruptComplete:()=>proxy.corruptOnce(`/api/v1/conversations/${groupId}/uploads/*/complete`),
  async fileCount(){return (await owner.call('GET',`/conversations/${groupId}/files`)).items.length;},
  async uploadEffects(before){const count=(await owner.call('GET',`/conversations/${groupId}/files`)).items.length;assert.equal(count,before+1);const rows=proxy.records.filter(r=>r.path.includes(`/conversations/${groupId}/uploads/`)&&r.corrupted!==undefined);const first=rows.find(r=>r.corrupted);const retries=rows.filter(r=>r.path===first.path);assert.equal(retries.length,2);assert.equal(retries[0].key,retries[1].key);return {fileCount:count,upload:retries.map(r=>({path:r.path,key:r.key,status:r.status,corrupted:r.corrupted}))};},
  async prepareBatch(mode){
   const group=await owner.call('POST',`/projects/${projectId}/groups`,{name:'Batch recovery '+mode,members:[{userId:self,positionIds:[]},{userId:peerId,positionIds:[]}]});
   const route=`/api/v1/conversations/${group.id}/uploads/*/complete`;
   if(mode==='before-whole')proxy.failWriteNth(route,2);else proxy.corruptNth(route,2);
   return group.id;
  },
  async batchEffects(id,count){
   const files=await wait(async()=>{const rows=(await owner.call('GET',`/conversations/${id}/files`)).items;return rows.length>=count?rows:false;});assert.equal(files.length,count);
   const messages=(await owner.call('GET',`/conversations/${id}/messages`)).items;assert.equal(messages.length,count);assert.ok(messages.every(m=>m.files.length===1));assert.equal((await owner.call('GET',`/groups/${id}/discussions`)).items.length,0);
   return files.map(f=>({id:f.id,name:f.name}));
  },
  corruptMessage:mode=>proxy.corruptOnce(`/api/v1/conversations/${groupId}/messages`,mode),
  async messageEffects(body,count,expectedKeys){
   const rows=(await owner.call('GET',`/conversations/${groupId}/messages`)).items.filter(m=>m.body===body);assert.equal(rows.length,count);
   const writes=proxy.records.filter(r=>r.path===`/api/v1/conversations/${groupId}/messages`&&JSON.parse(r.body).body===body);
   assert.equal(writes.length,expectedKeys);assert.equal(writes[0].key,writes[1].key);assert.equal(JSON.parse(writes[0].upstreamBody).data.message.id,JSON.parse(writes[1].upstreamBody).data.message.id);
   if(expectedKeys===3)assert.notEqual(writes[1].key,writes[2].key);
   return {messages:rows.map(m=>({id:m.id,seq:m.seq})),writes:writes.map(r=>({key:r.key,status:r.status,corrupted:r.corrupted}))};
  },
  fail:path=>proxy.fail('/api/v1'+path),restore:path=>proxy.restore('/api/v1'+path),
  async prepareHuman(){
   const positionName='Human responder '+randomUUID().slice(0,4);
   const position=await owner.call('POST',`/projects/${projectId}/positions`,{name:positionName,prompt:'Ask group members for explicit responses.',modelId:modelIds.human});
   const group=await owner.call('POST',`/projects/${projectId}/groups`,{name:'Human reply UI',purpose:'Replying and explicitly waking a role are separate actions.',members:[{userId:self,positionIds:[position.id]},{userId:peerId,positionIds:[]}]});
   return {groupId:group.id,roleLabel:'Browser owner · '+positionName+' AI'};
  },
  async partialReply(id){
   const d=await wait(async()=>{const d=(await owner.call('GET',`/groups/${id}/discussions`)).items[0];return d?.waitingFor.length===2?d:false;});
   const questionId=d.waitingFor[0].messageId;
   await peer.call('POST',`/conversations/${id}/messages`,{body:'The peer has answered their branch.',replyTo:questionId});
   const partial=(await owner.call('GET',`/groups/${id}/discussions`)).items[0];assert.equal(partial.waitingFor.length,1);assert.equal(partial.waitingFor[0].userId,self);return questionId;
  },
  async settled(id){await wait(async()=>(await owner.call('GET',`/groups/${id}/discussions`)).items[0]?.state==='settled');const messages=(await owner.call('GET',`/conversations/${id}/messages`)).items;const answer=messages.find(m=>m.body.includes('HUMAN_REPLY_CONFIRMED'));assert.ok(answer?.replyTo);assert.equal(answer.source,'web');},
 };
}
