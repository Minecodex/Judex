import {DiscussionPicker} from "../cooperation/DiscussionPicker";
import {LinkedObjectPicker} from "../cooperation/LinkedObjectPicker";
import { useState } from "react";
import { UIInput } from "../../components/ui/FormControls";
import { Button } from "../../components/ui/Button";
import { useWork } from "../work/store";
import { Dialog, Field } from "../work/ui";
import type { Task, Topic } from "../work/types";
import type { DiscussionSuggestion } from "./collaborationTypes";
export {TaskActivityDialog} from "./TaskActivityDialog";
export function ForkDialog({topic,afterSeq,onClose}:{topic:Topic;afterSeq?:number;onClose:()=>void}){
 const {state,project,t,text,act,go,route}=useWork();
 const [title,setTitle]=useState(text(topic.title)+" · "+t("coSpecialDiscussion")),[planIds,setPlans]=useState(topic.planIds),[taskIds,setTasks]=useState(topic.taskIds),[busy,setBusy]=useState(false);
 const submit=async()=>{setBusy(true);try{const r=await act("forkTopic",{topicId:topic.id,title,forkAfterSeq:afterSeq,planIds,taskIds},{toast:false});if(r.ok&&r.id){go({view:"topic",id:r.id,scopePlanId:route.scopePlanId&&planIds.includes(route.scopePlanId)?route.scopePlanId:undefined,scopeTaskId:route.scopeTaskId&&taskIds.includes(route.scopeTaskId)?route.scopeTaskId:undefined,taskContextId:route.taskContextId&&taskIds.includes(route.taskContextId)?route.taskContextId:undefined});onClose();}}finally{setBusy(false);}};
 return <Dialog title={t("coForkDialog")} onClose={onClose}>
  <div className="judex-collab-context-line">{t("coForkSource")}：{text(topic.title)} · {t("coForkPoint",{seq:afterSeq??topic.lastMessageSeq??topic.messages.at(-1)?.seq??topic.messages.length})}</div>
  <Field label={t("coForkTitle")}><UIInput value={title} onChange={e=>setTitle(e.target.value)} aria-label={t("coForkTitle")}/></Field>
  <LinkedObjectPicker plans={planIds} tasks={taskIds} onPlans={setPlans} onTasks={setTasks}/>
  <div className="judex-collab-dialog-actions"><Button variant="outline" onPress={onClose}>{t("cancel")}</Button><Button variant="primary" disabled={busy||!title.trim()} onPress={()=>void submit()} data-testid="collaboration-create-fork">{t("coForkCreate")}</Button></div>
 </Dialog>;
}
export function TaskDiscussionDialog({task,onClose}:{task:Task;onClose:()=>void}){
 const {state,t,text,go}=useWork();
 const plan=state.plans.find(p=>p.id===task.planId);
 const topics=state.topics.filter(v=>v.projectId===task.projectId&&(v.id===plan?.mainTopicId||v.taskIds.includes(task.id)));
 return <Dialog title={t("coDiscussionTarget")} onClose={onClose}><div className="judex-collab-dialog-topics">{topics.map(v=><Button key={v.id} variant="outline" onPress={()=>{go({view:"topic",id:v.id,taskContextId:task.id});onClose();}}>{v.id===plan?.mainTopicId?t("coMainDiscussion"):text(v.title)}</Button>)}</div></Dialog>;
}
export function SuggestionDialog({suggestion,mode,onClose}:{suggestion:DiscussionSuggestion;mode:"create"|"link";onClose:()=>void}){
 const {state,t,text,act,go}=useWork();
 const task=state.tasks.find(v=>v.id===suggestion.taskId),plan=state.plans.find(v=>v.id===suggestion.planId);
 const [title,setTitle]=useState(suggestion.title),[target,setTarget]=useState(suggestion.suggestedTopicId??""),[validTarget,setValidTarget]=useState(false),[busy,setBusy]=useState(false);
 const [planIds,setPlans]=useState(suggestion.planId?[suggestion.planId]:[]),[taskIds,setTasks]=useState([suggestion.taskId]);
 const submit=async()=>{if(busy||mode==="link"&&!validTarget)return;setBusy(true);try{const r=await act("resolveDiscussionSuggestion",{suggestionId:suggestion.id,expectedVersion:suggestion.version,mode,title,topicId:target||undefined,...(mode==="create"?{planIds,taskIds}:{})},{toast:false});if(r.ok){if(r.id)go({page:"chat",view:"topic",id:r.id,conversation:r.id,scopePlanId:undefined,scopeTaskId:suggestion.taskId,taskContextId:suggestion.taskId});onClose();}}finally{setBusy(false);}};
 return <Dialog wide={mode==="create"} title={t(mode==="create"?"coSeparate":"coSelectExisting")} onClose={onClose}>
  <div className="judex-collab-context-line">{t("coSourceRecord")}：{text(plan?.title??"")} › {text(task?.title??"")}</div>
  {mode==="create"?<><Field label={t("coTopicTitle")}><UIInput value={title} onChange={e=>setTitle(e.target.value)} aria-label={t("coTopicTitle")}/></Field><LinkedObjectPicker plans={planIds} tasks={taskIds} onPlans={setPlans} onTasks={setTasks} requiredPlans={suggestion.planId?[suggestion.planId]:[]} requiredTasks={[suggestion.taskId]}/></>:<Field label={t("coReplyTarget")}><DiscussionPicker value={target} onChange={setTarget} onValidityChange={setValidTarget}/></Field>}
  <div className="judex-collab-dialog-actions"><Button variant="outline" onPress={onClose}>{t("cancel")}</Button><Button variant="primary" disabled={busy||(mode==="create"?!title.trim():!validTarget)} onPress={()=>void submit()} data-testid="collaboration-resolve-suggestion">{t(mode==="create"?"chatNew":"coLinkContinue")}</Button></div>
 </Dialog>;
}
