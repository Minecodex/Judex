// B/C team creation through actual assignment/invitation product pages.
import {chromium,expect} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {scenes} from './scene-definitions.mjs';
const scene=process.argv[2],definition=scenes[scene];
if(!definition)throw new Error('Use approved B or C scene');
const root='.cache/real-host/20261012-closure';
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'));
const result=JSON.parse(fs.readFileSync(path.join(root,definition.ownerSession+'-project-result.json'),'utf8'));
if(result.state!=='committed'||!/^project:[0-9a-f-]{36}$/.test(result.resultRef))throw new Error('Confirmed public Kimi project required');
const projectID=result.resultRef.slice(8),baseURL=`http://127.0.0.1:${env.port}`,prefix='/api/v1/projects/'+projectID;
const evidenceFile=path.join(root,scene+'-team.json');
const evidence=fs.existsSync(evidenceFile)?JSON.parse(fs.readFileSync(evidenceFile,'utf8')):{projectID,positions:{},invitations:{}};
if(evidence.projectID!==projectID)throw new Error('Existing scene belongs to another project');
const save=()=>fs.writeFileSync(evidenceFile,JSON.stringify(evidence,null,2)+'\n');
const browser=await chromium.launch(),owner=await browser.newContext({baseURL,storageState:actors.U0.statePath,viewport:{width:1440,height:1000}});
const page=await owner.newPage();
const read=async suffix=>{const response=await owner.request.get(prefix+suffix);if(!response.ok())throw new Error('Scene read failed '+response.status());return(await response.json()).data;};
const choose=async(id,value)=>{await page.getByTestId(id).click();await page.locator('[data-option-value='+JSON.stringify(value)+']').click();await page.getByRole('listbox',{includeHidden:true}).waitFor({state:'detached'});};
try{
 if((await read('')).title!==definition.title)throw new Error('Project differs from the approved scenario');
 await page.goto('/?project='+projectID+'&settings=team');
 for(const [key,name,prompt] of definition.positions){
  if(evidence.positions[key])continue;
  const current=(await read('/positions')).items.find(p=>p.name===name);
  if(current){evidence.positions[key]=current.id;save();continue;}
  await page.getByTestId('new-position').click();await page.getByTestId('position-name').fill(name);await page.getByTestId('position-prompt').fill(prompt);
  await choose('position-model',env.models.position);await page.getByTestId('save-position').click();await page.getByRole('dialog').waitFor({state:'detached'});
  const created=(await read('/positions')).items.find(p=>p.name===name);if(!created)throw new Error('Position not persisted');
  evidence.positions[key]=created.id;save();console.log('Created '+scene+' position '+name+' through UI');
 }
 const ownerPosition=evidence.positions.owner;
 if(!(await read('/identities')).items.some(i=>i.templateId===ownerPosition&&i.currentBinding?.userId===actors.U0.userId)){
  await page.getByTestId('assign-position-'+ownerPosition).click();await choose('position-assignment-member',actors.U0.userId);
  await page.getByTestId('assign-existing-position').click();await page.getByRole('dialog').waitFor({state:'detached'});
 }
 for(const [actorID,roles] of definition.members){
  if(!evidence.invitations[actorID]){
   await page.getByTestId('next-invite').click();await page.getByTestId('next-invite-name').fill(actors[actorID].email);
   for(const role of roles)await page.getByTestId('invite-position-'+evidence.positions[role]).getByRole('checkbox').press('Space');
   const sent=page.waitForResponse(r=>r.url().endsWith(prefix+'/invitations')&&r.request().method()==='POST');
   await page.getByTestId('next-create-invite').click();const response=await sent;if(!response.ok())throw new Error('Invitation creation failed');
   const invitation=(await response.json()).data,url=new URL(invitation.inviteUrl,baseURL);if(url.origin!==baseURL)throw new Error('Invitation left the isolated environment');
   evidence.invitations[actorID]={id:invitation.id,url:url.href,roles,accepted:false};save();
   await page.getByRole('dialog').getByRole('button',{name:'取消',exact:true}).click();await page.getByRole('dialog').waitFor({state:'detached'});
  }
  if(!evidence.invitations[actorID].accepted){
   const member=await browser.newContext({baseURL,storageState:actors[actorID].statePath});
   try{const invite=await member.newPage();await invite.goto(evidence.invitations[actorID].url);await invite.getByRole('button',{name:'接受邀请',exact:true}).click();await invite.waitForURL(url=>url.pathname==='/');await member.storageState({path:actors[actorID].statePath});evidence.invitations[actorID].accepted=true;save();}
   finally{await member.close();}
  }
 }
 evidence.identities=(await read('/identities')).items.filter(i=>i.kind==='position');
 if(evidence.identities.length!==3||new Set(evidence.identities.map(i=>i.id)).size!==3)throw new Error('Expected three independent scene identities');
 evidence.createdVia='actual team assignment and member invitation UI';save();
 const teamContext={projectID,projectTitle:definition.title,positions:evidence.positions,identities:evidence.identities.map(i=>({identityId:i.id,templateId:i.templateId,userId:i.currentBinding?.userId})),readFrom:'actual membership product APIs',observedAt:new Date().toISOString()};
 fs.writeFileSync(path.join(env.localRoot,definition.folder,'team-context.json'),JSON.stringify(teamContext,null,2)+'\n');
 await page.reload();await page.screenshot({path:path.join(root,scene+'-team-created.png'),fullPage:true});
 console.log(scene+' three templates and three distinct identities created and verified');
}finally{await owner.close();await browser.close();}
