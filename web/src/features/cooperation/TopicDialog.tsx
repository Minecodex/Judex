import {LinkedObjectPicker} from "./LinkedObjectPicker";
import {useState} from "react";
import {useQuery,useQueryClient} from "@tanstack/react-query";
import {request} from "../../lib/api/client";
import {apiWsKey,mapTopic,type ApiTopic} from "../work/apiModel";
import {Button} from "../../components/ui/Button";
import {UIInput,UIWarning} from "../../components/ui/FormControls";
import {Dialog,Field} from "../work/ui";
import {useWork} from "../work/store";
import type {Topic} from "../work/types";

export function TopicDialog({onClose}:{onClose:()=>void}){
 const {state,project,route,t,mode}=useWork(),id=route.editTopicId;
 const query=useQuery({queryKey:[...apiWsKey(project.id,state.currentUserId),"topic",id],enabled:mode==="api"&&!!id,queryFn:()=>request<ApiTopic>(`/projects/${project.id}/topics/${id}`)});
 const topic=mode==="api"&&id?(query.data?mapTopic(query.data,project.id,[]):undefined):state.topics.find(v=>v.id===id);
 if(id&&!topic)return <Dialog title={t("coopLinkObjects")} wide onClose={onClose}>{query.isError?<UIWarning role="alert">{t("errNetwork")}<Button onPress={()=>void query.refetch()}>{t("shellRetry")}</Button></UIWarning>:<p role="status">{t("shellLoading")}</p>}</Dialog>;
 return <TopicForm key={id??"new"} topic={topic} onClose={onClose}/>;
}

function TopicForm({topic,onClose}:{topic?:Topic;onClose:()=>void}){
 const {state,project,route,t,text,act,go,mode}=useWork(),client=useQueryClient();
 const [title,setTitle]=useState(topic?text(topic.title):""),[plans,setPlans]=useState(topic?.planIds??(route.scopePlanId?[route.scopePlanId]:[])),[tasks,setTasks]=useState(topic?.taskIds??(route.scopeTaskId?[route.scopeTaskId]:[])),[version,setVersion]=useState(topic?.linksVersion??1),[busy,setBusy]=useState(false),[conflict,setConflict]=useState(false),[reloadFailed,setReloadFailed]=useState(false);
 const save=async()=>{
  if(busy||conflict)return;setBusy(true);
  try{
   const result=topic?await act("replaceTopicLinks",{topicId:topic.id,expectedLinksVersion:version,planIds:plans,taskIds:tasks},{toast:false}):await act("createDiscussion",{projectId:project.id,title,planIds:plans,taskIds:tasks},{toast:false});
   if(!result.ok){setConflict(result.errorCode==="VERSION_CONFLICT"||mode==="demo");return;}
   onClose();if(!topic)go({page:"chat",view:"topic",id:result.id,conversation:result.id,editor:undefined,editTopicId:undefined});
  }finally{setBusy(false);}
 };
 const reload=async()=>{
  if(!topic)return;setBusy(true);setReloadFailed(false);
  try{
   const raw=mode==="api"?await request<ApiTopic>(`/projects/${project.id}/topics/${topic.id}`):undefined,current=raw?mapTopic(raw,project.id,[]):topic;
   if(raw)client.setQueryData([...apiWsKey(project.id,state.currentUserId),"topic",topic.id],raw);
   setVersion(current.linksVersion??1);setPlans(current.planIds);setTasks(current.taskIds);setConflict(false);
  }catch{setReloadFailed(true);}finally{setBusy(false);}
 };
 return <Dialog wide title={t(topic?"coopLinkObjects":"coopNewDiscussion")} onClose={onClose}>
  <p className="judex-modal-description">{t("coopAssociationHint")}</p><Field label={t("workTitle")}><UIInput data-testid="discussion-title" aria-label={t("workTitle")} value={title} readOnly={!!topic} onChange={e=>setTitle(e.target.value)}/></Field>
  <LinkedObjectPicker plans={plans} tasks={tasks} onPlans={setPlans} onTasks={setTasks} requiredPlans={topic?state.plans.filter(p=>p.mainTopicId===topic.id).map(p=>p.id):route.scopePlanId?[route.scopePlanId]:[]} requiredTasks={topic?state.tasks.filter(t=>t.mainTopicId===topic.id).map(t=>t.id):route.scopeTaskId?[route.scopeTaskId]:[]}/>
  {conflict&&<UIWarning role="alert">{t("coopLinkConflict")}{topic&&<Button data-testid="topic-recheck" isPending={busy} onPress={()=>void reload()}>{t("coReviewLatestLinks")}</Button>}</UIWarning>}
  {reloadFailed&&<UIWarning role="alert">{t("errNetwork")}</UIWarning>}
  <div className="judex-modal-actions"><Button onPress={onClose}>{t("cancel")}</Button><Button variant="primary" data-testid={topic?"save-topic-links":"create-discussion"} disabled={!title.trim()||conflict} isPending={busy} onPress={()=>void save()}>{t(topic?"coSaveTopicLinks":"coopNewDiscussion")}</Button></div>
 </Dialog>;
}
