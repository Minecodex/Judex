import {useWork} from "../work/store";
import {apiWsKey,mapPlan,type ApiPlan} from "../work/apiModel";
import {useCollection} from "../../lib/api/collections";
import type {components} from "../../lib/api/schema";
import {canOwnPlan,canWork,canAcceptTask,ownsSeat} from "../work/selectors";
import type {Plan} from "../work/types";
export type PlanFilters={search:string;status:string;mine:boolean};
export function usePlanCards(filters:PlanFilters){
 const {state,project,mode,text}=useWork(),params=new URLSearchParams();
 if(filters.search)params.set("q",filters.search);if(filters.status)params.set("status",filters.status);if(filters.mine)params.set("mine","true");
 const query=useCollection<ApiPlan>([...apiWsKey(project.id,state.currentUserId),"cooperation","plans",filters],`/projects/${project.id}/plans?`+params,mode==="api");
 const items=mode==="api"?(query.data?.items??[]).map(p=>mapPlan(p,project.id)):state.plans.filter(p=>!p.discardedAt&&p.projectId===project.id&&(!filters.status||(p.businessStatus??p.status)===filters.status)&&text(p.title).toLowerCase().includes(filters.search.toLowerCase())&&(!filters.mine||canOwnPlan(state,p)||state.tasks.some(t=>t.planId===p.id&&canWork(state,t))));
 return {...query,items,pending:mode==="api"&&query.isPending,error:mode==="api"&&query.isError};
}
export type ActionCard=components["schemas"]["PendingAction"];
export function useDecisionCards(kind:string){
 const {state,project,mode,text}=useWork();
 const query=useCollection<ActionCard>([...apiWsKey(project.id,state.currentUserId),"cooperation","actions",kind],`/me/actions?projectId=${project.id}&category=decision${kind?"&kind="+kind:""}`,mode==="api");
 const demo:ActionCard[]=[];
 const add=(objectType:ActionCard["objectType"],id:string,k:ActionCard["kind"],summary:string)=>demo.push({id:objectType+":"+id+":"+k,objectType,objectId:id,kind:k,projectId:project.id,summary,createdAt:new Date(0).toISOString()});
 for(const p of state.proposals??[])if(p.projectId===project.id&&p.status==="pending"&&p.approvers.includes(state.currentUser)&&!p.votes[state.currentUser])add("proposal",p.id,"approve",text(p.title));
 for(const h of state.handoffs)if(h.projectId===project.id&&ownsSeat(state,h.receiverSeatId)&&h.sources.some(s=>s.status==="pending"))add("handoff",h.id,"receive",text(h.title));
 for(const t of state.tasks)if(t.projectId===project.id&&!t.executionException&&!t.discardedAt&&(t.businessStatus??t.status)==="delivered"&&canAcceptTask(state,t))add("task",t.id,"accept",text(t.title));
 for(const p of state.plans)if(!p.discardedAt&&p.projectId===project.id&&p.status==="active"&&canOwnPlan(state,p)){const tasks=state.tasks.filter(t=>!t.discardedAt&&(t.planId===p.id||p.referenceTaskIds.includes(t.id)));if(tasks.length&&tasks.every(t=>["accepted","cancelled"].includes(t.businessStatus??t.status)||t.planId===p.id&&(t.executionException||(t.businessStatus??t.status)==="draft")))add("plan",p.id,"accept",text(p.title));}
 return {...query,items:mode==="api"?query.data?.items??[]:demo.filter(v=>!kind||v.kind===kind),pending:mode==="api"&&query.isPending,error:mode==="api"&&query.isError};
}
export type DeliveryCard=components["schemas"]["DeliveryCard"];
export function useDeliveryCards(filter:string,type:string){
 const {state,project,mode,text}=useWork();
 const query=useCollection<DeliveryCard>([...apiWsKey(project.id,state.currentUserId),"cooperation","deliveries",filter,type],`/projects/${project.id}/deliveries?filter=${filter}&type=${type}`,mode==="api");
 const demo:DeliveryCard[]=[],personal=new Map<string,{waiting:boolean;revise:boolean}>();
 for(const t of state.tasks){const status=t.businessStatus??t.status;if(t.projectId!==project.id||!["delivered","rework","accepted"].includes(status))continue;
  personal.set("task:"+t.id,{waiting:!t.executionException&&canWork(state,t)&&status==="delivered",revise:!t.executionException&&canWork(state,t)&&status==="rework"});
  demo.push({id:"task:"+t.id,objectId:t.id,objectType:"task",taskId:t.id,planId:t.planId,title:text(t.title),status:t.executionException?"skipped":status,version:t.revision,summary:text(t.expected),incoming:!t.executionException&&canAcceptTask(state,t),outgoing:canWork(state,t),sourceCount:1,pendingCount:!t.executionException&&status==="delivered"?1:0,createdAt:new Date(0).toISOString()});
 }
 for(const h of state.handoffs){if(h.projectId!==project.id)continue;const pending=h.sources.filter(s=>s.status==="pending").length;const status=pending?"pending":h.sources.some(s=>s.status==="rejected")?"needs_revision":h.sources.length&&h.sources.every(s=>s.status==="accepted")?"accepted":"draft";
  const own=h.sources.filter(s=>ownsSeat(state,s.senderSeatId));personal.set("handoff:"+h.id,{waiting:own.some(s=>s.status==="pending"),revise:own.some(s=>["draft","rejected"].includes(s.status)||h.stale)});
  demo.push({id:"handoff:"+h.id,objectId:h.id,objectType:"handoff",taskId:h.taskId,planId:state.tasks.find(t=>t.id===h.taskId)?.planId??null,title:text(h.title),status,version:1,summary:h.sources.map(s=>text(s.summary)).join(" · "),incoming:ownsSeat(state,h.receiverSeatId),outgoing:h.sources.some(s=>ownsSeat(state,s.senderSeatId)),sourceCount:h.sources.length,pendingCount:pending,createdAt:new Date(0).toISOString()});
 }
 const items=demo.filter(d=>(type==="all"||d.objectType===type)&&(filter==="all"||filter==="receive"&&d.objectType==="handoff"&&d.incoming&&d.pendingCount>0||filter==="waiting"&&personal.get(d.id)?.waiting||filter==="revise"&&personal.get(d.id)?.revise));
 return {...query,items:mode==="api"?query.data?.items??[]:items,pending:mode==="api"&&query.isPending,error:mode==="api"&&query.isError};
}
export function planStats(plan:Plan,state:ReturnType<typeof useWork>["state"]){const tasks=state.tasks.filter(t=>!t.discardedAt&&(t.planId===plan.id||plan.referenceTaskIds.includes(t.id)));const computed={total:tasks.length,accepted:tasks.filter(t=>(t.businessStatus??t.status)==="accepted").length,active:tasks.filter(t=>!["accepted","cancelled"].includes(t.businessStatus??t.status)).length,cancelled:tasks.filter(t=>t.businessStatus==="cancelled").length,required:tasks.filter(t=>!["draft","cancelled"].includes(t.businessStatus??t.status)&&(!t.executionException||t.planId!==plan.id)).length,draft:tasks.filter(t=>(t.businessStatus??t.status)==="draft").length,skipped:tasks.filter(t=>t.executionException).length};return {...computed,...plan.taskStats};}
