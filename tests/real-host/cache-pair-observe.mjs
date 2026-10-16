// Live provider cache aggregates only. No prompts, request IDs or raw calls.
import fs from 'node:fs';
import {execFileSync} from 'node:child_process';
const sources=process.argv.slice(2);
if(sources.length!==2||sources.some(id=>!/^[-0-9a-f]{36}$/.test(id)))throw new Error('Two actual cache-test source UUIDs required');
const env=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/environment.json','utf8'));
const run=(...args)=>execFileSync('kubectl',args,{encoding:'utf8',windowsHide:true,maxBuffer:1<<20});
const ns=JSON.parse(run('get','namespace',env.namespace,'-o','json')).metadata;
if(ns.uid!==env.uid||ns.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Observation ownership changed');
const project='21a13532-7929-4c6c-99a7-01e044eee64e';
const sql=`WITH selected AS(SELECT source_id,batch_id,state FROM task_analyses WHERE project_id='${project}' AND source_id IN('${sources[0]}','${sources[1]}')),
 totals AS(SELECT a.source_id,a.state,i.kind,m.usage_json->'observation'->>'modelName' AS model,
 count(*) AS calls,count(*) FILTER(WHERE m.usage_known AND m.usage_json->'observation'->>'cacheReadTokens' IS NOT NULL) AS known_calls,
 sum((m.usage_json->'observation'->>'inputTokens')::bigint) FILTER(WHERE m.usage_known AND m.usage_json->'observation'->>'cacheReadTokens' IS NOT NULL) AS input_tokens,
 sum((m.usage_json->'observation'->>'cacheReadTokens')::bigint) FILTER(WHERE m.usage_known) AS cache_read_tokens
 FROM selected a JOIN agent_runs r ON r.batch_id=a.batch_id JOIN agent_identities i ON i.id=r.identity_id JOIN model_calls m ON m.run_id=r.id
 WHERE m.ended_at>now()-interval '24 hours' GROUP BY a.source_id,a.state,i.kind,model)
 SELECT jsonb_build_object('pairs',COALESCE((SELECT jsonb_agg(to_jsonb(t)||jsonb_build_object('weighted_read_ratio',CASE WHEN input_tokens>0 THEN cache_read_tokens::numeric/input_tokens ELSE NULL END)) FROM totals t),'[]'::jsonb),
 'selectedAnalyses',(SELECT jsonb_agg(to_jsonb(a)) FROM selected a),
 'retention',(SELECT jsonb_build_object('hours',24,'expired_call_rows',count(*),'expired_observation_rows',count(*) FILTER(WHERE usage_json?'observation'),'expired_request_id_rows',count(*) FILTER(WHERE provider_request_id IS NOT NULL),'accounting_rows_retained',count(*) FILTER(WHERE usage_json?'totalTokens')) FROM model_calls WHERE project_id='${project}' AND ended_at<=now()-interval '24 hours'));`;
const data=JSON.parse(run('-n',env.namespace,'exec','statefulset/'+env.release+'-postgresql','--','psql','-U','judex','-d','judex','-At','-c',sql));
if(data.selectedAnalyses?.length!==2)throw new Error('Both sources must be actual analyses in this owned project');
console.log(JSON.stringify({observedAt:new Date().toISOString(),scope:'two actual, explicit KX01 paired analysis requests; excludes Kimi CLI unknown usage',firstSource:sources[0],secondSource:sources[1],...data,interpretation:'known fields only; no global cold-cache claim, no required hit-rate threshold, no billing-savings inference; zero eligible expired rows is not a natural-expiry proof'},null,2));
