import test from 'node:test';
import assert from 'node:assert/strict';
import {prefixSnapshot,compareMessagePrefix,recordMessagePrefix,newPrefixAggregate} from './message-prefix.mjs';

const base={model:'m',messages:[{role:'system',content:'Project A, developer, workflow R1'},{role:'user',content:'Read fixed original'}]};
test('continuation keeps all prior message bytes',()=>{
 const result=compareMessagePrefix(prefixSnapshot(base),prefixSnapshot({...base,messages:[...base.messages,{role:'assistant',content:'Reading'}]}));
 assert.equal(result.completeMessages,2);assert.equal(result.commonBytes,result.beforeBytes);
});
test('project, role and workflow changes end the common prefix',()=>{
 for(const content of ['Project B, developer, workflow R1','Project A, reviewer, workflow R1','Project A, developer, workflow R2']){
  const result=compareMessagePrefix(prefixSnapshot(base),prefixSnapshot({...base,messages:[{role:'system',content},base.messages[1]]}));
  assert.equal(result.completeMessages,0);assert.ok(result.commonBytes<result.beforeBytes);
 }
});
test('changed preferences in first system message invalidate its full match',()=>{
 const a={...base,messages:[{role:'system',content:'Private preference: concise'},...base.messages]};
 const b={...a,messages:[{role:'system',content:'Private preference: detailed'},...base.messages]};
 assert.equal(compareMessagePrefix(prefixSnapshot(a),prefixSnapshot(b)).completeMessages,0);
});
test('tool and model changes are independently reported',()=>{
 const changed={...base,model:'other',tools:[{type:'function',function:{name:'read_original'}}]};
 const result=compareMessagePrefix(prefixSnapshot(base),prefixSnapshot(changed));
 assert.equal(result.sameTools,false);assert.equal(result.sameModel,false);assert.equal(result.completeMessages,2);
});
test('object key order does not cause false mismatch',()=>{
 const other={model:'m',messages:base.messages.map(m=>({content:m.content,role:m.role}))};
 assert.equal(compareMessagePrefix(prefixSnapshot(base),prefixSnapshot(other)).completeMessages,2);
});
test('durable aggregate contains no prompt text or per-request hashes',()=>{
 const aggregate=newPrefixAggregate();let previous=recordMessagePrefix(aggregate,null,base);
 recordMessagePrefix(aggregate,previous,{...base,messages:[...base.messages,{role:'assistant',content:'PRIVATE_RESULT'}]});
 assert.equal(aggregate.previousMessagesRetained,1);
 assert.ok(!JSON.stringify(aggregate).includes('PRIVATE_RESULT'));assert.ok(!JSON.stringify(aggregate).includes('developer'));
});
