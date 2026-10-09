import test from 'node:test';
import assert from 'node:assert/strict';
import {seedWork} from '../../web/src/features/work/seed.ts';
import {builtinWorkflowPresets,workflowPresetBody,diagramFromBody,validFlowBody,importDemoWorkflowPresets,saveDemoFlowDraft,publishDemoFlowDraft} from '../../web/src/features/work/workflowPresets.ts';
import {workflowCode} from '../../web/src/features/work/workflowGraph.ts';

test('all twenty scenarios expose complete researched workflows with real stages, review returns and branches',()=>{
  const advanced=builtinWorkflowPresets.workflows.filter(p=>p.complexity==='advanced');
  assert.equal(advanced.length,25);
  for(const scenario of builtinWorkflowPresets.scenarios)assert(advanced.some(p=>scenario.workflowIds.includes(p.id)),scenario.id);
  for(const preset of advanced)for(const locale of ['zh-CN','en']){
    const body=workflowPresetBody(preset,locale);
    assert(validFlowBody(body),preset.id);
    assert(body.nodes.length>=18);assert(body.nodes.filter(n=>n.kind==='decision').length>=5);
    assert(body.advisoryEdges.filter(e=>e.kind==='feedback').length>=5);
    assert(new Set(body.nodes.map(n=>n.phase)).size>=5);assert(preset.sources.every(s=>s.url.startsWith('https://')));
    assert(body.nodes.every(n=>n.allowedPositionIds.length===0&&!n.delegationUserIds));
    const diagram=diagramFromBody(body),code=workflowCode(diagram,locale==='en');
    assert(diagram.edges.length<body.advisoryEdges.length);assert(code.includes('subgraph'));assert(code.includes(' -. '));
    assert(code.includes('{'));assert(!code.includes('undefined'));
  }
});
test('complete presets coexist with prior copies and preserve their definitions across editing and publication',()=>{
  const state=seedWork();state.currentUser='林然';
  const base={catalogVersion:builtinWorkflowPresets.version,scenarioId:'software',locale:'zh-CN'};
  const simple=importDemoWorkflowPresets(state,'leaf',{...base,workflowIds:['software-standard']});
  const complete=importDemoWorkflowPresets(simple.state,'leaf',{...base,workflowIds:['software-delivery-advanced']});
  assert.equal(complete.createdCount,1);
  const old=simple.state.flows.find(f=>f.presetId==='software-standard');
  assert.deepEqual(complete.state.flows.find(f=>f.id===old.id),old);
  const flow=complete.state.flows.find(f=>f.presetId==='software-delivery-advanced');
  const body=structuredClone(flow.draft.body);body.nodes[0].phase='项目阶段定制';
  const saved=saveDemoFlowDraft(complete.state,'leaf',flow.id,1,body),stored=saved.state.flows.find(f=>f.id===flow.id);
  const published=publishDemoFlowDraft(saved.state,'leaf',flow.id,2,stored.draft.hash).state.flows.find(f=>f.id===flow.id);
  assert.equal(published.body.nodes[0].phase,'项目阶段定制');assert(published.body.advisoryEdges.some(e=>e.kind==='feedback'));
  assert.equal(published.presetId,'software-delivery-advanced');
  const invalid=structuredClone(body),edge=invalid.advisoryEdges.find(e=>e.kind==='feedback');edge.to='exception_review';
  assert.equal(validFlowBody(invalid),false);
});
