import test from 'node:test';
import assert from 'node:assert/strict';
import {seedWork} from '../../web/src/features/work/seed.ts';
import {builtinPositionPresets} from '../../web/src/features/work/positionPresets.ts';
import {builtinWorkflowPresets,importDemoWorkflowPresets,saveDemoFlowDraft,publishDemoFlowDraft,validFlowBody} from '../../web/src/features/work/workflowPresets.ts';
import {savePosition} from '../../web/src/features/work/actions.ts';
const request={catalogVersion:builtinWorkflowPresets.version,scenarioId:'software',workflowIds:['software-standard','software-bug'],locale:'zh-CN'};

test('workflow catalog uses the shared scenarios and copies independent drafts with parallel edges',()=>{
  assert.deepEqual(builtinWorkflowPresets.scenarios.map(s=>({id:s.id,name:s.name})),builtinPositionPresets.scenarios.map(s=>({id:s.id,name:s.name})));
  const state=seedWork();state.currentUser='林然';
  const result=importDemoWorkflowPresets(state,'leaf',request);
  assert.equal(result.createdCount,2);
  assert.deepEqual(result.state.positions,state.positions);assert.deepEqual(result.state.seats,state.seats);
  const flow=result.state.flows.find(f=>f.presetId==='software-standard');
  assert.equal(flow.status,'draft');assert.equal(flow.edges.length,6);
  const body=structuredClone(flow.draft.body);body.name='独立流程';body.nodes[0].name='独立节点';
  const saved=saveDemoFlowDraft(result.state,'leaf',flow.id,1,body);
  assert.ok(saved.state);
  assert.equal(savePosition(result.state,'leaf',{name:'新职位',prompt:'职责',flowId:flow.id,nodeId:'step1'}).error,'scope');
  const replay=importDemoWorkflowPresets(saved.state,'leaf',{...request,locale:'en'});
  assert.equal(replay.createdCount,0);assert.equal(replay.skippedCount,2);
  const edited=replay.state.flows.find(f=>f.id===flow.id);
  assert.equal(edited.name.zh,'独立流程');
  assert.equal(publishDemoFlowDraft(replay.state,'leaf',flow.id,2,'stale').error,'stale');
  const published=publishDemoFlowDraft(replay.state,'leaf',flow.id,2,edited.draft.hash);
  assert.equal(published.state.flows.find(f=>f.id===flow.id).status,'published');
  assert.equal(published.state.flows.find(f=>f.id===flow.id).presetId,'software-standard');
  assert.equal(builtinWorkflowPresets.workflows.find(p=>p.id==='software-standard').nodes[0].name.zh,'需求确认');
});
test('workflow import rejects unauthorized, duplicate, mixed-scenario and stale selections without mutation',()=>{
  const state=seedWork();assert.equal(importDemoWorkflowPresets(state,'leaf',request).error,'permission');state.currentUser='林然';
  const before=structuredClone(state);
  for(const workflowIds of [[],['software-bug','software-bug'],['software-standard','content-publish'],['missing']])
    assert.equal(importDemoWorkflowPresets(state,'leaf',{...request,workflowIds}).error,'scope');
  assert.equal(importDemoWorkflowPresets(state,'leaf',{...request,catalogVersion:'old'}).error,'stale');assert.deepEqual(state,before);
});
test('workflow editing rejects cycles and stale versions, while disconnected nodes remain disconnected',()=>{
  const state=seedWork();state.currentUser='林然';
  const imported=importDemoWorkflowPresets(state,'leaf',request),flow=imported.state.flows.find(f=>f.presetId==='software-standard');
  const cycle=structuredClone(flow.draft.body);cycle.advisoryEdges.push({from:'step6',to:'step1'});
  assert.equal(validFlowBody(cycle),false);
  assert.equal(saveDemoFlowDraft(imported.state,'leaf',flow.id,1,cycle).error,'scope');
  assert.equal(saveDemoFlowDraft(imported.state,'leaf',flow.id,2,flow.draft.body).error,'stale');
  const independent=structuredClone(flow.draft.body);independent.advisoryEdges=[];
  const saved=saveDemoFlowDraft(imported.state,'leaf',flow.id,1,independent);
  assert.deepEqual(saved.state.flows.find(f=>f.id===flow.id).edges,[]);
});
