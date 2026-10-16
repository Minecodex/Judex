// Repair only the independently observed missing-target C rules through U0 UI.
import fs from 'node:fs';
import path from 'node:path';
import {chromium,expect} from '@playwright/test';
const root='.cache/real-host/20261012-closure',e=JSON.parse(fs.readFileSync(root+'/environment.json','utf8'));
if(e.version!=='0.1.0-realhost.20261014.20')throw Error('Verified .20 required');
const a=JSON.parse(fs.readFileSync(root+'/actors/actors.json','utf8')).U0,team=JSON.parse(fs.readFileSync(root+'/C-team.json','utf8'));
const file=root+'/C-workflows-reviewed.json',reviewed=JSON.parse(fs.readFileSync(file,'utf8'));
const browser=await chromium.launch(),c=await browser.newContext({baseURL:`http://127.0.0.1:${e.port}`,storageState:a.statePath,viewport:{width:1440,height:1080}}),page=await c.newPage();
const prefix='/api/v1/projects/'+team.projectID;
const get=async suffix=>{const r=await c.request.get(prefix+suffix).catch(()=>{throw Error('Owned API transport failed; authentication metadata omitted');});if(!r.ok())throw Error('Owned API read failed '+r.status());return(await r.json()).data;};
const missing=rule=>!rule.targetId||rule.targetId==='00000000-0000-0000-0000-000000000000';
const changes=[];
try{
 const heads=(await get('/workflows')).items.filter(f=>/^C-W[12] /.test(f.name));if(heads.length!==2)throw Error('Exact C flows required');
 for(const head of heads){
  const versions=(await get('/workflows/'+head.id+'/versions')).items,current=versions.find(v=>v.id===head.publishedVersionId),entry=reviewed.workflows.find(f=>f.id===head.id);
  const old=versions.find(v=>v.id===(entry.originalPublishedVersionId??entry.publishedVersionId));
  if(!old||versions.some(v=>v.state==='draft'))throw Error('Published head without unreviewed draft required');
  const invalid=(old.body.hardRules??[]).filter(missing);if(invalid.length!==4)throw Error('Four observed missing-target rules required');
  const expected=structuredClone(old.body);expected.hardRules=expected.hardRules.filter(r=>!missing(r));
  if(current.id!==old.id){
   if(JSON.stringify(current.body)!==JSON.stringify(expected))throw Error('Existing publication differs from exact target repair');
   entry.originalPublishedVersionId=old.id;entry.publishedVersionId=current.id;
   const change={workflowId:head.id,oldVersionId:old.id,newVersionId:current.id,removedOnlyMissingTargetRules:invalid,actor:'U0',via:'actual owner UI publication recovered after post-click editor wait timeout',at:current.publishedAt};
   if(!(entry.publicationRevisions??[]).some(r=>r.newVersionId===current.id))(entry.publicationRevisions??=[]).push(change);
   changes.push(change);fs.writeFileSync(file,JSON.stringify(reviewed,null,2)+'\n');console.log('Recovered confirmed UI publication '+head.name);continue;
  }
  await page.goto('/?project='+team.projectID+'&settings=flows&settingsItem='+head.id);await page.getByTestId('edit-workflow').click();
  await expect(page.getByTestId('workflow-hard-rules')).toBeVisible();await expect(page.getByTestId('save-workflow-draft')).toBeDisabled();
  await page.screenshot({path:root+'/C-'+head.id+'-missing-target-blocked.png',fullPage:true});
  // Removing unresolved rules preserves every concrete gate and all nodes.
  for(let i=0;i<invalid.length;i++)await page.getByTestId('workflow-remove-hard-rule-0').click();
  await expect(page.getByTestId('save-workflow-draft')).toBeEnabled();
  const saving=page.waitForResponse(r=>r.url().endsWith('/workflows/'+head.id+'/draft')&&r.request().method()==='PUT');await page.getByTestId('save-workflow-draft').click();if(!(await saving).ok())throw Error('UI draft save failed');
  const draft=(await get('/workflows/'+head.id+'/versions')).items.find(v=>v.state==='draft');
  if(!draft||JSON.stringify(draft.body)!==JSON.stringify(expected))throw Error('Unrelated workflow fields changed');
  await page.screenshot({path:root+'/C-'+head.id+'-target-repair-review.png',fullPage:true});
  const publishing=page.waitForResponse(r=>r.url().endsWith('/workflows/'+head.id+'/publish')&&r.request().method()==='POST');await page.getByTestId('publish-workflow-draft').click();const r=await publishing;if(!r.ok())throw Error('Owner UI publication failed');const published=(await r.json()).data;
  await page.reload();
  const history=(await get('/workflows/'+head.id+'/versions')).items;
  if(!history.some(v=>v.id===old.id&&JSON.stringify(v.body)===JSON.stringify(old.body)))throw Error('Old published version changed');
  entry.originalPublishedVersionId??=entry.publishedVersionId;entry.publishedVersionId=published.id;
  const change={workflowId:head.id,oldVersionId:old.id,newVersionId:published.id,removedOnlyMissingTargetRules:invalid,actor:'U0',via:'actual owner edit/remove/save/publish UI',at:new Date().toISOString()};(entry.publicationRevisions??=[]).push(change);changes.push(change);fs.writeFileSync(file,JSON.stringify(reviewed,null,2)+'\n');console.log('U0 repaired '+head.name+' through actual editor');
 }
 const workflows=[];for(const flow of reviewed.workflows){const v=(await get('/workflows/'+flow.id+'/versions')).items.find(v=>v.id===flow.publishedVersionId);workflows.push({id:flow.id,versionId:v.id,publishedVersionId:v.id,state:v.state,body:v.body});}
 fs.writeFileSync(path.join(e.localRoot,'C-OfficeReports/workflow-context.json'),JSON.stringify({projectId:team.projectID,workflows},null,2)+'\n');
 const tasks=(await get('/tasks')).items;for(const [code,previous] of [['C12','C11'],['C13','C12'],['C21','C13'],['C22','C21']]){
  const task=tasks.find(t=>t.title.startsWith(code+' ')),context=await get('/tasks/'+task.id);
  const predecessor=tasks.find(t=>t.title.startsWith(previous+' ')),gate=context.requirements[0];
  if(context.requirements.length!==1||gate.kind!=='task_acceptance'||gate.targetId!==predecessor.id||gate.phase!=='start'||!gate.hard||gate.waived)throw Error('Exact real task acceptance gate changed or placeholder remains');
 }
 fs.writeFileSync(root+'/C-workflow-target-repair.json',JSON.stringify({version:e.version,changes,realTaskGatesPreserved:4,oldPublishedOriginalsPreserved:true,observedAt:new Date().toISOString()},null,2)+'\n');console.log('All4real task gates preserved; missing-target blockers removed');
}finally{await c.close();await browser.close();}
