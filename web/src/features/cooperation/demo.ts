import type {ActionPayloads} from "../work/storeTypes";
import {words,type Result,type WorkState} from "../work/types.ts";
import {member} from "../work/selectors.ts";
export function demoTaskMain(s:WorkState,taskId:string):Result&{id?:string}{
 const task=s.tasks.find(t=>t.id===taskId);if(!task||!member(s,task.projectId))return {error:"permission"};
 if(task.mainTopicId)return {state:s,id:task.mainTopicId};
 const id="task-main-"+task.id;
 return {state:{...s,tasks:s.tasks.map(t=>t.id===task.id?{...t,mainTopicId:id}:t),topics:[...s.topics,{id,projectId:task.projectId,title:words(typeof task.title==="string"?task.title:task.title.zh,typeof task.title==="string"?task.title:task.title.en),kind:"discussion",linksVersion:1,planIds:[],taskIds:[task.id],messages:[]}]},id};
}
export function demoReplaceLinks(s:WorkState,p:ActionPayloads["replaceTopicLinks"]):Result&{id?:string}{
 const topic=s.topics.find(t=>t.id===p.topicId);if(!topic||!member(s,topic.projectId))return {error:"permission"};
 if((topic.linksVersion??1)!==p.expectedLinksVersion)return {error:"stale"};
 if(p.planIds.some(id=>!s.plans.some(v=>v.id===id&&v.projectId===topic.projectId))||p.taskIds.some(id=>!s.tasks.some(v=>v.id===id&&v.projectId===topic.projectId)))return {error:"scope"};
 if(s.plans.some(v=>v.mainTopicId===topic.id&&!p.planIds.includes(v.id))||s.tasks.some(v=>v.mainTopicId===topic.id&&!p.taskIds.includes(v.id)))return {error:"scope"};
 return {state:{...s,topics:s.topics.map(t=>t.id===topic.id?{...t,planIds:[...new Set(p.planIds)],taskIds:[...new Set(p.taskIds)],linksVersion:(t.linksVersion??1)+1}:t)},id:topic.id};
}
