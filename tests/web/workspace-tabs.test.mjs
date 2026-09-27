import test from 'node:test';import assert from 'node:assert/strict';
import {addWorkspaceTab,removeWorkspaceTab,workspaceKey,workspaceRoute,workspaceTabValid} from '../../web/src/features/chat/workspaceTabModel.ts';
import {seedWork} from '../../web/src/features/work/seed.ts';
test('tools deduplicate while task details stay independent and closing preserves the neighbor',()=>{
 let tabs=[];for(const t of [{view:'plans'},{view:'resources'},{view:'overview'},{view:'task',id:'build'},{view:'task',id:'guide'},{view:'plans'}])tabs=addWorkspaceTab(tabs,t);
 assert.equal(tabs.length,5);assert.equal(workspaceKey(workspaceRoute({view:'tasks'})),'plans');
 assert.equal(removeWorkspaceTab(tabs,'resources','task:guide').active,'task:guide');assert.equal(removeWorkspaceTab([{view:'plans'}],'plans','plans').active,null);
});
test('persisted details cannot reference another project and configuration no longer opens as work tabs',()=>{
 const s=seedWork();assert.equal(workspaceTabValid(s,'leaf',{view:'task',id:'brand-copy'}),false);assert.equal(workspaceTabValid(s,'leaf',{view:'settings',id:'unknown'}),false);
 for(const view of ['settings','team','flows'])assert.equal(workspaceRoute({view}),null);
});
