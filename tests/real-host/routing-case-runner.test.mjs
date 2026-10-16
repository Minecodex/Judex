import test from 'node:test';
import assert from 'node:assert/strict';
import {mergeSavedCases,runRoutingCases} from './routing-case-runner.mjs';

const cases=[{case:'bound',clientSubmissionId:'client-a',file:'a.json'},{case:'plan',clientSubmissionId:'client-b',file:'b.json'},{case:'later',clientSubmissionId:'client-c',file:'c.json'}];
const saved=[{clientSubmissionId:'client-a',source:{sourceId:'source-a',analysisId:'analysis-a',batchId:'batch-a'}},{clientSubmissionId:'client-b',source:{sourceId:'source-b',analysisId:'analysis-b',batchId:'batch-b'}}];
const response=(id,state)=>({ok:true,exitCode:0,data:{id:'analysis-'+id,batchId:'batch-'+id,state}});

test('generic failed after a completed case stops every later mutation',async()=>{
 const writes=[],snapshots=[];
 const result=await runRoutingCases({cases,saved,call:async args=>{
  if(args[0]!=='submission-analysis'){writes.push(args);throw new Error('Unexpected mutation');}
  return response(args[1].slice(-1),args[1]==='source-a'?'completed':'failed');
 },checkpoint:async value=>snapshots.push(structuredClone(value)),wait:async()=>{throw new Error('Must stop');}});
 assert.deepEqual(writes,[]);
 assert.equal(result.stopped.reason,'analysis_failed');
 assert.deepEqual(result.cases.map(c=>c.sourceId),['source-a','source-b']);
 assert.equal(snapshots.at(-1).cases[1].analysis.batchId,'batch-b');
});

test('explicit retry keeps the original source, analysis and batch without resubmission',async()=>{
 const mutations=[],reads={};
 const result=await runRoutingCases({cases:cases.slice(0,2),saved,allowRetry:true,call:async args=>{
  if(args[0]==='task-analysis'){mutations.push(args);return {ok:true,exitCode:0,data:{}};}
  assert.equal(args[0],'submission-analysis');
  const id=args[1].slice(-1);reads[id]=(reads[id]??0)+1;
  return response(id,id==='a'||reads[id]>1?'completed':'failed');
 },checkpoint:async()=>{},wait:async()=>{throw new Error('No polling needed');}});
 assert.equal(result.stopped,null);
 assert.deepEqual(mutations,[['task-analysis','retry','analysis-b']]);
 assert.equal(result.cases[1].analysis.batchId,'batch-b');
});

test('a rejected retry cannot advance to the next source',async()=>{
 const mutations=[];
 const result=await runRoutingCases({cases:cases.slice(1),saved,allowRetry:true,call:async args=>{
  if(args[0]==='task-analysis'){mutations.push(args);return {ok:false,exitCode:8};}
  return response('b','failed');
 },checkpoint:async()=>{},wait:async()=>{}});
 assert.equal(result.stopped.reason,'retry_rejected');
 assert.equal(result.cases.length,1);
 assert.deepEqual(mutations,[['task-analysis','retry','analysis-b']]);
});

test('recovery rejects a changed batch before retrying or creating a later source',async()=>{
 const result=await runRoutingCases({cases:cases.slice(1),saved,allowRetry:true,call:async args=>{
  assert.equal(args[0],'submission-analysis');
  return {...response('b','failed'),data:{id:'analysis-b',batchId:'replacement',state:'failed'}};
 },checkpoint:async()=>{},wait:async()=>{}});
 assert.equal(result.stopped.reason,'batch_changed');
 assert.equal(result.cases[0].retryRequested,false);
});

test('a partial driver checkpoint preserves every unvisited existing source',()=>{
 const merged=mergeSavedCases(saved,[{clientSubmissionId:'client-a',sourceId:'source-a',analysis:{id:'analysis-a',batchId:'batch-a',state:'completed'}}]);
 assert.equal(merged.length,2);
 assert.equal(merged[1].source.sourceId,'source-b');
 assert.equal(merged[0].source.batchId,'batch-a');
});

test('merging checkpoints cannot silently replace a real source',()=>{
 assert.throws(()=>mergeSavedCases(saved,[{clientSubmissionId:'client-a',sourceId:'replacement'}]),/source changed/);
});
