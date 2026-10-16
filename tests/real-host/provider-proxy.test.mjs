import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import {startUsageRelay} from './model-usage-relay.mjs';

const listen=server=>new Promise(resolve=>server.listen(0,'127.0.0.1',()=>resolve(server.address().port)));
const close=server=>new Promise(resolve=>server.close(resolve));

test('test relay uses the selected local proxy without rewriting the request',{timeout:10000},async()=>{
 let observed,proxyCalls=0;
 const upstream=http.createServer(async(req,res)=>{
  const chunks=[];for await(const chunk of req)chunks.push(chunk);
  observed={body:JSON.parse(Buffer.concat(chunks)),authorization:req.headers.authorization};
  res.writeHead(200,{'content-type':'application/json'});res.end(JSON.stringify({model:'m',choices:[],usage:{prompt_tokens:1,completion_tokens:1}}));
 });
 const port=await listen(upstream);
 const proxy=http.createServer((req,res)=>{
  proxyCalls++;
  const target=new URL(req.url);
  assert.equal(target.hostname,'127.0.0.1');assert.equal(Number(target.port),port);
  const forward=http.request(target,{method:req.method,headers:req.headers},reply=>{res.writeHead(reply.statusCode,reply.headers);reply.pipe(res);});req.pipe(forward);
 });
 const proxyPort=await listen(proxy);
 const relay=await startUsageRelay({baseURL:'http://127.0.0.1:'+port+'/v1',apiKey:'test-key',modelName:'m',allowTestUpstream:true,proxyURL:'http://127.0.0.1:'+proxyPort});
 try{
  const body={model:'m',messages:[{role:'user',content:'Keep this request unchanged'}],reasoning_effort:'high'};
  const response=await fetch(relay.baseURL+'/chat/completions',{method:'POST',headers:{Authorization:'Bearer test-key','content-type':'application/json'},body:JSON.stringify(body)});
  assert.equal(response.status,200);await response.text();
  assert.equal(proxyCalls,1);assert.deepEqual(observed.body,body);assert.equal(observed.authorization,'Bearer test-key');
 }finally{await relay.close();await close(proxy);await close(upstream);}
});

test('test proxy rejects external hosts and embedded credentials',async()=>{
 for(const proxyURL of ['http://proxy.example:7892','http://user:password@127.0.0.1:7892','https://127.0.0.1:7892']){
  await assert.rejects(startUsageRelay({baseURL:'https://api.apimart.ai/v1',apiKey:'test-key',modelName:'m',proxyURL}),/credential-free loopback/);
 }
});
