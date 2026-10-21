import test from 'node:test';
import assert from 'node:assert/strict';
import {createRequester, APIError} from '../../web/src/lib/api/request.ts';
const input={method:'POST',body:JSON.stringify({body:'one intended message'})};
const success=value=>Response.json({data:value,meta:{requestId:'fixture'}});

test('separate explicit intents do not inherit another uncertain write key',async t=>{
 const c=createRequester('',()=>{}),keys=[];
 t.mock.method(globalThis,'fetch',async(_url,options)=>{keys.push(options.headers.get('Idempotency-Key'));if(keys.length===1)throw Error('lost');return success({});});
 await assert.rejects(c.request('/uploads',{...input,idempotencyKey:'first'}));
 await c.request('/uploads',{...input,idempotencyKey:'second'});
 await c.request('/uploads',{...input,idempotencyKey:'first'});
 assert.deepEqual(keys,['first','second','first']);
});

for(const [name,bad] of Object.entries({
 truncated:()=>new Response('{"data":',{status:200}),
 invalidEnvelope:()=>Response.json({result:{id:'committed'}}),
 nullEnvelope:()=>Response.json(null),
 ambiguousEnvelope:()=>Response.json({data:{id:'committed'},error:{code:'x'}}),
 interruptedBody:()=>new Response(new ReadableStream({start(controller){controller.error(Error('disconnected after headers'));}})),
})) test(`a committed write survives ${name}; retry confirms one effect, next intent is new`,async t=>{
 const c=createRequester('/api/v1',()=>{}),keys=[],effects=new Map();let first=true;
 t.mock.method(globalThis,'fetch',async(_url,options)=>{
  const key=options.headers.get('Idempotency-Key');keys.push(key);
  if(!effects.has(key))effects.set(key,{id:'message-'+effects.size});
  if(first){first=false;return bad();}
  return success(effects.get(key));
 });
 await assert.rejects(c.request('/messages',input),e=>e instanceof APIError&&e.code==='INVALID_RESPONSE'&&e.retryable);
 assert.deepEqual(await c.request('/messages',input),{id:'message-0'});
 assert.equal(keys[0],keys[1]);assert.equal(effects.size,1);
 assert.deepEqual(await c.request('/messages',input),{id:'message-1'});
 assert.notEqual(keys[1],keys[2]);assert.equal(effects.size,2);
});

test('network, server failures and malformed error bodies retain intent; valid 409 releases it',async t=>{
 const c=createRequester('/api/v1',()=>{}),keys=[];
 const replies=[()=>{throw Error('network');},()=>Response.json({error:{code:'INTERNAL',message:'unknown effect'}},{status:503}),()=>new Response('<html>',{status:502}),()=>Response.json({error:{code:'VERSION_CONFLICT',message:'reload'}},{status:409}),()=>success(null)];
 t.mock.method(globalThis,'fetch',async(_url,options)=>{keys.push(options.headers.get('Idempotency-Key'));return replies.shift()();});
 for(const code of ['NETWORK','INTERNAL','INVALID_RESPONSE','VERSION_CONFLICT'])await assert.rejects(c.request('/projects/one',input),e=>e.code===code);
 assert.equal(await c.request('/projects/one',input),null);
 assert.ok(keys.slice(0,4).every(k=>k===keys[0]));assert.notEqual(keys[4],keys[0]);
});

test('204 acknowledges writes; unauthorized still notifies; logout drops uncertain session intent',async t=>{
 let unauthorized=0;const c=createRequester('/api/v1',()=>unauthorized++),keys=[];
 c.setCSRFToken('one-session');
 t.mock.method(globalThis,'fetch',async(_url,options)=>{keys.push({key:options.headers.get('Idempotency-Key'),csrf:options.headers.get('X-CSRF-Token')});return keys.length===3?new Response('{',{status:401}):new Response(null,{status:204});});
 await c.request('/messages',input);await c.request('/messages',input);assert.notEqual(keys[0].key,keys[1].key);
 await assert.rejects(c.request('/messages',input));assert.equal(unauthorized,1);
 c.clearCSRFToken();await c.request('/messages',input);assert.notEqual(keys[2].key,keys[3].key);assert.equal(keys[3].csrf,null);
});

test('binary part retry uses content identity and preserves caller headers',async t=>{
 const c=createRequester('/api/v1',()=>{}),keys=[];
 t.mock.method(globalThis,'fetch',async(_url,options)=>{keys.push(options.headers.get('Idempotency-Key'));assert.equal(options.headers.get('Content-Type'),'application/octet-stream');if(keys.length<3)throw Error('network');return success({});});
 const part=body=>({method:'PUT',body:Uint8Array.from(body).buffer,headers:new Headers({'Content-Type':'application/octet-stream'})});
 await assert.rejects(c.request('/parts/1',part([1])));await assert.rejects(c.request('/parts/1',part([2])));await c.request('/parts/1',part([1]));
 assert.notEqual(keys[0],keys[1]);assert.equal(keys[0],keys[2]);
});
