import assert from 'node:assert/strict';
import path from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {client} from './api.mjs';
import {readEnvironment,writeEvidence} from './environment.mjs';
import {withNativeCLI} from './native-session.mjs';

const env=readEnvironment(),a=client(),b=client(),c=client(),d=client();
const [u,v,w,z]=[await a.register('Permission owner'),await b.register('Permission manager'),await c.register('Child member'),await d.register('Other project member')];
const report={status:'running',version:env.version,checks:[]};const save=()=>writeEvidence('permissions.json',report);save();
const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
async function wait(fn,timeout=90000){const end=Date.now()+timeout;while(Date.now()<end){const result=await fn();if(result)return result;await sleep(150);}throw new Error('Permission regression deadline');}
async function upload(who,room,text,file){const data=Buffer.from(text),body={name:'evidence.md',size:data.length,sha256:createHash('sha256').update(data).digest('hex'),...(file?{fileId:file.fileId,expectedVersion:file.version}:{})};const upload=await who.call('POST',`/conversations/${room}/uploads`,body);await who.call('PUT',`/conversations/${room}/uploads/${upload.id}/parts/1`,data,{binary:true});return who.call('POST',`/conversations/${room}/uploads/${upload.id}/complete`,{});}
function deniedCLI(cli,...args){let denied=false;try{cli(...args);}catch(error){denied=true;assert.equal(JSON.parse(String(error.stdout)).ok,false);}assert.ok(denied,'CLI exposed an inaccessible resource');}
try{
 const p=await a.call('POST','/projects',{name:'Permission matrix',defaultModelId:env.models.tools});for(const userId of [v,w])await a.call('POST',`/projects/${p.id}/members`,{userId,role:'member'});
 const role=await a.call('POST',`/projects/${p.id}/positions`,{name:'Scope reader',prompt:'REVOCATION_GATE: read only the approved group snapshot.'});
 const root=await a.call('POST',`/projects/${p.id}/groups`,{name:'PRIVATE_PARENT_NAME',members:[{userId:u,positionIds:[]},{userId:v,positionIds:[]}]});
 const rootFile=await upload(a,root.id,'ROOT_ONLY_EVIDENCE_V1');
 const child=await b.call('POST',`/projects/${p.id}/groups`,{name:'Joined child',parentId:root.id,members:[{userId:v,positionIds:[]},{userId:w,positionIds:[role.id]}]});
 const sibling=await b.call('POST',`/projects/${p.id}/groups`,{name:'Independent sibling',parentId:root.id,members:[{userId:v,positionIds:[]},{userId:w,positionIds:[]}]});
 const visible=(await c.call('GET',`/projects/${p.id}/groups`)).items;assert.equal(visible.length,2);assert.ok(visible.every(g=>g.parentId===null));assert.ok(!JSON.stringify(visible).includes(root.id));assert.ok(!JSON.stringify(visible).includes(root.name));
 await a.call('GET',`/groups/${child.id}`,undefined,{status:404});await d.call('GET',`/groups/${child.id}`,undefined,{status:404});
 for(const route of [`/groups/${root.id}`,`/conversations/${root.id}/messages?q=ROOT_ONLY`,`/groups/${root.id}/context`,`/conversations/${root.id}/file-versions/${rootFile.id}`])await c.call('GET',route,undefined,{status:404});
 const file=await upload(c,child.id,'CHILD_ONLY_EVIDENCE');const stream=await c.stream(`/conversations/${child.id}/events`);assert.equal(stream.status,200);const reader=stream.body.getReader();
 await withNativeCLI(c,[p.id],async(cli,directory)=>{
  cli('group','use',child.id);deniedCLI(cli,'group','show','--group',root.id);deniedCLI(cli,'message','list','--group',root.id);
  cli('push','--text','Read the exact approved child file');
  const source=await wait(async()=>{const facts=(await b.call('GET',`/groups/${child.id}/context?limit=100`)).items;return facts.find(k=>k.kind==='tool'&&k.content.name==='read'&&JSON.stringify(k.content.result).includes('CHILD_ONLY_EVIDENCE'));},240000);
  assert.ok(!JSON.stringify(source).includes('ROOT_ONLY_EVIDENCE'));assert.ok(!JSON.stringify(source).includes(rootFile.id));
  const run=(await b.call('GET',`/groups/${child.id}/runs`)).items[0];const projected=JSON.parse(execFileSync('kubectl',['-n',env.sandboxNamespace,'get','pvc','-l','judex.run='+run.id,'-o','json'],{encoding:'utf8'})).items;assert.equal(projected.length,1);assert.equal(projected[0].metadata.labels['judex.group'],child.id);
  const before=(await b.call('GET',`/conversations/${child.id}/messages`)).items.filter(m=>m.roleAgentId).length;
  const current=await b.call('GET',`/groups/${child.id}`);await b.call('PATCH',`/projects/${p.id}/groups/${child.id}`,{name:current.name,purpose:current.purpose,workflowIds:current.workflowIds,expectedVersion:current.version,members:[{userId:v,positionIds:[]},{userId:u,positionIds:[]}]});
  for(const route of [`/groups/${child.id}`,`/conversations/${child.id}/messages?q=CHILD_ONLY`,`/groups/${child.id}/context`,`/groups/${child.id}/snapshots`,`/conversations/${child.id}/file-versions/${file.id}`,`/conversations/${child.id}/file-versions/${file.id}/preview`])await c.call('GET',route,undefined,{status:404});
  const deniedContent=await c.stream(`/conversations/${child.id}/file-versions/${file.id}/content`);assert.equal(deniedContent.status,404);await deniedContent.body.cancel();
  deniedCLI(cli,'message','list');deniedCLI(cli,'context','read');deniedCLI(cli,'file','download',file.id,path.join(directory,'denied.md'));deniedCLI(cli,'watch','--limit','1');
  let closed=false;const end=Date.now()+10000;while(Date.now()<end){const read=await Promise.race([reader.read(),sleep(1000).then(()=>null)]);if(read?.done){closed=true;break;}}assert.ok(closed,'SSE remained open after membership removal');
  await wait(async()=>(await b.call('GET',`/groups/${child.id}/runs`)).items[0]?.state==='cancelled');await sleep(2200);assert.equal((await b.call('GET',`/conversations/${child.id}/messages`)).items.filter(m=>m.roleAgentId).length,before);
  await wait(()=>JSON.parse(execFileSync('kubectl',['-n',env.sandboxNamespace,'get','pvc','-l','judex.run='+run.id,'-o','json'],{encoding:'utf8'})).items.length===0);
  assert.deepEqual((await c.call('GET',`/projects/${p.id}/groups`)).items.map(g=>g.id),[sibling.id]);cli('group','use',sibling.id);assert.equal(cli('message','list').ok,true);
  report.checks.push('hidden-parent-name-and-ID','project-owner-not-implicit-group-member','unjoined-project-and-outsider-denied','search-and-file-boundary','actual-child-sandbox-projection','live-membership-revocation-cancels-run','revoked-HTTP-preview-content-context','revoked-native-CLI-and-SSE','revoked-run-cannot-publish-late','revoked-projection-reclaimed','sibling-membership-independent');save();

  const other=await c.call('POST','/projects',{name:'Explicit cross-project share'});for(const userId of [v,z])await c.call('POST',`/projects/${other.id}/members`,{userId,role:'member'});
  const target=await c.call('POST',`/projects/${other.id}/groups`,{name:'Exact cross-project target',members:[{userId:w,positionIds:[]},{userId:v,positionIds:[]},{userId:z,positionIds:[]}]});
  await b.call('POST',`/groups/${root.id}/shares`,{targetGroupId:target.id,fileVersionIds:[rootFile.id],summary:'Only the exact original version is shared.'});
  const updated=await upload(a,root.id,'ROOT_ONLY_EVIDENCE_V2',rootFile);
  const shared=await d.stream(`/conversations/${target.id}/file-versions/${rootFile.id}/content`);assert.equal(shared.status,200);assert.ok((await shared.text()).includes('ROOT_ONLY_EVIDENCE_V1'));
  await d.call('GET',`/conversations/${target.id}/file-versions/${updated.id}`,undefined,{status:404});await d.call('GET',`/groups/${root.id}/context`,undefined,{status:404});
  const targetKnowledge=await d.call('GET',`/groups/${target.id}/context?limit=100`);assert.ok(!JSON.stringify(targetKnowledge).includes('ROOT_ONLY_EVIDENCE_V2'));
  const members=(await a.call('GET',`/projects/${p.id}/members`)).items;await a.call('DELETE',`/projects/${p.id}/members/${w}`,{expectedVersion:members.find(m=>m.userId===w).version});
  deniedCLI(cli,'group','show','--group',sibling.id);assert.equal(cli('project','list').data.items.length,0);await c.call('GET',`/groups/${sibling.id}`,undefined,{status:404});assert.ok((await c.call('GET',`/projects/${other.id}`)).id===other.id);
  const retained=await d.stream(`/conversations/${target.id}/file-versions/${rootFile.id}/content`);assert.equal(retained.status,200);await retained.body.cancel();
  const dm=await c.call('POST','/direct-conversations',{userId:z});await c.call('POST',`/conversations/${dm.id}/messages`,{body:'Private across project membership changes'});await b.call('GET',`/conversations/${dm.id}/messages`,undefined,{status:404});assert.equal((await d.call('GET',`/conversations/${dm.id}/messages`)).items.length,1);
  report.checks.push('cross-project-exact-version-share','share-does-not-expand-context','project-removal-revokes-all-own-groups','other-project-and-existing-share-retained','platform-DM-third-party-denied');
 });
 report.status='passed';report.completedAt=new Date().toISOString();save();console.log(JSON.stringify(report));
}catch(error){report.status='failed';report.error=String(error);save();throw error;}
