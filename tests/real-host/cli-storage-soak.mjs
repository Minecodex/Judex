// Actual native CLI / PG / object-store workload. All request intervals and
// failures are retained so fault overlap is verifiable after recovery.
import fs from 'node:fs';import path from 'node:path';import crypto from 'node:crypto';import {execFile,execFileSync} from 'node:child_process';
import {overlappingFaults} from './fault-windows.mjs';
const root=path.resolve(process.argv[2]??''),plan=JSON.parse(fs.readFileSync(root+'/soak-plan.json','utf8')),e=JSON.parse(fs.readFileSync(root+'/environment.json','utf8'));
if(!plan.projectId||plan.durationMs!==3600000||plan.workers!==4)throw Error('Explicit frozen one-hour plan required');
const metadata=JSON.parse(execFileSync('kubectl',['--context','docker-desktop','get','namespace',e.namespace,'-o','json'],{windowsHide:true,encoding:'utf8'})).metadata;
if(metadata.uid!==e.uid||metadata.labels?.['judex.dev/test-run']!==e.runId)throw Error('Namespace ownership changed');
if(fs.existsSync(root+'/soak-progress.json'))throw Error('Existing run must not be overwritten');
const scene=path.join(e.localRoot,'A-MiniIssue'),cli=path.join(scene,e.cliToolsRoot,'judex.exe'),config=path.join(scene,e.cliToolsRoot,'config/KX01'),base='http://127.0.0.1:'+e.port;
const input=root+'/soak-inputs',downloads=root+'/soak-downloads';fs.mkdirSync(input,{recursive:true});fs.mkdirSync(downloads,{recursive:true});
const started=Date.now(),queue=[],records=[],active=new Set();let sequence=0,requestSequence=0,burstDone=false,halt=false;
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
const report={registeredAt:new Date(started).toISOString(),namespace:e.namespace,uid:e.uid,version:e.version,plan,phase:'running',generated:0,completed:0,peakBacklog:0,hashFailures:0,metadataShaChecks:0,downloadShaChecks:0,completedFiles:[],requests:0,failures:0,plannedFailures:0,unplannedFailures:0};
function windows(){return fs.existsSync(root+'/fault-windows.json')?JSON.parse(fs.readFileSync(root+'/fault-windows.json','utf8')).windows:[];}
function overlaps(begin,end){return overlappingFaults(begin,end,windows());}
function redact(value){let text=String(value??'');try{for(const f of fs.readdirSync(config).filter(f=>f.includes('credential'))){const s=JSON.parse(fs.readFileSync(path.join(config,f),'utf8'));for(const [k,v] of Object.entries(s))if((k==='KX01'||k.startsWith('access:'))&&typeof v==='string'&&v)text=text.split(v).join('[redacted]');}}catch{}return text.replace(/\bsk-[\w-]{12,}\b/g,'[redacted]').replace(/Bearer\s+[^\s"']+/gi,'Bearer [redacted]').slice(0,1000);}
function save(){report.elapsedMs=Date.now()-started;report.backlog=queue.length;report.requests=records.length;const failures=records.filter(r=>!r.ok);report.failures=failures.length;report.plannedFailures=failures.filter(r=>overlaps(r.startedMs,r.endedMs).length).length;report.unplannedFailures=report.failures-report.plannedFailures;report.unplannedFailureRate=report.requests?report.unplannedFailures/report.requests:0;fs.writeFileSync(root+'/soak-progress.json',JSON.stringify(report,null,2)+'\n');}
function generate(){const name='strict-soak-'+e.runId+'-'+String(sequence++).padStart(5,'0')+'.txt',header=JSON.stringify({run:e.runId,name})+'\n',body=Buffer.from((header+'actual durable storage payload\n'.repeat(3000)).slice(0,plan.payloadBytes)),file=path.join(input,name);if(body.length!==plan.payloadBytes)throw Error('Payload size mismatch');fs.writeFileSync(file,body,{flag:'wx'});queue.push({name,file,sha256:hash(body),attempts:0});report.generated++;report.peakBacklog=Math.max(report.peakBacklog,queue.length);}
const native=(worker,operation,args)=>new Promise(resolve=>{
 const begin=Date.now(),id=++requestSequence;const child=execFile(cli,['--server',base,'--profile','KX01','--project',plan.projectId,'--json',...args],{cwd:scene,windowsHide:true,encoding:'utf8',timeout:25000,maxBuffer:2<<20,env:{...process.env,JUDEX_CONFIG_DIR:config,JUDEX_TOKEN:''}},(error,stdout)=>{
  active.delete(child);let result;try{result=JSON.parse(stdout)}catch{result={ok:false,error:'Native response was not structured JSON'};}
  const end=Date.now(),record={id,worker,operation,startedAt:new Date(begin).toISOString(),endedAt:new Date(end).toISOString(),startedMs:begin,endedMs:end,durationMs:end-begin,exitCode:error?.code??0,ok:result.ok===true&&!error,overlappingFaultsAtReturn:overlaps(begin,end),error:result.ok?null:redact(typeof result.error==='string'?result.error:JSON.stringify(result.error)),versionId:result.data?.versionId??null};records.push(record);fs.appendFileSync(root+'/soak-requests.jsonl',JSON.stringify(record)+'\n');resolve({...result,exitCode:record.exitCode,ok:record.ok});
 });active.add(child);
});
for(let i=0;i<plan.initialBacklog;i++)generate();save();
const workers=Array.from({length:plan.workers},async(_,worker)=>{
 let last;
 while(!halt&&Date.now()-started<plan.durationMs){
  const begin=Date.now();
  if(fs.existsSync(root+'/stop-soak.json')){halt=true;break;}
  if(!burstDone&&begin-started>=plan.burst.afterMs){burstDone=true;for(let i=0;i<plan.burst.files;i++)generate();}
  if(!queue.length&&begin-started<plan.produceUntilMs)generate();
  const item=queue.shift();
  if(item){item.attempts++;const result=await native(worker,'upload',['material','upload',item.file]);
   if(result.ok&&result.data?.versionId){if(result.data.sha256!==item.sha256){report.hashFailures++;halt=true;throw Error('Actual uploaded SHA differs');}report.metadataShaChecks++;report.completed++;last={name:item.name,versionId:result.data.versionId,sha256:item.sha256,attempts:item.attempts,acknowledgedAt:new Date().toISOString()};report.completedFiles.push(last);}
   else queue.push(item);
  }
  if(last&&(report.completed%8===0||!item)){
   const output=path.join(downloads,worker+'-'+last.versionId+'-'+requestSequence+'.txt'),result=await native(worker,'download',['material','download','--version',last.versionId,'--output',output]);
   if(result.ok){if(!fs.existsSync(output)||hash(fs.readFileSync(output))!==last.sha256){report.hashFailures++;halt=true;throw Error('Actual downloaded SHA differs');}report.downloadShaChecks++;}
  }
  await new Promise(r=>setTimeout(r,Math.max(0,plan.perWorkerIntervalMs-(Date.now()-begin))));
 }
});
const timer=setInterval(()=>{save();console.log(JSON.stringify({seconds:Math.floor(report.elapsedMs/1000),completed:report.completed,backlog:report.backlog,failures:report.failures,unplanned:report.unplannedFailures}));},45000);
try{await Promise.all(workers);report.phase=halt?'interrupted':'completed';report.completedAt=new Date().toISOString();save();report.requestsClassifiedByFinalFaultWindows=true;report.thresholdsPassed=!halt&&report.backlog===0&&report.hashFailures===0&&report.unplannedFailureRate<=plan.thresholds.unplannedFailureRate;fs.writeFileSync(root+'/soak-result.json',JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify({phase:report.phase,thresholdsPassed:report.thresholdsPassed,completed:report.completed,unplanned:report.unplannedFailures}));}
catch(error){halt=true;report.phase='failed';report.error=error.message;save();throw error;}
finally{clearInterval(timer);for(const child of active)child.kill();}
