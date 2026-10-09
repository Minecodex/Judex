// Runs the real browser collaboration flows against an owned Helm deployment.
import { execFileSync, spawn } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import net from 'node:net';
import { createRequire } from 'node:module';
const require=createRequire(import.meta.url);
const image=process.env.JUDEX_TEST_IMAGE;
const gatewayImage=process.env.JUDEX_TEST_GATEWAY_IMAGE;
const converterImage=process.env.JUDEX_TEST_CONVERTER_IMAGE;
const realModelNamespace=process.env.JUDEX_REAL_MODEL_NAMESPACE;
if(gatewayImage&&!/^[\w./:-]+:[\w.-]+$/.test(gatewayImage))throw new Error("Invalid controlled gateway image");
const acceptanceGrep=process.env.JUDEX_K8S_GREP ?? 'P01|P02|P03|P04|P05|T01|T02|T03|T04|T05|T06|W01|W02|A01|A02|F03|B03|B16|R11/R12';
if(!image || !/^[\w./:-]+:[\w.-]+$/.test(image)) throw new Error('Set JUDEX_TEST_IMAGE to the image built from the current production web and server.');
const runId='judex-ui-'+randomUUID().slice(0,8),namespace=runId,release='ui';
const artifact=path.resolve('.cache/k8s',runId);fs.mkdirSync(artifact,{recursive:true});
const browserOutput=path.join(artifact,'business-results'),realModelOutput=path.join(artifact,'real-model-results');
const run=(name,args,input)=>execFileSync(name,args,{encoding:'utf8',input,windowsHide:true,timeout:240000,maxBuffer:16*1024*1024});
const kube=(...args)=>run('kubectl',args);
let owned=false,uid,forward,exitCode=0,results={runId,namespace,image,acceptanceGrep,checks:[]};
const ownership=()=>{const ns=JSON.parse(kube('get','namespace',namespace,'-o','json'));if(ns.metadata.uid!==uid || ns.metadata.labels?.['judex.dev/test-run']!==runId)throw new Error('Namespace ownership changed');};
try {
  kube('get','--raw=/readyz','--request-timeout=10s');
  const reservation=net.createServer();await new Promise(resolve=>reservation.listen(0,'127.0.0.1',resolve));
  const port=reservation.address().port;await new Promise(resolve=>reservation.close(resolve));
  const base=`http://127.0.0.1:${port}`;
  const ns=JSON.parse(run('kubectl',['create','-f','-','-o','json'],JSON.stringify({apiVersion:'v1',kind:'Namespace',metadata:{name:namespace,labels:{'judex.dev/test-run':runId}}})));
  // kubectl input is passed through stdin, never mixed into shell command text.
  uid=ns.metadata.uid;owned=true;
  if(gatewayImage||converterImage)run(process.execPath,['tests/k8s/load-images.mjs',namespace,runId,uid,image,...[gatewayImage,converterImage].filter(Boolean)]);
  const index=image.lastIndexOf(':');
  const values={environment:'development',image:{repository:image.slice(0,index),tag:image.slice(index+1),pullPolicy:'Never'},sandbox:{enabled:false},
    server:{mode:'all',allowedOrigins:[base],previewOrigin:`http://localhost:${port}`,materialProjection:{enabled:false},materialPreview:{enabled:!!converterImage,image:converterImage??'judex/material-converter:local-materials'}},
    postgresql:{embedded:{storage:'1Gi'}},objectStorage:{embedded:{storage:'1Gi',maxVolumes:2,growthCount:1,volumeSizeLimitMB:256}}};
  if(gatewayImage){
    const labels={'judex.dev/test-run':runId,app:'controlled-gateway'};
    run('kubectl',['apply','-f','-'],JSON.stringify({apiVersion:'v1',kind:'List',items:[
      {apiVersion:'v1',kind:'Secret',metadata:{name:'controlled-model',namespace,labels},stringData:{key:'controlled-fixture-key'}},
      {apiVersion:'apps/v1',kind:'Deployment',metadata:{name:'controlled-gateway',namespace,labels},spec:{replicas:1,selector:{matchLabels:{app:'controlled-gateway'}},template:{metadata:{labels},spec:{containers:[{name:'gateway',image:gatewayImage,imagePullPolicy:'Never',ports:[{containerPort:8080}],readinessProbe:{tcpSocket:{port:8080}},resources:{requests:{cpu:'10m',memory:'16Mi'},limits:{cpu:'250m',memory:'64Mi'}}}]}}}},
      {apiVersion:'v1',kind:'Service',metadata:{name:'controlled-gateway',namespace,labels},spec:{selector:{app:'controlled-gateway'},ports:[{port:8080,targetPort:8080}]}}
    ]}));
    values.server.modelEnvSecret='controlled-model';
    values.server.modelGateway={protocol:'openai-compatible',baseUrl:'http://controlled-gateway:8080/v1',model:'collaboration-fixture',apiKeyKey:'key'};
    results.controlledModel=gatewayImage;
  }
  const valuesPath=path.join(artifact,'values.json');fs.writeFileSync(valuesPath,JSON.stringify(values,null,2));
  console.log(`Installing ${image} into ${namespace}`);
  run('helm',['upgrade','--install',release,'deploy/helm/judex','-n',namespace,'-f',valuesPath,'--wait','--timeout','240s']);
  ownership();
  // This owned fixture creates many distinct accounts behind one forwarded IP.
  // Production keeps its configured quota; dedicated rate-limit tests cover it.
  kube('-n',namespace,'set','env','deployment/'+release,'JUDEX_REGISTER_PER_IP=1000');
  kube('-n',namespace,'rollout','status','deployment/'+release,'--timeout=180s');
  results.registrationTestLimit=1000;
  results.checks.push('Owned deployment, PostgreSQL and object storage are Ready');
  forward=spawn('kubectl',['-n',namespace,'port-forward','service/'+release,String(port)+':8080'],{windowsHide:true,stdio:['ignore','pipe','pipe']});
  let forwardLog='';forward.stdout.on('data',chunk=>{forwardLog+=chunk});forward.stderr.on('data',chunk=>{forwardLog+=chunk});
  const deadline=Date.now()+60000;
  while(true) {if(forward.exitCode!==null)throw new Error('Port-forward exited: '+forwardLog);try{if((await fetch(base+'/readyz')).ok)break;}catch{}if(Date.now()>deadline)throw new Error('API readiness timed out');await new Promise(resolve=>setTimeout(resolve,250));}
  const cli=path.join(artifact,process.platform==='win32'?'judex.exe':'judex');execFileSync('go',['build','-o',cli,'./cmd/judex'],{cwd:process.env.JUDEX_TEST_SOURCE??process.cwd(),windowsHide:true,stdio:'inherit',timeout:120000});
  const discovered=run(process.execPath,[require.resolve('@playwright/test/cli'),'test','--config','tests/e2e/business.config.ts','--grep',acceptanceGrep,'--list']);
  const match=discovered.match(/Total:\s+(\d+)\s+tests?/);if(!match||Number(match[1])===0)throw new Error('Acceptance selection matched zero tests');results.discoveredTests=Number(match[1]);
  console.log('Running authenticated browser UI and two-person collaboration acceptance');
  const command=spawn(process.execPath,[require.resolve('@playwright/test/cli'),'test','--config','tests/e2e/business.config.ts',
    '--grep',acceptanceGrep],{windowsHide:true,stdio:'inherit',env:{...process.env,JUDEX_E2E_BASE_URL:base,JUDEX_E2E_CLI:cli,JUDEX_E2E_ARTIFACT:artifact,JUDEX_E2E_COLLABORATION_GATEWAY:gatewayImage?'1':'0'}});
  const status=await new Promise(resolve=>command.once('exit',resolve));
  if(status!==0)throw new Error(`Kubernetes browser acceptance failed (${status})`);
  results.checks.push('Configured authenticated browser acceptance passed: '+acceptanceGrep);
  fs.cpSync(browserOutput,path.join(artifact,'controlled-browser'),{recursive:true});
  if(realModelNamespace){
    // Reuse only the existing deployment's model configuration. Credentials are
    // held in memory and sent through stdin into an owned Secret, never logged.
    ownership();
    const source=JSON.parse(kube('-n',realModelNamespace,'get','deployment',process.env.JUDEX_REAL_MODEL_DEPLOYMENT??'judex','-o','json')).spec.template.spec.containers.find(c=>c.name==='server');
    if(!source)throw new Error('Configured real model server container is unavailable');
    const resolveEnv=name=>{
      const entry=source.env?.find(e=>e.name===name);
      if(entry?.value!==undefined)return entry.value;
      if(entry?.valueFrom?.secretKeyRef){const ref=entry.valueFrom.secretKeyRef;return Buffer.from(JSON.parse(kube('-n',realModelNamespace,'get','secret',ref.name,'-o','json')).data[ref.key]??'','base64').toString();}
      for(const envFrom of source.envFrom??[]){if(envFrom.secretRef){const data=JSON.parse(kube('-n',realModelNamespace,'get','secret',envFrom.secretRef.name,'-o','json')).data;if(data[name])return Buffer.from(data[name],'base64').toString();}}
      throw new Error('Real model configuration is missing '+name);
    };
    const names=['JUDEX_MODEL_PROTOCOL','JUDEX_MODEL_BASE_URL','JUDEX_MODEL_NAME','JUDEX_MODEL_API_KEY'];
    const model=Object.fromEntries(names.map(name=>[name,resolveEnv(name)]));
    run('kubectl',['apply','-f','-'],JSON.stringify({apiVersion:'v1',kind:'Secret',metadata:{name:'real-analysis-model',namespace,labels:{'judex.dev/test-run':runId}},stringData:{key:model.JUDEX_MODEL_API_KEY}}));
    const deployment=JSON.parse(kube('-n',namespace,'get','deployment',release,'-o','json'));
    const server=deployment.spec.template.spec.containers.find(c=>c.name==='server');
    server.env=server.env.filter(e=>!names.includes(e.name)&&e.name!=='JUDEX_MODEL_CATALOG_FILE');
    server.env.push(...names.filter(name=>name!=='JUDEX_MODEL_API_KEY').map(name=>({name,value:model[name]})),{name:'JUDEX_MODEL_API_KEY',valueFrom:{secretKeyRef:{name:'real-analysis-model',key:'key'}}},{name:'JUDEX_MODEL_CATALOG_FILE',value:''});
    const patchPath=path.join(artifact,'real-model-env-patch.json');
    // This patch contains only public model settings and an owned Secret ref.
    fs.writeFileSync(patchPath,JSON.stringify({spec:{template:{spec:{containers:[{name:'server',env:server.env}]}}}}));
    run('kubectl',['patch','deployment',release,'-n',namespace,'--type=strategic','--patch-file',patchPath]);
    kube('-n',namespace,'rollout','status','deployment/'+release,'--timeout=180s');
    if(forward.exitCode===null){const stopped=new Promise(resolve=>forward.once('exit',resolve));forward.kill();await stopped;}
    forward=spawn('kubectl',['-n',namespace,'port-forward','service/'+release,String(port)+':8080'],{windowsHide:true,stdio:['ignore','pipe','pipe']});
    forward.stdout.on('data',()=>{});forward.stderr.on('data',()=>{});
    const restartDeadline=Date.now()+60000;
    while(true){try{if((await fetch(base+'/readyz')).ok)break;}catch{}if(forward.exitCode!==null||Date.now()>restartDeadline)throw new Error('Real-model deployment readiness timed out');await new Promise(resolve=>setTimeout(resolve,250));}
    results.realModel={protocol:model.JUDEX_MODEL_PROTOCOL,name:model.JUDEX_MODEL_NAME};
    const smoke=spawn(process.execPath,[require.resolve('@playwright/test/cli'),'test','--config','tests/e2e/business.config.ts','--grep','C02'],{windowsHide:true,stdio:'inherit',env:{...process.env,JUDEX_E2E_BASE_URL:base,JUDEX_E2E_CLI:cli,JUDEX_E2E_ARTIFACT:artifact,JUDEX_REAL_MODEL_SMOKE:'1'}});
    if(await new Promise(resolve=>smoke.once('exit',resolve))!==0){
      // Persist bounded diagnostic facts before cleaning the owned database.
      // Do not export prompts, tool arguments, credentials or model responses.
      const failureSQL=`SELECT jsonb_build_object('toolCalls',(SELECT jsonb_agg(t) FROM (SELECT name,state,count(*) AS count FROM tool_calls GROUP BY name,state) t),'runs',(SELECT jsonb_agg(t) FROM (SELECT state,count(*) AS count FROM agent_runs GROUP BY state) t),'models',(SELECT jsonb_agg(t) FROM (SELECT state,error_code,count(*) AS count FROM model_calls GROUP BY state,error_code) t))`;
      try{fs.writeFileSync(path.join(artifact,'real-model-failure-facts.json'),kube('-n',namespace,'exec','statefulset/'+release+'-postgresql','--','psql','-U','judex','-d','judex','-At','-c',failureSQL));}catch{}
      throw new Error('Real configured-model analysis smoke failed');
    }
    const analysis=JSON.parse(fs.readFileSync(path.join(artifact,'real-model-analysis.json'),'utf8')).analysis;
    if(!/^[0-9a-f-]{36}$/.test(analysis.batchId))throw new Error('Real analysis batch id is missing');
    const proofSQL=`SELECT jsonb_build_object('modelCalls',(SELECT count(*) FROM model_calls c JOIN agent_runs r ON r.id=c.run_id WHERE r.batch_id='${analysis.batchId}'),'roleRuns',(SELECT count(*) FROM agent_runs r WHERE r.batch_id='${analysis.batchId}' AND r.parent_run_id IS NOT NULL AND r.state='succeeded'),'materialReadToolCalls',(SELECT count(*) FROM tool_calls c JOIN agent_runs r ON r.id=c.run_id WHERE r.batch_id='${analysis.batchId}' AND c.name='read_material' AND c.state='succeeded'),'analysisToolCalls',(SELECT count(*) FROM tool_calls c JOIN agent_runs r ON r.id=c.run_id WHERE r.batch_id='${analysis.batchId}' AND c.name='record_task_analysis' AND c.state='succeeded'))`;
    const proof=JSON.parse(kube('-n',namespace,'exec','statefulset/'+release+'-postgresql','--','psql','-U','judex','-d','judex','-At','-c',proofSQL).trim());
    if(proof.roleRuns<1||proof.modelCalls<2||proof.analysisToolCalls<1||proof.materialReadToolCalls<1)throw new Error('Real model did not complete the required role and structured-analysis chain');
    results.realModel.proof=proof;
    fs.writeFileSync(path.join(artifact,'real-model-proof.json'),JSON.stringify(proof,null,2));
    results.checks.push('Real configured-model role review and persisted task analysis passed');
  }
  results.deployments=JSON.parse(kube('-n',namespace,'get','deployments','-o','json')).items.map(item=>({name:item.metadata.name,ready:item.status.readyReplicas,image:item.spec.template.spec.containers[0].image}));
  // Preserve visual evidence before another local test run replaces its output.
  fs.cpSync(browserOutput,path.join(artifact,'screenshots'),{recursive:true});
  if(fs.existsSync(realModelOutput))fs.cpSync(realModelOutput,path.join(artifact,'real-model-browser'),{recursive:true});
} catch(error) {
  exitCode=1;results.error=error.message;console.error(error.message);
  if(owned)try{fs.writeFileSync(path.join(artifact,'pods.json'),kube('-n',namespace,'get','pods','-o','json'));fs.writeFileSync(path.join(artifact,'events.txt'),kube('-n',namespace,'get','events','--sort-by=.lastTimestamp'));}catch{}
  if(owned&&fs.existsSync(browserOutput))fs.cpSync(browserOutput,path.join(artifact,'failed-browser'),{recursive:true});
} finally {
  if(forward && forward.exitCode===null)forward.kill();
  if(owned)try{ownership();console.log('Removing owned namespace '+namespace);kube('delete','namespace',namespace,'--wait=true','--timeout=120s');results.cleanedUp=true;}catch(error){exitCode=1;results.cleanupError=error.message;}
  results.exitCode=exitCode;fs.writeFileSync(path.join(artifact,'manifest.json'),JSON.stringify(results,null,2));
  process.exitCode=exitCode;console.log('Kubernetes UI evidence: '+artifact);
}
