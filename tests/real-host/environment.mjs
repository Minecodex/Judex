// Infrastructure only. Business projects, users and decisions are created by
// the actual CLI/browser/Kimi product flows and must not be seeded here.
import {execFileSync, spawn} from 'node:child_process';
import {randomUUID} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
const [mode, manifestPath, version] = process.argv.slice(2);
const run=(name,args,input)=>execFileSync(name,args,{encoding:'utf8',input,windowsHide:true,timeout:300000,maxBuffer:16*1024*1024});
const kube=(...args)=>run('kubectl',args);
let state;
const save=()=>fs.writeFileSync(manifestPath,JSON.stringify(state,null,2)+'\n');
const own=()=>{
 const ns=JSON.parse(kube('get','namespace',state.namespace,'-o','json'));
 if(ns.metadata.uid!==state.uid||ns.metadata.labels?.['judex.dev/test-run']!==state.runId)throw new Error('Namespace ownership changed');
};
if(!manifestPath?.startsWith('.cache/real-host/'))throw new Error('Use a task-owned .cache/real-host manifest');
if(mode==='prepare'){
 if(fs.existsSync(manifestPath))throw new Error('Manifest exists; use deploy/forward to resume');
 const runId='judex-ui-realhost-'+new Date().toISOString().replace(/[-:T]/g,'').slice(0,14)+'-'+randomUUID().slice(0,6);
 fs.mkdirSync(path.dirname(manifestPath),{recursive:true});
 const ns=JSON.parse(run('kubectl',['create','-f','-','-o','json'],JSON.stringify({apiVersion:'v1',kind:'Namespace',metadata:{name:runId,labels:{'judex.dev/test-run':runId}}})));
 const models={primary:randomUUID(),position:randomUUID(),free:randomUUID(),visionFree:randomUUID()};
 state={runId,namespace:runId,uid:ns.metadata.uid,release:'test',localRoot:'D:/workspace/judex-real-host/20261012-closure',models,port:18182,stage:'prepared',createdAt:new Date().toISOString()};save();
 const entries=[
  {id:models.primary,displayName:'GLM 5.3 Coding Plan',model:'glm-5.3',baseUrl:'https://open.bigmodel.cn/api/coding/paas/v4',quotaFallbackIds:[models.position,models.free]},
  {id:models.position,displayName:'GLM 5.3 Flash Coding Plan',model:'glm-5.3-flash',baseUrl:'https://open.bigmodel.cn/api/coding/paas/v4',quotaFallbackIds:[models.primary,models.free]},
  {id:models.free,displayName:'GLM 4.5 Flash quota fallback',model:'glm-4.5-flash',baseUrl:'https://open.bigmodel.cn/api/paas/v4'},
  {id:models.visionFree,displayName:'GLM 4.6V Flash quota fallback',model:'glm-4.6v-flash',baseUrl:'https://open.bigmodel.cn/api/paas/v4',vision:true},
 ].map(e=>({provider:'openai-compatible',apiKeyEnv:'JUDEX_GLM_CODING_KEY',maxInputTokens:128000,maxOutputTokens:8192,toolCalling:true,streaming:true,vision:false,...e}));
 run('kubectl',['create','-f','-'],JSON.stringify({apiVersion:'v1',kind:'Secret',metadata:{name:'real-model-catalog',namespace:runId,labels:{'judex.dev/test-run':runId}},stringData:{'models.yaml':JSON.stringify({models:entries})}}));
 console.log(JSON.stringify({manifest:manifestPath,namespace:runId,uid:state.uid,models}));
}else{
 state=JSON.parse(fs.readFileSync(manifestPath,'utf8'));own();
 if(mode==='configure-analysis-models'){
  const secret=JSON.parse(kube('-n',state.namespace,'get','secret','real-model-catalog','-o','json'));
  if(secret.metadata.labels?.['judex.dev/test-run']!==state.runId)throw new Error('Model catalog ownership changed');
  const catalog=JSON.parse(Buffer.from(secret.data['models.yaml'],'base64').toString('utf8'));
  for(const id of [state.models.primary,state.models.position]){
   const entry=catalog.models.find(m=>m.id===id);if(!entry||!['glm-5.3','glm-5.3-flash'].includes(entry.model))throw new Error('Actual admitted Coding Plan models required');
   entry.requestOptions={...entry.requestOptions,thinkingType:'disabled'};
  }
  kube('-n',state.namespace,'patch','secret','real-model-catalog','--type=json','-p',JSON.stringify([{op:'test',path:'/metadata/resourceVersion',value:secret.metadata.resourceVersion},{op:'replace',path:'/data/models.yaml',value:Buffer.from(JSON.stringify(catalog)).toString('base64')} ]));
  state.analysisModelThinkingType='disabled';save();console.log('Owned analysis model options configured; token caps unchanged; next deployment loads them');
 }else if(mode==='deploy'){
  if(!/^[0-9A-Za-z][0-9A-Za-z.+-]*$/.test(version??''))throw new Error('Pass the built release version');
  const folder=path.resolve('dist/release',version);
  const built=JSON.parse(fs.readFileSync(path.join(folder,'manifest.json'),'utf8'));
  if(built.version!==version)throw new Error('Release mismatch');
  const images={server:'judex/server:collaboration-realhost-'+version,parser:'judex/analysis-sandbox:collaboration-realhost-'+version,converter:'judex/material-converter:collaboration-realhost-'+version};
  run('docker',['tag','judex/analysis-sandbox:real-host-20261012',images.parser]);
  run('docker',['tag','judex/material-converter:local-materials',images.converter]);
  run(process.execPath,['tests/k8s/load-images.mjs',state.namespace,state.runId,state.uid,...Object.values(images)]);
  const base=`http://127.0.0.1:${state.port}`;
  const values={environment:'development',image:{repository:'judex/server',tag:images.server.split(':')[1],pullPolicy:'Never'},
   server:{mode:'all',allowedOrigins:[base],previewOrigin:`http://localhost:${state.port}`,modelCatalogSecret:'real-model-catalog',modelEnvSecret:'real-model-key',
    modelGateway:{protocol:'openai-compatible',baseUrl:'https://open.bigmodel.cn/api/coding/paas/v4',model:'glm-5.3',apiKeyKey:'JUDEX_GLM_CODING_KEY'},
    sandboxImage:images.parser,sandboxCPU:'500m',sandboxMemory:'512Mi',materialProjection:{enabled:true},materialPreview:{enabled:true,image:images.converter}},
   sandbox:{enabled:true},opensandbox:{'opensandbox-controller':{enabled:false,crds:{install:false}}},
   postgresql:{embedded:{storage:'1Gi'}},objectStorage:{embedded:{storage:'2Gi',maxVolumes:2,growthCount:1,volumeSizeLimitMB:256}}};
  const valuesPath=path.join(path.dirname(manifestPath),'values.json');fs.writeFileSync(valuesPath,JSON.stringify(values,null,2));
  state.images=images;state.version=version;state.sourceTreeHash=built.sourceTreeHash;state.stage='deploying';save();
  run('helm',['upgrade','--install',state.release,'deploy/helm/judex','-n',state.namespace,'-f',valuesPath,'--wait','--timeout','240s']);
  own();kube('-n',state.namespace,'set','env','deployment/'+state.release,'JUDEX_REGISTER_PER_IP=1000');
  kube('-n',state.namespace,'rollout','status','deployment/'+state.release,'--timeout=120s');
  const sandbox=JSON.parse(kube('get','namespace',state.namespace+'-sandboxes','-o','json'));
  kube('label','namespace',sandbox.metadata.name,'judex.dev/test-run='+state.runId);
  state.sandboxUID=sandbox.metadata.uid;state.stage='ready';save();console.log('Owned real-model deployment Ready: '+state.namespace);
 }else if(mode==='forward'){
  if(state.stage!=='ready')throw new Error('Deploy first');
  if(!Number.isInteger(state.port)||state.port<1024||state.port>65535)throw new Error('Owned unprivileged port required');
  // A pod rollout invalidates kubectl's selected target even while its old
  // listener remains. Reconnect only this run's identified forward process.
  const listeners=run('powershell',['-NoProfile','-Command',`Get-NetTCPConnection -LocalPort ${state.port} -State Listen -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique | ForEach-Object { Get-CimInstance Win32_Process -Filter "ProcessId=$_" } | Select-Object ProcessId,Name,CommandLine | ConvertTo-Json -Compress`]);
  if(listeners.trim())for(const item of [JSON.parse(listeners)].flat()){
   if(item.Name!=='kubectl.exe'||!item.CommandLine.includes(state.namespace)||!item.CommandLine.includes('port-forward')||!item.CommandLine.includes(`${state.port}:8080`))throw new Error('Existing listener does not belong to this test run');
   process.kill(item.ProcessId);
  }
  let stopped=false,current;
  process.on('SIGINT',()=>{stopped=true;current?.kill();});
  while(!stopped){
   own();current=spawn('kubectl',['-n',state.namespace,'port-forward','service/'+state.release,`${state.port}:8080`],{windowsHide:true,stdio:'inherit'});
   const health=()=>new Promise(resolve=>{const request=http.get({hostname:'127.0.0.1',port:state.port,path:'/readyz',agent:new http.Agent(),timeout:1500},response=>{response.resume();resolve(response.statusCode===200);});request.on('error',()=>resolve(false));request.on('timeout',()=>{request.destroy();resolve(false);});});
   let checking=false;
   const check=setInterval(async()=>{if(checking)return;checking=true;try{if(!await health())current.kill();}finally{checking=false;}},5000);
   await new Promise(resolve=>current.once('exit',resolve));clearInterval(check);
   if(!stopped)await new Promise(resolve=>setTimeout(resolve,1000));
  }
 }else if(mode==='cleanup'){
  const sandbox=JSON.parse(kube('get','namespace',state.namespace+'-sandboxes','-o','json'));
  if(sandbox.metadata.uid!==state.sandboxUID||sandbox.metadata.labels?.['judex.dev/test-run']!==state.runId)throw new Error('Sandbox namespace ownership changed');
  kube('delete','namespace',state.namespace+'-sandboxes',state.namespace,'--wait=true','--timeout=120s');state.stage='cleaned';save();
 }else throw new Error('Use prepare, deploy, forward or cleanup');
}
