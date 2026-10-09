import {useDiscussionNavigation} from "../cooperation/useDiscussionNavigation";
import {taskStatusKey} from "./collaborationPresentation";
import { useState } from "react";
import { GitFork, Paperclip, Sparkles } from "lucide-react";
import { UIWarning } from "../../components/ui/FormControls";
import { Button } from "../../components/ui/Button";
import { useWork } from "../work/store";
import type { DiscussionSuggestion, TaskActivity } from "./collaborationTypes";
import {usePlanSummary,useTaskActivity,useDiscussionSuggestions} from "./collaborationData";
import { SuggestionDialog } from "./CollaborationDialogs";
export function AnalysisText({activity,compact=false}:{activity:TaskActivity;compact?:boolean}){
 const {t,mode,act}=useWork(),a=activity.analysis;
 if(!a)return <span>{t("coNotAnalyzed")}</span>;
 const key=a.state==="queued"?"coQueued":a.state==="running"?"coRunning":a.state==="completed"?"coCompleted":a.errorCode==="MODEL_UNAVAILABLE"?"coModelUnavailable":a.state==="failed"?"coFailed":"coWaiting";
 if(compact)return <p className="judex-collab-meta judex-collab-summary-preview">{a.state==="completed"?a.summary:t(key)}</p>;
 return <div className="judex-collab-analysis"><span>{t(key)}{mode==="demo"?" · "+t("coSimulated"):""}</span>{a.summary&&<p>{a.summary}</p>}{a.basis?.length>0&&<div><strong>{t("coBasis")}</strong>{a.basis.map((b,i)=><p key={i}>{b}</p>)}</div>}{a.disagreements.map((d,i)=><p key={i}>{d}</p>)}{a.state==="failed"&&<Button size="sm" variant="outline" onPress={()=>void act("retryTaskAnalysis",{analysisId:a.id},{toast:false})}>{t("coRetryAnalysis")}</Button>}</div>;
}
export function SuggestionCard({suggestion}:{suggestion:DiscussionSuggestion}){
 const {state,t,text,act,go}=useWork(),[dialog,setDialog]=useState<"create"|"link"|null>(null);
 const task=state.tasks.find(v=>v.id===suggestion.taskId),plan=state.plans.find(p=>p.id===suggestion.planId);
 const main=async()=>{const r=await act("resolveDiscussionSuggestion",{suggestionId:suggestion.id,expectedVersion:suggestion.version,mode:"main"},{toast:false});if(r.ok&&r.id)go({page:"chat",view:"topic",id:r.id,scopePlanId:suggestion.planId??undefined,scopeTaskId:undefined,taskContextId:suggestion.taskId});};
 return <article className="judex-co-suggestion" data-testid={"discussion-suggestion-"+suggestion.id}>
  <div className="judex-collab-suggestion-label"><Sparkles size={16}/>{t("coSuggestion")}</div><h3>{suggestion.title}</h3><p>{suggestion.reason}</p><div className="judex-collab-meta">{text(task?.title??"")} · {text(plan?.title??"")}</div>
  <div className="judex-collab-actions"><Button size="sm" variant="primary" onPress={()=>setDialog("create")}>{t("coSeparate")}</Button><Button size="sm" variant="outline" disabled={!plan?.mainTopicId} onPress={()=>void main()}>{t("coContinueMain")}</Button><Button size="sm" variant="outline" onPress={()=>setDialog("link")}>{t("coSelectExisting")}</Button></div>
  <Button size="sm" variant="ghost" className="judex-collab-source-link" onPress={()=>go({view:"task",id:suggestion.taskId,activityId:suggestion.sourceRef.id})}>{t("coOriginalRecord")}</Button>
  {dialog&&<SuggestionDialog suggestion={suggestion} mode={dialog} onClose={()=>setDialog(null)}/>}
 </article>;
}
export function PlanConversationCards({planId}:{planId:string}){
 const {state,t,text,go}=useWork(),summary=usePlanSummary(planId),navigation=useDiscussionNavigation();
 if(summary.pending)return <p role="status">{t("shellLoading")}</p>;
 if(summary.isError)return <UIWarning>{t("errNetwork")}<Button size="sm" onPress={()=>void summary.refetch()}>{t("shellRetry")}</Button></UIWarning>;
 return <div className="judex-collab-plan-cards">
  {summary.data?.tasks.map(item=>{
   const task=state.tasks.find(v=>v.id===item.taskId);if(!task)return null;
   const raw=task.businessStatus??task.status;
   const status=t(taskStatusKey(raw));
   return <article key={task.id} className="judex-co-summary" data-testid={"plan-task-summary-"+task.id}><div className="judex-collab-card-top"><h3>{text(task.title)}</h3><span className={"judex-collab-tag"+(raw==="delivered"?" judex-collab-tag-warn":"")}>{status}</span></div><div className="judex-collab-meta">{t("coRecordCount",{count:item.activityCount})}</div><p className="judex-collab-summary-preview">{item.latestActivity?.text??text(task.expected)}</p>{item.latestActivity&&<AnalysisText activity={item.latestActivity} compact/>}<div className="judex-collab-actions"><Button size="sm" variant="outline" onPress={()=>go({view:"task",id:task.id})}>{t("coViewRecords")}</Button><Button size="sm" variant="outline" onPress={()=>void navigation.task(task)}>{t("coDiscussTask")}</Button></div></article>;
  })}
  {summary.data?.suggestions.filter(s=>s.state==="pending").map(s=><SuggestionCard key={s.id} suggestion={s}/>)}
 </div>;
}
export function TaskConversationCards({taskId}:{taskId:string}){
 const {state,t,go}=useWork(),records=useTaskActivity(taskId),suggestions=useDiscussionSuggestions(taskId),latest=records.items[0];
 return <div className="judex-collab-plan-cards">{latest&&<article className="judex-co-summary"><div className="judex-co-card-top"><strong>{t("coRecords")}</strong><span className="judex-co-meta">{t("coopLoadedRecords",{count:records.items.length})}</span></div><p className="judex-collab-summary-preview">{latest.text}</p><AnalysisText activity={latest} compact/><Button size="sm" variant="outline" onPress={()=>go({view:"task",id:taskId,activityId:latest.id})}>{t("coOriginalRecord")}</Button></article>}{suggestions.items.filter(s=>s.state==="pending").map(s=><SuggestionCard key={s.id} suggestion={s}/>)}{suggestions.hasNextPage&&<Button size="sm" onPress={()=>void suggestions.fetchNextPage()}>{t("lcLoadMore")}</Button>}</div>;
}
