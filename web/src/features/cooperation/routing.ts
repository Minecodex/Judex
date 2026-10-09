import type {Route} from "../work/types.ts";

export type HubTab="plans"|"decisions"|"deliveries"|"materials";
const tabs:HubTab[]=["plans","decisions","deliveries","materials"];
export function projectFromURL(pathname:string,search:string){
 const part=pathname.match(/^\/projects\/([^/]+)/)?.[1];
 return part?decodeURIComponent(part):new URLSearchParams(search).get("project");
}
// Shared by the app shell, demo source, deep links and browser history. View is
// the inspector selection; page and scope describe the outer navigation.
export function parseCooperationRoute(pathname:string,search:string):Partial<Route>{
 const q=new URLSearchParams(search),parts=pathname.split("/").filter(Boolean).map(decodeURIComponent);
 const projectId=projectFromURL(pathname,search)??"leaf";
 const hubTab=tabs.includes(q.get("tab") as HubTab)?q.get("tab") as HubTab:"plans";
 const base:Partial<Route>={projectId,hubTab,id:q.get("item")??undefined,focusTaskId:q.get("focus")??undefined,proposalTopicId:q.get('proposalTopic')??undefined,editTopicId:q.get("editTopic")??undefined,editor:["plan","task","topic","proposal"].includes(q.get("editor")??"")?q.get("editor") as Route["editor"]:undefined,invite:q.get("invite")==="1",originProjects:q.get("origin")==="projects",scopePlanId:q.get("scopePlan")??undefined,scopeTaskId:q.get("scopeTask")??undefined};
 if(q.get('source'))base.sourceId=q.get('source')!;
 if(parts[0]==="projects"){
  if(['overview','records','flow'].includes(q.get('section')??''))base.taskSection=q.get('section') as import('../work/runtimeTypes').TaskSection;
  if(parts[2]==="settings")return {...base,page:(q.get("returnPage")??"hub") as Route["page"],settingsSection:(parts[3]??"project") as Route["settingsSection"]};
  if(parts[2]==="plans"){
   base.scopePlanId=parts[3];base.scopeTaskId=undefined;
   if(parts[4]==="route")return {...base,page:"route",view:q.get("task")?"task":"plan",id:q.get("task")??parts[3]};
   return {...base,page:"chat",conversation:parts[5]||q.get("chat")||undefined,view:(q.get("view")??"topic") as Route["view"],id:q.get("item")??parts[5]};
  }
  if(parts[2]==="tasks")return {...base,page:"chat",scopePlanId:undefined,scopeTaskId:parts[3],taskContextId:q.has("taskContext")?q.get("taskContext")||undefined:parts[3],conversation:parts[5]??undefined,view:(q.get("view")??"topic") as Route["view"],id:q.get("item")??parts[5]};
  if(parts[2]==="chat")return {...base,page:"chat",conversation:parts[3],view:(q.get("view")??"topic") as Route["view"],id:q.get("item")??parts[3]};
  return {...base,page:q.get("origin")==="projects"&&q.get("invite")==="1"?"projects":"hub",view:(q.get("view")??"plans") as Route["view"]};
 }
 if(!q.get("project"))return {...base,page:"projects",view:"plans"};
 if(q.get("chat")||q.get("view")==="topic"||q.get("view")==="workspace")return {...base,page:"chat"};
 if(q.get("view")==="plan")return {...base,page:"route",scopePlanId:q.get("item")??undefined};
 const view=q.get("view");return {...base,page:"hub",hubTab:view==="resources"?"materials":view==="decisions"?"decisions":view==="task"||view==="handoff"||view==="handoffs"?"deliveries":hubTab};
}
export function cooperationURL(r:Route){
 const q=new URLSearchParams(),p="/projects/"+encodeURIComponent(r.projectId);let path=p;
 if(r.page==="projects"&&!r.settingsSection&&!r.invite&&!r.editor)return "/";
 if(r.settingsSection){path=p+"/settings/"+r.settingsSection;q.set("returnPage",r.page??"hub");}
 else if(r.page==="route"&&r.scopePlanId){path=p+"/plans/"+encodeURIComponent(r.scopePlanId)+"/route";if(r.view==="task"&&r.id)q.set("task",r.id);}
 else if(r.page==="chat"){
  const topic=r.conversation??(r.view==="topic"?r.id:undefined);
  const scope=r.scopeTaskId?"/tasks/"+encodeURIComponent(r.scopeTaskId):r.scopePlanId?"/plans/"+encodeURIComponent(r.scopePlanId):"";
  path=p+scope+"/chat"+(topic?"/"+encodeURIComponent(topic):"");
 }
 if(r.editTopicId)q.set("editTopic",r.editTopicId);if(r.page==="projects"&&r.invite)q.set("returnPage","projects");
 if(r.editor)q.set("editor",r.editor);if(r.invite)q.set("invite","1");if(r.originProjects)q.set("origin","projects");
 if(r.editor==='proposal'&&r.proposalTopicId)q.set('proposalTopic',r.proposalTopicId);
 if(r.hubTab&&r.hubTab!=="plans")q.set("tab",r.hubTab);
 if(r.settingsSection||r.page==="chat"||r.page==="hub"){
  if(r.view!=="topic"&&r.view!=="home")q.set("view",r.view);
  if(r.id&&r.view!=="topic")q.set("item",r.id);
  if(r.settingsSection&&r.conversation)q.set("chat",r.conversation);
  if(r.settingsSection&&r.scopePlanId)q.set("scopePlan",r.scopePlanId);
  if(r.settingsSection&&r.scopeTaskId)q.set("scopeTask",r.scopeTaskId);
 }
 if(r.taskContextId)q.set("taskContext",r.taskContextId);else if(r.page==="chat"&&r.scopeTaskId)q.set("taskContext","");
 if(r.focusTaskId)q.set("focus",r.focusTaskId);
 if(r.activityId)q.set("activity",r.activityId);
 if(r.view==='handoff'&&r.sourceId)q.set('source',r.sourceId);
 if(r.view==='task'&&r.taskSection)q.set('section',r.taskSection);
 if(r.messageSeq!==undefined)q.set("message",String(r.messageSeq));
 if(r.settingsItem)q.set("settingsItem",r.settingsItem);
 return path+(q.size?"?"+q:"");
}
