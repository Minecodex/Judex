import {TaskDrawer} from './TaskDrawer';
import {useReturnFocus} from './useReturnFocus';
import {CooperationPlanCard} from './CooperationPlanCard';
import {requestRouteFullscreen} from './PlanCanvas';
import {useViewPreference} from "./useViewPreference";
import {useState} from "react";
import {Card,Tabs,Input} from "@heroui/react";
import {ArrowLeft,ArrowRight,GitBranch,MessageCircle,Plus,Search,Settings,UserPlus,Check,FileText} from "lucide-react";
import {Button} from "../../components/ui/Button";
import {ActionGroup} from "../../components/ui/ActionGroup";
import {UISelect,UIOption,UIStatus,UIWarning} from "../../components/ui/FormControls";
import {EmptyState} from "../../components/ui/Presentation";
import {LoadMore} from "../../lib/api/collections";
import {useWork} from "../work/store";
import {Dialog,Person} from "../work/ui";
import {ResourcesPage} from "../work/ProjectPages";
import {AcceptanceDialog} from "../work/Dialogs";
import {TaskInspector} from "../chat/TaskInspector";
import {HandoffPage} from "../work/HandoffPages";
import {ProposalReviewDialog} from "./FrozenReview";
import {planStats,usePlanCards,useDecisionCards,useDeliveryCards,type PlanFilters,type ActionCard} from "./queries";
import {useDiscussionNavigation} from "./useDiscussionNavigation";
import {RoutePreview,RemotePlanDecision} from "./PlanRoute";
import {useReadingPosition} from "../chat/useReadingPosition";
import type {Plan} from "../work/types";
import type {HubTab} from "./routing";

export function ProjectHub(){
 const {state,project,route,go,t,text,management}=useWork(),tab=route.hubTab??"plans";
 const decisions=useDecisionCards("");
 const tabs=[["plans","coopPlans"],["decisions","coopDecisions"],["deliveries","coopDeliveries"],["materials","coopMaterials"]] as const;
 const reading=useReadingPosition("hub:"+(state.currentUserId??state.currentUser)+":"+project.id+":"+tab);
 return <div className="judex-co-scroll" ref={reading}><main className="judex-co-main"><Button size="sm" variant="ghost" className="judex-co-back" onPress={()=>go({page:"projects",scopePlanId:undefined,scopeTaskId:undefined})}><ArrowLeft/>{t("portalProjects")}</Button>
  <div className="judex-co-page-heading"><div><div><h1>{text(project.title)}</h1><p>{t("coopHubHint")}</p></div><ActionGroup className="judex-co-actions"><Button variant="outline" data-testid="project-settings" onPress={()=>go({settingsSection:"project",settingsItem:undefined})}><Settings/>{t("accountSettings")}</Button><Button variant="primary" onPress={()=>go({editor:"plan"})}><Plus/>{t("workNewPlan")}</Button></ActionGroup></div></div>
  <Tabs selectedKey={tab} onSelectionChange={v=>go({page:"hub",hubTab:v as HubTab,view:v==="materials"?"resources":v==="deliveries"?"handoffs":v as "plans"|"decisions",id:undefined})} className="judex-co-hub-tabs"><Tabs.ListContainer><Tabs.List aria-label={t("coopHub")}>{tabs.map(([id,key])=><Tabs.Tab id={id} key={id} data-testid={"hub-tab-"+id}>{t(key)}{id==="decisions"&&(decisions.data?.totalCount??(decisions.pending?0:decisions.items.length))>0&&<span className="judex-co-tab-count">{decisions.data?.totalCount??decisions.items.length}</span>}<Tabs.Indicator/></Tabs.Tab>)}</Tabs.List></Tabs.ListContainer><Tabs.Panel id={tab}>{tab==="plans"?<PlansBoard/>:tab==="decisions"?<DecisionsBoard/>:tab==="deliveries"?<DeliveriesBoard/>:<ResourcesPage/>}</Tabs.Panel></Tabs>
 </main></div>;
}
function PlansBoard(){
 const {state,project,t,go,text}=useWork(),key="judex.cooperation.plans."+(state.currentUserId??state.currentUser)+":"+project.id;
 const read=():PlanFilters=>{try{return {...{search:"",status:"",mine:false},...JSON.parse(sessionStorage.getItem(key)||"{}")};}catch{return {search:"",status:"",mine:false};}};
 const [saved,setSaved]=useState(()=>({key,value:read()})),filters=saved.key===key?saved.value:read(),query=usePlanCards(filters),[preview,setPreview]=useState<Plan>();
 const set=(v:Partial<PlanFilters>)=>{const value={...filters,...v};setSaved({key,value});sessionStorage.setItem(key,JSON.stringify(value));};
 return <><div className="judex-co-toolbar"><div className="judex-co-actions">{[["","coopAll"],["mine","coopMine"],["active","coopActive"],["draft","coDraft"],["accepted","coAccepted"],["cancelled","coopCancel"]].map(([value,label])=><Button key={value} size="sm" variant="ghost" aria-pressed={value==="mine"?filters.mine:!filters.mine&&filters.status===value} onPress={()=>set({mine:value==="mine",status:value==="mine"?"":value})}>{t(label as "coopAll")}</Button>)}</div><div className="judex-co-search"><Search/><Input aria-label={t("coopSearchPlans")} placeholder={t("coopSearchPlans")} value={filters.search} onChange={e=>set({search:e.target.value})}/></div></div>
  {query.pending?<p role="status">{t("shellLoading")}</p>:query.error?<UIWarning>{t("errNetwork")}<Button onPress={()=>void query.refetch()}>{t("shellRetry")}</Button></UIWarning>:query.items.length?<div className="judex-co-plan-grid">{query.items.map(plan=><PlanCard key={plan.id} plan={plan} preview={()=>{requestRouteFullscreen();setPreview(plan);}}/>)}</div>:<EmptyState icon={<GitBranch/>} title={t("coopNoPlans")} description={t("coopEmptyPlanHint")}><Button onPress={()=>go({editor:"plan"})}>{t("workNewPlan")}</Button><Button variant="outline" onPress={()=>go({editor:"topic",scopePlanId:undefined,scopeTaskId:undefined})}>{t("coopStartDiscussion")}</Button></EmptyState>}
  <LoadMore query={query}/><div className="judex-co-secondary"><Button variant="ghost" size="sm" onPress={()=>go({page:"chat",view:"home",id:undefined,conversation:undefined,scopePlanId:undefined,scopeTaskId:undefined})}><MessageCircle/>{t("coopHistory")}</Button></div>{preview&&<RoutePreview plan={preview} onClose={()=>setPreview(undefined)}/>}
 </>;
}
function PlanCard({plan,preview}:{plan:Plan;preview:()=>void}){return <CooperationPlanCard plan={plan} preview={preview}/>;}
function DecisionsBoard(){
 const {t,go,route}=useWork(),[kind,setKind]=useViewPreference("decisions",""),query=useDecisionCards(kind);
 const selected=route.id&&["task","plan","proposal","handoff"].includes(route.view)?{objectId:route.id,objectType:route.view} as ActionCard:undefined;
 return <><p className="judex-co-meta">{t("coopDecisionHint")}</p><div className="judex-co-toolbar"><UISelect aria-label={t("coopDecisions")} value={kind} onChange={e=>setKind(e.target.value)}><UIOption value="">{t("coopAll")}</UIOption><UIOption value="approve">{t("coopArrangement")}</UIOption><UIOption value="receive">{t("coopReceipt")}</UIOption><UIOption value="accept">{t("coopTaskAcceptance")} / {t("coopPlanAcceptance")}</UIOption></UISelect></div>{query.pending?<p role="status">{t("shellLoading")}</p>:query.error?<UIWarning>{t("errNetwork")}<Button onPress={()=>void query.refetch()}>{t("shellRetry")}</Button></UIWarning>:query.items.length?<div className="judex-co-action-grid">{query.items.map(a=><Card key={a.id} className="judex-co-action-card"><Card.Header><span className="judex-co-meta"><Check/>{t(a.kind==="receive"?"coopReceipt":a.kind==="approve"?"coopArrangement":a.objectType==="plan"?"coopPlanAcceptance":"coopTaskAcceptance")}</span><Card.Title>{a.summary}</Card.Title></Card.Header><Card.Footer><Button variant="primary" size="sm" data-testid={"decision-open-"+a.objectId} onPress={()=>{go({page:"hub",hubTab:"decisions",view:a.objectType,id:a.objectId});}}>{t("coopReview")}<ArrowRight/></Button></Card.Footer></Card>)}</div>:<EmptyState icon={<Check/>} title={t("coopNoDecisions")}/>}<LoadMore query={query}/>{selected&&<DecisionDetails action={selected} onClose={()=>{go({page:"hub",view:"decisions",id:undefined});}}/>}</>;
}
function DecisionDetails({action,onClose}:{action:ActionCard;onClose:()=>void}){
 const {state,project,t,dataPending}=useWork();
 if(action.objectType==="proposal")return <ProposalReviewDialog id={action.objectId} onClose={onClose}/>;
 if(action.objectType==="plan")return <PlanDecision id={action.objectId} onClose={onClose}/>;
 const task=state.tasks.find(v=>v.id===action.objectId),handoff=state.handoffs.find(v=>v.id===action.objectId);
 if(dataPending)return <Dialog title={t("coopReview")} onClose={onClose}><p role="status">{t("shellLoading")}</p></Dialog>;
 if(task)return <AcceptanceDialog task={task} onClose={onClose}/>;
 return <Dialog title={t("coopReceipt")} onClose={onClose} wide>{handoff?<HandoffPage handoff={handoff}/>:<UIWarning>{t("coopTaskNotFound")}</UIWarning>}</Dialog>;
}
function PlanDecision({id,onClose}:{id:string;onClose:()=>void}){const {state,t}=useWork();const plan=state.plans.find(p=>p.id===id);return plan?<AcceptanceDialog plan={plan} onClose={onClose}/>:<RemotePlanDecision id={id} onClose={onClose}/>;}
function DeliveriesBoard(){
 const {state,t,go,route,dataPending}=useWork(),[filter,setFilter]=useViewPreference("delivery-filter","all"),[type,setType]=useViewPreference("delivery-type","all"),query=useDeliveryCards(filter,type),drawerFocus=useReturnFocus();
 const selected=route.id&&["task","handoff"].includes(route.view)?{id:route.id,type:route.view}:undefined;
 const task=state.tasks.find(v=>v.id===(selected?.type==="task"?selected.id:undefined)),handoff=state.handoffs.find(v=>v.id===selected?.id);
 return <><section className={"judex-delivery-layout"+(selected?.type==="task"?" judex-delivery-layout-drawer":"")}><div className="judex-delivery-list"><p className="judex-co-meta">{t("coopDeliveriesHint")}</p><div className="judex-co-toolbar"><div className="judex-co-actions">{[["all","coopAll"],["receive","coopToReceive"],["waiting","coopWaiting"],["revise","coopToRevise"]].map(([v,k])=><Button key={v} size="sm" variant="ghost" aria-pressed={filter===v} onPress={()=>setFilter(v)}>{t(k as "coopAll")}</Button>)}</div><UISelect aria-label={t("coopDeliveries")} value={type} onChange={e=>setType(e.target.value)}><UIOption value="all">{t("coopAll")}</UIOption><UIOption value="task">{t("coopDelivery")}</UIOption><UIOption value="handoff">{t("coopReceipt")}</UIOption></UISelect></div>
  {query.pending?<p role="status">{t("shellLoading")}</p>:query.error?<UIWarning>{t("errNetwork")}<Button onPress={()=>void query.refetch()}>{t("shellRetry")}</Button></UIWarning>:query.items.length?<div className="judex-co-action-grid">{query.items.map(d=><Card key={d.id} className="judex-co-action-card" data-testid={"delivery-card-"+d.objectId}><Card.Header><div className="judex-co-card-top"><span className="judex-co-meta"><FileText/>{t(d.objectType==="task"?"coopDelivery":"coopReceipt")}</span><UIStatus>{t(d.status==="skipped"?"taskDetailsSkipped":d.status==="delivered"?"coAwaitAcceptance":d.status==="accepted"?"coAccepted":d.status==="pending"?"coopPending":d.status==="needs_revision"||d.status==="rework"?"coopNeedsRevision":"coopDraftDelivery")}</UIStatus></div><Card.Title>{d.title}</Card.Title><Card.Description>{d.summary}</Card.Description></Card.Header><Card.Footer><Button size="sm" variant="outline" data-focus-key={"delivery-"+d.objectId} onPress={()=>{drawerFocus.capture("delivery-"+d.objectId);go({page:"hub",hubTab:"deliveries",view:d.objectType,id:d.objectId});}}>{t("coopViewRecords")}<ArrowRight/></Button></Card.Footer></Card>)}</div>:<EmptyState icon={<FileText/>} title={t("coopNoDeliveries")}/>}<LoadMore query={query}/></div>
  {selected?.type==="task"&&<TaskDrawer taskId={selected.id} task={task} pending={dataPending} section={route.activityId?"records":route.taskSection??"overview"} onSection={section=>go({view:"task",id:selected.id,taskSection:section,activityId:undefined})} onClose={()=>{go({page:"hub",view:"handoffs",id:undefined,taskSection:undefined,activityId:undefined});drawerFocus.restore("delivery-"+selected.id);}}/>}</section>
  {route.view==="handoff"&&route.id&&<Dialog wide title={t("coopDeliveries")} onClose={()=>go({page:"hub",view:"handoffs",id:undefined})}>{dataPending?<p role="status">{t("shellLoading")}</p>:handoff?<HandoffPage handoff={handoff}/>:<UIWarning>{t("coopTaskNotFound")}</UIWarning>}</Dialog>}
 </>;
}
