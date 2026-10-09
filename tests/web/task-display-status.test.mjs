import test from 'node:test';
import assert from 'node:assert/strict';
import {seedWork} from '../../web/src/features/work/seed.ts';
import {taskDisplayStatus} from '../../web/src/features/work/runtime.ts';

test('display blockers respect stage, hard conditions, server satisfaction and execution exceptions',()=>{
 const state=seedWork(),task=structuredClone(state.tasks.find(t=>t.id==='build'));
 task.status='ready';task.businessStatus='ready';task.requirements=[{id:'external',kind:'task',ref:'uncached',hard:true,at:'start',label:{zh:'前置',en:'Prerequisite'},satisfied:false}];
 assert.equal(taskDisplayStatus(task,state),'blocked');assert.equal(task.status,'ready');
 task.requirements[0].satisfied=true;assert.equal(taskDisplayStatus(task,state),'ready');
 task.requirements[0].satisfied=false;task.requirements[0].at='accept';assert.equal(taskDisplayStatus(task,state),'ready');
 task.requirements[0].at='both';assert.equal(taskDisplayStatus(task,state),'blocked');
 task.requirements[0].hard=false;assert.equal(taskDisplayStatus(task,state),'ready');
 task.requirements[0].hard=true;assert.equal(taskDisplayStatus(task,state,false),'ready');
 task.businessStatus='working';assert.equal(taskDisplayStatus(task,state,true),'working');
 task.executionException={reason:'保留原阶段'};assert.equal(taskDisplayStatus(task,state,true),'skipped');
 task.discardedAt='2026-10-09T00:00:00Z';assert.equal(taskDisplayStatus(task,state,true),'discarded');
});
