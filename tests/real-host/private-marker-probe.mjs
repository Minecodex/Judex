// Diagnose controlled test-marker presence without emitting private text.
import fs from 'node:fs';
import {execFileSync} from 'node:child_process';
const source=process.argv[2];
if(!/^[0-9a-f-]{36}$/.test(source??''))throw new Error('Actual source required');
const env=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/environment.json','utf8'));
const run=(...args)=>execFileSync('kubectl',args,{encoding:'utf8',windowsHide:true,maxBuffer:1<<20});
const ns=JSON.parse(run('get','namespace',env.namespace,'-o','json')).metadata;
if(ns.uid!==env.uid||ns.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Owned namespace required');
const sql=`SELECT jsonb_agg(jsonb_build_object('name',t.name,'state',t.state,'result',t.result_json)) FROM task_analyses a JOIN agent_runs r ON r.batch_id=a.batch_id JOIN tool_calls t ON t.run_id=r.id WHERE a.source_id='${source}' AND t.name='call_agent'`;
const results=JSON.parse(run('-n',env.namespace,'exec','statefulset/'+env.release+'-postgresql','--','psql','-U','judex','-d','judex','-At','-c',sql));
const markers=['RH_BACKEND_PRIVATE','RH_OPS_PRIVATE','RH_QA_ALPHA_PRIVATE','RH_QA_BETA_PRIVATE'];
const found=[];
function walk(value,keyPath){
 if(typeof value==='string'){
  const present=markers.filter(marker=>value.includes(marker));
  if(present.length)found.push({path:keyPath,controlledMarkerNames:present,stringLength:value.length,offsets:present.map(marker=>value.indexOf(marker)),matchesFullKnownBackendPreference:value.includes('RH_BACKEND_PRIVATE：先核对 API 与数据一致性，以可复现证据说明边界。此偏好属于后端职位。')});
 }else if(value&&typeof value==='object')for(const [key,child]of Object.entries(value))walk(child,keyPath+'.'+key);
}
walk(results,'tools');
console.log(JSON.stringify({sourceId:source,observedAt:new Date().toISOString(),found,scope:'only controlled marker names/paths/counts; no private prompt text emitted'}));
