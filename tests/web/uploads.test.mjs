import test from 'node:test';
import assert from 'node:assert/strict';
import {createUploadManager, PendingUploadBatch} from '../../web/src/features/workspace/uploads.ts';
import {createRequester} from '../../web/src/lib/api/request.ts';

const files=()=>[new File(['A'],'a.md',{type:'text/markdown'}),new File(['B'],'b.md',{type:'text/markdown'})];
for(const failure of ['before-create','before-complete','after-complete']){
 for(const retry of ['whole','subset','resume']){
  test(`batch retains confirmed files after ${failure}, ${retry} retry confirms only missing results`,async t=>{
   const effects=new Map(),uploads=[],versions=[],keys=[];let fault=true;
   t.mock.method(globalThis,'fetch',async(url,options)=>{
    const key=options.headers.get('Idempotency-Key'),body=options.body instanceof ArrayBuffer?null:JSON.parse(options.body);
    keys.push([url,key]);const cached=effects.get(key);if(cached)return Response.json({data:cached});
    const second=body?.name==='b.md'||url.includes('/upload-1/');
    if(fault&&second&&((failure==='before-create'&&body?.name)||(failure==='before-complete'&&url.endsWith('/complete')))){
     fault=false;throw Error('connection lost before server commit');
    }
    let data={};
    if(url.endsWith('/uploads')){data={id:`upload-${uploads.length}`,partSize:8,partCount:1};uploads.push(data);}
    if(url.endsWith('/complete')){data={id:`version-${versions.length}`};versions.push(data);}
    effects.set(key,data);
    if(fault&&second&&failure==='after-complete'&&url.endsWith('/complete')){fault=false;return new Response('{"data":');}
    return Response.json({data});
   });
   const manager=createUploadManager(createRequester('',()=>{}).request);
   await assert.rejects(manager.upload('room',files()));
   assert.deepEqual(manager.status('room').items.map(i=>i.confirmed),[true,false]);
   assert.equal(manager.status('room').running,false);
   if(retry==='resume')assert.equal(await manager.resume('room'),true);
   else assert.equal(await manager.upload('room',retry==='subset'?[files()[1]]:files()),true);
   assert.equal(uploads.length,2);assert.equal(versions.length,2);assert.equal(manager.status('room'),null);
   // A later successful selection is a genuine new intent, not hash dedupe.
   await manager.upload('room',[files()[0]]);
   assert.equal(uploads.length,3);assert.equal(versions.length,3);
   const completeA=keys.filter(([url])=>url.includes('/upload-0/complete'));
   assert.equal(completeA.length,1);
  });
 }
}

test('pending batch retains successful subset, rejects different selection, explicit close starts a new intent',async()=>{
 const effects=new Map();let fail=true,created=0;
 const request=async(path,options)=>{
  const key=options.idempotencyKey;
  if(effects.has(key))return effects.get(key);
  const body=typeof options.body==='string'?JSON.parse(options.body):{};
  if(body.name==='b.md'&&fail){fail=false;throw Error('offline');}
  const result=path.endsWith('/uploads')?{id:String(++created),partCount:0,partSize:8}:{id:'version'};
  effects.set(key,result);return result;
 };
 const manager=createUploadManager(request);
 await assert.rejects(manager.upload('one',files()));
 assert.equal(await manager.upload('one',[files()[0]]),false);
 assert.equal(created,1);assert.ok(manager.status('one'));
 await assert.rejects(manager.upload('one',[new File(['C'],'c.md')]),PendingUploadBatch);
 manager.close('one');await manager.upload('one',[files()[0]]);assert.equal(created,2);
 await manager.upload('two',[files()[0]]);assert.equal(created,3);
});
