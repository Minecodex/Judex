// Read-only, bounded diagnostics for an analysis in this owned scene.
import fs from 'node:fs';
import {execFileSync} from 'node:child_process';
const id=process.argv[2];
if(!/^[0-9a-f-]{36}$/.test(id??''))throw new Error('Analysis UUID required');
const env=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/environment.json','utf8'));
const run=(...args)=>execFileSync('kubectl',args,{encoding:'utf8',windowsHide:true,maxBuffer:2<<20});
const namespace=JSON.parse(run('get','namespace',env.namespace,'-o','json'));
if(namespace.metadata.uid!==env.uid||namespace.metadata.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Observation ownership changed');
const sql=`WITH analysis AS(SELECT batch_id FROM task_analyses WHERE id='${id}')
SELECT jsonb_build_object(
 'budget',(SELECT jsonb_build_object('state',b.state,'attempts',b.model_attempts,'used',b.used_tokens,'reserved',b.reserved_tokens) FROM discussion_batches b JOIN analysis a ON a.batch_id=b.id),
 'calls',(SELECT jsonb_agg(jsonb_build_object('run',r.id,'parent',r.parent_run_id,'kind',i.kind,'model',m.usage_json->'observation'->>'modelName','attempt',m.attempt,'state',m.state,'used',m.usage_json->>'totalTokens','in',m.usage_json->'observation'->>'inputTokens','reserved',m.reserved_tokens) ORDER BY m.started_at) FROM model_calls m JOIN agent_runs r ON r.id=m.run_id JOIN agent_identities i ON i.id=r.identity_id JOIN analysis a ON a.batch_id=r.batch_id),
 'tools',(SELECT jsonb_agg(jsonb_build_object('run',r.id,'name',t.name,'state',t.state,'bytes',octet_length(t.result_json::text),'preview',left(t.result_json::text,420)) ORDER BY t.started_at) FROM tool_calls t JOIN agent_runs r ON r.id=t.run_id JOIN analysis a ON a.batch_id=r.batch_id),
 'latestToolArguments',(SELECT jsonb_agg(jsonb_build_object('name',c->>'name','arguments',left(c->>'arguments',800))) FROM context_checkpoints p JOIN analysis a ON a.batch_id=(SELECT batch_id FROM agent_runs WHERE id=p.run_id) CROSS JOIN LATERAL jsonb_array_elements(p.manifest->'messages') m CROSS JOIN LATERAL jsonb_array_elements(COALESCE(m->'tool_calls',m->'toolCalls','[]')) c WHERE p.created_at=(SELECT max(x.created_at) FROM context_checkpoints x WHERE x.run_id=p.run_id)),
 'assistantMessages',(SELECT jsonb_agg(left(m->>'content',1800)) FROM context_checkpoints p JOIN analysis a ON a.batch_id=(SELECT batch_id FROM agent_runs WHERE id=p.run_id) CROSS JOIN LATERAL jsonb_array_elements(p.manifest->'messages') m WHERE m->>'role'='assistant' AND p.created_at=(SELECT max(x.created_at) FROM context_checkpoints x WHERE x.run_id=p.run_id))
);`;
console.log(run('-n',env.namespace,'exec','statefulset/'+env.release+'-postgresql','--','psql','-U','judex','-d','judex','-At','-c',sql).trim());
