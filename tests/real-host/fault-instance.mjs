// Own a separate local fault target. Formal work is still performed by Kimi.
// start reads the current config; it never diagnoses or repairs that config.
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import net from 'node:net';
import {createHash,randomUUID} from 'node:crypto';
import {spawn,execFileSync} from 'node:child_process';
import {chromium} from '@playwright/test';

const [mode]=process.argv.slice(2);
if(!['prepare','start','stop','status'].includes(mode))throw new Error('Use prepare/start/stop/status');
const root=path.resolve('.cache/real-host/20261012-closure');
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const local=path.resolve(env.localRoot,'A-MiniIssue'),fault=path.resolve(local,'fault');
if(!fault.startsWith(local+path.sep))throw new Error('Fault directory escapes the approved scene');
const file=path.join(fault,'scenario.json'),configFile=path.join(fault,'config.json');
const python='C:/Users/23574/AppData/Local/Programs/Python/Python312/python.exe';
const hash=file=>createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const kube=(...args)=>JSON.parse(execFileSync('kubectl',args,{encoding:'utf8',windowsHide:true}));
const ns=kube('get','namespace',env.namespace,'-o','json').metadata;
if(ns.uid!==env.uid||ns.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Namespace ownership changed');
const within=value=>path.resolve(value).startsWith(fault+path.sep);
const save=state=>fs.writeFileSync(file,JSON.stringify(state,null,2)+'\n');
const load=()=>{
 const state=JSON.parse(fs.readFileSync(file,'utf8'));
 if(state.protocol!=='judex-real-host-fault/v1'||state.runId!==env.runId||state.namespaceUID!==env.uid||state.root!==fault||state.port!==18482)throw new Error('Fault ownership mismatch');
 return state;
};
const processInfo=pid=>{
 if(!Number.isSafeInteger(pid)||pid<1)throw new Error('Invalid owned PID');
 const output=execFileSync('powershell',['-NoProfile','-Command',`Get-CimInstance Win32_Process -Filter "ProcessId = ${pid}" | Select-Object ProcessId,ParentProcessId,ExecutablePath,CommandLine,CreationDate | ConvertTo-Json -Compress`],{encoding:'utf8',windowsHide:true}).trim();
 return output?JSON.parse(output):null;
};
const request=(route,method='GET',body)=>new Promise((resolve,reject)=>{
 const bytes=body===undefined?null:Buffer.from(JSON.stringify(body));
 const req=http.request({hostname:'127.0.0.1',port:18482,path:route,method,agent:false,timeout:2000,headers:bytes?{'Content-Type':'application/json','Content-Length':bytes.length}:{}},res=>{
  let chunks=[];res.on('data',chunk=>chunks.push(chunk));res.on('end',()=>{const raw=Buffer.concat(chunks);resolve({status:res.statusCode,body:JSON.parse(raw.toString('utf8'))});});
 });
 req.on('error',reject);req.on('timeout',()=>req.destroy(new Error('Owned fault HTTP timeout')));
 req.end(bytes);
});
const assertPortFree=()=>new Promise((resolve,reject)=>{
 const probe=net.createServer();probe.once('error',()=>reject(new Error('Fault port is already occupied; no process will be stopped')));
 probe.listen(18482,'127.0.0.1',()=>probe.close(resolve));
});
const launch=(source,db,logPath)=>{
 if(!within(source)||!within(db)||!within(logPath))throw new Error('Process paths escape the owned fault directory');
 const args=['-m','backend.server','--db',db,'--port','18482','--web-root',path.join(source,'web')];
 const log=fs.createWriteStream(logPath,{flags:'wx'});
 const child=spawn(python,args,{cwd:source,windowsHide:true,env:{...process.env,PYTHONIOENCODING:'utf-8',PYTHONPYCACHEPREFIX:path.join(fault,'pycache')},stdio:['ignore','pipe','pipe']});
 child.stdout.pipe(log,{end:false});child.stderr.pipe(log,{end:false});
 const completed=new Promise((resolve,reject)=>{child.once('error',reject);child.once('close',code=>{log.end();resolve(code);});});
 return {child,completed,args,log};
};

if(mode==='prepare'){
 if(fs.existsSync(fault)&&fs.readdirSync(fault).length)throw new Error('An existing fault target must be reviewed, never overwritten');
 const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'));
 const browser=await chromium.launch(),context=await browser.newContext({baseURL:`http://127.0.0.1:${env.port}`,storageState:actors.U0.statePath});
 let prerequisite,release;
 try{
  const get=async id=>{const response=await context.request.get(`/api/v1/projects/21a13532-7929-4c6c-99a7-01e044eee64e/tasks/${id}`);if(!response.ok())throw new Error('Formal prerequisite read failed');return(await response.json()).data;};
  prerequisite=await get('f118d0a6-cb9c-42fa-8a78-60947a1bf739');
  release=await get('8b355144-610b-4349-8031-662cb3174ee0');
  if(prerequisite.status!=='accepted'||!prerequisite.latestAcceptanceId||release.status!=='accepted'||!release.latestAcceptanceId)throw new Error('A33 and A24 must be formally accepted before fault injection');
 }finally{await context.close();await browser.close();}
 await assertPortFree();
 const receipt=JSON.parse(fs.readFileSync(path.join(local,'receipts/KA06-A24-delivery.json'),'utf8'));
 const material=receipt.materials.find(m=>m.versionId==='ac1f9342-62c3-46fd-bb93-2edadab8e053');
 if(!material||material.sha256!=='15fe1472bab45a6f3ff495e7dcca1eaed6e3c180935a90df1c4c5c57bf9c4d3a')throw new Error('Accepted immutable A24 release required');
 const archive=path.resolve(local,material.file);
 if(!archive.startsWith(local+path.sep)||hash(archive)!==material.sha256)throw new Error('Release bytes differ from the accepted original');
 fs.mkdirSync(path.join(fault,'history'),{recursive:true});fs.mkdirSync(path.join(fault,'data'));
 const unpack=path.join(fault,'original');
 const extraction=execFileSync(python,['-c',
  'import pathlib,sys,zipfile,hashlib,json; root=pathlib.Path(sys.argv[2]).resolve(); z=zipfile.ZipFile(sys.argv[1]); entries=z.infolist(); assert len(entries)==30 and sum(e.file_size for e in entries)<4194304\nfor e in entries:\n p=root/e.filename; assert not pathlib.PurePosixPath(e.filename).is_absolute() and ".." not in pathlib.PurePosixPath(e.filename).parts and ":" not in e.filename and "\\\\" not in e.filename and (e.external_attr>>16)&0o170000!=0o120000 and e.file_size<1048576 and p.resolve().is_relative_to(root)\n if not e.is_dir(): p.parent.mkdir(parents=True,exist_ok=True); p.write_bytes(z.read(e))\nsource=next(root.rglob("SOURCES.json")).parent; print(str(source))',archive,unpack],{encoding:'utf8',windowsHide:true}).trim();
 const source=path.resolve(extraction),db=path.join(fault,'data/miniissue.db');
 if(!within(source)||!fs.existsSync(path.join(source,'backend/server.py')))throw new Error('Unexpected source tree');
 const sourceHashes={};
 const walk=dir=>{for(const e of fs.readdirSync(dir,{withFileTypes:true})){const target=path.join(dir,e.name);if(e.isDirectory())walk(target);else sourceHashes[path.relative(source,target).replaceAll('\\','/')]=hash(target);}};
 walk(source);
 const good={source,db,port:18482};
 fs.writeFileSync(path.join(fault,'history/config-before.json'),JSON.stringify(good,null,2)+'\n');
 const first=launch(source,db,path.join(fault,'history/before.log'));
 try{
  let ready=false;
  for(let i=0;i<40;i++){try{ready=(await request('/api/health')).status===200;}catch{}if(ready)break;await new Promise(r=>setTimeout(r,200));}
  if(!ready)throw new Error('Owned source did not become healthy before injection');
  for(const title of ['Fault 演练 ÄPFEL','Literal 100% complete','Literal under_score','恢复后仍在的数据']){
   const result=await request('/api/issues','POST',{title,description:'A-P4 synthetic fixture only',severity:'medium'});
   if(result.status!==201)throw new Error('Owned seed create failed');
  }
  const before=await request('/api/issues?page=1&page_size=100');
  if(before.status!==200||before.body.total!==4)throw new Error('Owned initial rows not observed');
  fs.writeFileSync(path.join(fault,'history/before-http.json'),JSON.stringify({observedAt:new Date().toISOString(),health:await request('/api/health'),issues:before},null,2)+'\n');
 }finally{first.child.kill();await first.completed;}
 const beforeDbSha256=hash(db);
 // Controlled wrong configuration: SQLite receives an existing directory.
 // It fails startup without deleting/changing the four-row database.
 fs.writeFileSync(configFile,JSON.stringify({...good,db:path.join(fault,'data')},null,2)+'\n');
 const broken=launch(source,path.join(fault,'data'),path.join(fault,'history/injected-startup.log'));
 const injectedPid=broken.child.pid,exitCode=await broken.completed;
 let unavailable=false;
 try{await request('/api/health');}catch{unavailable=true;}
 if(!exitCode||!unavailable||hash(db)!==beforeDbSha256)throw new Error('Controlled failure/data-preservation assertion failed');
 const state={protocol:'judex-real-host-fault/v1',instanceId:randomUUID(),runId:env.runId,namespaceUID:env.uid,root:fault,port:18482,source,sourceMaterialVersionId:material.versionId,sourceArchiveSha256:material.sha256,sourceHashes,db,beforeDbSha256,prerequisite:{taskId:prerequisite.id,version:prerequisite.version,acceptanceId:prerequisite.latestAcceptanceId},releaseAcceptanceId:release.latestAcceptanceId,injection:{type:'local invalid SQLite path configuration',pid:injectedPid,exitCode,healthUnavailable:unavailable,config:configFile,log:path.join(fault,'history/injected-startup.log'),at:new Date().toISOString()},activeProcess:null,history:[]};
 save(state);console.log(JSON.stringify({prepared:true,instanceId:state.instanceId,port:18482,exitCode,healthUnavailable:unavailable,preservedDbSha256:beforeDbSha256,config:configFile}));
}else if(mode==='start'){
 const state=load(),config=JSON.parse(fs.readFileSync(configFile,'utf8'));
 if(config.source!==state.source||config.port!==18482||!within(config.db)||!fs.existsSync(config.db))throw new Error('Current config must keep the owned source/port/existing local data path');
 if(state.activeProcess&&processInfo(state.activeProcess.pid))throw new Error('An owned process is already running');
 for(const [relative,sha] of Object.entries(state.sourceHashes))if(hash(path.join(state.source,relative))!==sha)throw new Error('Fault recovery must preserve the accepted program bytes');
 await assertPortFree();
 const logPath=path.join(fault,'history',`start-${Date.now()}.log`),run=launch(state.source,path.resolve(config.db),logPath);
 const info=processInfo(run.child.pid);
 if(!info){await run.completed;throw new Error('Owned process exited before its identity was observed');}
 state.activeProcess={pid:run.child.pid,parentPid:process.pid,creationDate:info.CreationDate,db:path.resolve(config.db),argv:run.args,log:logPath,startedAt:new Date().toISOString()};
 save(state);console.log(JSON.stringify({started:true,...state.activeProcess}));
 const heartbeat=setInterval(()=>console.log('Owned fault process running on 18482'),45000);
 const exitCode=await run.completed;clearInterval(heartbeat);
 const current=load();
 if(current.activeProcess?.pid===run.child.pid){current.history.push({...current.activeProcess,exitCode,endedAt:new Date().toISOString()});current.activeProcess=null;save(current);}
 process.exitCode=exitCode??1;
}else if(mode==='stop'){
 const state=load(),active=state.activeProcess;
 if(!active){console.log('No owned fault process recorded');}
 else{
  const info=processInfo(active.pid);
  if(info){
   if(info.ParentProcessId!==active.parentPid||info.CreationDate!==active.creationDate||path.resolve(info.ExecutablePath).toLowerCase()!==path.resolve(python).toLowerCase()||!info.CommandLine.includes(active.db)||!within(active.db))throw new Error('PID identity mismatch; no process will be stopped');
   process.kill(active.pid);console.log('Stopped only the identity-verified owned fault PID '+active.pid);
  }else console.log('Recorded owned process has already exited');
 }
}else{
 const state=load();let health;
 try{health=await request('/api/health');}catch(error){health={unavailable:true,code:error.code??'timeout'};}
 console.log(JSON.stringify({instanceId:state.instanceId,health,recordedProcess:state.activeProcess,dbSha256:hash(state.db),matchesBeforeDbSha256:hash(state.db)===state.beforeDbSha256}));
}
