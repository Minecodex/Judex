// Test-only loopback transport. No retry, prompt/response persistence or key log.
import http from 'node:http';
import https from 'node:https';
import zlib from 'node:zlib';
import {PassThrough} from 'node:stream';
import {newPrefixAggregate,recordMessagePrefix} from './message-prefix.mjs';
import {providerLookup} from './provider-lookup.mjs';

const counter=value=>Number.isSafeInteger(value)&&value>=0?value:null;
const hopHeaders=new Set(['host','connection','keep-alive','proxy-authenticate','proxy-authorization','te','trailer','transfer-encoding','upgrade']);
export async function startUsageRelay({baseURL,apiKey,modelName,allowTestUpstream=false,requiredReasoningEffort,maxCalls,resolveAddress,proxyURL}){
 if(maxCalls!==undefined&&(!Number.isSafeInteger(maxCalls)||maxCalls<1))throw new Error('Positive test request budget required');
 const upstream=new URL(baseURL);
 const official=['https://open.bigmodel.cn/api/coding/paas/v4','https://open.bigmodel.cn/api/paas/v4','https://api.kimi.com/coding/v1','https://api.apimart.ai/v1'];
 if(!allowTestUpstream&&!official.includes(baseURL))throw new Error('Only the approved model endpoint is allowed');
 if(allowTestUpstream&&(upstream.protocol!=='http:'||upstream.hostname!=='127.0.0.1'))throw new Error('Controlled upstream must be loopback');
 if(proxyURL){
  const proxy=new URL(proxyURL);
  if(proxy.protocol!=='http:'||proxy.hostname!=='127.0.0.1'||proxy.username||proxy.password||proxy.pathname!=='/'||proxy.search||proxy.hash)throw new Error('Test proxy must be a credential-free loopback HTTP endpoint');
  const [major,minor]=process.versions.node.split('.').map(Number);
  if(major<24||major===24&&minor<5)throw new Error('Native test proxy requires Node 24.5 or later');
  if(resolveAddress)throw new Error('Choose one test network route');
 }
 const transport=upstream.protocol==='https:'?https:http;
 const agent=new transport.Agent({keepAlive:true,...(resolveAddress?{lookup:providerLookup(upstream.hostname,resolveAddress)}:{}),...(proxyURL?{proxyEnv:{HTTPS_PROXY:proxyURL,HTTP_PROXY:proxyURL,NO_PROXY:''}}:{})});
 const aggregate={source:'real provider response through test-only loopback relay',model:modelName,calls:0,usageKnownCalls:0,cacheKnownCalls:0,inputTokens:0,outputTokens:0,cacheSubsetInputTokens:0,cacheReadTokens:0,cacheWriteKnownCalls:0,cacheWriteTokens:0,statusCounts:{},transportErrors:{},clientAborts:0,rejectedRequests:0,requestReasoningEffortCounts:{},returnedModelCounts:{},returnedModelUnknownCalls:0,lastProviderError:null};
 const active=new Set();
 aggregate.transportPath=proxyURL?'explicit loopback proxy':resolveAddress?'test DNS pin':'system route';
 aggregate.messagePrefix=newPrefixAggregate();
 let previousPrefix=null;
 const server=http.createServer(async(request,response)=>{
  if(request.method!=='POST'||request.url!==upstream.pathname+'/chat/completions'||request.headers.authorization!==`Bearer ${apiKey}`){aggregate.rejectedRequests++;response.writeHead(403);response.end();return;}
  let chunks=[],length=0;
  try{
   for await(const chunk of request){length+=chunk.length;if(length>64*1024*1024)throw new Error('Body limit');chunks.push(chunk);}
  }catch{response.writeHead(413);response.end();return;}
  let body=Buffer.concat(chunks);chunks=[];
  let parsed;
  try{parsed=JSON.parse(body);if(parsed.model!==modelName)throw new Error('Model mismatch');}
  catch{aggregate.rejectedRequests++;response.writeHead(400);response.end();return;}
  if(requiredReasoningEffort&&parsed.reasoning_effort!==requiredReasoningEffort){
   aggregate.rejectedRequests++;response.writeHead(400,{'content-type':'application/json'});
   response.end(JSON.stringify({error:{type:'invalid_request_error',message:'Configured reasoning_effort='+requiredReasoningEffort+' is required; request was not sent upstream'}}));return;
  }
  const effort=typeof parsed.reasoning_effort==='string'?parsed.reasoning_effort:'unset';
  aggregate.requestMaxOutputCounts??={};
  const maxOutput=String(parsed.max_tokens??parsed.max_completion_tokens??'unset');
  aggregate.requestMaxOutputCounts[maxOutput]=(aggregate.requestMaxOutputCounts[maxOutput]??0)+1;
  aggregate.requestReasoningEffortCounts[effort]=(aggregate.requestReasoningEffortCounts[effort]??0)+1;
  if(maxCalls!==undefined&&aggregate.calls>=maxCalls){aggregate.rejectedRequests++;response.writeHead(400,{'content-type':'application/json'});response.end(JSON.stringify({error:{type:'test_request_budget_exhausted',message:'Preregistered test request budget reached; request was not sent upstream'}}));return;}
  aggregate.calls++;
  previousPrefix=recordMessagePrefix(aggregate.messagePrefix,previousPrefix,parsed);
  const headers=Object.fromEntries(Object.entries(request.headers).filter(([name])=>!hopHeaders.has(name)));
  headers.host=upstream.host;
  const usage={input:null,output:null,read:null,write:null};
  let finalized=false,parserBuffer='',overflow=false,returnedModel=null;
  const consumeJSON=value=>{
   if(typeof value?.model==='string'&&value.model.length<=120)returnedModel=value.model.split(apiKey).join('[redacted]');
   if(value?.error){const error=value.error;aggregate.lastProviderError={type:String(error.type??error.code??'provider_error').slice(0,80),message:String(error.message??'').split(apiKey).join('[redacted]').slice(0,600)};}
   const u=value?.usage;if(!u)return;
   const details=u.prompt_tokens_details??{};
   const hasWrite=details.cache_write_tokens!=null||details.cache_creation_tokens!=null;
   const write=details.cache_write_tokens!=null&&details.cache_creation_tokens!=null&&details.cache_write_tokens!==details.cache_creation_tokens?null:(details.cache_write_tokens??details.cache_creation_tokens);
   for(const [key,value] of [['input',u.prompt_tokens],['output',u.completion_tokens],['read',details.cached_tokens],['write',write]]){
    if(value!==undefined)usage[key]=counter(value);
   }
   if(hasWrite&&write===null)usage.write=null;
  };
  const finish=()=>{
   if(finalized)return;finalized=true;
   if(returnedModel)aggregate.returnedModelCounts[returnedModel]=(aggregate.returnedModelCounts[returnedModel]??0)+1;
   else aggregate.returnedModelUnknownCalls++;
   if(overflow)return;
   if(usage.input!==null&&usage.output!==null){aggregate.usageKnownCalls++;aggregate.inputTokens+=usage.input;aggregate.outputTokens+=usage.output;}
   if(usage.input!==null&&usage.read!==null&&usage.read<=usage.input&&(usage.write===null||usage.read+usage.write<=usage.input)){
    aggregate.cacheKnownCalls++;aggregate.cacheSubsetInputTokens+=usage.input;aggregate.cacheReadTokens+=usage.read;
   }
   if(usage.input!==null&&usage.write!==null&&usage.write<=usage.input&&(usage.read===null||usage.read+usage.write<=usage.input)){
    aggregate.cacheWriteKnownCalls++;aggregate.cacheWriteTokens+=usage.write;
   }
  };
  const forward=transport.request(new URL(request.url,upstream.origin),{method:'POST',headers,agent},incoming=>{
   aggregate.statusCounts[incoming.statusCode]=(aggregate.statusCounts[incoming.statusCode]??0)+1;
   response.writeHead(incoming.statusCode,Object.fromEntries(Object.entries(incoming.headers).filter(([name])=>!hopHeaders.has(name))));
   // A separate decoding branch observes usage; client bytes stay untouched.
   const encoding=incoming.headers['content-encoding'];
   const decoder=encoding==='gzip'?zlib.createGunzip():encoding==='br'?zlib.createBrotliDecompress():encoding==='deflate'?zlib.createInflate():null;
   const observed=decoder??new PassThrough();
   const streaming=String(incoming.headers['content-type']).includes('text/event-stream');
   observed.setEncoding('utf8');
   observed.on('data',chunk=>{
    if(overflow)return;
    parserBuffer+=chunk;
    if(parserBuffer.length>2*1024*1024){overflow=true;parserBuffer='';return;}
    if(streaming){let index;while((index=parserBuffer.indexOf('\n'))>=0){const line=parserBuffer.slice(0,index).replace(/\r$/,'');parserBuffer=parserBuffer.slice(index+1);if(line.startsWith('data:')&&line.slice(5).trim()!=='[DONE]')try{consumeJSON(JSON.parse(line.slice(5)));}catch{}}}
   });
   observed.once('end',()=>{if(!streaming&&!overflow)try{consumeJSON(JSON.parse(parserBuffer));}catch{}parserBuffer='';finish();});
   observed.once('error',()=>{overflow=true;parserBuffer='';finish();});
   incoming.once('aborted',()=>{finish();response.destroy();});
   incoming.once('error',()=>{finish();response.destroy();});
   incoming.pipe(observed);
   incoming.pipe(response);
  });
  active.add(forward);
  forward.once('close',()=>active.delete(forward));
  forward.once('error',error=>{
   const code=/^[A-Z_0-9]+$/.test(error.code??'')?error.code:'transport_error';
   aggregate.transportErrors[code]=(aggregate.transportErrors[code]??0)+1;finish();
   if(!response.headersSent){response.writeHead(502,{'content-type':'application/json'});response.end(JSON.stringify({error:{message:'Owned test relay upstream transport failure',code}}));}else response.destroy();
  });
  response.once('close',()=>{if(!response.writableEnded){aggregate.clientAborts++;forward.destroy();finish();}});
  forward.end(body);body=null;
 });
 await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});
 return {
  baseURL:`http://127.0.0.1:${server.address().port}${upstream.pathname}`,
  snapshot:()=>structuredClone(aggregate),
  close:async()=>{for(const request of active)request.destroy();await new Promise(resolve=>{server.close(resolve);server.closeAllConnections();});agent.destroy();return structuredClone(aggregate);},
 };
}
