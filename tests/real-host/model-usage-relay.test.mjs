import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import zlib from 'node:zlib';
import {startUsageRelay} from './model-usage-relay.mjs';
const key='synthetic-test-key',model='glm-5.3-flash';
async function fixture(handler,options={}){
 const upstream=http.createServer(handler);await new Promise(resolve=>upstream.listen(0,'127.0.0.1',resolve));
 const relay=await startUsageRelay({baseURL:`http://127.0.0.1:${upstream.address().port}/v4`,apiKey:key,modelName:model,allowTestUpstream:true,...options});
 return{relay,close:async()=>{await relay.close();await new Promise(resolve=>{upstream.close(resolve);upstream.closeAllConnections();});}};
}
const send=(relay,body=JSON.stringify({model,messages:[{role:'user',content:'synthetic'}]}),token=key)=>fetch(relay.baseURL+'/chat/completions',{method:'POST',headers:{authorization:'Bearer '+token,'content-type':'application/json'},body});
test('SSE forwarding keeps exact request/body and compressed response bytes; usage is cumulative',async()=>{
 const body=JSON.stringify({model,messages:[{role:'user',content:'synthetic prompt must never be stored'}],stream:true,stream_options:{include_usage:true}});
 const wire='data: {"choices":[{"delta":{"content":"actual"}}]}\n\ndata: {"usage":{"prompt_tokens":1000,"completion_tokens":1,"prompt_tokens_details":{"cached_tokens":800}}}\n\ndata: {"usage":{"completion_tokens":50}}\n\ndata: [DONE]\n\n';
 const f=await fixture(async(req,res)=>{let received='';for await(const chunk of req)received+=chunk;assert.equal(received,body);assert.equal(req.headers.authorization,'Bearer '+key);res.writeHead(200,{'content-type':'text/event-stream','content-encoding':'gzip'});const bytes=zlib.gzipSync(wire);res.write(bytes.subarray(0,9));res.end(bytes.subarray(9));});
 try{const result=await send(f.relay,body);assert.equal(await result.text(),wire);await new Promise(resolve=>setImmediate(resolve));const a=f.relay.snapshot();assert.equal(a.calls,1);assert.equal(a.usageKnownCalls,1);assert.equal(a.inputTokens,1000);assert.equal(a.outputTokens,50);assert.equal(a.cacheReadTokens,800);assert.equal(a.cacheWriteKnownCalls,0);assert(!JSON.stringify(a).includes(key));assert(!JSON.stringify(a).includes('synthetic prompt'));}finally{await f.close();}
});

test('required high effort is enforced before forwarding without rewriting the request',async()=>{
 let calls=0;const body=JSON.stringify({model,reasoning_effort:'high',stream:false});
 const f=await fixture(async(req,res)=>{calls++;let received='';for await(const c of req)received+=c;assert.equal(received,body);res.end(JSON.stringify({model,usage:{prompt_tokens:10,completion_tokens:2}}));},{requiredReasoningEffort:'high'});
 try{
  assert.equal((await send(f.relay)).status,400);
  assert.equal((await send(f.relay,JSON.stringify({model,reasoning_effort:'low'}))).status,400);
  assert.equal(calls,0);
  assert.equal((await send(f.relay,body)).status,200);
  assert.equal(calls,1);
  assert.deepEqual(f.relay.snapshot().requestReasoningEffortCounts,{high:1});
 }finally{await f.close();}
});
test('missing and invalid cache stay unknown; explicit zero enters denominator',async()=>{
 let index=0;const details=[undefined,{cached_tokens:0},{cached_tokens:1001}];
 const f=await fixture((req,res)=>{req.resume();res.writeHead(200,{'content-type':'application/json'});res.end(JSON.stringify({usage:{prompt_tokens:1000,completion_tokens:50,prompt_tokens_details:details[index++]}}));});
 try{for(let i=0;i<3;i++)await(await send(f.relay)).text();const a=f.relay.snapshot();assert.equal(a.calls,3);assert.equal(a.usageKnownCalls,3);assert.equal(a.cacheKnownCalls,1);assert.equal(a.cacheSubsetInputTokens,1000);assert.equal(a.cacheReadTokens,0);}finally{await f.close();}
});
test('observation decoding cannot change non-UTF8 response bytes',async()=>{
 const bytes=Buffer.from([0,255,128,10,13]);
 const f=await fixture((req,res)=>{req.resume();res.writeHead(503,{'content-type':'application/octet-stream'});res.end(bytes);});
 try{assert.deepEqual(Buffer.from(await(await send(f.relay)).arrayBuffer()),bytes);assert.equal(f.relay.snapshot().usageKnownCalls,0);}finally{await f.close();}
});
test('gateway cache-write spelling keeps explicit zero and rejects conflicting aliases',async()=>{
 let index=0;const details=[{cached_tokens:0,cache_write_tokens:0},{cached_tokens:100,cache_creation_tokens:20},{cached_tokens:100,cache_write_tokens:0,cache_creation_tokens:20},{cached_tokens:100,cache_write_tokens:null,cache_creation_tokens:20}];
 const f=await fixture((req,res)=>{req.resume();res.writeHead(200,{'content-type':'application/json'});res.end(JSON.stringify({usage:{prompt_tokens:1000,completion_tokens:5,prompt_tokens_details:details[index++]}}));});
 try{for(let i=0;i<4;i++)await(await send(f.relay)).text();const a=f.relay.snapshot();assert.equal(a.cacheWriteKnownCalls,3);assert.equal(a.cacheWriteTokens,40);assert.equal(a.cacheKnownCalls,4);}finally{await f.close();}
});
test('paid-call test budget rejects excess requests before forwarding',async()=>{
 let calls=0;const f=await fixture((req,res)=>{calls++;req.resume();res.writeHead(200,{'content-type':'application/json'});res.end('{}');},{maxCalls:1});
 try{assert.equal((await send(f.relay)).status,200);const blocked=await send(f.relay);assert.equal(blocked.status,400);assert.equal((await blocked.json()).error.type,'test_request_budget_exhausted');assert.equal(calls,1);assert.equal(f.relay.snapshot().calls,1);}finally{await f.close();}
});
test('rejected authorization, path and model never reach upstream',async()=>{
 let calls=0;const f=await fixture((req,res)=>{calls++;req.resume();res.end();});
 try{assert.equal((await send(f.relay,undefined,'wrong')).status,403);assert.equal((await fetch(f.relay.baseURL+'/models')).status,403);assert.equal((await send(f.relay,JSON.stringify({model:'other'}))).status,400);assert.equal(calls,0);assert.equal(f.relay.snapshot().calls,0);}finally{await f.close();}
});
test('provider errors are forwarded once, without manufactured usage or retry',async()=>{
 const payload=JSON.stringify({error:{code:'1308',message:'synthetic quota ceiling'}});let calls=0;
 const f=await fixture((req,res)=>{calls++;req.resume();res.writeHead(429,{'content-type':'application/json'});res.end(payload);});
 try{const result=await send(f.relay);assert.equal(result.status,429);assert.equal(await result.text(),payload);assert.equal(calls,1);assert.equal(f.relay.snapshot().usageKnownCalls,0);assert.equal(f.relay.snapshot().statusCounts[429],1);}finally{await f.close();}
});

test('returned model is counted once and provider error aggregate redacts credentials',async()=>{
 let calls=0;const error=JSON.stringify({error:{type:'access_terminated_error',message:'Quota reached '+key}});
 const f=await fixture((req,res)=>{req.resume();calls++;if(calls===1){res.writeHead(200,{'content-type':'text/event-stream'});res.end('data: {"model":"returned-model"}\n\ndata: {"model":"returned-model","usage":{"prompt_tokens":10,"completion_tokens":2}}\n\ndata: [DONE]\n\n');}else{res.writeHead(403,{'content-type':'application/json'});res.end(error);}});
 try{await(await send(f.relay)).text();const failed=await send(f.relay);assert.equal(failed.status,403);assert.equal(await failed.text(),error);const a=f.relay.snapshot();assert.deepEqual(a.returnedModelCounts,{'returned-model':1});assert.equal(a.returnedModelUnknownCalls,1);assert.equal(a.lastProviderError.type,'access_terminated_error');assert(!JSON.stringify(a).includes(key));assert.equal(a.calls,2);}finally{await f.close();}
});
