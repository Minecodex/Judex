import fs from 'node:fs';import path from 'node:path';import crypto from 'node:crypto';import {execFile,execFileSync,spawn} from 'node:child_process';import {promisify} from 'node:util';
const exec=promisify(execFile),root=path.resolve(process.argv[2]??''),file=root+'/environment.json',plan=JSON.parse(fs.readFileSync(root+'/soak-plan.json','utf8'));let e=JSON.parse(fs.readFileSync(file,'utf8'));
const load=JSON.parse(fs.readFileSync(root+'/soak-progress.json','utf8')),started=Date.parse(load.registeredAt),scene=path.join(e.localRoot,'A-MiniIssue'),cli=path.join(scene,e.cliToolsRoot,'judex.exe'),config=path.join(scene,e.cliToolsRoot,'config/KX01');
const kube=async(...args)=>(await exec('kubectl',['--context','docker-desktop',...args],{windowsHide:true,encoding:'utf8',timeout:130000,maxBuffer:4<<20})).stdout;
const own=async()=>{const m=JSON.parse(await kube('get','namespace',e.namespace,'-o','json')).metadata;if(m.uid!==e.uid||m.labels?.['judex.dev/test-run']!==e.runId)throw Error('Test ownership changed');};await own();
const report={registeredAt:new Date().toISOString(),namespace:e.namespace,uid:e.uid,plan:plan.faults,windows:[],phase:'running'};
function save(){const p=root+'/fault-windows.json',temp=p+'.next';fs.writeFileSync(temp,JSON.stringify(report,null,2)+'\n');fs.renameSync(temp,p);}
save();
const native=args=>new Promise(resolve=>execFile(cli,['--server','http://127.0.0.1:'+e.port,'--profile','KX01','--project',plan.projectId,'--json',...args],{cwd:scene,windowsHide:true,encoding:'utf8',timeout:8000,maxBuffer:1<<20,env:{...process.env,JUDEX_CONFIG_DIR:config,JUDEX_TOKEN:''}},(error,stdout)=>{let r;try{r=JSON.parse(stdout)}catch{r={ok:false}}resolve({...r,ok:r.ok===true&&!error,exitCode:error?.code??0});}));
async function forward(){
 e=JSON.parse(fs.readFileSync(file,'utf8'));const pid=e.forward?.pid;
 if(Number.isSafeInteger(pid)){const raw=execFileSync('powershell.exe',['-NoProfile','-NonInteractive','-Command',`Get-CimInstance Win32_Process -Filter 'ProcessId=${pid}' | Select-Object Name,CommandLine | ConvertTo-Json -Compress`],{windowsHide:true,encoding:'utf8'}).trim();if(raw){const p=JSON.parse(raw);if(p.Name!=='kubectl.exe'||p.CommandLine!==`kubectl --context docker-desktop -n ${e.namespace} port-forward service/test ${e.port}:8080`)throw Error('Forward ownership changed');try{process.kill(pid)}catch{}}}
 const child=spawn('kubectl',['--context','docker-desktop','-n',e.namespace,'port-forward','service/test',e.port+':8080'],{windowsHide:true,stdio:['ignore','pipe','pipe']});const log=fs.createWriteStream(root+'/managed-forward.log',{flags:'a'});child.stdout.pipe(log);child.stderr.pipe(log);child.once('close',()=>log.end());e.forward={pid:child.pid,namespace:e.namespace,port:e.port,startedAt:new Date().toISOString()};fs.writeFileSync(file,JSON.stringify(e,null,2)+'\n');
}
for(const fault of plan.faults){
 if(!['deployment/test','pod/test-postgresql-0','pod/test-seaweedfs-0'].includes(fault.resource))throw Error('Unregistered resource');
 while(Date.now()-started<fault.atMs)await new Promise(r=>setTimeout(r,Math.min(30000,fault.atMs-(Date.now()-started))));
 await own();const begun=Date.now(),item={id:crypto.randomUUID(),kind:fault.kind,resource:fault.resource,startedAt:new Date(begun).toISOString(),probes:[]};report.windows.push(item);save();
 try{
  const before=JSON.parse(await kube('-n',e.namespace,'get',fault.resource,'-o','json'));item.beforeUID=before.metadata.uid;
  if(fault.resource==='deployment/test'){item.beforeGeneration=before.metadata.generation;await kube('-n',e.namespace,'rollout','restart',fault.resource);await kube('-n',e.namespace,'rollout','status',fault.resource,'--timeout=90s');await forward();}
  else{await kube('-n',e.namespace,'delete',fault.resource,'--wait=false');for(;;){const p=JSON.parse(await kube('-n',e.namespace,'get',fault.resource,'-o','json').catch(()=>'{"metadata":{},"status":{}}'));if(p.metadata.uid&&p.metadata.uid!==item.beforeUID&&p.status.containerStatuses?.every(c=>c.ready))break;if(Date.now()-begun>90000)throw Error('Replacement pod not ready');await new Promise(r=>setTimeout(r,500));}}
  const name='fault-probe-'+item.id+'.txt',filePath=root+'/'+name,bytes=Buffer.from(JSON.stringify({id:item.id,namespace:e.namespace})+'\n');fs.writeFileSync(filePath,bytes,{flag:'wx'});let version;
  while(Date.now()-begun<plan.thresholds.recoveryMs){
   const probeAt=new Date().toISOString();let uploaded;
   if(!version){uploaded=await native(['material','upload',filePath]);item.probes.push({at:probeAt,operation:'upload',ok:uploaded.ok,exitCode:uploaded.exitCode});if(uploaded.ok)version=uploaded.data?.versionId;}
   if(version){const output=root+'/probe-download-'+item.id+'-'+item.probes.length+'.txt',downloaded=await native(['material','download','--version',version,'--output',output]);item.probes.push({at:new Date().toISOString(),operation:'download',ok:downloaded.ok,exitCode:downloaded.exitCode});if(downloaded.ok&&fs.existsSync(output)&&fs.readFileSync(output).equals(bytes)){item.recoveryMs=Date.now()-begun;item.closedAt=new Date().toISOString();item.provenVersionId=version;item.passed=true;break;}}
   save();await new Promise(r=>setTimeout(r,500));
  }
  if(!item.passed)throw Error('Full upload/download recovery exceeded frozen 120s');
  const after=JSON.parse(await kube('-n',e.namespace,'get',fault.resource,'-o','json'));item.afterUID=after.metadata.uid;item.afterGeneration=after.metadata.generation;save();console.log(JSON.stringify({kind:item.kind,recoveryMs:item.recoveryMs,fullWriteReadVerified:true}));
 }catch(error){item.passed=false;item.error=error.message;report.phase='failed';save();throw error;}
}
report.phase='completed';report.completedAt=new Date().toISOString();save();
