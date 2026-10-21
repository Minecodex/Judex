import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {randomUUID} from 'node:crypto';
import {client,base} from './api.mjs';
import {withNativeCLI} from './native-session.mjs';
import {startBrowserProxy} from './browser-proxy.mjs';

const who=client(),peer=client(),user=await who.register('Composite CLI owner'),other=await peer.register('Composite CLI peer');
const project=await who.call('POST','/projects',{name:'Composite operation recovery'});
await who.call('POST',`/projects/${project.id}/members`,{userId:other,role:'member'});
const proxy=await startBrowserProxy(base),checks=[];
try {
 await withNativeCLI(who,[project.id],async(cli,directory)=>{
  const first=path.join(directory,'first.md'),second=path.join(directory,'second.md');
  fs.writeFileSync(first,'# First attachment\nFixed original content.');fs.writeFileSync(second,'# Second attachment\nIndependent content.');
  for(const fault of ['second-create','second-complete','final-truncated','final-envelope']){
   const group=await who.call('POST',`/projects/${project.id}/groups`,{name:fault,members:[{userId:user,positionIds:[]},{userId:other,positionIds:[]}]});
   const push=`/api/v1/groups/${group.id}/push`,upload=`/api/v1/conversations/${group.id}/uploads`;
   if(fault==='second-create')proxy.failWriteNth(upload,2);
   else if(fault==='second-complete')proxy.corruptNth(upload+'/*/complete',2);
   else proxy.corruptOnce(push,fault==='final-envelope'?'envelope':'truncated');
   const args=['push','--group',group.id,'--record-only','--text',fault,'--file',first,'--file',second];
   await assert.rejects(cli(...args));
   const pending=(await cli('pending')).data;assert.equal(pending.filter(p=>p.kind==='file-message').length,1);
   const before=(await who.call('GET',`/conversations/${group.id}/files`)).items;
   assert.equal(before.length,fault==='second-create'?1:2);
   assert.equal((await cli(...args)).ok,true);
   const messages=(await who.call('GET',`/conversations/${group.id}/messages`)).items,files=(await who.call('GET',`/conversations/${group.id}/files`)).items;
   assert.equal(messages.length,1);assert.equal(files.length,2);assert.equal(messages[0].files.length,2);
   assert.ok(before.every(v=>files.some(f=>f.id===v.id)));assert.equal((await cli('pending')).data.length,0);
   if(fault.startsWith('final-')){
    const requests=proxy.records.filter(r=>r.path===push);assert.equal(requests.length,2);assert.equal(requests[0].key,requests[1].key);assert.equal(requests[0].body,requests[1].body);
   }
   assert.equal((await who.call('GET',`/groups/${group.id}/discussions`)).items.length,0);
   checks.push('native-cross-process-'+fault);
   await cli(...args);assert.equal((await who.call('GET',`/conversations/${group.id}/files`)).items.length,4);assert.equal((await who.call('GET',`/conversations/${group.id}/messages`)).items.length,2);
   checks.push('confirmed-new-intent-'+fault);
  }
  const group=await who.call('POST',`/projects/${project.id}/groups`,{name:'Explicit composite identity',members:[{userId:user,positionIds:[]},{userId:other,positionIds:[]}]});
  const key=randomUUID(),args=['push','--group',group.id,'--record-only','--text','explicit','--file',first,'--file',second,'--request-id',key];
  proxy.corruptOnce(`/api/v1/groups/${group.id}/push`);
  await assert.rejects(cli(...args));await cli(...args);await cli(...args);
  assert.equal((await who.call('GET',`/conversations/${group.id}/messages`)).items.length,1);
  assert.equal((await who.call('GET',`/conversations/${group.id}/files`)).items.length,2);checks.push('explicit-request-id-stable-after-confirmation');
  const member=(await who.call('GET',`/projects/${project.id}/members`)).items.find(m=>m.userId===other);
  // Remove the CLI owner from the group through the other member. The cached
  // explicit result must still be rejected using current server permissions.
  const detail=await peer.call('GET',`/groups/${group.id}`);
  const third=client(),thirdId=await third.register('Replacement member');
  await who.call('POST',`/projects/${project.id}/members`,{userId:thirdId,role:'member'});
  await peer.call('PATCH',`/projects/${project.id}/groups/${group.id}`,{name:detail.name,expectedVersion:detail.version,purpose:detail.purpose,workflowIds:detail.workflowIds,members:[{userId:other,positionIds:[]},{userId:thirdId,positionIds:[]}]});
  await assert.rejects(cli(...args),error=>JSON.parse(String(error.stdout)).error.includes('member')||JSON.parse(String(error.stdout)).ok===false);assert.ok(member);checks.push('composite-replay-rechecks-membership');
 },{baseURL:proxy.url,asyncCommands:true});
 console.log(JSON.stringify({status:'passed',checks,requests:proxy.records}));
}finally{await proxy.close();}
