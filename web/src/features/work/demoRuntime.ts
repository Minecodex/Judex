import {uid} from './seed.ts';
import {words,type WorkState,type Task,type Plan,type Requirement,type Result} from './types.ts';
import type {ActionPayloads} from './storeTypes.ts';
import type {DiscardReview,ExecutionReview,ExceptionWaiver} from './runtimeTypes.ts';
import {workCapabilities} from './runtime.ts';
import type {WorkProposal} from '../chat/proposalModel.ts';
export const requirementSignature=(taskId:string,r:Requirement)=>JSON.stringify([taskId,r.id,r.kind,r.ref,r.at??'both',r.hard,r.label]);
const version=(item:Task|Plan)=>item.revision??1;
export function demoDiscardReview(s:WorkState,kind:'task'|'plan',id:string):DiscardReview{
 const item=(kind==='task'?s.tasks:s.plans).find(v=>v.id===id)!,tasks=kind==='plan'?s.tasks.filter(t=>t.planId===id&&!t.discardedAt):[];
 const ids=kind==='task'?[id]:tasks.map(t=>t.id);
 const linked=s.tasks.filter(t=>!t.discardedAt&&!ids.includes(t.id)&&(ids.includes(t.parentId??'')||t.requirements.some(r=>r.kind==='task'&&ids.includes(r.ref))));
 const refs=s.plans.filter(p=>!p.discardedAt&&p.id!==id&&p.referenceTaskIds.some(t=>ids.includes(t)));
 const blockers=[...linked,...refs,...tasks.filter(t=>(t.businessStatus??t.status)!=='draft')].map(t=>({objectId:t.id,objectType:'seatIds' in t?'task':'plan',phase:'both',reason:t.title.zh}));
 const data={targetId:id,targetType:kind,targetVersion:version(item),title:item.title.zh,tasks:tasks.map(t=>({taskId:t.id,planId:t.planId,title:t.title.zh,status:t.businessStatus??t.status,version:version(t),alreadyStarted:false,requirements:[]})),blockers};return {...data,reviewHash:JSON.stringify(data)};
}
export function demoExecutionReview(s:WorkState,id:string,operation:'skip'|'restore'):ExecutionReview{
 const task=s.tasks.find(t=>t.id===id)!,affected=new Set([id]);let changed=true;
 while(changed){changed=false;for(const t of s.tasks){if(affected.has(t.id)||t.discardedAt)continue;if(t.requirements.some(r=>r.kind==='task'&&affected.has(r.ref))){affected.add(t.id);changed=true}}}
 const affectedTasks=s.tasks.filter(t=>t.id!==id&&affected.has(t.id)&&!t.discardedAt).map(t=>({taskId:t.id,planId:t.planId,title:t.title.zh,status:t.businessStatus??t.status,version:version(t),alreadyStarted:['working','delivered','accepted','rework'].includes(t.businessStatus??t.status),requirements:t.requirements.filter(r=>r.kind==='task'&&r.ref===id&&!r.inheritedFrom).map(r=>({id:r.id,sourceTaskId:t.id,targetId:r.ref,phase:r.at??'both',kind:'task_acceptance' as const,hard:r.hard,label:r.label.zh,fingerprint:requirementSignature(t.id,r)}))}));
 const data={taskId:id,targetVersion:version(task),title:task.title.zh,operation,current:task.executionException??null,affectedTasks,referencingPlanIds:s.plans.filter(p=>p.referenceTaskIds.includes(id)).map(p=>p.id)};return {...data,reviewHash:JSON.stringify(data)};
}
function withdraw(s:WorkState,id:string){for(const p of s.proposals??[])if(p.status==='pending'&&(p.workChange?.id===id||p.planId===id)){p.status='rejected';p.reason='Draft review withdrawn';}}
function record(s:WorkState,t:Task,operation:string,reason:string){s.taskActivities??=[];s.taskActivities.push({id:uid(),taskId:t.id,kind:'decision',sourceType:'work_change',source:'web',text:reason,actorUserId:s.currentUserId??s.currentUser,actorName:s.currentUser,identityId:null,createdAt:new Date().toISOString(),materialVersionIds:[],materials:[],topicIds:[],analysis:null});s.events.push({id:uid(),projectId:t.projectId,targetId:t.id,actor:s.currentUser,text:words(operation,operation),at:Date.now()});}
export function applyDemoWorkFields(s:WorkState,kind:'task'|'plan',id:string,fields:Record<string,unknown>){
 const item=(kind==='task'?s.tasks:s.plans).find(v=>v.id===id)!;const str=(k:string)=>String(fields[k]??'');if('title' in fields)item.title=words(str('title'),str('title'));if('acceptanceCriteria' in fields)item.criteria=str('acceptanceCriteria').split('\n').filter(Boolean).map(v=>words(v,v));
 if(kind==='plan'){const p=item as Plan;if('goal' in fields)p.goal=words(str('goal'),str('goal'));if('ownerIdentityId' in fields)p.ownerSeatId=str('ownerIdentityId');}
 else{const t=item as Task;if('expectedOutput' in fields)t.expected=words(str('expectedOutput'),str('expectedOutput'));if('reviewerIdentityId' in fields)t.reviewerSeatId=str('reviewerIdentityId');if('participantIdentityIds' in fields)t.seatIds=fields.participantIdentityIds as string[];if('requirements' in fields)t.requirements=(fields.requirements as {id?:string;phase:'start'|'accept'|'both';kind:'task_acceptance'|'handoff_receipt'|'material_ready';targetId:string;hard:boolean;label:string}[]).map(r=>({id:r.id??uid(),at:r.phase,kind:r.kind==='task_acceptance'?'task':r.kind==='handoff_receipt'?'receipt':'evidence',ref:r.targetId,hard:r.hard,label:words(r.label,r.label)}));if(t.status==='delivered')t.status='rework';}
 item.revision=version(item)+1;
}
export function updateDemoDraft(s:WorkState,p:ActionPayloads['updateWorkDraft']):Result{
 const item=(p.kind==='task'?s.tasks:s.plans).find(v=>v.id===p.id);if(!item||!workCapabilities(s,item).editDraft)return {error:'permission'};if(version(item)!==p.expectedVersion)return {error:'stale'};
 const next=structuredClone(s);applyDemoWorkFields(next,p.kind,p.id,p.fields);withdraw(next,p.id);return {state:next};
}
export function discardDemoWork(s:WorkState,p:ActionPayloads['discardWork']):Result{
 const item=(p.kind==='task'?s.tasks:s.plans).find(v=>v.id===p.id);if(!item||!workCapabilities(s,item).discardDraft)return {error:'permission'};const review=demoDiscardReview(s,p.kind,p.id);if(version(item)!==p.expectedVersion||review.reviewHash!==p.reviewHash)return {error:'stale'};if(review.blockers.length)return {error:'blocked'};
 const next=structuredClone(s),target=(p.kind==='task'?next.tasks:next.plans).find(t=>t.id===p.id)!;target.discardedAt=new Date().toISOString();target.businessStatus='cancelled';target.revision=version(target)+1;withdraw(next,p.id);
 if(p.kind==='plan')for(const t of next.tasks.filter(t=>t.planId===p.id)){t.discardedAt=target.discardedAt;t.businessStatus='cancelled';t.revision++;withdraw(next,t.id)};return {state:next};
}
export function changeDemoExecution(s:WorkState,p:ActionPayloads['executionException']):Result{
 const task=s.tasks.find(t=>t.id===p.taskId);if(!task||!workCapabilities(s,task)[p.operation==='skip'?'skip':'restore'])return {error:'permission'};const review=demoExecutionReview(s,p.taskId,p.operation);if(review.targetVersion!==p.expectedVersion||review.reviewHash!==p.reviewHash)return {error:'stale'};if(!p.reason.trim())return {error:'required'};
 if(p.operation==='restore'&&review.affectedTasks.some(t=>t.alreadyStarted)&&!p.acknowledgeStarted)return {error:'required'};
 const valid=review.affectedTasks.flatMap(t=>t.requirements.filter(r=>r.hard).map(r=>t.taskId+r.id+r.fingerprint));if(p.waivers.some(w=>!valid.includes(w.taskId+w.requirementId+w.fingerprint)))return {error:'stale'};
 const next=structuredClone(s),target=next.tasks.find(t=>t.id===p.taskId)!;target.executionException=p.operation==='skip'?{id:uid(),reason:p.reason,previousStatus:(target.businessStatus??target.status) as 'ready'|'working'|'rework',actorUserId:s.currentUserId??s.currentUser,createdAt:new Date().toISOString(),waivers:p.waivers}:null;target.revision++;
 for(const t of next.tasks){t.requirements=t.requirements.filter(r=>!r.inheritedFrom).map(r=>({...r,waived:false}));}
 const raw=new Map(next.tasks.map(t=>[t.id,structuredClone(t.requirements)]));
 const expand=(consumer:Task,r:Requirement,seen:Set<string>):Requirement[]=>{const prev=next.tasks.find(t=>t.id===r.ref);if(r.kind!=='task'||!r.hard||!prev?.executionException)return [r];const waived=prev.executionException.waivers.some(w=>w.taskId===consumer.id&&w.requirementId===r.id&&w.fingerprint===requirementSignature(consumer.id,r));if(!waived)return [r];const out:Requirement[]=[{...r,waived:true}];if(!seen.has(prev.id)){seen.add(prev.id);for(const up of raw.get(prev.id)??[]){if(up.hard)out.push(...expand(prev,up,new Set(seen)).map(v=>({...v,inheritedFrom:prev.id,at:r.at})));}}return out;};
 for(const t of next.tasks)t.requirements=(raw.get(t.id)??[]).flatMap(r=>expand(t,r,new Set([t.id])));
 record(next,target,p.operation,p.reason);return {state:next};
}
export function proposeDemoWorkChange(s:WorkState,p:ActionPayloads['proposeWorkChange']):Result&{id?:string}{
 const item=(p.kind==='task'?s.tasks:s.plans).find(v=>v.id===p.id);if(!item||!workCapabilities(s,item).proposeChange)return {error:'permission'};if(version(item)!==p.expectedVersion)return {error:'stale'};
 const task=p.kind==='task'?item as Task:undefined,plan=task?s.plans.find(v=>v.id===task.planId):item as Plan;
 const seatIds=[...new Set([...(task?.seatIds??[]),task?.reviewerSeatId??'',plan?.ownerSeatId??'',...(p.fields.participantIdentityIds as string[]??[]),String(p.fields.reviewerIdentityId??''),String(p.fields.ownerIdentityId??'')].filter(Boolean))];
 const bindings=seatIds.map(seatId=>({seatId,person:s.seats.find(v=>v.id===seatId)?.person??''})),approvers=[...new Set(bindings.map(b=>b.person).filter(Boolean))];if(!approvers.length)approvers.push(s.projects.find(v=>v.id===item.projectId)?.members.find(m=>m.role==='owner')?.name??s.currentUser);
 const id=uid(),proposal:WorkProposal={id,projectId:item.projectId,topicId:task?.mainTopicId??plan?.mainTopicId??'',title:String(p.fields.title??item.title.zh),goal:String(p.fields.goal??p.fields.expectedOutput??('goal' in item?item.goal.zh:item.expected.zh)),criteria:String(p.fields.acceptanceCriteria??item.criteria.map(v=>v.zh).join('\n')),ownerSeatId:plan?.ownerSeatId??'',flowId:item.flowId,flowVersion:s.flows.find(v=>v.id===item.flowId)?.version??0,tasks:[],revision:1,status:'pending',sender:s.currentUser,bindings,approvers,votes:{},createdAt:Date.now(),workChange:{kind:p.kind,id:p.id,expectedVersion:p.expectedVersion,fields:p.fields}};
 const next=structuredClone(s);next.proposals??=[];next.proposals.push(proposal);return {state:next,id};
}
