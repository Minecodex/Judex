import test from 'node:test';
import assert from 'node:assert/strict';
import {materialDate,formatLabel,materialEvidence} from '../../web/src/features/materials/materialModel.ts';
import {initialConversationContext,conversationContextValue,conversationContextFromValue} from '../../web/src/features/chat/conversationDraft.ts';
import {conversationTabDestination} from '../../web/src/features/chat/conversationTabs.ts';
test('missing historical dates stay unrecorded, and sharing preserves the fixed uploader and version',()=>{
  for(const value of [null,undefined,'','not-a-date'])assert.equal(materialDate(value,'zh-CN','未记录'),'未记录');
  const file={versionId:'original-version',title:'共享记录.md',authorName:'实际上传者',uploadedAt:'2026-10-09T00:00:00Z',mime:'text/markdown'};
  const result=materialEvidence(file);assert.equal(result.versionId,'original-version');assert.equal(result.author,'实际上传者');assert.equal(result.name,file.title);
  assert.equal(formatLabel({title:'示例.png',format:'image'}),'PNG');assert.equal(formatLabel({title:'资料.zip',format:'archive'}),'ZIP');
});

test('draft work context survives an entry task, including explicit whole-discussion and plan choices',()=>{
  const key='project:actor:topic',cache=new Map(),route={scopeTaskId:'task-A',taskContextId:'task-A'};
  assert.deepEqual(initialConversationContext(key,cache,route),{kind:'task',id:'task-A'});
  for(const context of [{kind:'task',id:'task-B'},{kind:'plan',id:'plan-B'},{kind:'topic'}]){
    cache.set(key,{body:'draft',files:[],attachments:true,context});
    assert.deepEqual(initialConversationContext(key,cache,route),context);
    assert.deepEqual(conversationContextFromValue(conversationContextValue(context)),context);
  }
  const destination=conversationTabDestination({kind:'topic',id:'discussion',scopeTaskId:'task-A'},{view:'resources'},'task-B');
  assert.equal(destination.view,'resources');assert.equal(destination.taskContextId,'task-B');assert.equal(destination.scopeTaskId,'task-A');assert.equal(destination.conversation,'discussion');
});
