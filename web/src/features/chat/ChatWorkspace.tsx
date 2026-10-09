import {useDiscussionNavigation} from "../cooperation/useDiscussionNavigation";
import {useRef,useState,useEffect} from "react";
import {Tabs} from "@heroui/react";
import {ArrowLeft,Plus} from "lucide-react";
import {Button} from "../../components/ui/Button";
import {UIWarning} from "../../components/ui/FormControls";
import {useWork} from "../work/store";
import {SidebarConversations} from "./SidebarConversations";
import {useSplitLayout} from "./useSplitLayout";
import {useConversationTabs} from "./useConversationTabs";
import {ConversationTabBar} from "./ConversationTabBar";
import {Conversation,type ConversationDraft} from "./Conversation";
import {WorkspaceTabs} from "./WorkspaceTabs";
import {HandoffPage} from "../work/HandoffPages";
import {ProjectConversationOverview} from "./ProjectConversationOverview";

export function ChatDiscussionView({draftCache}:{draftCache?:Map<string,ConversationDraft>}){
 const {state,project,route,go,t,text,membership,storageError,dataPending,dataFailed}=useWork();
 const [panel,setPanel]=useState(true),split=useSplitLayout(panel&&!!membership),conversations=useConversationTabs(),cache=useRef(draftCache??new Map<string,ConversationDraft>());
 const topic=conversations.current.kind==="topic"?state.topics.find(v=>v.id===conversations.current.id):undefined;
 const handoff=conversations.current.kind==="handoff"?state.handoffs.find(v=>v.id===conversations.current.id):undefined;
 const navigation=useDiscussionNavigation();
 const task=state.tasks.find(v=>v.id===route.scopeTaskId),plan=state.plans.find(v=>v.id===route.scopePlanId),scopeTitle=text(task?.title??plan?.title??project.title);
 useEffect(()=>split.restore(),[project.id,state.currentUserId??state.currentUser,route.scopeTaskId,route.scopePlanId]);
 const back=()=>task?.planId?go({page:"route",scopePlanId:task.planId,scopeTaskId:undefined,view:"plan",id:task.planId}):go({page:"hub",view:"plans",id:undefined,scopePlanId:undefined,scopeTaskId:undefined});
 return <div className={"judex-chat-app"+(!panel?" judex-chat-focused":"")+(split.expanded&&panel?" judex-chat-expanded":"")} ref={split.ref} data-resizing={split.resizing} style={{gridTemplateColumns:split.columns}}>
  <aside className="judex-chat-sidebar"><div className="judex-co-sidebar-heading"><Button size="sm" variant="ghost" data-testid="workspace-back-projects" onPress={back}><ArrowLeft/>{t(task?.planId?"coopBackRoute":"coopBackHub")}</Button><span className="judex-co-meta">{t("coopScope")}</span><h2 title={scopeTitle}>{scopeTitle}</h2>{plan?.status==="draft"&&<small>{t("coopDraft")}</small>}</div><Button variant="primary" className="judex-chat-new" data-testid="new-discussion" onPress={()=>go({editor:"topic",editTopicId:undefined})}><Plus/>{t("coopNewDiscussion")}</Button><SidebarConversations restore={split.restore}/></aside>
  {membership&&split.divider("left")}<main className="judex-chat-main" hidden={split.expanded}><Tabs className="judex-conversation-root" selectedKey={conversations.active} onSelectionChange={key=>{const tab=conversations.tabs.find(t=>(t.kind==="home"?"home":t.kind+":"+t.id)===key);if(tab)conversations.open(tab);}}><ConversationTabBar model={conversations} panelOpen={panel} onTogglePanel={()=>setPanel(!panel)}/>{storageError&&<UIWarning>{t("storageError")}</UIWarning>}<Tabs.Panel className="judex-active-tab-panel" id={conversations.active}>
   {dataFailed?<UIWarning role="alert">{t("coopTaskNotFound")}</UIWarning>:!membership?<UIWarning>{t("errForbidden")}</UIWarning>:handoff?<div className="judex-chat-handoff"><HandoffPage handoff={handoff}/></div>:topic?<Conversation key={project.id+":"+(state.currentUserId??state.currentUser)+":"+topic.id} topic={topic} cache={cache.current}/>:route.conversation||dataPending?<p role="status">{t("shellLoading")}</p>:task?<div className="judex-collab-empty"><h3>{text(task.title)}</h3><Button variant="primary" isPending={navigation.busy===task.id} onPress={()=>void navigation.task(task)}>{t("coopDiscussTask")}</Button></div>:<ProjectConversationOverview/>}
  </Tabs.Panel></Tabs></main>{panel&&!split.expanded&&split.divider("right")}{panel&&<aside className="judex-chat-inspector"><WorkspaceTabs expanded={split.expanded} onExpand={split.toggleWide} onClose={()=>setPanel(false)}/></aside>}
 </div>;
}