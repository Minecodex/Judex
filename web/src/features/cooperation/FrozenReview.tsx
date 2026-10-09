import {useState} from "react";
import {useQuery} from "@tanstack/react-query";
import {request} from "../../lib/api/client";
import type {components} from "../../lib/api/schema";
import {Button} from "../../components/ui/Button";
import {UIWarning,UITextArea} from "../../components/ui/FormControls";
import {useWork} from "../work/store";
import {Dialog} from "../work/ui";
import {ReviewDetails,type EvidenceReview} from "../work/ReviewDetails";
import type {Plan,Task} from "../work/types";
import {ProposalCard} from "../chat/ProposalCard";

type Acceptance=components["schemas"]["AcceptanceReview"]&{manifest?:Record<string,unknown>;blockers?:{reason:string;phase?:string}[]};
export function ApiAcceptanceDialog({task,plan,onClose}:{task?:Task;plan?:Plan;onClose:()=>void}){
 const {project,state,t,text,act,go,locale}=useWork(),[instance]=useState(()=>crypto.randomUUID()),[busy,setBusy]=useState(false),[failed,setFailed]=useState(false),[reason,setReason]=useState("");
 const type=plan?"plan":"task",id=plan?.id??task!.id;
 const review=useQuery({queryKey:["frozenAcceptance",state.currentUserId,project.id,type,id,instance],queryFn:()=>request<Acceptance>(`/projects/${project.id}/${type}s/${id}/acceptance-review`),staleTime:Infinity,refetchOnWindowFocus:false});
 const data=review.data,manifest=data?.manifest??{},changed=!!task&&!!data&&data.targetVersion!==task.revision;
 const evidence:EvidenceReview|undefined=data?{reviewId:data.reviewId,reviewHash:data.reviewHash,targetVersion:data.targetVersion??1,reports:(data.reports??[]).map(r=>({reportId:String(r.reportId),text:String(r.text??""),materialVersionIds:Array.isArray(r.materialVersionIds)?r.materialVersionIds.map(String):[]})),blockers:data.blockers}:undefined;
 const decide=async(accept:boolean)=>{if(!data)return;setBusy(true);setFailed(false);try{const frozenReview={reviewId:data.reviewId,reviewHash:data.reviewHash,targetVersion:data.targetVersion??1};const r=plan?await act("planAction",{planId:plan.id,op:"accept",frozenReview}):await act("taskAction",{taskId:task!.id,op:"accept",decision:accept?"accept":"reject",reason:reason||undefined,frozenReview});if(r.ok)onClose();else setFailed(true);}finally{setBusy(false);}};
 return <Dialog wide title={t(plan?"workPlanAccept":"workTaskAccept")} onClose={onClose}><p className="judex-modal-description">{t(plan?"workPlanReviewHint":"workNoReceiptIsAcceptance")}</p>{review.isPending?<p role="status">{t("shellLoading")}</p>:review.isError?<UIWarning>{t("errNetwork")}<Button onPress={()=>void review.refetch()}>{t("shellRetry")}</Button></UIWarning>:<>
  <h3>{String(manifest.title??text(plan?.title??task!.title))}</h3><p>{String(manifest.criteria??(plan?.criteria??task!.criteria).map(text).join("\n"))}</p>
  {Array.isArray(manifest.tasks)&&manifest.tasks.map((v,i)=>{const row=v as Record<string,unknown>;const exception=row.executionException as import('../work/runtimeTypes').ExecutionException|undefined;const actor=project.members.find(m=>m.userId===exception?.actorUserId)?.name;return <section key={i} className="judex-task-frozen-evidence"><div className="judex-co-review-row"><strong>{String(row.title??row.taskId)}</strong><span>{exception?t('taskDetailsSkipped'):String(row.status)}</span></div>{exception&&<div className="judex-task-note"><strong>{t('taskDetailsExceptionRecord')}</strong><p>{exception.reason}</p><small>{actor??''} · {new Date(exception.createdAt).toLocaleString(locale)}</small>{exception.waivers.length>0&&<p>{t('taskDetailsWaived')} · {exception.waivers.map(w=>text(state.tasks.find(v=>v.id===w.taskId)?.title??w.taskId)).join(' · ')}</p>}</div>}</section>;})}
  {evidence&&<ReviewDetails evidence={evidence}/>}{evidence?.reports?.map(r=><div key={r.reportId} className="judex-co-actions"><Button size="sm" variant="outline" onPress={()=>{onClose();go({view:"task",id:task?.id??id,activityId:r.reportId});}}>{t("coopViewRecords")}</Button>{r.materialVersionIds?.map(mid=><a key={mid} href={`/api/v1/projects/${project.id}/material-versions/${mid}/content`} target="_blank" rel="noreferrer">{t("workEvidence")} · {mid}</a>)}</div>)}
  {!plan&&<UITextArea aria-label={t("workRejectReason")} placeholder={t("workRejectReason")} value={reason} onChange={e=>setReason(e.target.value)}/>}
  {(changed||failed)&&<UIWarning role="alert">{t("workErrorStale")}<Button onPress={()=>{setFailed(false);void review.refetch();}}>{t("shellRetry")}</Button></UIWarning>}
 </> }<div className="judex-modal-actions"><Button onPress={onClose}>{t("cancel")}</Button>{task&&<Button variant="outline" disabled={!data||!reason.trim()||changed||failed} isPending={busy} onPress={()=>void decide(false)}>{t("workReject")}</Button>}<Button variant="primary" data-testid="confirm-final-acceptance" disabled={!data||changed||failed||!!data.blockers?.length} isPending={busy} onPress={()=>void decide(true)}>{t(plan?"workPlanAccept":"workTaskAccept")}</Button></div></Dialog>;
}

export function ProposalReviewDialog({id,onClose}:{id:string;onClose:()=>void}){
 const {project,state,mode,t,text,act}=useWork(),[instance]=useState(()=>crypto.randomUUID()),[reason,setReason]=useState(""),[busy,setBusy]=useState(false),[failed,setFailed]=useState(false);
 const review=useQuery({queryKey:["frozenProposal",state.currentUserId,project.id,id,instance],enabled:mode==="api",queryFn:()=>request<components["schemas"]["ProposalReview"]>(`/projects/${project.id}/proposals/${id}/review`),staleTime:Infinity,refetchOnWindowFocus:false});
 const data=review.data,proposal=state.proposals?.find(p=>p.id===id);
 const canAct=data?.canAct??(data?.status==='pending'&&data.slots.some(slot=>slot.canDecide&&slot.state==='pending'));
 const decide=async(accept:boolean)=>{if(!data)return;setBusy(true);try{const r=await act("decideProposal",{proposalId:id,revision:data.version??1,accept,reason:reason||undefined,frozenReview:data});if(r.ok)onClose();else setFailed(true);}finally{setBusy(false);}};
 return <Dialog wide title={t("coopArrangement")} onClose={onClose}>{mode==="demo"?proposal?<ProposalCard proposal={proposal}/>:<p>{t("coopNoDecisions")}</p>:review.isPending?<p role="status">{t("shellLoading")}</p>:review.isError?<UIWarning>{t("errNetwork")}<Button onPress={()=>void review.refetch()}>{t("shellRetry")}</Button></UIWarning>:data&&<>
  <ReviewDetails changes={data.changes.map(c=>({...c,targetId:c.targetId??undefined,clientRef:c.clientRef??undefined}))}/><h3>{t("chatApprovers")}</h3>{data.slots.map(s=>{const seat=state.seats.find(v=>v.id===s.authorityId),role=state.positions.find(v=>v.id===seat?.positionId);return <div className="judex-co-review-row" key={s.id}><span>{s.displayName} · {role?text(role.name):s.authorityType}</span><span>{s.state}</span></div>;})}
  <UITextArea aria-label={t("workRejectReason")} placeholder={t("workRejectReason")} value={reason} onChange={e=>setReason(e.target.value)}/>{failed&&<UIWarning>{t("workErrorStale")}<Button onPress={()=>{setFailed(false);void review.refetch();}}>{t("shellRetry")}</Button></UIWarning>}
  <div className="judex-modal-actions"><Button onPress={onClose}>{t("cancel")}</Button><Button variant="outline" disabled={!reason.trim()||failed||!canAct} isPending={busy} onPress={()=>void decide(false)}>{t("workReject")}</Button><Button variant="primary" data-testid={"approve-proposal-"+id} disabled={failed||!canAct} isPending={busy} onPress={()=>void decide(true)}>{t("chatApprove")}</Button></div>
 </> }</Dialog>;
}
