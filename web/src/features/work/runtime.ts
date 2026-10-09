import {manage,canOwnPlan,member,blockers} from './selectors.ts';
import type {Plan,Task,WorkState} from './types.ts';
import type {WorkCapabilities} from './runtimeTypes.ts';
export function workCapabilities(state:WorkState,item:Plan|Task):WorkCapabilities{
 if(item.capabilities)return item.capabilities;
 const task='seatIds' in item?item:undefined,plan=task?state.plans.find(p=>p.id===task.planId):item as Plan;
 const active=!!member(state,item.projectId),manager=manage(state,item.projectId),owner=!!plan&&canOwnPlan(state,plan),author=item.createdBy===(state.currentUserId??state.currentUser),status=item.businessStatus??item.status;
 const draft=active&&status==='draft'&&!item.discardedAt&&(manager||owner||author);
 return {editDraft:draft,discardDraft:draft,proposeChange:active&&!item.discardedAt&&!['accepted','cancelled'].includes(status),skip:!!task&&manager&&!task.executionException&&!item.discardedAt&&['ready','working','delivered','rework'].includes(status),restore:!!task&&manager&&!!task.executionException};
}
export function taskDisplayStatus(task:Task,state:WorkState,startBlocked?:boolean){
 if(task.discardedAt)return 'discarded';
 if(task.executionException)return 'skipped';
 const phase=task.businessStatus??task.status;
 return phase==='ready'&&(startBlocked??blockers(state,task,'start').length>0)?'blocked':phase;
}
