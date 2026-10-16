// Read-only source-scoped analysis/cache aggregates. Never emit prompt text,
// provider request IDs, keys or individual call observation rows.
import fs from 'node:fs';
import {execFileSync} from 'node:child_process';
const source=process.argv[2];
if(!/^[0-9a-f-]{36}$/.test(source??''))throw new Error('Actual source UUID required');
const env=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/environment.json','utf8'));
const run=(...args)=>execFileSync('kubectl',args,{encoding:'utf8',windowsHide:true,maxBuffer:1<<20});
const ns=JSON.parse(run('get','namespace',env.namespace,'-o','json')).metadata;
if(ns.uid!==env.uid||ns.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Observation ownership changed');
const sql=`WITH selected AS(SELECT id,project_id,source_id,batch_id,state,error_code FROM task_analyses WHERE source_id='${source}'),
 calls AS(SELECT r.identity_id,i.kind,m.state,m.usage_known,m.usage_json->'observation'->>'modelName' AS model,
 (m.usage_json->'observation'->>'inputTokens')::bigint AS input_tokens,
 (m.usage_json->'observation'->>'cacheReadTokens')::bigint AS read_tokens
 FROM selected a JOIN agent_runs r ON r.batch_id=a.batch_id JOIN agent_identities i ON i.id=r.identity_id JOIN model_calls m ON m.run_id=r.id WHERE m.ended_at>now()-interval '24 hours'),
 valid AS(SELECT *,usage_known AND input_tokens>=0 AND read_tokens>=0 AND read_tokens<=input_tokens AS cache_known FROM calls),
 totals AS(SELECT identity_id,kind,model,state,count(*) AS calls,count(*) FILTER(WHERE cache_known) AS cache_known_calls,
 count(*) FILTER(WHERE cache_known IS NOT TRUE) AS cache_unknown_calls,
 sum(input_tokens) FILTER(WHERE cache_known) AS input_tokens,sum(read_tokens) FILTER(WHERE cache_known) AS cache_read_tokens FROM valid GROUP BY identity_id,kind,model,state),
 contexts AS(SELECT r.identity_id,i.kind,r.binding_version,count(m.value) AS observed_messages,
 count(m.value) FILTER(WHERE m.value->>'role'='system') AS persisted_system_messages,
 CASE WHEN count(m.value) FILTER(WHERE m.value->>'role'='system')>0 THEN bool_or(m.value->>'role'='system' AND strpos(COALESCE(m.value->>'content',''),'RH_BACKEND_PRIVATE')>0) ELSE NULL END AS backend_marker_in_system,
 CASE WHEN count(m.value) FILTER(WHERE m.value->>'role'='system')>0 THEN bool_or(m.value->>'role'='system' AND strpos(COALESCE(m.value->>'content',''),'RH_OPS_PRIVATE')>0) ELSE NULL END AS ops_marker_in_system,
 max((p.manifest->'metadata'->>'preferenceRevision')::bigint) AS preference_revision,
 bool_or(p.manifest->'metadata'->>'preferenceHash' IS NOT NULL AND p.manifest->'metadata'->>'preferenceHash'<>'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855') AS nonempty_preference_hash,
 min(m.ordinality) FILTER(WHERE m.value->>'role'='system' AND strpos(COALESCE(m.value->>'content',''),'RH_')>0) AS private_marker_message_position,
 min(m.ordinality) FILTER(WHERE m.value->>'role'='user' AND strpos(COALESCE(m.value->>'content',''),'已发布流程候选')>0) AS workflow_message_position
 FROM selected a JOIN agent_runs r ON r.batch_id=a.batch_id JOIN agent_identities i ON i.id=r.identity_id
 LEFT JOIN LATERAL(SELECT p.manifest FROM context_checkpoints p WHERE p.run_id=r.id ORDER BY p.created_at DESC LIMIT 1)p ON true
 LEFT JOIN LATERAL jsonb_array_elements(COALESCE(p.manifest->'messages','[]'::jsonb)) WITH ORDINALITY m ON true
 GROUP BY r.identity_id,i.kind,r.binding_version)
 SELECT jsonb_build_object('analyses',COALESCE((SELECT jsonb_agg(to_jsonb(a)) FROM selected a),'[]'::jsonb),
 'usage',COALESCE((SELECT jsonb_agg(to_jsonb(t)||jsonb_build_object('weighted_read_ratio',CASE WHEN input_tokens>0 THEN cache_read_tokens::numeric/input_tokens ELSE NULL END)) FROM totals t),'[]'::jsonb),
 'tools',COALESCE((SELECT jsonb_agg(to_jsonb(t)) FROM (SELECT c.name,c.state,count(*) AS calls FROM selected a JOIN agent_runs r ON r.batch_id=a.batch_id JOIN tool_calls c ON c.run_id=r.id GROUP BY c.name,c.state)t),'[]'::jsonb),
 'contextMetadata',COALESCE((SELECT jsonb_agg(to_jsonb(c)) FROM contexts c),'[]'::jsonb),
 'controlledMarkerPublicCheck',jsonb_build_object('analysis_summary_marker_count',(SELECT count(*) FROM selected a JOIN task_analyses t ON t.id=a.id WHERE strpos(COALESCE(t.summary,''),'RH_')>0),
 'call_agent_result_marker_count',(SELECT count(*) FROM selected a JOIN agent_runs r ON r.batch_id=a.batch_id JOIN tool_calls t ON t.run_id=r.id WHERE t.name='call_agent' AND strpos(COALESCE(t.result_json::text,''),'RH_')>0)),
 'retention',(SELECT jsonb_build_object('hours',24,'first_call_ended_at',min(m.ended_at),'expired_rows',count(*) FILTER(WHERE m.ended_at<=now()-interval '24 hours'),
 'expired_observation_rows',count(*) FILTER(WHERE m.ended_at<=now()-interval '24 hours' AND m.usage_json?'observation'),
 'expired_request_id_rows',count(*) FILTER(WHERE m.ended_at<=now()-interval '24 hours' AND m.provider_request_id IS NOT NULL),
 'expired_accounting_rows_retained',count(*) FILTER(WHERE m.ended_at<=now()-interval '24 hours' AND m.usage_json?'totalTokens')) FROM model_calls m WHERE m.project_id IN(SELECT project_id FROM selected)));
`;
const data=JSON.parse(run('-n',env.namespace,'exec','statefulset/'+env.release+'-postgresql','--','psql','-U','judex','-d','judex','-At','-c',sql));
if(!data.analyses.length)throw new Error('No actual source analysis found');
console.log(JSON.stringify({observedAt:new Date().toISOString(),scope:'actual source analysis; recent24h known/unknown cache fields, durable aggregate only',sourceId:source,...data,interpretation:'zero eligible expired rows does not prove natural TTL; native Kimi usage excluded; system prompts are intentionally absent from checkpoints, private text presence is unknown; metadata hash/revision is separate evidence; marker scan covers controlled markers only'},null,2));
