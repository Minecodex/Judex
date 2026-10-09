// One server cache feeds every project page. Only active pages and explicit
// editors request details; hub cards never fan out to all task/review records.
import {useQueries,useQuery,useQueryClient} from "@tanstack/react-query";
import {useAuth} from "../auth/AuthProvider";
import {errorKey} from "../auth/errors";
import {useProjectEvents} from "../../lib/api/sse";
import {useCollection} from "../../lib/api/collections";
import {request} from "../../lib/api/client";
import {apiWorkspaceQueries,apiWsKey,assembleWorkState,mapTopic,type ApiTask,type ApiPlan,type ApiTopic,type ApiMember} from "./apiModel";
import {apiActions,type ApiActContext} from "./apiActions";
import {useWorkbenchBase} from "./storeBase";
import {allPeople,manage,member} from "./selectors";
import type {ActFn,ActResult,WorkStore} from "./storeTypes";

export type ApiWorkbench={store:WorkStore|null;failed:boolean;retry:()=>void};
const unique=(ids:(string|undefined|null)[])=>[...new Set(ids.filter((id):id is string=>!!id))];
function savedIds(key:string,kind:"topic"|"task"){
 try{const raw=JSON.parse(sessionStorage.getItem(key)||"[]");const rows=Array.isArray(raw)?raw:raw.tabs??[];return unique(rows.filter((r:{kind?:string;view?:string})=>kind==="topic"?r.kind==="topic":r.view==="task").map((r:{id?:string})=>r.id)).slice(-16);}catch{return [];}
}
export function useApiWorkbench(projectId:string):ApiWorkbench{
 const base=useWorkbenchBase(),{user}=useAuth(),route={...base.route,projectId};
 const uid=user?.id??"",q=apiWorkspaceQueries(projectId,uid),prefix=apiWsKey(projectId,uid),client=useQueryClient();
 const settings=!!route.settingsSection,configuration=["team","flows","preferences"].includes(route.settingsSection??""),editor=!!route.editor||!!route.invite;
 const chat=route.page==="chat",work=chat||route.page==="route"||route.view==="task"||route.view==="handoff",metadata=work||configuration||editor||route.view==="task"||route.view==="handoff"||route.view==="proposal";
 const bootstrap=useQuery({...q.bootstrap,enabled:!!user});
 const projects=useQuery({...q.projects,enabled:false});
 const members=useQuery({...q.members,enabled:metadata});
 const positions=useQuery({...q.positions,enabled:metadata});
 const identities=useQuery({...q.identities,enabled:metadata});
 const plans=useQuery({...q.plans,enabled:configuration||editor});
 const tasks=useQuery({...q.tasks,enabled:configuration||editor});
 const handoffs=useQuery({...q.handoffs,enabled:work||configuration||route.hubTab==="deliveries"||route.hubTab==="decisions"});
 const invitations=useQuery({...q.invitations,enabled:route.settingsSection==="team"});
 const preferences=useQuery({...q.preferences,enabled:configuration||editor});
 const audit=useQuery({...q.audit,enabled:route.settingsSection==="team"});
 const workflows=useQuery({...q.workflows,enabled:metadata});
 const proposals=useQuery({...q.proposals,enabled:chat||editor});
 const scopeQuery=new URLSearchParams();if(route.scopeTaskId)scopeQuery.set("taskId",route.scopeTaskId);else if(route.scopePlanId)scopeQuery.set("planId",route.scopePlanId);
 const topics=useCollection<ApiTopic>([...prefix,"topics",route.scopePlanId??"",route.scopeTaskId??"",""],`/projects/${projectId}/topics?`+scopeQuery,chat);
 const currentTopicId=route.conversation??(route.view==="topic"?route.id:undefined);
 const selectedHandoff=route.view==="handoff"?handoffs.data?.find(h=>h.id===route.id):undefined;
 const rawTopicSnapshot=topics.data?.items.find(v=>v.id===currentTopicId)??client.getQueryData<ApiTopic>([...prefix,"topic",currentTopicId]);
 const topicSnapshot=rawTopicSnapshot?mapTopic(rawTopicSnapshot,projectId,[]):undefined;
 const taskIds=unique([route.scopeTaskId,route.taskContextId,route.view==="task"?route.id:undefined,selectedHandoff?.targetTaskId,...(selectedHandoff?.sources??[]).map(s=>s.sourceTaskId),...(chat?topicSnapshot?.taskIds??[]:[]),...(chat?savedIds("judex.workspace.tabs.v1."+projectId+":"+uid,"task"):[])]);
 const details=useQueries({queries:taskIds.map(id=>({...q.taskDetail(id),enabled:work||route.view==="task"}))});
 const detailedTasks=details.flatMap(r=>r.data?[r.data]:[]);
 const planId=route.scopePlanId??detailedTasks.find(t=>t.id===(route.scopeTaskId??selectedHandoff?.targetTaskId??(route.view==="task"?route.id:undefined)))?.planId??undefined;
 const selectedPlan=useQuery({queryKey:[...prefix,"plan",planId],enabled:!!planId&&work,queryFn:()=>request<ApiPlan>(`/projects/${projectId}/plans/${planId}`)});
 const scopedTasks=useQuery({queryKey:[...prefix,"tasks","plan",planId],enabled:work&&!!planId,queryFn:async()=>{const items:ApiTask[]=[];let cursor="";do{const page=await request<{items:ApiTask[];nextCursor:string|null}>(`/projects/${projectId}/tasks?planId=${planId}&includeReferences=true&limit=100${cursor?"&cursor="+encodeURIComponent(cursor):""}`);items.push(...page.items);cursor=page.nextCursor??"";}while(cursor);return items;}});
 const relatedPlanIds=unique([...(chat?topicSnapshot?.planIds??[]:[]),...detailedTasks.map(t=>t.planId)]).filter(id=>id!==planId);
 const relatedPlans=useQueries({queries:relatedPlanIds.map(id=>({queryKey:[...prefix,"plan",id],enabled:work,queryFn:()=>request<ApiPlan>(`/projects/${projectId}/plans/${id}`)}))});
 const relatedTasks=useQueries({queries:relatedPlanIds.filter(id=>topicSnapshot?.planIds.includes(id)).map(id=>({queryKey:[...prefix,"tasks","plan",id],enabled:chat,queryFn:async()=>{const items:ApiTask[]=[];let cursor="";do{const p=await request<{items:ApiTask[];nextCursor:string|null}>(`/projects/${projectId}/tasks?planId=${id}&limit=100${cursor?"&cursor="+encodeURIComponent(cursor):""}`);items.push(...p.items);cursor=p.nextCursor??"";}while(cursor);return items;}}))});
 const topicIds=unique([route.conversation??(route.view==="topic"?route.id:undefined),route.scopeTaskId?detailedTasks.find(t=>t.id===route.scopeTaskId)?.mainTopicId:selectedPlan.data?.mainTopicId,...(chat?savedIds("judex.chat.tabs.v1."+projectId+":"+uid,"topic"):[])]).filter(id=>!id.startsWith("handoff:"));
 const topicDetails=useQueries({queries:topicIds.map(id=>({queryKey:[...prefix,"topic",id],enabled:chat,queryFn:()=>request<ApiTopic>(`/projects/${projectId}/topics/${id}`)}))});
 const wantedFlows=unique([selectedPlan.data?.workflowId,...detailedTasks.map(t=>t.workflowId)]);
 const versions=useQueries({queries:(workflows.data??[]).filter(w=>configuration||editor||wantedFlows.includes(w.id)).map(w=>({...q.workflowVersions(w.id),enabled:metadata}))});
 const versionFlows=(workflows.data??[]).filter(w=>settings||editor||wantedFlows.includes(w.id));
 const pendingProposals=(proposals.data??[]).filter(p=>p.kind==="work_arrangement"&&(editor||!route.conversation||p.topicId===route.conversation));
 const reviews=useQueries({queries:pendingProposals.map(p=>q.proposalReview(p.id))});
 const invalidate=()=>{void client.invalidateQueries({queryKey:apiWsKey(projectId)});void client.invalidateQueries({queryKey:["projectLanding"]});void client.invalidateQueries({queryKey:["projectRecent"]});};
 useProjectEvents(projectId,bootstrap.data?.eventCursor,row=>{if(row.type!=='material.preview.changed')invalidate();});
 const failed=bootstrap.isError;
 if(!user||!bootstrap.data)return {store:null,failed,retry:invalidate};
 const currentMember:ApiMember={userId:user.id,displayName:user.displayName,email:user.email,role:bootstrap.data.project.role,state:"active",joinedAt:new Date().toISOString()};
 const merge=<T extends {id:string}>(...sets:T[][])=>[...new Map(sets.flat().map(v=>[v.id,v])).values()];
 const state=assembleWorkState(projectId,{
  bootstrap:bootstrap.data,projects:projects.data??[bootstrap.data.project],members:members.data??[currentMember],positions:positions.data??[],identities:identities.data??bootstrap.data.identities,
  plans:merge(plans.data??[],selectedPlan.data?[selectedPlan.data]:[],relatedPlans.flatMap(r=>r.data?[r.data]:[])),tasks:merge(tasks.data??[],scopedTasks.data??[],detailedTasks,relatedTasks.flatMap(r=>r.data??[])),handoffs:handoffs.data??[],
  topics:merge(topics.data?.items??[],topicDetails.flatMap(r=>r.data?[r.data]:[])),topicMessages:{},invitations:invitations.data??[],preferences:preferences.data??[],audit:audit.data??[],
  workflows:workflows.data??[],workflowVersions:Object.fromEntries(versionFlows.map((w,i)=>[w.id,versions[i]?.data??[]])),proposals:proposals.data??[],proposalReviews:Object.fromEntries(pendingProposals.map((p,i)=>[p.id,reviews[i]?.data])),currentUser:user.displayName,currentUserId:user.id,
 });
 const act:ActFn=async(name,payload,opts)=>{
  const execute=apiActions[name] as ((projectId:string,payload:never,ctx:ApiActContext)=>Promise<ActResult>)|undefined;
  if(!execute){base.setToast(base.t("shellUnavailable"));return {ok:false};}
  try{const result=await execute(projectId,payload as never,{state,userId:user.id,displayName:user.displayName});invalidate();if(name==="createProject"&&result.id){base.go({projectId:result.id,page:"hub",view:"plans"});}
   if(opts?.toast!==false)base.setToast(base.t("workSaved"));return result;
  }catch(e){base.setToast(base.t(errorKey(e)??"errNetwork"));return {ok:false,errorCode:typeof e==="object"&&e&&"code" in e?String(e.code):undefined};}
 };
 const required=[...(work?[workflows,...versions]:[]),...(route.view==="handoff"?[handoffs,...details]:[]),...(metadata?[members,positions,identities]:[]),...(configuration||editor?[workflows,...versions,preferences,...(editor||route.settingsSection==="team"?[plans,tasks]:[])]:[]),...((route.scopeTaskId||route.view==="task")?[details[taskIds.indexOf(route.scopeTaskId??route.id??"")]]:[]),...(planId&&work?[selectedPlan,scopedTasks]:[])].filter(Boolean);
 return {store:{mode:"api",state,route,go:base.go,locale:base.locale,setLocale:base.setLocale,theme:base.theme,setTheme:base.setTheme,toast:base.toast,setToast:base.setToast,storageError:false,text:base.text,t:base.t,act,switchPerson:()=>{},reset:()=>{},project:state.projects.find(p=>p.id===projectId)!,membership:member(state,projectId),management:manage(state,projectId),people:allPeople(state),dataRetry:invalidate,dataPending:required.some(r=>r.isPending),dataFailed:required.some(r=>r.isError)||bootstrap.isError||topicDetails.some((r,i)=>topicIds[i]===route.conversation&&r.isError)},failed,retry:invalidate};
}
