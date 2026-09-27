import test from 'node:test';import assert from 'node:assert/strict';
import {HOME_TAB,openTab,closeTab,tabKey,routeTab} from '../../web/src/features/chat/conversationTabs.ts';
test('tabs deduplicate by type and id, closing an inactive tab keeps the active conversation',()=>{
 let tabs=[HOME_TAB];tabs=openTab(tabs,{kind:'topic',id:'same'});tabs=openTab(tabs,{kind:'topic',id:'same'});tabs=openTab(tabs,{kind:'handoff',id:'same'});assert.equal(tabs.length,3);
 const result=closeTab(tabs,'topic:same','handoff:same');assert.equal(result.active,'handoff:same');assert.equal(result.tabs.length,2);assert.equal(closeTab(result.tabs,'handoff:same','handoff:same').active,'home');
});
test('right workspace routes keep typed handoff context separate from topic ids',()=>{
 const base={design:'studio',projectId:'p',view:'tasks'};assert.deepEqual(routeTab({...base,conversation:'handoff:h'}),{kind:'handoff',id:'h'});assert.equal(tabKey(routeTab({...base,conversation:'t'})),'topic:t');assert.equal(tabKey(routeTab({...base,conversation:undefined})),'home');
});
