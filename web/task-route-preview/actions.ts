import {canEdit,canWork,deleteBlocked,descendants,hasCycle,tx,unmet,type Patch,type RecordItem,type State,type Task} from './model';
import type {TaskRoutePreviewKey} from '../src/i18n/taskRoutePreview';
export type Action=
 |{kind:'edit';type:'task'|'plan';id:string;patch:Patch;version:number}
 |{kind:'delete';type:'task'|'plan';id:string}
 |{kind:'confirm';type:'task'|'plan';id:string;version:number}
 |{kind:'create';planId:string;patch:Patch}
 |{kind:'skip';id:string;reason:string;waivers:string[];version:number}
 |{kind:'restore';id:string;reason:string;version:number;acknowledged:boolean}
 |{kind:'start';id:string}
 |{kind:'report';id:string;text:string;delivery?:boolean};
export type Result={state?:State;notice?:TaskRoutePreviewKey;error?:TaskRoutePreviewKey;id?:string};
export function apply(state:State,action:Action):Result{
 const s=structuredClone(state),task='id' in action?s.tasks.find(t=>t.id===action.id):undefined;
 const item='type' in action?(action.type==='task'?task:s.plans.find(p=>p.id===action.id)):task;
 const now=new Date().toISOString(),uid=()=>crypto.randomUUID();
 const record=(t:Task,summary:RecordItem['summary'])=>s.records.unshift({id:uid(),taskId:t.id,kind:'decision',author:s.role,source:'web',at:now,summary,original:summary});
 if(action.kind==='create'){
  const plan=s.plans.find(p=>p.id===action.planId);if(!plan||plan.status==='accepted'||!canEdit(s,plan))return {error:'d4Permission'};
  const p=action.patch,id=uid();if(!p.title.zh.trim()||!p.description.zh.trim()||!p.criteria.length)return {error:'d4Required'};
  if(p.before?.some(id=>!s.tasks.some(t=>t.id===id&&t.planId===plan.id)))return {error:'d4StatusChanged'};
  s.tasks.push({id,planId:plan.id,title:p.title,description:p.description,criteria:p.criteria,owner:p.owner,reviewer:p.reviewer??'manager',status:'draft',before:p.before??[],conditions:[],version:1,workflowAssigned:false,flowNode:p.title,responsibility:p.description});
  return {state:s,notice:'d4Created',id};
 }
 if(!item)return {error:'d4StatusChanged'};
 if(action.kind==='delete'){
  if(!canEdit(s,item))return {error:'d4Permission'};
  if(deleteBlocked(s,action.type,action.id))return {error:'d4DeleteBlocked'};
  const ids=action.type==='task'?[action.id]:s.tasks.filter(t=>t.planId===action.id).map(t=>t.id);
  s.tasks=s.tasks.filter(t=>!ids.includes(t.id));if(action.type==='plan')s.plans=s.plans.filter(p=>p.id!==action.id);
  s.changes=s.changes.filter(c=>!ids.includes(c.id)&&c.id!==action.id);
  return {state:s,notice:'d4Deleted'};
 }
 if(action.kind==='edit'){
  if(!canEdit(s,item)||item.status==='accepted')return {error:'d4Permission'};
  if(item.version!==action.version)return {error:'d4StatusChanged'};
  const p=action.patch;if(!p.title.zh.trim()||!p.description.zh.trim()||!p.criteria.length)return {error:'d4Required'};
  if(task&&action.type==='task'&&p.before&&hasCycle(s,task.id,p.before))return {error:'d4Cycle'};
  if(item.status!=='draft'){
   s.changes=s.changes.filter(c=>!(c.type===action.type&&c.id===action.id));s.changes.push({type:action.type,id:action.id,patch:p,version:item.version});
   return {state:s,notice:'d4ChangeSubmitted'};
  }
  item.title=p.title;item.criteria=p.criteria;item.owner=p.owner;item.version++;item.pending=false;
  if(action.type==='task'&&task){task.description=p.description;task.reviewer=p.reviewer??task.reviewer;task.before=p.before??task.before;}
  else s.plans.find(v=>v.id===action.id)!.goal=p.description;
  return {state:s,notice:'d4Saved'};
 }
 if(action.kind==='confirm'){
  if(s.role!=='manager')return {error:'d4Permission'};
  const change=s.changes.find(c=>c.id===action.id&&c.type===action.type);
  if(!change||change.version!==item.version||action.version!==item.version)return {error:'d4StatusChanged'};
  const p=change.patch;if(task&&p.before&&hasCycle(s,task.id,p.before))return {error:'d4Cycle'};
  item.title=p.title;item.criteria=p.criteria;item.owner=p.owner;item.version++;
  if(action.type==='task'&&task){task.description=p.description;task.reviewer=p.reviewer??task.reviewer;task.before=p.before??task.before;if(task.status==='delivered')task.status='rework';record(task,tx('任务安排变更已确认。','Task arrangement changes confirmed.'));}
  else s.plans.find(v=>v.id===action.id)!.goal=p.description;
  s.changes=s.changes.filter(c=>c!==change);return {state:s,notice:'d4ChangeConfirmed'};
 }
 if(!task)return {error:'d4StatusChanged'};
 if(action.kind==='skip'){
  if(s.role!=='manager')return {error:'d4Permission'};
  if(task.skip||!['ready','working','rework'].includes(task.status)||task.version!==action.version)return {error:'d4StatusChanged'};
  if(!action.reason.trim())return {error:'d4ReasonRequired'};
  const valid=s.tasks.flatMap(t=>t.conditions.filter(c=>c.kind==='acceptance'&&c.target===task.id).map(c=>c.id));
  if(action.waivers.some(id=>!valid.includes(id)))return {error:'d4StatusChanged'};
  task.skip={reason:action.reason.trim(),at:now,waivers:action.waivers};task.version++;
  record(task,tx('管理员临时跳过：'+action.reason,'Administrator temporarily skipped: '+action.reason));return {state:s,notice:'d4SkipSaved'};
 }
 if(action.kind==='restore'){
  if(s.role!=='manager')return {error:'d4Permission'};
  if(!task.skip||task.version!==action.version)return {error:'d4StatusChanged'};
  if(!action.reason.trim())return {error:'d4ReasonRequired'};
  if(descendants(s,task.id).some(t=>['working','delivered','accepted','rework'].includes(t.status))&&!action.acknowledged)return {error:'d4StatusChanged'};
  task.skip=undefined;task.version++;record(task,tx('管理员恢复执行：'+action.reason,'Administrator resumed execution: '+action.reason));return {state:s,notice:'d4Restored'};
 }
 if(!canWork(s,task)||task.skip)return {error:'d4Permission'};
 if(action.kind==='start'){
  if(!['ready','rework'].includes(task.status)||unmet(s,task).length)return {error:'d4StatusChanged'};
  task.status='working';task.version++;record(task,tx('已开始任务。','Task started.'));return {state:s,notice:'d4Started'};
 }
 if(action.kind==='report'){
  if(!['working','rework'].includes(task.status)||!action.text.trim()||action.delivery&&unmet(s,task).length)return {error:'d4StatusChanged'};
  s.records.unshift({id:uid(),taskId:task.id,kind:action.delivery?'delivery':'progress',author:s.role,source:'web',at:now,summary:action.text.trim(),original:action.text.trim()});if(action.delivery)task.status='delivered';task.version++;
  return {state:s,notice:'d4ReportSaved'};
 }
 return {error:'d4StatusChanged'};
}
