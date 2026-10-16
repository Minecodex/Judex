// Operator transport probes replay a real Kimi-authored source, never new work.
import fs from 'node:fs';
import path from 'node:path';
import {execFile,execFileSync} from 'node:child_process';
import {randomUUID,createHash} from 'node:crypto';
const root='.cache/real-host/20261012-closure',out='.cache/real-host/20261013-kimi-resume';
const environment=JSON.parse(fs.readFileSync(root+'/environment.json','utf8'));
const run=(...args)=>execFileSync('kubectl',['--context','docker-desktop',...args],{encoding:'utf8',windowsHide:true,maxBuffer:1<<20});
const ns=JSON.parse(run('get','namespace',environment.namespace,'-o','json')).metadata;
if(ns.uid!==environment.uid||ns.labels?.['judex.dev/test-run']!==environment.runId)throw new Error('Owned namespace required');
const workspace=path.join(environment.localRoot,'A-MiniIssue');
const inputFile=path.join(workspace,'cases/KX01-routing/1-bound.json');
const body=JSON.parse(fs.readFileSync(inputFile,'utf8'));
const project='21a13532-7929-4c6c-99a7-01e044eee64e',actor='98e2f197-1c7e-4b46-9fd6-7917fa20b05b';
if(body.taskId!=='f3309487-a97d-4880-95ab-6eb1d290e69c'||body.identityId!=='4b02a3ee-4c71-488a-a985-827d641558f7'||!/^[-0-9a-f]{36}$/.test(body.clientSubmissionId)||body.purpose!=='message')throw new Error('Actual preregistered bound source required');
const query=()=>JSON.parse(run('-n',environment.namespace,'exec','statefulset/'+environment.release+'-postgresql','--','psql','-U','judex','-d','judex','-At','-c',
 `SELECT jsonb_build_object('sources',count(*),'ids',jsonb_agg(s.id),'analyses',(SELECT count(*) FROM task_analyses a JOIN submissions x ON a.source_id=x.id WHERE x.project_id='${project}' AND x.actor_user_id='${actor}' AND x.client_submission_id='${body.clientSubmissionId}'),'batches',(SELECT count(DISTINCT a.batch_id) FROM task_analyses a JOIN submissions x ON a.source_id=x.id WHERE x.project_id='${project}' AND x.actor_user_id='${actor}' AND x.client_submission_id='${body.clientSubmissionId}')) FROM submissions s WHERE s.project_id='${project}' AND s.actor_user_id='${actor}' AND s.client_submission_id='${body.clientSubmissionId}'`));
const before=query();
if(before.sources!==1||before.analyses!==1||before.batches!==1)throw new Error('One existing Kimi source and analysis required before any replay');
const requestId=randomUUID();
const cli=(file)=>new Promise((resolve,reject)=>execFile(path.join(workspace,'.tools/judex.exe'),['--profile','KX01','--json','--request-id',requestId,'submit','--file',file],
 {cwd:workspace,windowsHide:true,encoding:'utf8',maxBuffer:1<<20,env:{...process.env,JUDEX_TOKEN:'',JUDEX_CONFIG_DIR:path.join(workspace,'.tools/config/KX01')}},(error,stdout)=>{
  let parsed;try{parsed=JSON.parse(stdout);}catch{return reject(new Error('Structured CLI result missing'));}
  resolve({exitCode:error?.code??0,ok:parsed.ok,data:parsed.data,error:parsed.error});
 }));
// Intentional concurrency is the transport behavior under test.
const pair=await Promise.all([cli(inputFile),cli(inputFile)]);
const changedFile=path.resolve(out,'idempotency-changed-bound.json');
fs.writeFileSync(changedFile,JSON.stringify({...body,text:body.text+' Operator same-key changed-body rejection probe.'},null,2)+'\n');
const conflict=await cli(changedFile),after=query();
const compact=result=>({exitCode:result.exitCode,ok:result.ok,sourceId:result.data?.id,error:result.error});
const result={observedAt:new Date().toISOString(),kind:'live CLI concurrent replay and changed-payload conflict',scope:'operator replay of an already registered real Kimi source; no new message, formal task or model budget reset',
 originalClientSubmissionId:body.clientSubmissionId,originalBodySha256:createHash('sha256').update(fs.readFileSync(inputFile)).digest('hex'),before,pair:pair.map(compact),conflict:compact(conflict),after,
 passed:pair.every(r=>r.exitCode===0&&r.ok&&r.data?.id===before.ids[0])&&conflict.exitCode!==0&&conflict.ok===false&&JSON.stringify(after)===JSON.stringify(before)};
fs.writeFileSync(out+'/live-cli-idempotency.json',JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(result));
if(!result.passed)process.exitCode=1;
