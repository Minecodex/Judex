// Fault injection exists only in this loopback test proxy, never in product code.
import http from 'node:http';
import {Transform} from 'node:stream';
import {StringDecoder} from 'node:string_decoder';

export async function startBrowserProxy(target,port=0){
 const upstream=new URL(target),faults=new Map(),corruptions=new Map(),writeFaults=new Map(),sockets=new Set(),records=[];let hideConfig=false,hiddenEvents=0;
 const server=http.createServer((req,res)=>{
  const path=new URL(req.url,'http://localhost').pathname,status=req.method==='GET'&&faults.get(path);
  if(status){res.writeHead(status,{'Content-Type':'application/json'});res.end(JSON.stringify({error:{code:'DEPENDENCY_DOWN',message:'Controlled browser test failure'},requestId:'browser-fixture'}));return;}
  const headers={...req.headers,host:upstream.host};
  if(headers.origin===`http://localhost:${server.address().port}`||headers.origin===`http://127.0.0.1:${server.address().port}`)headers.origin=upstream.origin;
  // Audit only synthetic chat writes; never retain auth bodies or cookies.
  const audited=req.method==='POST'&&(/^\/api\/v1\/conversations\/[^/]+\/(messages|uploads|uploads\/[^/]+\/complete)$/.test(path)||/^\/api\/v1\/groups\/[^/]+\/push$/.test(path)),record=audited?{path,key:headers['idempotency-key'],body:''}:null;
  if(record){records.push(record);req.on('data',part=>record.body+=part);}
  const wildcard=path.replace(/\/uploads\/[^/]+\/complete$/,'/uploads/*/complete'),writeKey=writeFaults.has(path)?path:wildcard,writeFault=audited&&writeFaults.get(writeKey);
  if(writeFault&&--writeFault.remaining===0){writeFaults.delete(writeKey);record.status=503;record.beforeCommit=true;req.resume();res.writeHead(503,{'Content-Type':'application/json'});res.end(JSON.stringify({error:{code:'DEPENDENCY_DOWN',message:'Controlled pre-commit failure'}}));return;}
  const out=http.request(upstream.origin+req.url,{method:req.method,headers},incoming=>{
   if(audited){const parts=[];incoming.on('data',part=>parts.push(part));incoming.on('end',()=>{
    record.status=incoming.statusCode;record.upstreamBody=Buffer.concat(parts).toString();
    const faultKey=corruptions.has(path)?path:wildcard,scheduled=incoming.statusCode>=200&&incoming.statusCode<300?corruptions.get(faultKey):undefined,bad=scheduled&&--scheduled.remaining===0?scheduled.mode:undefined;if(bad)corruptions.delete(faultKey);
    const replacement=bad&&incoming.statusCode<300?(bad==='envelope'?'{}':'{"data":'):record.upstreamBody;
    const responseHeaders={...incoming.headers};delete responseHeaders['content-length'];delete responseHeaders['transfer-encoding'];res.writeHead(incoming.statusCode,responseHeaders);res.end(replacement);record.corrupted=Boolean(bad);
   });return;}
   res.writeHead(incoming.statusCode,incoming.headers);
   if(incoming.headers['content-type']?.includes('text/event-stream')){
    let buffer='';const decoder=new StringDecoder('utf8');incoming.pipe(new Transform({transform(chunk,_encoding,done){buffer+=decoder.write(chunk);let end;while((end=buffer.indexOf('\n\n'))>=0){const frame=buffer.slice(0,end+2);buffer=buffer.slice(end+2);if(hideConfig&&/^event: (project.changed|personal-prompt.changed)$/m.test(frame))hiddenEvents++;else this.push(frame);}done();}})).pipe(res);
   }else incoming.pipe(res);
  });
  out.on('error',()=>{if(!res.headersSent)res.writeHead(502,{'Content-Type':'application/json'});res.end(JSON.stringify({error:{code:'DEPENDENCY_DOWN',message:'Test upstream unavailable'}}));});
  res.on('close',()=>out.destroy());req.pipe(out);
 });
 server.on('connection',socket=>{sockets.add(socket);socket.on('close',()=>sockets.delete(socket));});
 await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(port,'127.0.0.1',resolve);});
 return {url:`http://localhost:${server.address().port}`,peerURL:`http://127.0.0.1:${server.address().port}`,records,corruptOnce:(path,mode='truncated')=>corruptions.set(path,{remaining:1,mode}),corruptNth:(path,n,mode='truncated')=>corruptions.set(path,{remaining:n,mode}),failWriteNth:(path,n)=>writeFaults.set(path,{remaining:n}),hideConfiguration:value=>{hideConfig=value;},hiddenCount:()=>hiddenEvents,fail:(path,status=503)=>faults.set(path,status),restore:path=>faults.delete(path),close:()=>new Promise(resolve=>{for(const socket of sockets)socket.destroy();server.close(resolve);})};
}
