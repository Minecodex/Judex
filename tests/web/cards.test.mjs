import test from 'node:test';
import assert from 'node:assert/strict';
import {seedWork} from '../../web/src/features/work/seed.ts';
import {createWork} from '../../web/src/features/work/actions.ts';
test('subtasks stay within the same project and plan and always begin as drafts',()=>{
 const state=seedWork();const draft={kind:'task',title:'验证异常恢复',description:'验证异常场景',criteria:'附验证依据',seatId:'maker',flowId:'delivery',planId:'leaf-first',parentId:'build'};
 const result=createWork(state,'leaf',draft);assert.equal(result.state.tasks.at(-1).parentId,'build');assert.equal(result.state.tasks.at(-1).status,'draft');
 assert.equal(createWork(state,'leaf',{...draft,planId:'leaf-next'}).error,'scope');assert.equal(createWork(state,'leaf',{...draft,planId:null}).error,'scope');assert.equal(createWork(state,'leaf',{...draft,kind:'plan'}).error,'scope');
});
