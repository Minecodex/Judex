import {history,id,planTasks,ready,requiredAcceptance,stamp,W,type Activity,type Decision,type State,type Task,type Text,type Topic} from "./model";
import type {CooperationKey} from "../src/i18n/cooperationPreview";
export type Action=
 |{kind:"user";userId:string}
 |{kind:"ensure-task";taskId:string}
 |{kind:"message";topicId:string;body:string;taskId?:string}
 |{kind:"report";taskId:string;recordKind:Activity["kind"];body:string;source:"web"|"cli";topicId?:string}
 |{kind:"topic";projectId:string;title:string;planIds:string[];taskIds:string[];parentId?:string;forkAfterSeq?:number;recordId?:string;topicId?:string}
 |{kind:"suggestion";suggestionId:string;mode:"create"|"main"|"existing";title?:string;topicId?:string}
 |{kind:"decision";decisionId:string;version:number;approve:boolean;reason:string}
 |{kind:"receipt";handoffId:string;version:number;accept:boolean;reason:string}
 |{kind:"start";taskId:string}
 |{kind:"reopen";taskId:string;reason:string}
 |{kind:"arrangement";projectId:string;sourceTopicId:string;title:string;goal:string;taskTitle:string}
 |{kind:"project";title:string;goal:string}
 |{kind:"save-project";projectId:string;title:string;goal:string}
 |{kind:"save-position";positionId:string;prompt:string}
 |{kind:"preference";positionId:string;prompt:string}
 |{kind:"invite";projectId:string;name:string;positionId:string};
export type Result={state?:State;id?:string;notice?:CooperationKey;error?:CooperationKey};
const taskStatusDecision=(s:State,t:Task)=>{
 s.decisions=s.decisions.filter(d=>!(d.taskId===t.id&&d.status==="pending"));
 s.decisions.push({id:id(),projectId:t.projectId,kind:"task",title:t.title,description:W("核对本次固定交付与当前完成条件。","Review this fixed delivery and current criteria."),planId:t.planId,taskId:t.id,sourceTopicId:t.mainTopicId??s.plans.find(p=>p.id===t.planId)!.mainTopicId,version:t.version,status:"pending",actors:[t.reviewerId],approvedBy:[]});
};
function updateReady(s:State){
 for(const t of s.tasks)if(["ready","blocked"].includes(t.status))t.status=ready(s,t)?"ready":"blocked";
 for(const p of s.plans)if(p.status==="active"&&requiredAcceptance(s,p)&&!s.decisions.some(d=>d.planId===p.id&&d.kind==="plan"&&d.status==="pending"))
  s.decisions.push({id:id(),projectId:p.projectId,kind:"plan",title:p.title,description:W("核对已验收任务与计划整体目标。","Review accepted tasks and the whole plan goal."),planId:p.id,sourceTopicId:p.mainTopicId,version:1,status:"pending",actors:[p.ownerId],approvedBy:[]});
}
function append(s:State,topic:Topic,authorId:string,body:Text,taskId?:string,recordId?:string,ai=false){
 topic.messages.push({id:id(),seq:(history(s,topic).at(-1)?.seq??0)+1,authorId,kind:ai?"ai":"person",at:stamp(),text:body,taskId,recordId});
}
function createTopic(s:State,a:Extract<Action,{kind:"topic"}>):Result{
 const tasks=s.tasks.filter(t=>a.taskIds.includes(t.id));
 const plans=[...new Set([...a.planIds,...tasks.map(t=>t.planId)])];
 if(!a.title.trim()||!plans.length||tasks.length!==a.taskIds.length||tasks.some(t=>t.projectId!==a.projectId)||plans.some(pid=>!s.plans.some(p=>p.id===pid&&p.projectId===a.projectId)))return {error:"cpNoPermission"};
 const existing=s.topics.find(t=>t.id===a.topicId);
 if(existing&&existing.projectId!==a.projectId)return {error:"cpNoPermission"};
 if(existing){existing.planIds=plans;existing.taskIds=a.taskIds;existing.title=W(a.title);return {state:s,id:existing.id};}
 const parent=s.topics.find(t=>t.id===a.parentId);
 if(a.parentId&&(!parent||parent.projectId!==a.projectId))return {error:"cpNoPermission"};
 const point=a.forkAfterSeq??(parent?history(s,parent).at(-1)?.seq??0:0);
 if(parent&&point>0&&!history(s,parent).some(m=>m.seq===point))return {error:"cpFreshReview"};
 const topic:Topic={id:id(),projectId:a.projectId,title:W(a.title),planIds:plans,taskIds:a.taskIds,kind:"special",messages:[],parentId:parent?.id,forkAfterSeq:point,recordIds:a.recordId?[a.recordId]:[]};
 s.topics.push(topic);return {state:s,id:topic.id};
}
export function apply(current:State,a:Action):Result{
 const s=structuredClone(current),actor=s.userId;
 if(a.kind==="user"){s.userId=a.userId;return {state:s};}
 if(a.kind==="project"){
  const pid=id();s.projects.push({id:pid,title:W(a.title),goal:W(a.goal),ownerId:actor,memberIds:[actor],status:"active"});return {state:s,id:pid};
 }
 if(a.kind==="save-project"){
  const p=s.projects.find(p=>p.id===a.projectId);if(!p||p.ownerId!==actor)return {error:"cpNoPermission"};p.title=W(a.title);p.goal=W(a.goal);return {state:s};
 }
 if(a.kind==="save-position"){const p=s.positions.find(p=>p.id===a.positionId);if(!p||s.projects.find(v=>v.id===p.projectId)?.ownerId!==actor)return {error:"cpNoPermission"};p.prompt=W(a.prompt);return {state:s};}
 if(a.kind==="preference"){if(s.people.find(p=>p.id===actor)?.positionId!==a.positionId)return {error:"cpNoPermission"};s.preferences[actor+":"+a.positionId]=a.prompt;return {state:s};}
 if(a.kind==="invite"){
  const p=s.projects.find(p=>p.id===a.projectId);if(p?.ownerId!==actor)return {error:"cpNoPermission"};
  s.invites??=[];s.invites.push({projectId:a.projectId,email:a.name,positionId:a.positionId});return {state:s,notice:"cpInviteCreated"};
 }
 if(a.kind==="ensure-task"){
  const t=s.tasks.find(t=>t.id===a.taskId);if(!t)return {error:"cpNoPermission"};
  if(t.mainTopicId)return {state:s,id:t.mainTopicId,notice:"cpAlreadyTaskTopic"};
  t.mainTopicId="task-main-"+t.id;
  s.topics.push({id:t.mainTopicId,projectId:t.projectId,title:t.title,planIds:[t.planId],taskIds:[t.id],kind:"task-main",messages:[],recordIds:[]});
  return {state:s,id:t.mainTopicId,notice:"cpFirstTaskTopic"};
 }
 if(a.kind==="topic")return createTopic(s,a);
 if(a.kind==="message"){
  const topic=s.topics.find(t=>t.id===a.topicId);if(!topic||!a.body.trim())return {error:"cpNoPermission"};
  if(a.taskId){
   return apply(current,{kind:"report",taskId:a.taskId,recordKind:"reply",body:a.body,source:"web",topicId:topic.id});
  }
  append(s,topic,actor,W(a.body));append(s,topic,"ai",W("已记录补充。可以继续核对事实；形成正式安排或验收仍需要明确决定。","Your input is recorded. Review the facts; formal arrangements and acceptance need explicit decisions."),undefined,undefined,true);
  return {state:s};
 }
 if(a.kind==="report"){
  const task=s.tasks.find(t=>t.id===a.taskId);if(!task||!a.body.trim())return {error:"cpNoPermission"};
  if(["progress","delivery"].includes(a.recordKind)&&task.makerId!==actor)return {error:"cpNoPermission"};
  if(a.recordKind==="progress"&&!["working","rework"].includes(task.status))return {error:"cpNotReady"};
  if(a.recordKind==="delivery"&&(!["working","ready","rework"].includes(task.status)||!ready(s,task)))return {error:"cpNotReady"};
  const topic=s.topics.find(t=>t.id===a.topicId);if(a.recordKind==="reply"&&(!topic||topic.projectId!==task.projectId))return {error:"cpNoPermission"};
  const record:Activity={id:id(),taskId:task.id,kind:a.recordKind,text:W(a.body),actorId:actor,source:a.source,materialIds:[],at:stamp(),topicIds:topic?[topic.id]:[],analysis:a.recordKind==="question"?W("问题与证据已整理，建议相关职责持续协商。是否建立讨论，由成员选择。","The question is recorded. Continued coordination may help; a member chooses the discussion."):a.recordKind==="delivery"?W("交付已登记，最终验收仍等待有权人确认。","Delivery registered; authorized acceptance is still required."):W("更新已记录，正式状态仍以任务和决定为准。","Update recorded; tasks and decisions remain the formal facts.")};
  s.activities.push(record);
  if(a.recordKind==="delivery"){task.status="delivered";task.version++;taskStatusDecision(s,task);}
  if(a.recordKind==="question"){
   const source=topic??s.topics.find(v=>v.id===s.plans.find(p=>p.id===task.planId)?.mainTopicId);
   s.suggestions.push({id:id(),recordId:record.id,taskId:task.id,title:W("持续核对："+task.title.zh,"Review: "+task.title.en),reason:W("需要参与职责共同核对问题与依据。","Relevant responsibilities should review the issue and evidence."),status:"pending",parentId:source?.id,forkAfterSeq:source?history(s,source).at(-1)?.seq??0:0});
  }
  if(topic){append(s,topic,actor,W(a.body),task.id,record.id);append(s,topic,"ai",record.analysis,task.id,record.id,true);}
  updateReady(s);return {state:s,id:record.id,notice:topic?undefined:"cpRecordSaved"};
 }
 if(a.kind==="suggestion"){
  const suggestion=s.suggestions.find(v=>v.id===a.suggestionId);if(!suggestion)return {error:"cpFreshReview"};
  if(suggestion.status==="handled")return {state:s,id:suggestion.resultTopicId};
  const task=s.tasks.find(t=>t.id===suggestion.taskId)!;
  let topicId=a.mode==="main"?s.plans.find(p=>p.id===task.planId)!.mainTopicId:a.topicId;
  if(a.mode==="create"){
   const result=createTopic(s,{kind:"topic",projectId:task.projectId,title:a.title??suggestion.title.zh,planIds:[task.planId],taskIds:[task.id],parentId:suggestion.parentId,forkAfterSeq:suggestion.forkAfterSeq,recordId:suggestion.recordId});
   if(result.error)return result;topicId=result.id;
  }
  const topic=s.topics.find(t=>t.id===topicId);if(!topic||topic.projectId!==task.projectId)return {error:"cpNoPermission"};
  topic.taskIds=[...new Set([...topic.taskIds,task.id])];topic.planIds=[...new Set([...topic.planIds,task.planId])];topic.recordIds=[...new Set([...topic.recordIds,suggestion.recordId])];
  s.activities.find(r=>r.id===suggestion.recordId)!.topicIds.push(topic.id);
  suggestion.status="handled";suggestion.resultTopicId=topic.id;return {state:s,id:topic.id};
 }
 if(a.kind==="start"){
  const t=s.tasks.find(v=>v.id===a.taskId);if(!t||t.makerId!==actor)return {error:"cpNoPermission"};if(!ready(s,t)||!["ready","rework"].includes(t.status))return {error:"cpNotReady"};t.status="working";t.version++;return {state:s};
 }
 if(a.kind==="reopen"){
  const t=s.tasks.find(v=>v.id===a.taskId);if(!t||t.reviewerId!==actor||t.status!=="accepted"||!a.reason.trim())return {error:"cpNoPermission"};
  t.status="rework";t.version++;return {state:s};
 }
 if(a.kind==="receipt"){
  const h=s.handoffs.find(v=>v.id===a.handoffId);if(!h||h.receiverId!==actor)return {error:"cpNoPermission"};
  if(h.status!=="pending"||h.version!==a.version)return {error:"cpFreshReview"};
  if(!a.accept&&!a.reason.trim())return {error:"cpReasonRequired"};
  h.status=a.accept?"accepted":"rejected";h.reason=a.reason;
  const source=s.tasks.find(t=>t.id===h.sourceTaskId)!;if(!a.accept&&source.status!=="accepted")source.status="rework";
  updateReady(s);return {state:s,notice:"cpDecisionDone"};
 }
 if(a.kind==="decision"){
  const d=s.decisions.find(v=>v.id===a.decisionId);if(!d||!d.actors.includes(actor)||d.approvedBy.includes(actor))return {error:"cpNoPermission"};
  if(d.status!=="pending"||d.version!==a.version)return {error:"cpFreshReview"};
  if(!a.approve&&!a.reason.trim())return {error:"cpReasonRequired"};
  if(d.kind==="task"&&s.tasks.find(t=>t.id===d.taskId)?.status!=="delivered")return {error:"cpFreshReview"};
  if(d.kind==="plan"&&!requiredAcceptance(s,s.plans.find(p=>p.id===d.planId)!))return {error:"cpNotReady"};
  if(!a.approve){d.status="rejected";d.reason=a.reason;if(d.kind==="task")s.tasks.find(t=>t.id===d.taskId)!.status="rework";return {state:s,notice:"cpDecisionDone"};}
  d.approvedBy.push(actor);
  if(d.actors.every(who=>d.approvedBy.includes(who))){
   d.status="approved";
   if(d.kind==="task"){const t=s.tasks.find(t=>t.id===d.taskId)!;t.status="accepted";t.version++;}
   if(d.kind==="plan")s.plans.find(p=>p.id===d.planId)!.status="accepted";
   if(d.kind==="arrangement"){s.plans.find(p=>p.id===d.planId)!.status="active";for(const t of planTasks(s,d.planId))if(t.status==="draft")t.status=ready(s,t)?"ready":"blocked";}
   updateReady(s);return {state:s,notice:"cpDecisionDone"};
  }
  return {state:s,notice:"cpAwaitOthers"};
 }
 if(a.kind==="arrangement"){
  const parent=s.topics.find(t=>t.id===a.sourceTopicId),project=s.projects.find(v=>v.id===a.projectId);
  if(!project?.memberIds.includes(actor)||a.sourceTopicId&&(!parent||parent.projectId!==a.projectId))return {error:"cpNoPermission"};
  const maker=project.memberIds.includes("gu")?"gu":actor;
  const planId=id(),topicId="main-"+planId;
  s.plans.push({id:planId,projectId:a.projectId,title:W(a.title),goal:W(a.goal),criteria:[W(a.goal)],ownerId:actor,status:"draft",workflowId:s.flows.find(f=>f.projectId===a.projectId)?.id??"product",mainTopicId:topicId});
  s.tasks.push({id:id(),projectId:a.projectId,planId,title:W(a.taskTitle),goal:W(a.goal),criteria:[W(a.goal)],makerId:maker,reviewerId:actor,positionId:maker==="gu"?"dev":s.people.find(v=>v.id===actor)!.positionId,nodeId:"make",status:"draft",version:1,dependsOn:[]});
  s.topics.push({id:topicId,projectId:a.projectId,title:W(a.title),planIds:[planId],taskIds:[],kind:"plan-main",messages:[],recordIds:[],parentId:parent?.id,forkAfterSeq:parent?history(s,parent).at(-1)?.seq??0:0});
  s.decisions.push({id:id(),projectId:a.projectId,kind:"arrangement",title:W(a.title),description:W(a.goal),planId,sourceTopicId:parent?.id??topicId,version:1,status:"pending",actors:[...new Set([project.ownerId,maker])],approvedBy:[]});
  return {state:s,id:planId};
 }
 return {error:"cpNoPermission"};
}
