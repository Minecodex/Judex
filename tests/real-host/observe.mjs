import {execFileSync} from 'node:child_process';
import fs from 'node:fs';
const env=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/environment.json','utf8'));
const run=(...args)=>execFileSync('kubectl',args,{encoding:'utf8',windowsHide:true,maxBuffer:4*1024*1024});
const namespace=JSON.parse(run('get','namespace',env.namespace,'-o','json'));
if(namespace.metadata.uid!==env.uid||namespace.metadata.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Observation namespace ownership changed');
const result=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/KA01-project-result.json','utf8'));
const project=result.resultRef?.slice(8);if(!/^[0-9a-f-]{36}$/.test(project))throw new Error('Actual scene project required');
const sql=`SELECT jsonb_build_object(
 'projectId','${project}',
 'workflows',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'name',name,'hasDraft',EXISTS(SELECT 1 FROM workflow_versions v WHERE v.definition_id=d.id AND v.state='draft'),'published',published_version_id IS NOT NULL)),'[]'::jsonb) FROM workflow_definitions d WHERE d.project_id='${project}'),
 'analyses',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'state',state,'batchId',batch_id,'sourceId',source_id,'summary',left(summary,400),'error',error_code)),'[]'::jsonb) FROM task_analyses WHERE project_id='${project}'),
 'runs',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'parent',parent_run_id,'state',state,'identityId',identity_id)),'[]'::jsonb) FROM agent_runs WHERE project_id='${project}'),
 'tools',(SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]'::jsonb) FROM (SELECT name,state,count(*) AS calls FROM tool_calls WHERE project_id='${project}' GROUP BY name,state ORDER BY name,state)x),
 'cache',(SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]'::jsonb) FROM (
 SELECT usage_json->'observation'->>'modelName' AS model,count(*) AS calls,
 count(*) FILTER(WHERE usage_known AND usage_json->'observation'->>'cacheReadTokens' IS NOT NULL) AS cache_known_calls,
 sum((usage_json->'observation'->>'inputTokens')::bigint) FILTER(WHERE usage_known AND usage_json->'observation'->>'cacheReadTokens' IS NOT NULL) AS input_tokens,
 sum((usage_json->'observation'->>'cacheReadTokens')::bigint) FILTER(WHERE usage_known) AS cache_read_tokens
 FROM model_calls WHERE project_id='${project}' AND ended_at>now()-interval '24 hours' GROUP BY 1)x)
);`;
console.log(run('-n',env.namespace,'exec','statefulset/'+env.release+'-postgresql','--','psql','-U','judex','-d','judex','-At','-c',sql).trim());
