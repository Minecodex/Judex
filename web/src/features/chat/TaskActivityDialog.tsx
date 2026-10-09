import {useState} from "react";
import {useQueryClient} from "@tanstack/react-query";
import {UIWarning,UIOption,UISelect,UITextArea} from "../../components/ui/FormControls";
import {Button} from "../../components/ui/Button";
import {request as readTask} from "../../lib/api/client";
import {useWork} from "../work/store";
import {Dialog,Field,Upload,Person} from "../work/ui";
import {mapTask,apiWorkspaceQueries,apiWsKey,type ApiTask} from "../work/apiModel";
import {ownsSeat} from "../work/selectors";
import {TaskResponsibilityPicker} from "../work/TaskResponsibilityPicker";
import type {Task} from "../work/types";
import type {TaskInputKind} from "./collaborationTypes";
import {useWorkspaceDraft} from "./useWorkspaceDraft";
import {useWorkspaceAttachments} from "./useWorkspaceAttachments";
import {useScopedRequest} from "./useScopedRequest";
import {taskStatusKey} from "./collaborationPresentation";

type Snapshot=Pick<Task,"title"|"expected"|"criteria"|"seatIds"|"reviewerSeatId"|"revision"|"status">;
const snapshot=(task:Task):Snapshot=>({title:task.title,expected:task.expected,criteria:task.criteria,seatIds:task.seatIds,reviewerSeatId:task.reviewerSeatId,revision:task.revision,status:task.status});
type Draft={selected:string;expectedVersion:number;inputKind:TaskInputKind;body:string;topicId:string;identityId:string;snapshot?:Snapshot};

export function TaskActivityDialog({task,kind,local=false,onClose,onSaved}:{task:Task;kind:TaskInputKind;local?:boolean;onClose:()=>void;onSaved?:(id?:string)=>void}){
 const {state,project,t,text,act,mode}=useWork(),client=useQueryClient();
 const held=(item:Task)=>item.seatIds.filter(id=>ownsSeat(state,id));
 const own=held(task);
 const [draft,setDraft,clearDraft]=useWorkspaceDraft<Draft>("task-report:"+task.id,{selected:task.id,expectedVersion:task.revision,inputKind:kind,body:"",topicId:"",identityId:own.length===1?own[0]:"",snapshot:snapshot(task)},task.id);
 const {selected,expectedVersion,inputKind,body,topicId}=draft;
 const [files,setFiles,clearFiles]=useWorkspaceAttachments("task-report:"+task.id),[busy,setBusy]=useState(false),[needsReview,setNeedsReview]=useState(false),[review,setReview]=useState<{scope:string;task:Task}>(),[checking,setChecking]=useState(false),[checkFailed,setCheckFailed]=useState(false);
 const scope=JSON.stringify([mode,project.id,state.currentUserId??state.currentUser,selected,inputKind]),request=useScopedRequest(scope);
 const current=state.tasks.find(v=>v.id===selected)??task,plan=state.plans.find(p=>p.id===current.planId);
 const available=state.topics.filter(v=>v.projectId===project.id&&(v.taskIds.includes(current.id)||v.id===plan?.mainTopicId));
 const formal=inputKind==="progress"||inputKind==="delivery",stale=formal&&(needsReview||expectedVersion!==current.revision);
 const identityValid=!formal||current.seatIds.includes(draft.identityId)&&ownsSeat(state,draft.identityId);
 const reviewed=review?.scope===scope&&review.task.id===current.id&&review.task.projectId===project.id?review.task:undefined;
 const recheck=async()=>{
  const pending=request.begin();
  setChecking(true);setCheckFailed(false);
  try{
   const raw=mode==="api"?await readTask<ApiTask>(`/projects/${project.id}/tasks/${current.id}`,{signal:pending.signal}):undefined;
   if(!pending.isCurrent())return;
   if(raw){
    client.setQueryData(apiWorkspaceQueries(project.id,state.currentUserId).taskDetail(current.id).queryKey,raw);
    client.setQueriesData<ApiTask[]>({queryKey:[...apiWsKey(project.id,state.currentUserId),"tasks"]},items=>Array.isArray(items)?items.map(v=>v.id===raw.id?raw:v):items);
   }
   const latest=raw?mapTask(raw,project.id):structuredClone(current);
   if(latest.id===current.id&&latest.projectId===project.id)setReview({scope,task:latest});
  }catch{if(pending.isCurrent())setCheckFailed(true);}finally{if(pending.isCurrent())setChecking(false);}
 };
 const confirmReview=()=>{
  if(!reviewed||checking||reviewed.revision!==current.revision)return;
  const own=held(reviewed),identityId=own.includes(draft.identityId)?draft.identityId:own.length===1?own[0]:"";
  setDraft(v=>({...v,expectedVersion:reviewed.revision,snapshot:snapshot(reviewed),identityId}));setNeedsReview(false);setReview(undefined);
 };
 const selectTask=(id:string)=>{
  const next=state.tasks.find(v=>v.id===id);if(!next)return;
  request.invalidate();setChecking(false);
  const own=held(next);setDraft(v=>({...v,selected:id,expectedVersion:next.revision,snapshot:snapshot(next),topicId:"",identityId:own.length===1?own[0]:""}));setNeedsReview(false);setReview(undefined);setCheckFailed(false);
 };
 const submit=async()=>{
  if(busy||stale||reviewed||checking)return;
  setBusy(true);try{
   const r=await act("recordTaskActivity",{taskId:current.id,identityId:draft.identityId||undefined,kind:inputKind,body,files,...(inputKind==="reply"?{topicId}:{}),simulatedLocal:local&&mode==="demo",...(formal?{expectedTaskVersion:expectedVersion}:{})},{toast:false});
   if(r.ok){clearDraft();clearFiles();onSaved?.(r.id);onClose();}
   else if(formal&&(r.errorCode==="VERSION_CONFLICT"||r.errorCode==="FORBIDDEN"||mode==="demo"))setNeedsReview(true);
  }finally{setBusy(false);}
 };
 return <Dialog title={t(local?"coLocalPushTitle":"coReportTitle")} onClose={onClose} wide={!!reviewed}>
  <div className="judex-collab-context-line">{text(project.title)} › {text(plan?.title??"")} › {text(current.title)}</div>
  <Field label={t("coCurrentTask")}><UISelect data-testid="report-task-select" value={selected} onChange={e=>selectTask(e.target.value)} aria-label={t("coCurrentTask")}>{state.tasks.filter(v=>v.projectId===project.id).map(v=><UIOption key={v.id} value={v.id}>{text(v.title)}</UIOption>)}</UISelect></Field>
  <Field label={t("coInputKind")}><UISelect data-testid="collaboration-input-kind" value={inputKind} onChange={e=>{request.invalidate();setChecking(false);setDraft(v=>({...v,inputKind:e.target.value as TaskInputKind}));setReview(undefined);}} aria-label={t("coInputKind")}>
   <UIOption value="progress">{t("coProgressKind")}</UIOption><UIOption value="delivery">{t("coDeliveryKind")}</UIOption><UIOption value="question">{t("coQuestionKind")}</UIOption><UIOption value="reply">{t("coReplyKind")}</UIOption>
  </UISelect></Field>
  {formal&&<Field label={t("workChooseSeat")}><TaskResponsibilityPicker task={current} value={draft.identityId} onChange={identityId=>setDraft(v=>({...v,identityId}))} testId="report-identity"/></Field>}
  {inputKind==="reply"&&<Field label={t("coReplyTarget")}><UISelect value={topicId} onChange={e=>setDraft(v=>({...v,topicId:e.target.value}))} aria-label={t("coReplyTarget")}><UIOption value="">{t("coSelectTopic")}</UIOption>{available.map(v=><UIOption key={v.id} value={v.id}>{text(v.title)}</UIOption>)}</UISelect></Field>}
  {stale&&<UIWarning role="alert" data-testid="report-stale-warning">{t("coReportChanged",{before:expectedVersion,after:current.revision})}<Button size="sm" variant="outline" isPending={checking} data-testid="report-recheck" onPress={()=>void recheck()}>{t("coReviewLatestTask")}</Button></UIWarning>}
  {checkFailed&&<UIWarning role="alert">{t("errNetwork")}<Button size="sm" isPending={checking} onPress={()=>void recheck()}>{t("shellRetry")}</Button></UIWarning>}
  {reviewed&&<div className="judex-co-task-facts" data-testid="report-task-review">
   <strong>{t("coLatestTaskAgreement")} · v{reviewed.revision}</strong><p>{text(reviewed.title)} · {t(taskStatusKey(reviewed.businessStatus??reviewed.status))}</p><p>{text(reviewed.expected)}</p>
   {reviewed.criteria.map((value,i)=><p key={i}>{text(value)}</p>)}<div>{reviewed.seatIds.map(id=><Person key={id} seatId={id} small/>)}</div><p>{t("workReviewer")} · <Person seatId={reviewed.reviewerSeatId} small/></p>
   {draft.snapshot&&<small>{t("coPreviousTaskAgreement")} · v{draft.snapshot.revision} · {text(draft.snapshot.expected)}</small>}
   <Button variant="outline" data-testid="report-confirm-recheck" disabled={checking||reviewed.revision!==current.revision} onPress={confirmReview}>{t("coConfirmTaskKeepDraft")}</Button>
   {reviewed.revision!==current.revision&&<Button size="sm" isPending={checking} onPress={()=>void recheck()}>{t("coReviewLatestTask")}</Button>}
  </div>}
  <Field label={t("coReportBody")}><UITextArea data-testid="collaboration-report-body" value={body} onChange={e=>setDraft(v=>({...v,body:e.target.value}))} placeholder={t("coReportPlaceholder")} aria-label={t("coReportBody")}/></Field>
  <Upload files={files} onChange={setFiles}/><small className="judex-co-meta">{t("coopDraftAttachmentHint")}</small>
  <div className="judex-collab-dialog-actions"><Button variant="outline" onPress={onClose}>{t("cancel")}</Button><Button variant="primary" disabled={busy||checking||stale||!!reviewed||!identityValid||(!body.trim()&&!files.length)||inputKind==="reply"&&!topicId} onPress={()=>void submit()} data-testid="collaboration-submit-record">{t(local?"coPushToTask":"coSubmitRecord")}</Button></div>
 </Dialog>;
}
