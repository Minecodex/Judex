// Native Kimi may invoke this bounded CLI driver; it never calls a model itself.
import fs from 'node:fs';
import path from 'node:path';
import {execFile,execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';

export function mergeSavedCases(original,latest){
 const byClient=new Map(original.map(c=>[c.clientSubmissionId,c]));
 for(const item of latest){
  const old=byClient.get(item.clientSubmissionId);
  const oldSource=old?.source?.sourceId??old?.sourceId;
  const newSource=item.source?.sourceId??item.sourceId;
  if(oldSource&&newSource&&oldSource!==newSource)throw new Error('Recovered source changed');
  byClient.set(item.clientSubmissionId,{...old,...item,source:old?.source??item.source});
 }
 return [...byClient.values()];
}

export async function runRoutingCases({cases,saved=[],call,checkpoint,wait,allowRetry=false,maxPolls=60}){
 const result={cases:[],stopped:null};
 const save=async()=>checkpoint({...result,recordedAt:new Date().toISOString()});
 const stop=async(reason,item)=>{result.stopped={reason,case:item.case};await save();return result;};
 for(const item of cases){
  const old=saved.find(c=>c.clientSubmissionId===item.clientSubmissionId);
  const sourceId=old?.source?.sourceId??old?.sourceId;
  const entry={case:item.case,clientSubmissionId:item.clientSubmissionId,sourceId:sourceId??null,source:old?.source??null,analysis:null,retryRequested:false};
  result.cases.push(entry);
  if(!entry.sourceId){
   const response=await call(['submit','--file',item.file],item.clientSubmissionId);
   if(!response.ok||response.exitCode!==0||!response.data?.id)return stop('submission_failed',entry);
   entry.sourceId=response.data.id;
   // Persist the actual effect immediately, before polling or another case.
   await save();
  }
  let response=await call(['submission-analysis',entry.sourceId]);
  if(!response.ok||response.exitCode!==0)return stop('analysis_read_failed',entry);
  entry.analysis=response.data;
  const originalBatch=old?.source?.batchId??old?.analysis?.batchId;
  const originalAnalysis=old?.source?.analysisId??old?.analysis?.id;
  if(originalBatch&&response.data.batchId!==originalBatch)return stop('batch_changed',entry);
  if(originalAnalysis&&response.data.id!==originalAnalysis)return stop('analysis_changed',entry);
  entry.source??={sourceId:entry.sourceId,analysisId:response.data.id,batchId:response.data.batchId};
  if(response.data.state==='failed'&&allowRetry){
   const retry=await call(['task-analysis','retry',response.data.id],item.clientSubmissionId);
   entry.retryRequested=true;
   await save();
   if(!retry.ok||retry.exitCode!==0)return stop('retry_rejected',entry);
   response=await call(['submission-analysis',entry.sourceId]);
   if(!response.ok||response.exitCode!==0)return stop('analysis_read_failed',entry);
   entry.analysis=response.data;
  }
  for(let count=0;;count++){
   const analysis=entry.analysis;
   if(originalBatch&&analysis.batchId!==originalBatch)return stop('batch_changed',entry);
   if(originalAnalysis&&analysis.id!==originalAnalysis)return stop('analysis_changed',entry);
   await save();
   if(analysis.state==='completed')break;
   // Every non-progress state stops the matrix, including generic failed.
   if(!['queued','running'].includes(analysis.state))return stop('analysis_'+analysis.state,entry);
   if(count>=maxPolls)return stop('poll_limit',entry);
   await wait();
   response=await call(['submission-analysis',entry.sourceId]);
   if(!response.ok||response.exitCode!==0)return stop('analysis_read_failed',entry);
   entry.analysis=response.data;
  }
 }
 await save();
 return result;
}

if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url)){
 const root=path.resolve('.cache/real-host/20261012-closure');
 const environment=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
 const ns=JSON.parse(execFileSync('kubectl',['--context','docker-desktop','get','namespace',environment.namespace,'-o','json'],{encoding:'utf8',windowsHide:true})).metadata;
 if(ns.uid!==environment.uid||ns.labels?.['judex.dev/test-run']!==environment.runId)throw new Error('Owned namespace required');
 const workspace=path.resolve(environment.localRoot,'A-MiniIssue');
 const manifest=JSON.parse(fs.readFileSync(path.join(workspace,'cases/KX01-routing/manifest.json'),'utf8'));
 const file=path.join(root,'KX01-routing-driver-checkpoint.json');
 const recoveryFile=path.resolve('.cache/real-host/20261013-kimi-resume/routing-live-checkpoint.json');
 const original=JSON.parse(fs.readFileSync(recoveryFile,'utf8')).cases;
 const saved=mergeSavedCases(original,fs.existsSync(file)?JSON.parse(fs.readFileSync(file,'utf8')).cases:[]);
 if(manifest.cases.length!==7||manifest.cases.some(c=>!/^[-0-9a-f]{36}$/.test(c.clientSubmissionId)||!path.resolve(c.file).startsWith(workspace+path.sep)))throw new Error('Exact native preregistered owned cases required');
 const call=(args,requestId)=>new Promise((resolve,reject)=>execFile(path.join(workspace,'.tools/judex.exe'),
  ['--profile','KX01','--project','21a13532-7929-4c6c-99a7-01e044eee64e','--json',...(requestId?['--request-id',requestId]:[]),...args],
  {cwd:workspace,windowsHide:true,encoding:'utf8',maxBuffer:1<<20,env:{...process.env,JUDEX_TOKEN:'',JUDEX_CONFIG_DIR:path.join(workspace,'.tools/config/KX01')}},(error,stdout)=>{
   try{resolve({...JSON.parse(stdout),exitCode:error?.code??0});}catch{reject(new Error('Structured CLI result unavailable'));}
  }));
 const result=await runRoutingCases({cases:manifest.cases,saved,call,
  checkpoint:async value=>fs.writeFileSync(file,JSON.stringify(value,null,2)+'\n'),wait:()=>new Promise(resolve=>setTimeout(resolve,30000)),allowRetry:process.argv.includes('--retry-existing')});
 console.log(JSON.stringify({stopped:result.stopped,cases:result.cases.map(c=>({case:c.case,sourceId:c.sourceId,analysisId:c.analysis?.id,batchId:c.analysis?.batchId,state:c.analysis?.state,retryRequested:c.retryRequested}))}));
 if(result.stopped)process.exitCode=8;
}
