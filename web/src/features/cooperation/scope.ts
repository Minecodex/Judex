import type {Plan,Task,Topic,Route} from "../work/types.ts";
export function effectivePlanIds(topic:Pick<Topic,"planIds"|"taskIds">,tasks:Task[]){return [...new Set([...topic.planIds,...topic.taskIds.flatMap(id=>{const p=tasks.find(t=>t.id===id)?.planId;return p?[p]:[];})])];}
export function topicInScope(topic:Topic,route:Pick<Route,"scopePlanId"|"scopeTaskId">,tasks:Task[]){
 return route.scopeTaskId?topic.taskIds.includes(route.scopeTaskId):route.scopePlanId?effectivePlanIds(topic,tasks).includes(route.scopePlanId):true;
}
export function scopeMain(route:Pick<Route,"scopePlanId"|"scopeTaskId">,plans:Plan[],tasks:Task[]){return route.scopeTaskId?tasks.find(t=>t.id===route.scopeTaskId)?.mainTopicId:route.scopePlanId?plans.find(p=>p.id===route.scopePlanId)?.mainTopicId:undefined;}
export function discussionTasks(topic:Pick<Topic,"projectId"|"planIds"|"taskIds">,tasks:Task[]){
 return tasks.filter(task=>task.projectId===topic.projectId&&(topic.taskIds.includes(task.id)||!!task.planId&&topic.planIds.includes(task.planId)));
}
