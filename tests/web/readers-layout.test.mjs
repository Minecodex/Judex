import test from 'node:test';import assert from 'node:assert/strict';
import {resizePair,defaultLayout,arrange,remember} from '../../web/src/features/workspace/layoutStore.ts';
import {readerKind,formats} from '../../web/src/features/workspace/readers/formats.ts';
test('pane drag preserves the adjacent total and respects the collapse threshold',()=>{
 const min={list:200,chat:300,reader:250},visible=['list','chat','reader'];
 const resized=resizePair(defaultLayout,'list','chat',100,1400,visible,min);assert.equal(resized.layout.weights.reader,35);assert.equal(resized.layout.weights.list+resized.layout.weights.chat,65);assert.ok(resized.layout.weights.list>22);assert.equal(resized.collapse,undefined);
 const hidden=resizePair(defaultLayout,'list','chat',-300,1400,visible,min);assert.equal(hidden.collapse,'list');assert.ok(hidden.layout.weights.list>0);
 const read=resizePair(defaultLayout,'chat','reader',600,1400,visible,min);assert.equal(read.collapse,'reader');assert.ok(read.layout.weights.chat>0);
 assert.deepEqual(defaultLayout.weights,{list:22,chat:43,reader:35});
 const two=arrange(defaultLayout,['list','chat']);assert.equal(two.weights.list,22);assert.equal(two.weights.chat,78);
 const remembered=remember(defaultLayout,{...two,weights:{...two.weights,list:28,chat:72}},['list','chat']);
 assert.equal(arrange(remembered,['list','chat']).weights.list,28);assert.equal(arrange(remembered,['list','chat','reader']).weights.list,22);
});
test('format registry chooses purpose-built readers and honours converted/unsupported content',()=>{
 assert.equal(readerKind('budget.CSV','text'),'table');assert.equal(readerKind('book.xlsx','pdf'),'workbook');assert.equal(readerKind('slides.pptx','pdf'),'pdf');assert.equal(readerKind('bundle.zip','html'),'html');assert.equal(readerKind('malformed.png','unsupported'),'unsupported');assert.equal(readerKind('flow.mmd','text'),'diagram');assert.equal(readerKind('notes.markdown','text'),'markdown');assert.equal(readerKind('archive.7z','archive'),'archive');assert.ok(new Set(Object.values(formats).flat()).size>=60);
});
