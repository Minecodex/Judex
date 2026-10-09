import type { Result, Topic, WorkState } from "../work/types.ts";
import { words } from "../work/types.ts";
import { uid } from "../work/seed.ts";
import {member,canWork,blockers,ownsSeat} from "../work/selectors.ts";
import type { ForkTopicInput, ResolveSuggestionInput, TaskActivityInput, TaskActivity } from "./collaborationTypes";

// Only the explicit demo data source uses these simulated records and results.
export function ensureDemoMainTopics(state: WorkState): WorkState {
  const s = structuredClone(state);
  s.taskActivities ??= []; s.discussionSuggestions ??= [];
  for (const plan of s.plans) {
    if (plan.mainTopicId && s.topics.some(t => t.id === plan.mainTopicId)) continue;
    plan.mainTopicId = "main:" + plan.id;
    s.topics.push({id:plan.mainTopicId,projectId:plan.projectId,title:plan.title,kind:"discussion",mainPlanId:plan.id,planIds:[plan.id],taskIds:[],messages:[]});
  }
  return s;
}
export function demoHistory(s: WorkState, topic: Topic, ceiling = Infinity, seen = new Set<string>()): Topic["messages"] {
  if (seen.has(topic.id)) return [];
  seen.add(topic.id);
  const parent = s.topics.find(t => t.id === topic.parentTopicId);
  const inherited = parent ? demoHistory(s,parent,Math.min(ceiling,topic.forkAfterSeq ?? 0),seen).map(m=>({...m,inherited:true,originTopicId:m.originTopicId??parent.id})) : [];
  const own = topic.messages.map((m,i)=>({...m,seq:m.seq??(topic.forkAfterSeq??0)+i+1,originTopicId:m.originTopicId??topic.id,inherited:false})).filter(m=>m.seq<=ceiling);
  return [...inherited,...own];
}
export function demoFork(s: WorkState,p: ForkTopicInput): Result & {id?:string} {
  const parent=s.topics.find(t=>t.id===p.topicId);
  if (!parent||!member(s,parent.projectId)) return {error:"permission"};
  const history=demoHistory(s,parent), point=p.forkAfterSeq??history.at(-1)?.seq??0;
  if (point<0 || (point>0&&!history.some(m=>m.seq===point))) return {error:"scope"};
  const next=structuredClone(s), id=uid();
  next.topics.push({id,projectId:parent.projectId,title:words(p.title),kind:"discussion",parentTopicId:parent.id,forkAfterSeq:point,sourceRefs:p.sourceRefs??[],planIds:p.planIds,taskIds:p.taskIds,messages:[]});
  return {state:next,id};
}
export function demoTaskActivity(s: WorkState,p: TaskActivityInput): Result & {id?:string} {
  const task=s.tasks.find(t=>t.id===p.taskId);
  if (!task||!member(s,task.projectId)) return {error:"permission"};
  if(task.discardedAt||(task.executionException&&(p.kind==='progress'||p.kind==='delivery')))return {error:'blocked'};
  if((p.kind==="progress"||p.kind==="delivery")&&!canWork(s,task))return {error:"permission"};
  if((p.kind==="progress"||p.kind==="delivery")&&p.expectedTaskVersion!==undefined&&p.expectedTaskVersion!==task.revision)return {error:"stale"};
  if((p.kind==="progress"||p.kind==="delivery")){
    const own=task.seatIds.filter(id=>ownsSeat(s,id));
    if(p.identityId?!own.includes(p.identityId):own.length!==1)return {error:"permission"};
  }
  if (p.kind==="progress"&&!["working","rework"].includes(task.status)) return {error:"accepted"};
  if (p.kind==="delivery"&&!["ready","working","rework"].includes(task.status)) return {error:"accepted"};
  if(p.kind==="delivery"&&blockers(s,task,"start").length)return {error:"blocked"};
  const next=ensureDemoMainTopics(s),id=uid(),now=new Date().toISOString();
  const analysisId=uid(), sourceType=p.kind==="progress"||p.kind==="delivery"?"report":"submission";
  const activity:TaskActivity={
    id,taskId:task.id,kind:p.kind,sourceType,source:p.simulatedLocal?"cli":"web",text:p.body,
    actorUserId:null,actorName:s.currentUser,identityId:null,createdAt:now,
    materialVersionIds:p.files.map(f=>f.id),materials:p.files.map(f=>({versionId:f.id,name:f.name})),topicIds:p.topicId?[p.topicId]:[],
    analysis:{id:analysisId,taskId:task.id,batchId:null,sourceType,sourceId:id,state:"completed",summary:p.kind==="question"?"已整理待协商的问题。":p.kind==="delivery"?"交付已登记，最终验收仍待确认。":"本次更新已记录。",basis:[],disagreements:[],errorCode:null,updatedAt:now},
  };
  next.taskActivities!.push(activity);
  if(p.kind==="progress")next.tasks.find(t=>t.id===task.id)!.revision++;
  if(p.kind==="delivery"){const target=next.tasks.find(t=>t.id===task.id)!;target.status="delivered";target.revision++;target.files=[...p.files,{id,name:"交付记录",text:p.body,author:s.currentUser,at:Date.parse(now)}];}
  if(p.kind==="question"){
    const plan=next.plans.find(v=>v.id===task.planId),parent=next.topics.find(t=>t.id===plan?.mainTopicId);
    const existing=next.topics.find(t=>t.id!==plan?.mainTopicId&&t.taskIds.includes(task.id));
    next.discussionSuggestions!.push({id:uid(),analysisId,taskId:task.id,planId:task.planId,title:task.title.zh+"问题讨论",reason:"需要相关职责持续核对证据与处理方案。",parentTopicId:parent?.id??null,forkAfterSeq:parent?demoHistory(next,parent).at(-1)?.seq??0:0,suggestedTopicId:existing?.id??null,state:"pending",version:1,resultTopicId:null,sourceRef:{type:sourceType,id},createdAt:now});
  }
  if(p.kind==="reply"&&p.topicId){
    const topic=next.topics.find(t=>t.id===p.topicId);
    if(!topic||topic.projectId!==task.projectId)return {error:"scope"};
    topic.messages.push({id:uid(),actor:s.currentUser,kind:"person",text:words(p.body),at:Date.now(),taskId:task.id,sourceRef:{type:sourceType,id},seq:(demoHistory(next,topic).at(-1)?.seq??0)+1});
    topic.messages.push({id:uid(),actor:"Judex",kind:"ai",text:words(activity.analysis!.summary),at:Date.now(),taskId:task.id,seq:(demoHistory(next,topic).at(-1)?.seq??0)+1});
  }
  return {state:next,id};
}
export function demoResolve(s:WorkState,p:ResolveSuggestionInput):Result & {id?:string}{
  const suggestion=s.discussionSuggestions?.find(v=>v.id===p.suggestionId),task=s.tasks.find(t=>t.id===suggestion?.taskId);
  if(!suggestion||!task||!member(s,task.projectId))return {error:"permission"};
  if(suggestion.state!=="pending")return {state:s,id:suggestion.resultTopicId??undefined};
  if(p.expectedVersion!==suggestion.version)return {error:"stale"};
  let next=structuredClone(s),id:string|undefined;
  if(p.mode==="create"){
    const planIds=[...new Set([...(p.planIds??[]),...(task.planId?[task.planId]:[])])];
    if(suggestion.parentTopicId){
      const r=demoFork(next,{topicId:suggestion.parentTopicId,title:p.title||suggestion.title,forkAfterSeq:suggestion.forkAfterSeq,planIds,taskIds:[task.id,...(p.taskIds??[])],sourceRefs:[suggestion.sourceRef]});
      if(r.error)return r;next=r.state!;id=r.id;
    }else{id=uid();next.topics.push({id,projectId:task.projectId,title:words(p.title||suggestion.title),kind:"discussion",planIds,taskIds:[...new Set([task.id,...(p.taskIds??[])])],sourceRefs:[suggestion.sourceRef],messages:[]});}
  }else if(p.mode==="main")id=next.plans.find(v=>v.id===task.planId)?.mainTopicId;
  else if(p.mode==="link")id=p.topicId;
  if(p.mode!=="dismiss"){
    const topic=next.topics.find(t=>t.id===id);if(!topic||topic.projectId!==task.projectId)return {error:"scope"};
    const plans=[...new Set([...topic.planIds,...(task.planId?[task.planId]:[]),...(p.planIds??[])])],tasks=[...new Set([...topic.taskIds,task.id,...(p.taskIds??[])])];
    if(plans.some(id=>!next.plans.some(v=>v.id===id&&v.projectId===task.projectId))||tasks.some(id=>!next.tasks.some(v=>v.id===id&&v.projectId===task.projectId)))return {error:"scope"};
    if(p.mode!=="create"&&(plans.length!==topic.planIds.length||tasks.length!==topic.taskIds.length))topic.linksVersion=(topic.linksVersion??1)+1;
    topic.planIds=plans;topic.taskIds=tasks;
    const source={...suggestion.sourceRef,taskId:task.id};
    topic.sourceRefs=[...new Map([...(topic.sourceRefs??[]),source].map(ref=>[ref.type+":"+ref.id,{...ref,taskId:ref.taskId??task.id}])).values()];
    const activity=next.taskActivities?.find(a=>a.id===suggestion.sourceRef.id);if(activity)activity.topicIds=[...new Set([...activity.topicIds,topic.id])];
  }
  const changed=next.discussionSuggestions!.find(v=>v.id===suggestion.id)!;
  changed.state=p.mode==="dismiss"?"dismissed":"handled";changed.resultTopicId=id??null;changed.version++;
  return {state:next,id};
}
