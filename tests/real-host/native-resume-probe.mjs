// Public CLI A/B probe: retain the transient model definition while selecting
// the native model explicitly. The relay detects any unintended GLM traffic.
import fs from 'node:fs';
import {execFileSync,spawn} from 'node:child_process';
import {startUsageRelay} from './model-usage-relay.mjs';
const root='.cache/real-host/20261012-closure';
const env=JSON.parse(fs.readFileSync(root+'/environment.json','utf8'));
const read=(...args)=>JSON.parse(execFileSync('kubectl',args,{encoding:'utf8',windowsHide:true}));
const namespace=read('get','namespace',env.namespace,'-o','json');
if(namespace.metadata.uid!==env.uid||namespace.metadata.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Owned namespace required');
const secret=read('-n',env.namespace,'get','secret','real-model-key','-o','json');
if(secret.metadata.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Owned key Secret required');
const key=Buffer.from(secret.data.JUDEX_GLM_CODING_KEY,'base64').toString('utf8');
const relay=await startUsageRelay({baseURL:'https://open.bigmodel.cn/api/coding/paas/v4',apiKey:key,modelName:'glm-5.3-flash'});
const startedAt=new Date().toISOString();
const child=spawn(process.execPath,['tests/real-host/kimi-session.mjs','KX01','tests/real-host/prompts/KX01-native-window-check.md','kimi'],{windowsHide:true,env:{...process.env,KIMI_MODEL_NAME:'glm-5.3-flash',KIMI_MODEL_API_KEY:key,KIMI_MODEL_PROVIDER_TYPE:'openai',KIMI_MODEL_BASE_URL:relay.baseURL,KIMI_MODEL_MAX_CONTEXT_SIZE:'128000',KIMI_MODEL_MAX_OUTPUT_SIZE:'8192',KIMI_MODEL_CAPABILITIES:'tool_use,image_in'},stdio:['ignore','pipe','pipe']});
let output='';
const emit=chunk=>{const safe=chunk.toString().split(key).join('[redacted]');output+=safe;process.stdout.write(safe);};
child.stdout.on('data',emit);child.stderr.on('data',emit);
const exitCode=await new Promise(resolve=>{child.once('error',()=>resolve(1));child.once('close',code=>resolve(code??1));});
const observation=await relay.close();
const result={startedAt,finishedAt:new Date().toISOString(),explicitModel:'kimi-code/k3',session:'KX01',exitCode,nativeMarkerReturned:output.includes('KX01: NATIVE_KIMI_READY'),reservedAliasBootstrap:true,unexpectedGLMAliasCalls:observation.calls,unexpectedAliasRejectedRequests:observation.rejectedRequests,nativeUsage:'unknown'};
fs.writeFileSync(root+'/native-resume-probe.json',JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(result));process.exitCode=exitCode;
