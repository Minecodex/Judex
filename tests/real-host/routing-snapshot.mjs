// Recover only real sources matching the native session's preregistered keys.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
import {chromium} from '@playwright/test';
const root='.cache/real-host/20261012-closure',out='.cache/real-host/20261013-kimi-resume';
const environment=JSON.parse(fs.readFileSync(root+'/environment.json','utf8'));
const run=(...args)=>execFileSync('kubectl',['--context','docker-desktop',...args],{encoding:'utf8',windowsHide:true,maxBuffer:1<<20});
const ns=JSON.parse(run('get','namespace',environment.namespace,'-o','json')).metadata;
if(ns.uid!==environment.uid||ns.labels?.['judex.dev/test-run']!==environment.runId)throw new Error('Owned namespace required');
const manifest=JSON.parse(fs.readFileSync(path.join(environment.localRoot,'A-MiniIssue/cases/KX01-routing/manifest.json'),'utf8'));
if(manifest.cases.length!==7||manifest.cases.some(c=>!/^[-0-9a-f]{36}$/.test(c.clientSubmissionId)))throw new Error('Seven fixed preregistered sources required');
const project='21a13532-7929-4c6c-99a7-01e044eee64e';
const keys=manifest.cases.map(c=>`'${c.clientSubmissionId}'`).join(',');
const rows=JSON.parse(run('-n',environment.namespace,'exec','statefulset/'+environment.release+'-postgresql','--','psql','-U','judex','-d','judex','-At','-c',
 `SELECT COALESCE(jsonb_agg(jsonb_build_object('clientId',s.client_submission_id,'sourceId',s.id,'actorUserId',s.actor_user_id,'analysisId',a.id,'batchId',a.batch_id,'state',a.state,'errorCode',a.error_code)),'[]'::jsonb) FROM submissions s LEFT JOIN task_analyses a ON a.source_id=s.id AND a.project_id=s.project_id WHERE s.project_id='${project}' AND s.client_submission_id IN (${keys})`));
const actor=JSON.parse(fs.readFileSync(root+'/actors/actors.json','utf8')).U1;
if(rows.some(r=>r.actorUserId!==actor.userId)||new Set(rows.map(r=>r.clientId)).size!==rows.length)throw new Error('Exact native U1 sources required');
const browser=await chromium.launch(),context=await browser.newContext({baseURL:`http://127.0.0.1:${environment.port}`,storageState:actor.statePath});
try{
 const get=async suffix=>{const r=await context.request.get(`/api/v1/projects/${project}${suffix}`).catch(()=>{throw new Error('Authenticated read transport failed; request metadata omitted');});if(!r.ok())throw new Error('Read failed '+r.status());return(await r.json()).data;};
 const cases=[];
 for(const item of manifest.cases){
  const row=rows.find(r=>r.clientId===item.clientSubmissionId);
  const analysis=row?await get('/submissions/'+row.sourceId+'/analysis'):null;
  const markers=['RH_BACKEND_PRIVATE','RH_OPS_PRIVATE','RH_QA_ALPHA_PRIVATE','RH_QA_BETA_PRIVATE'].filter(m=>JSON.stringify(analysis).includes(m));
  cases.push({case:item.case,clientSubmissionId:item.clientSubmissionId,expectedPreregistered:item.expectedPreregistered,source:row??null,analysis:markers.length?null:analysis,privateControlledMarkerCount:markers.length,executed:Boolean(row)});
 }
 const task=await get('/tasks/f3309487-a97d-4880-95ab-6eb1d290e69c');
 const result={observedAt:new Date().toISOString(),kind:'operator read-only recovery of actual native routing sources after quota interruption',projectId:project,
  preregisteredAt:manifest.createdAtUtc,cases,formalTask:{id:task.id,status:task.status,version:task.version,workflowId:task.workflowId,nodeId:task.nodeId,latestAcceptanceId:task.latestAcceptanceId},
  completed:cases.filter(c=>c.analysis?.state==='completed').length,notSubmitted:cases.filter(c=>!c.executed).length,
  scope:'Actual model outputs only; incomplete cases are not passes. Native final report absent. Resume existing source/analysis/batch; never reset or resubmit completed cases.'};
 fs.writeFileSync(out+'/routing-live-checkpoint.json',JSON.stringify(result,null,2)+'\n');
 console.log(JSON.stringify({observedAt:result.observedAt,completed:result.completed,notSubmitted:result.notSubmitted,cases:cases.map(c=>({case:c.case,source:c.source,state:c.analysis?.state,privateControlledMarkerCount:c.privateControlledMarkerCount})),formalTask:result.formalTask}));
}finally{await context.close();await browser.close();}
