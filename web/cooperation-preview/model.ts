export type Text={zh:string;en:string};
export const W=(zh:string,en=zh):Text=>({zh,en});
export type Person={id:string;name:Text;positionId:string};
export type Project={id:string;title:Text;goal:Text;ownerId:string;memberIds:string[];status:"active"|"draft"|"accepted"};
export type Plan={id:string;projectId:string;title:Text;goal:Text;criteria:Text[];ownerId:string;status:"draft"|"active"|"accepted"|"cancelled";workflowId:string;mainTopicId:string};
export type TaskStatus="draft"|"ready"|"working"|"blocked"|"delivered"|"accepted"|"rework";
export type Task={id:string;projectId:string;planId:string;title:Text;goal:Text;criteria:Text[];makerId:string;reviewerId:string;positionId:string;nodeId:string;status:TaskStatus;version:number;dependsOn:string[];receiptId?:string;mainTopicId?:string};
export type Message={id:string;seq:number;authorId:string;text:Text;taskId?:string;recordId?:string;kind:"person"|"ai";at:string;inherited?:boolean;originTopicId?:string};
export type Topic={id:string;projectId:string;title:Text;planIds:string[];taskIds:string[];kind:"plan-main"|"task-main"|"special";messages:Message[];parentId?:string;forkAfterSeq?:number;recordIds:string[]};
export type Activity={id:string;taskId:string;kind:"progress"|"delivery"|"question"|"reply";text:Text;actorId:string;source:"web"|"cli";materialIds:string[];at:string;analysis:Text;topicIds:string[]};
export type Material={id:string;projectId:string;name:string;version:number;authorId:string;body:Text};
export type Suggestion={id:string;recordId:string;taskId:string;title:Text;reason:Text;status:"pending"|"handled";parentId?:string;forkAfterSeq:number;resultTopicId?:string};
export type Decision={id:string;projectId:string;kind:"arrangement"|"task"|"plan";title:Text;description:Text;planId:string;taskId?:string;sourceTopicId:string;version:number;status:"pending"|"approved"|"rejected";actors:string[];approvedBy:string[];reason?:string};
export type Handoff={id:string;projectId:string;sourceTaskId:string;targetTaskId:string;senderId:string;receiverId:string;materialIds:string[];version:number;status:"pending"|"accepted"|"rejected";reason?:string};
export type Position={id:string;projectId:string;name:Text;prompt:Text;nodes:string[]};
export type FlowNode={id:string;name:Text;responsibility:Text;positions:string[]};
export type Flow={id:string;projectId:string;name:Text;version:number;nodes:FlowNode[];edges:[string,string][]};
export type State={schema:1;userId:string;projects:Project[];plans:Plan[];tasks:Task[];topics:Topic[];activities:Activity[];materials:Material[];suggestions:Suggestion[];decisions:Decision[];handoffs:Handoff[];positions:Position[];flows:Flow[];people:Person[];preferences:Record<string,string>;invites?:{projectId:string;email:string;positionId:string}[]};
export type Route={page:"projects"|"hub"|"route"|"chat";projectId:string;planId?:string;taskId?:string;topicId?:string;tab?:string;messageSeq?:number};
export function history(s:State,topic:Topic,ceiling=Infinity,seen=new Set<string>()):Message[]{
 if(seen.has(topic.id))return [];seen.add(topic.id);
 const parent=s.topics.find(t=>t.id===topic.parentId);
 const prefix=parent?history(s,parent,Math.min(ceiling,topic.forkAfterSeq??0),seen).map(m=>({...m,inherited:true,originTopicId:m.originTopicId??parent.id})):[];
 return [...prefix,...topic.messages.filter(m=>m.seq<=ceiling).map(m=>({...m,inherited:false,originTopicId:topic.id}))];
}
export function linkedPlans(s:State,t:Topic){
 return [...new Set([...t.planIds,...t.taskIds.flatMap(id=>{const task=s.tasks.find(v=>v.id===id);return task?[task.planId]:[]})])];
}
export function topicsInScope(s:State,route:Route){
 return s.topics.filter(t=>t.projectId===route.projectId&&(route.taskId?t.taskIds.includes(route.taskId):route.planId?linkedPlans(s,t).includes(route.planId):true));
}
export const id=()=>crypto.randomUUID();
export const stamp=()=>new Date().toLocaleTimeString([], {hour:"2-digit",minute:"2-digit"});
export function planTasks(s:State,pid:string){return s.tasks.filter(t=>t.planId===pid);}
export function ready(s:State,t:Task){return t.dependsOn.every(id=>["delivered","accepted"].includes(s.tasks.find(v=>v.id===id)?.status??""))&&(!t.receiptId||s.handoffs.find(h=>h.id===t.receiptId)?.status==="accepted");}
export function requiredAcceptance(s:State,p:Plan){const tasks=planTasks(s,p.id);return !!tasks.length&&tasks.every(t=>t.status==="accepted");}
export function ownDecisions(s:State,projectId:string){
 return s.decisions.filter(d=>d.projectId===projectId&&d.status==="pending"&&d.actors.includes(s.userId)&&!d.approvedBy.includes(s.userId)&&
 (d.kind!=="task"||s.tasks.find(t=>t.id===d.taskId)?.status==="delivered")&&(d.kind!=="plan"||requiredAcceptance(s,s.plans.find(p=>p.id===d.planId)!)));
}
export function routeFromHash():Route{
 const [page,projectId,scope,scopeId,topicId,seq]=location.hash.slice(1).split("/");
 if(page==="route")return {page,projectId:projectId||"launch",planId:scope};
 if(page==="chat")return {page,projectId:projectId||"launch",planId:scope==="plan"?scopeId:undefined,taskId:scope==="task"?scopeId:undefined,topicId,messageSeq:seq?Number(seq):undefined};
 if(page==="hub")return {page,projectId:projectId||"launch",tab:scope||"plans"};
 return {page:"projects",projectId:"launch"};
}
export function routeHash(r:Route){
 if(r.page==="projects")return "#projects";
 if(r.page==="hub")return `#hub/${r.projectId}/${r.tab??"plans"}`;
 if(r.page==="route")return `#route/${r.projectId}/${r.planId}`;
 return `#chat/${r.projectId}/${r.taskId?"task":"plan"}/${r.taskId??r.planId}/${r.topicId}${r.messageSeq?"/"+r.messageSeq:""}`;
}
