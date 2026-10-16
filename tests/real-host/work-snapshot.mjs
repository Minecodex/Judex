// Read-only current formal work, separate from the model's original draft.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
import {chromium} from '@playwright/test';
const root='.cache/real-host/20261012-closure',scene=process.argv[2]??'A';
if(!['A','B','C'].includes(scene))throw new Error('Known scene required');
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const namespace=JSON.parse(execFileSync('kubectl',['get','namespace',env.namespace,'-o','json'],{encoding:'utf8',windowsHide:true}));
if(namespace.metadata.uid!==env.uid||namespace.metadata.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Owned namespace changed');
const team=JSON.parse(fs.readFileSync(path.join(root,scene+'-team.json'),'utf8'));
const actor=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8')).U0;
const browser=await chromium.launch(),context=await browser.newContext({baseURL:`http://127.0.0.1:${env.port}`,storageState:actor.statePath});
try{
 const get=async suffix=>{const response=await context.request.get(`/api/v1/projects/${team.projectID}${suffix}`);if(!response.ok())throw new Error('Current work read failed '+response.status());return(await response.json()).data;};
 const plans=(await get('/plans')).items.filter(p=>p.title.startsWith(scene+'-P'));
 const tasks=(await get('/tasks')).items.filter(t=>plans.some(p=>p.id===t.planId));
 const snapshot={projectId:team.projectID,observedAt:new Date().toISOString(),plans,tasks,source:'current named U0 browser-authenticated read-only product APIs'};
 fs.writeFileSync(path.join(root,scene+'-current-work.json'),JSON.stringify(snapshot,null,2)+'\n');
 const directory={A:'A-MiniIssue',B:'B-VisualAssets',C:'C-OfficeReports'}[scene];
 fs.writeFileSync(path.join(env.localRoot,directory,'work-context.json'),JSON.stringify(snapshot,null,2)+'\n');
 for(const plan of plans){
  const code=plan.title.split(' ')[0],file=path.join(root,code+'-approved.json');
  if(fs.existsSync(file)){const original=JSON.parse(fs.readFileSync(file,'utf8'));original.plan=plan;original.tasks=tasks.filter(t=>t.planId===plan.id);original.currentReadAt=snapshot.observedAt;fs.writeFileSync(file,JSON.stringify(original,null,2)+'\n');}
 }
 console.log(JSON.stringify({scene,plans:plans.map(p=>({code:p.title.split(' ')[0],status:p.status,version:p.version,referenceTaskIds:p.referenceTaskIds})),tasks:tasks.map(t=>({code:t.title.split(' ')[0],status:t.status,version:t.version})),accepted:tasks.filter(t=>t.status==='accepted').length}));
}finally{await context.close();await browser.close();}
