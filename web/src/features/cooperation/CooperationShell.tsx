import {ProjectListPage} from "./ProjectListPage";
import {Dialog} from "../work/ui";
import {useRef,useState,useEffect} from "react";
import {ArrowLeft,Check,Plus,Search,X} from "lucide-react";
import {Input} from "@heroui/react";
import {Button} from "../../components/ui/Button";
import {EmptyState} from "../../components/ui/Presentation";
import {UINotice,UIWarning,UIOption,UISelect,UIDisclosure} from "../../components/ui/FormControls";
import {useWork,WorkProvider} from "../work/store";
import {ProjectDialog} from "../work/ProjectPages";
import {InviteDialog} from "../work/TeamPages";
import {CreateWorkDialog} from "../work/Dialogs";
import {SettingsScreen} from "../settings/SettingsScreen";
import {ChatDiscussionView} from "../chat/ChatWorkspace";
import type {ConversationDraft} from "../chat/Conversation";
import {ProjectHub} from "./ProjectHub";
import {PlanRoute} from "./PlanRoute";
import {PortalHeader} from "./PortalHeader";
import {ProjectCards} from "./ProjectCards";
import {TopicDialog} from "./TopicDialog";
import {MaterialSharingProvider} from '../materials/MaterialSharing';
import {ProposalDialog} from '../chat/ProposalDialog';
type Props={mode?:"api"|"demo";projectId?:string;onLogout:()=>Promise<void>;onBackToProjects?:()=>void;draftCache?:Map<string,ConversationDraft>;dialogOnly?:boolean};
export default function CooperationWorkspace(props:Props){const cache=useRef(props.draftCache??new Map<string,ConversationDraft>());return <WorkProvider mode={props.mode} projectId={props.projectId}><MaterialSharingProvider draftCache={cache.current}><CooperationShell {...props} draftCache={cache.current}/></MaterialSharingProvider></WorkProvider>;}
function CooperationShell({onLogout,onBackToProjects,draftCache,dialogOnly}:Props){
 const {state,project,route,go,text,t,toast,setToast,mode,people,switchPerson,dataPending,dataFailed}=useWork();
 const cache=useRef(draftCache??new Map<string,ConversationDraft>()),page=route.page??"hub";
 const back=()=>onBackToProjects?onBackToProjects():go({page:"projects",settingsSection:undefined,scopePlanId:undefined,scopeTaskId:undefined,invite:false,editor:undefined});
 const closeInvite=()=>route.originProjects?back():go({invite:false,originProjects:false});
 const scopeTask=state.tasks.find(v=>v.id===route.scopeTaskId),scopePlan=state.plans.find(v=>v.id===route.scopePlanId);
 const crumbs=page==="projects"?[]:[{title:text(project.title),onPress:()=>go({page:"hub",view:"plans",scopePlanId:undefined,scopeTaskId:undefined,settingsSection:undefined})},...(scopeTask?[{title:text(scopeTask.title)}]:scopePlan?[{title:text(scopePlan.title),onPress:()=>go({page:"route",view:"plan",id:scopePlan.id,scopePlanId:scopePlan.id,scopeTaskId:undefined})}]:[])];
 const demo=mode==="demo"?<UIDisclosure className="judex-account-preview" title={t("chatDemoPerson")}><p>{t("accountPreviewNote")}</p><UISelect value={state.currentUser} data-testid="work-person" aria-label={t("chatDemoPerson")} onChange={e=>switchPerson(e.target.value)}>{people.map(p=><UIOption key={p}>{p}</UIOption>)}</UISelect></UIDisclosure>:undefined;
 useEffect(()=>{document.title="Judex · "+(page==="projects"?t("portalProjects"):text(scopeTask?.title??scopePlan?.title??project.title));},[page,scopeTask?.id,scopePlan?.id,project.id,t("portalProjects")]);
 const editor=route.editor;
 return <div className="judex-co-app">
  {!dialogOnly&&<><PortalHeader name={state.currentUser} onLogout={onLogout} onProjects={back} onSecurity={mode==="api"?()=>go({settingsSection:"security",settingsItem:undefined}):undefined} crumbs={crumbs} demoControls={demo}/>
   <div className="judex-co-page" hidden={!!route.settingsSection}>{page==="projects"?<DemoProjects/>:page==="route"?<PlanRoute/>:page==="chat"?<ChatDiscussionView draftCache={cache.current}/>:<ProjectHub/>}</div>
   {route.settingsSection&&(dataPending?<p role="status">{t("shellLoading")}</p>:dataFailed?<UIWarning>{t("errNetwork")}</UIWarning>:<SettingsScreen/>)}
  </>}
  {route.invite&&(dataPending?<LoadingDialog onClose={closeInvite}/>:<InviteDialog onClose={closeInvite}/>)}
  {editor&&(dataPending?<LoadingDialog onClose={()=>go({editor:undefined,editTopicId:undefined,proposalTopicId:undefined})}/>:editor==="topic"?<TopicDialog onClose={()=>go({editor:undefined,editTopicId:undefined})}/>:editor==='proposal'?state.topics.some(topic=>topic.id===(route.proposalTopicId??route.conversation))?<ProposalDialog topicId={(route.proposalTopicId??route.conversation)!} onClose={()=>go({editor:undefined,proposalTopicId:undefined})}/>:<LoadingDialog onClose={()=>go({editor:undefined,proposalTopicId:undefined})}/>:<CreateWorkDialog kind={editor} onClose={()=>go({editor:undefined})}/>)}
  {toast&&<UINotice className="judex-toast" role="status"><Check/>{toast}<Button aria-label={t("close")} onPress={()=>setToast("")}><X/></Button></UINotice>}
 </div>;
}
function LoadingDialog({onClose}:{onClose:()=>void}){const {t}=useWork();return <Dialog title={t("shellLoading")} onClose={onClose}><p role="status">{t("shellLoading")}</p></Dialog>;}
function DemoProjects(){
 const {state,route,go,t,text}=useWork(),[search,setSearch]=useState(""),[ownership,setOwnership]=useState("all"),[creating,setCreating]=useState(false);
 const projects=state.projects.filter(p=>{const mine=p.members.find(m=>m.name===state.currentUser);return !!mine&&text(p.title).toLowerCase().includes(search.toLowerCase())&&(ownership==="all"||(ownership==="owned"?mine.role==="owner":mine.role!=="owner"));});
 const enter=(id:string)=>go({projectId:id,page:"hub",view:"plans",id:undefined,settingsSection:undefined,scopePlanId:undefined,scopeTaskId:undefined});
 const recent=state.topics.filter(v=>v.kind!=="handoff"&&projects.some(p=>p.id===v.projectId)).slice().sort((a,b)=>(b.messages.at(-1)?.at??0)-(a.messages.at(-1)?.at??0)).slice(0,3).map(v=>({projectId:v.projectId,topicId:v.id,title:text(v.title),projectTitle:text(state.projects.find(p=>p.id===v.projectId)!.title),lastActivityAt:new Date(v.messages.at(-1)?.at??0).toISOString()}));
 return <><ProjectListPage items={projects.map(p=>({id:p.id,title:text(p.title),description:text(p.description),role:p.members.find(m=>m.name===state.currentUser)!.role,memberCount:p.members.length,topicCount:state.topics.filter(v=>v.projectId===p.id&&v.kind!=="handoff").length,members:p.members.map(m=>({id:m.name,name:m.name}))}))} recent={recent} search={search} setSearch={setSearch} ownership={ownership} setOwnership={setOwnership} onEnter={(id,topic)=>topic?go({projectId:id,page:"chat",scopePlanId:undefined,scopeTaskId:undefined,view:"topic",id:topic,conversation:topic}):enter(id)} onInvite={id=>go({projectId:id,page:"projects",invite:true,originProjects:true})} onSettings={id=>go({projectId:id,page:"projects",settingsSection:"project"})} onCreate={()=>setCreating(true)}/>{creating&&<ProjectDialog onClose={()=>setCreating(false)}/>}</>;
}
