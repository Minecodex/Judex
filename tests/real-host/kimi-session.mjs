// Public Kimi CLI only; explicit session IDs isolate the fourteen actor roles.
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import {spawn,execFileSync} from 'node:child_process';
import {startUsageRelay} from './model-usage-relay.mjs';
import {assessContextWindow} from './kimi-context-preflight.mjs';
const [label,promptFile,modelMode='kimi']=process.argv.slice(2);
const controlledContext=process.env.JUDEX_TEST_KIMI_CONTEXT_SIZE;
if(controlledContext&&!/^(32768|65536|128000)$/.test(controlledContext))throw new Error('Use a declared bounded context limit for the compression control');
const controlledOutput=process.env.JUDEX_TEST_KIMI_MAX_OUTPUT;
if(controlledOutput&&!/^(8192|16384|24576)$/.test(controlledOutput))throw new Error('Use a declared bounded output limit for the compression control');
if(controlledOutput&&Number(controlledOutput)>=Number(controlledContext??128000))throw new Error('Output allowance must leave room for input');
if(!['kimi','kimi-k3','apimart-glm-5.3-flash','glm-5.3','glm-5.3-flash','glm-4.5-flash','glm-4.6v-flash'].includes(modelMode))throw new Error('Use an explicitly approved model');
const scenes={KA01:['A-MiniIssue','U0'],KA02:['A-MiniIssue','U1'],KA03:['A-MiniIssue','U2'],KA04:['A-MiniIssue','U3'],KA05:['A-MiniIssue','U4'],KA06:['A-MiniIssue','U1'],
 KB01:['B-VisualAssets','U0'],KB02:['B-VisualAssets','U2'],KB03:['B-VisualAssets','U3'],KC01:['C-OfficeReports','U0'],KC02:['C-OfficeReports','U4'],KC03:['C-OfficeReports','U2'],KX01:['A-MiniIssue','U1'],KX02:['A-MiniIssue','U5']};
if(!scenes[label])throw new Error('Unknown planned scene session');
const base=path.resolve(process.env.JUDEX_REAL_HOST_ROOT??'.cache/real-host/20261012-closure'),sessions=path.join(base,'sessions'),observations=path.join(base,'kimi-observations');
fs.mkdirSync(sessions,{recursive:true});fs.mkdirSync(observations,{recursive:true});
// Raw invocation detail is temporary. Durable reports contain only outcome and
// final public business text; never preserve per-call usage/request IDs.
for(const file of fs.readdirSync(observations)){
 const target=path.join(observations,file);
 if(!/^(KA\d\d|KB\d\d|KC\d\d|KX\d\d)-\d+\.(jsonl|stderr)$/.test(file))continue;
 try{if(fs.statSync(target).mtimeMs<Date.now()-86400000)fs.unlinkSync(target);}
 catch(error){
  // Two planned sessions may race on the same expired file. Windows can
  // report EPERM while the other unlink completes; only tolerate absence.
  if(['ENOENT','EPERM'].includes(error.code)&&!fs.existsSync(target))continue;
  throw error;
 }
}
let active=0;
for(const file of fs.readdirSync(sessions).filter(f=>f.endsWith('.lock'))){
 const target=path.join(sessions,file),lock=JSON.parse(fs.readFileSync(target,'utf8'));
 try{process.kill(lock.pid,0);active++;}catch{if(lock.label+'.lock'!==file)throw new Error('Lock ownership mismatch');fs.unlinkSync(target);}
}
if(active>=4)throw new Error('Planned Kimi concurrency limit is four');
const recordFile=path.join(sessions,label+'.json'),lockFile=path.join(sessions,label+'.lock');
const record=fs.existsSync(recordFile)?JSON.parse(fs.readFileSync(recordFile,'utf8')):{label,actor:scenes[label][1],invocations:[]};
if(record.invocations.length&&!record.sessionId)throw new Error('Recover the existing CLI session ID before resuming this label');
fs.writeFileSync(lockFile,JSON.stringify({label,pid:process.pid,startedAt:new Date().toISOString()}),{flag:'wx'});
const env=JSON.parse(fs.readFileSync(path.join(base,'environment.json'),'utf8'));
const cwd=path.join(env.localRoot,scenes[label][0]);
const sessionIndex=path.join(os.homedir(),'.kimi-code/session_index.jsonl');
const indexedSessions=()=>{
 if(!fs.existsSync(sessionIndex))return [];
 return fs.readFileSync(sessionIndex,'utf8').split(/\r?\n/).flatMap(line=>{try{const row=JSON.parse(line);return row.workDir&&path.resolve(row.workDir)===path.resolve(cwd)?[row]:[];}catch{return [];}});
};
const beforeSessionIds=new Set(indexedSessions().map(row=>row.sessionId));
let contextPreflight=null;
if(controlledContext&&record.sessionId){
 const selected=indexedSessions().find(row=>row.sessionId===record.sessionId);
 if(selected?.sessionDir&&path.basename(selected.sessionDir)===record.sessionId){
  const wire=path.join(selected.sessionDir,'agents/main/wire.jsonl');
  const completed=fs.existsSync(wire)?fs.readFileSync(wire,'utf8').split(/\r?\n/).flatMap(line=>{try{const row=JSON.parse(line);return row.type==='context.apply_compaction'?[{tokensAfter:row.tokensAfter}]:[];}catch{return [];}}):[];
  contextPreflight=assessContextWindow(completed,Number(controlledContext),Number(controlledOutput??8192));
  if(!contextPreflight.admitted){
   record.preflightRejectedInvocations??=[];
   record.preflightRejectedInvocations.push({recordedAt:new Date().toISOString(),contextPreflight,providerCalls:0});
   fs.writeFileSync(recordFile,JSON.stringify(record,null,2)+'\n');fs.unlinkSync(lockFile);
   console.error(JSON.stringify({label,exitCode:2,providerCalls:0,contextPreflight}));process.exit(2);
  }
 }
}
const toolRoot=path.join(cwd,env.cliToolsRoot??'.tools'),cliPath=path.join(toolRoot,'judex.exe');
const cliVersion=JSON.parse(execFileSync(cliPath,['--json','version'],{encoding:'utf8',windowsHide:true})).data.cli;
const invocationTime=new Date().toISOString();
const skillRoot=env.cliToolsRoot?path.join(toolRoot,'skills/judex'):path.join(cwd,'.agents/skills/judex');
if(!fs.existsSync(path.join(skillRoot,'references/reporting.md')))throw new Error('Packaged reporting contract missing');
const platformContext=env.platformModelName?`当前隔离平台协调者和职位模型=${env.platformModelName}，模式=${env.platformModelMode}，思考=${env.analysisModelThinkingType}。回执 recordedAt/createdAt 等时间必须由脚本读取系统时钟或复制实际API返回生成，不能手填近似分钟并称其为系统实测。\n`:'';
const provenanceGuide='来源核对：以本任务正式要求的固定版本为准；明确指定的历史输入仍按原UUID/SHA使用。凡要求本轮上游成果，先CLI读取该任务当前context及acceptances，找acceptanceId与当前latestAcceptanceId相同的accept记录，再固定材料versionId/SHA。历史记录id不等于acceptanceId。协议示例、同名目录或旧producer状态不能代替当前接受快照；方法脚本可复用，输入来源须正确。已有真实检查可明确复用未变固定字节，不重做无关测试。\n';
const prompt=platformContext+provenanceGuide+`本次操作员实测公开 CLI 版本=${cliVersion}；本次调用开始 UTC=${invocationTime}。旧会话版本、估写时间和旧业务状态不能当作当前事实。任务状态与身份仍须你经本人 CLI 回读，回执以实际返回/系统时间为准。\n发行 Skill 的实际路径：${skillRoot}；提交/报告合同：${path.join(skillRoot,'references/reporting.md')}。不要在 MiniIssue 的 backend 中寻找 Judex 协议。同版本已读取的合同可复用，仅操作/版本变化时读必要部分，避免重复浏览相同帮助。实际下载语法：material download --version UUID --output NEW_FILE（全原件不带--entry）。\n跨 shell 调用：本次进程已设置本人 JUDEX_CONFIG_DIR；Python subprocess 可直接以参数数组调用 ${cliPath} --profile ${label} --json ...，并指定 encoding="utf-8"，无需拼 cmd.exe 命令。使用包装 .cmd 时采用绝对路径；本地命令未找到不是网络或模型失败，应先修正命令，不能重复无效轮询。不要打印环境变量/凭据。\n\n`+fs.readFileSync(promptFile,'utf8');
const skillDiscoveryMode=process.env.JUDEX_REAL_HOST_SKILL_DISCOVERY??'explicit';
if(!['explicit','default'].includes(skillDiscoveryMode))throw new Error('Known skill discovery mode required');
const args=[...(skillDiscoveryMode==='explicit'?['--skills-dir',path.join(cwd,'.agents/skills')]:[]),'--output-format','stream-json','--prompt',prompt];
const invocationEnv={...process.env,JUDEX_CONFIG_DIR:path.join(toolRoot,'config',label),JUDEX_TEST_LABEL:label,JUDEX_TOKEN:'',KIMI_DISABLE_TELEMETRY:'1',KIMI_LOG_LEVEL:'off',PYTHONUTF8:'1',PYTHONIOENCODING:'utf-8'};
// Native subjects share the admitted workspace toolchain. Windows' unrelated
// global Python lacks Office/PDF modules; do not make each subject discover or
// install another environment. These overrides affect only this invocation.
const dependencies=path.join(os.homedir(),'.cache/codex-runtimes/codex-primary-runtime/dependencies');
const pythonRuntime=path.join(dependencies,'python'),nodeRuntime=path.join(dependencies,'node/bin');
const bundledPython=process.platform==='win32'&&fs.existsSync(path.join(pythonRuntime,'python.exe'));
if(bundledPython){
 const inheritedPath=process.env.PATH??process.env.Path??'';
 for(const key of Object.keys(invocationEnv))if(key.toUpperCase()==='PATH')delete invocationEnv[key];
 invocationEnv.PATH=[pythonRuntime,nodeRuntime,inheritedPath].join(path.delimiter);
 invocationEnv.NODE_PATH=[path.resolve('node_modules'),path.join(dependencies,'node/node_modules'),invocationEnv.NODE_PATH??''].filter(Boolean).join(path.delimiter);
}
let invocationKey=invocationEnv.KIMI_MODEL_API_KEY??'';
if(modelMode==='kimi')args.unshift('--model','kimi-code/k3');
else if(modelMode==='kimi-k3'){
 if(!invocationKey)throw new Error('Kimi K3 requires a process-local API key');
 args.unshift('--model','__kimi_env_model__');
 Object.assign(invocationEnv,{KIMI_MODEL_NAME:'k3',KIMI_MODEL_PROVIDER_TYPE:'openai',KIMI_MODEL_BASE_URL:'https://api.kimi.com/coding/v1',KIMI_MODEL_MAX_CONTEXT_SIZE:'262144',KIMI_MODEL_MAX_OUTPUT_SIZE:'8192',KIMI_MODEL_CAPABILITIES:'tool_use,image_in',KIMI_LOG_LEVEL:'off'});
}
else if(modelMode==='apimart-glm-5.3-flash'){
 if(!invocationKey)throw new Error('APIMart requires a process-local API key');
 args.unshift('--model','__kimi_env_model__');
 // The admitted APIMart model rejects thinking=disabled. Declare its public
 // always_thinking capability so CLI compaction retains the configured high
 // effort too; the relay still checks actual bytes without rewriting them.
 Object.assign(invocationEnv,{KIMI_MODEL_NAME:'glm-5.3-flash',KIMI_MODEL_PROVIDER_TYPE:'openai',KIMI_MODEL_BASE_URL:'https://api.apimart.ai/v1',KIMI_MODEL_MAX_CONTEXT_SIZE:controlledContext??'128000',KIMI_MODEL_MAX_OUTPUT_SIZE:controlledOutput??'8192',KIMI_MODEL_CAPABILITIES:'tool_use,image_in,thinking,always_thinking',KIMI_MODEL_THINKING_EFFORT:'high',KIMI_LOG_LEVEL:'off'});
}
else{
 const kube=(...args)=>JSON.parse(execFileSync('kubectl',args,{encoding:'utf8',windowsHide:true}));
 const ns=kube('get','namespace',env.namespace,'-o','json');
 if(ns.metadata.uid!==env.uid||ns.metadata.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Namespace ownership changed');
 const secret=kube('-n',env.namespace,'get','secret','real-model-key','-o','json');
 if(secret.metadata.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Key Secret ownership changed');
 invocationKey=Buffer.from(secret.data.JUDEX_GLM_CODING_KEY,'base64').toString('utf8');
 // 0.36.1 restores the session model ahead of an env-only default. Its shipped
 // applyEnvModelConfig exposes this alias; selecting it via the public -m flag
 // keeps the original session and never writes a key/provider to config.toml.
 args.unshift('--model','__kimi_env_model__');
 const baseURL=modelMode.startsWith('glm-4.')?'https://open.bigmodel.cn/api/paas/v4':'https://open.bigmodel.cn/api/coding/paas/v4';
 Object.assign(invocationEnv,{KIMI_MODEL_NAME:modelMode,KIMI_MODEL_API_KEY:invocationKey,KIMI_MODEL_PROVIDER_TYPE:'openai',KIMI_MODEL_BASE_URL:baseURL,KIMI_MODEL_MAX_CONTEXT_SIZE:modelMode.startsWith('glm-4.')?'65536':'128000',KIMI_MODEL_MAX_OUTPUT_SIZE:'8192',KIMI_MODEL_CAPABILITIES:['glm-5.3-flash','glm-4.6v-flash'].includes(modelMode)?'tool_use,image_in':'tool_use',KIMI_LOG_LEVEL:'off'});
}
// Keep provider counters only in this invocation's memory, then retain a small
// aggregate. Existing direct invocations remain explicitly unobserved.
const maxCalls=process.env.JUDEX_TEST_KIMI_CALL_LIMIT?Number(process.env.JUDEX_TEST_KIMI_CALL_LIMIT):undefined;
const relay=modelMode==='kimi'?null:await startUsageRelay({baseURL:invocationEnv.KIMI_MODEL_BASE_URL,apiKey:invocationKey,modelName:invocationEnv.KIMI_MODEL_NAME,requiredReasoningEffort:modelMode==='apimart-glm-5.3-flash'?'high':undefined,maxCalls,resolveAddress:process.env.JUDEX_TEST_PROVIDER_ADDRESS,proxyURL:process.env.JUDEX_TEST_MODEL_PROXY});
if(relay)invocationEnv.KIMI_MODEL_BASE_URL=relay.baseURL;
if(record.sessionId)args.unshift('--session',record.sessionId);
const start=Date.now(),stdoutFile=path.join(observations,label+'-'+start+'.jsonl'),stderrFile=path.join(observations,label+'-'+start+'.stderr');
const stdout=fs.createWriteStream(stdoutFile),stderr=fs.createWriteStream(stderrFile);
let buffer='',finalText='',toolCalls=0,warningCount=0;
const child=spawn('kimi',args,{cwd,windowsHide:true,env:invocationEnv,stdio:['ignore','pipe','pipe']});
const redact=chunk=>invocationKey?chunk.toString().split(invocationKey).join('[redacted]'):chunk.toString();
const emit=line=>{
 let item;try{item=JSON.parse(line)}catch{return;}
 if(item.type==='session.resume_hint'){record.sessionId=item.session_id;return;}
 if(item.tool_calls)toolCalls+=item.tool_calls.length;
 if(item.role==='assistant'&&typeof item.content==='string'&&item.content.trim()){finalText=item.content;console.log(label+': '+item.content.slice(0,1600));}
};
child.stdout.on('data',chunk=>{const safe=redact(chunk);stdout.write(safe);buffer+=safe;let index;while((index=buffer.indexOf('\n'))>=0){emit(buffer.slice(0,index));buffer=buffer.slice(index+1);}});
child.stderr.on('data',chunk=>{const safe=redact(chunk);stderr.write(safe);warningCount+=(safe.match(/wire\.unknown_record/g)??[]).length;});
let timer=setInterval(()=>console.log(label+' running '+Math.floor((Date.now()-start)/1000)+'s'),45000);
const timeout=setTimeout(()=>{console.log(label+' reached the invocation time limit');child.kill();},30*60*1000);
const result=await new Promise(resolve=>{child.on('error',error=>resolve({code:1,error:error.message}));child.on('close',(code,signal)=>resolve({code:code??1,signal}));});
clearInterval(timer);clearTimeout(timeout);stdout.end();stderr.end();if(buffer.trim())emit(buffer);
// Failed first prompts may omit resume_hint. Recover only one new CLI-owned
// metadata entry for this exact workspace and invocation creation window.
if(!record.sessionId){
 const candidates=indexedSessions().filter(row=>!beforeSessionIds.has(row.sessionId)&&/^session_[0-9a-f-]{36}$/.test(row.sessionId??'')&&row.sessionDir&&path.basename(row.sessionDir)===row.sessionId&&path.resolve(row.sessionDir).startsWith(path.resolve(os.homedir(),'.kimi-code/sessions')+path.sep)&&fs.existsSync(row.sessionDir)&&Math.abs(fs.statSync(row.sessionDir).birthtimeMs-start)<180000);
 if(candidates.length===1){record.sessionId=candidates[0].sessionId;record.sessionIdRecovery={source:'unique new CLI-owned metadata for exact workspace and invocation creation time',recoveredAt:new Date().toISOString()};}
 else console.log(label+' session metadata unresolved; explicit recovery required before resuming');
}
const modelObservation=relay?await relay.close():{source:'native Kimi stream-json',usage:'unknown'};
record.invocations.push({startedAt:new Date(start).toISOString(),cliVersion,durationMs:Date.now()-start,exitCode:result.code,modelMode,skillDiscoveryMode,bundledWorkspaceRuntime:bundledPython,requestedModel:invocationEnv.KIMI_MODEL_NAME??'k3',reasoningEffort:modelMode==='apimart-glm-5.3-flash'?'high':null,toolCalls,restoreWarnings:warningCount,modelObservation,promptFile:path.relative(base,path.resolve(promptFile)),publicResult:finalText});
record.invocations.at(-1).controlledContextLimit=controlledContext?Number(controlledContext):null;
record.invocations.at(-1).controlledOutputLimit=controlledOutput?Number(controlledOutput):null;
record.invocations.at(-1).contextPreflight=contextPreflight;
fs.writeFileSync(recordFile,JSON.stringify(record,null,2)+'\n');fs.unlinkSync(lockFile);
console.log(JSON.stringify({label,sessionId:record.sessionId,exitCode:result.code,durationMs:Date.now()-start,restoreWarnings:warningCount,toolCalls,modelObservation}));
process.exitCode=result.code;
