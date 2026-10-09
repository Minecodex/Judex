import test from 'node:test';
import assert from 'node:assert/strict';
import {seedWork} from '../../web/src/features/work/seed.ts';
import {builtinPositionPresets, importDemoPositionPresets} from '../../web/src/features/work/positionPresets.ts';
import {savePosition} from '../../web/src/features/work/actions.ts';
const request={catalogVersion:builtinPositionPresets.version,scenarioId:'software',roleIds:['frontend-developer','qa-engineer'],locale:'zh-CN'};

test('preset import is selective, independent and duplicates stay skipped after editing or switching language',()=>{
  const state=seedWork();state.currentUser='林然';
  const result=importDemoPositionPresets(state,'leaf',request);
  assert.equal(result.createdCount,2);
  assert.deepEqual(result.state.seats,state.seats);
  assert.deepEqual(result.state.flows,state.flows);
  assert.deepEqual(result.state.projects,state.projects);
  const added=result.state.positions.filter(position=>position.presetId);
  assert.equal(added.length,2);assert.deepEqual(added[0].bindings,[]);
  const edited=savePosition(result.state,'leaf',{id:added[0].id,name:'前端职责定制',prompt:'本项目的独立职责',flowId:'',nodeId:''});
  assert.ok(edited.state);assert.equal(edited.state.positions.find(position=>position.id===added[0].id).presetId,'frontend-developer');
  const replay=importDemoPositionPresets(edited.state,'leaf',{...request,locale:'en'});
  assert.equal(replay.createdCount,0);assert.equal(replay.skippedCount,2);
  assert.equal(replay.state.positions.find(position=>position.id===added[0].id).prompt.zh,'本项目的独立职责');
  assert.notEqual(builtinPositionPresets.roles.find(role=>role.id==='frontend-developer').prompt.zh,'本项目的独立职责');
});

test('preset imports require project management and reject the entire invalid selection',()=>{
  const state=seedWork();assert.equal(importDemoPositionPresets(state,'leaf',request).error,'permission');
  state.currentUser='林然';const before=structuredClone(state);
  assert.equal(importDemoPositionPresets(state,'leaf',{...request,roleIds:['frontend-developer','screenwriter']}).error,'scope');
  assert.equal(importDemoPositionPresets(state,'leaf',{...request,roleIds:['qa-engineer','qa-engineer']}).error,'scope');
  assert.equal(importDemoPositionPresets(state,'leaf',{...request,catalogVersion:'old'}).error,'stale');
  assert.deepEqual(state,before);
});
