import test from 'node:test';import assert from 'node:assert/strict';import {graphSource} from '../../web/src/features/workspace/workflowGraph.ts';
test('workflow graph preserves Chinese labels, branches and feedback without raw Mermaid injection',()=>{
 const source=graphSource({nodes:[{id:'outside\nclick n0 "https://evil.example"',name:'收到需求',kind:'start'},{id:'b',name:'是否通过"\n<script>',kind:'decision'}],edges:[{from:'outside\nclick n0 "https://evil.example"',to:'b',kind:'sequence',label:'进入评审'},{from:'b',to:'outside\nclick n0 "https://evil.example"',kind:'feedback',label:'退回|补充'},{from:'missing',to:'b',label:'bad'}]});
 assert.match(source,/n0\(\["收到需求"\]\)/);assert.match(source,/n1\{"是否通过＂ ＜script＞"\}/);assert.match(source,/n1 -\.->\|"退回｜补充"\| n0/);assert.doesNotMatch(source,/https:|<script>|click|missing|bad|&#/);assert.equal(source.split('\n').length,5);
});
