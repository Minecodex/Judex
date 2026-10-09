import test from "node:test";
import assert from "node:assert/strict";
import {parseCooperationRoute,cooperationURL,projectFromURL} from "../../web/src/features/cooperation/routing.ts";
import {effectivePlanIds,topicInScope,discussionTasks} from "../../web/src/features/cooperation/scope.ts";
import {demoTaskMain,demoReplaceLinks} from "../../web/src/features/cooperation/demo.ts";
import {seedWork} from "../../web/src/features/work/seed.ts";
import {ensureDemoMainTopics,demoTaskActivity} from "../../web/src/features/chat/demoCollaboration.ts";
import {openTab} from "../../web/src/features/chat/conversationTabs.ts";
import {member,ownsSeat} from "../../web/src/features/work/selectors.ts";

test("canonical routes preserve scope independently from inspector and input task",()=>{
 const route={design:"studio",projectId:"alpha",page:"chat",view:"task",id:"task-b",conversation:"shared",scopePlanId:"plan-a",taskContextId:"task-a",activityId:"old-report",messageSeq:3};
 const value=new URL(cooperationURL(route),"https://judex.test");const parsed=parseCooperationRoute(value.pathname,value.search);
 assert.equal(parsed.scopePlanId,"plan-a");assert.equal(parsed.conversation,"shared");assert.equal(parsed.view,"task");assert.equal(parsed.id,"task-b");
 assert.equal(value.searchParams.get("taskContext"),"task-a");assert.equal(value.searchParams.get("activity"),"old-report");
 assert.equal(projectFromURL(value.pathname,value.search),"alpha");
 const settingsURL=new URL(cooperationURL({...route,settingsSection:"team"}),value);const settings=parseCooperationRoute(settingsURL.pathname,settingsURL.search);
 assert.equal(settings.page,"chat");assert.equal(settings.scopePlanId,"plan-a");assert.equal(settings.settingsSection,"team");
});
test("project cards lead to hub, explicit legacy discussion remains a discussion",()=>{
 assert.equal(parseCooperationRoute("/","?project=alpha").page,"hub");
 assert.equal(parseCooperationRoute("/","?project=alpha&chat=shared").page,"chat");
 assert.equal(parseCooperationRoute("/","").page,"projects");
 assert.equal(cooperationURL({design:"studio",projectId:"alpha",page:"projects",view:"plans"}),"/");
 assert.match(cooperationURL({design:"studio",projectId:"alpha",page:"projects",view:"plans",settingsSection:"project"}),/^\/projects\/alpha\/settings\/project/);
});
test("task discussion is created only by explicit get-or-create, reporting does not create it",()=>{
 let state=ensureDemoMainTopics(seedWork());state.currentUser="顾言";state.tasks.find(t=>t.id==="build").status="working";
 const count=state.topics.length;state=demoTaskActivity(state,{taskId:"build",kind:"progress",body:"Another update",files:[]}).state;
 assert.equal(state.topics.length,count);assert.equal(state.tasks.find(t=>t.id==="build").mainTopicId,undefined);
 const first=demoTaskMain(state,"build");const second=demoTaskMain(first.state,"build");
 assert.equal(first.id,second.id);assert.equal(second.state.topics.length,count+1);assert.equal(second.state.tasks.find(t=>t.id==="build").status,"working");
});
test("one topic is visible through multiple scopes without changing task ownership",()=>{
 const state=ensureDemoMainTopics(seedWork()),task=state.tasks.find(t=>t.id==="build");const second=state.plans.find(p=>p.id!==task.planId&&p.projectId===task.projectId);
 const topic={planIds:[task.planId,second.id],taskIds:[task.id]};const refs=effectivePlanIds(topic,state.tasks);
 assert.equal(refs.filter(id=>id===task.planId).length,1);assert(topicInScope(topic,{scopePlanId:second.id},state.tasks));assert(topicInScope(topic,{scopeTaskId:task.id},state.tasks));assert.equal(task.planId,"leaf-first");
});
test("association revisions conflict and required main links remain pinned",()=>{
 let state=ensureDemoMainTopics(seedWork());const result=demoTaskMain(state,"build");state=result.state;
 const base={topicId:result.id,expectedLinksVersion:1,planIds:[],taskIds:[]};assert.equal(demoReplaceLinks(state,base).error,"scope");
 const edited=demoReplaceLinks(state,{...base,taskIds:["build"]});assert.equal(edited.state.topics.find(t=>t.id===result.id).linksVersion,2);
 assert.equal(demoReplaceLinks(edited.state,{...base,taskIds:["build"]}).error,"stale");
});
test("reopening the same topic records the explicit new entry scope without another tab",()=>{
 const before=[{kind:"topic",id:"shared",scopePlanId:"plan-a"}];const after=openTab(before,{kind:"topic",id:"shared",scopePlanId:"plan-b",scopeTaskId:undefined});
 assert.equal(after.length,1);assert.equal(after[0].scopePlanId,"plan-b");assert.equal(before[0].scopePlanId,"plan-a");
});
test("equal display names cannot grant another user's project or responsibility",()=>{
 const state=seedWork();state.currentUser="Same name";state.currentUserId="first";state.projects[0].members=[{name:"Same name",userId:"second",role:"owner"}];state.seats[0]={...state.seats[0],person:"Same name",userId:"second"};
 assert.equal(member(state,state.projects[0].id),undefined);assert.equal(ownsSeat(state,state.seats[0].id),false);
});
test("explicit tasks do not bring every task in the parent plan into message context",()=>{
 const state=seedWork(),task=state.tasks.find(v=>v.id==="build");assert.deepEqual(discussionTasks({projectId:"leaf",planIds:[],taskIds:[task.id]},state.tasks).map(v=>v.id),[task.id]);
 const topics={projectId:"leaf",planIds:["leaf-first"],taskIds:["brand-copy"]};assert(discussionTasks(topics,state.tasks).every(v=>v.projectId==="leaf"));
});
