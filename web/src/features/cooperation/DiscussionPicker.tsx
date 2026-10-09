import {useEffect,useState} from "react";
import {useQuery} from "@tanstack/react-query";
import {UIInput,UISelect,UIOption,UIWarning} from "../../components/ui/FormControls";
import {Button} from "../../components/ui/Button";
import {useCollection,LoadMore} from "../../lib/api/collections";
import {request} from "../../lib/api/client";
import {useWork} from "../work/store";
import {apiWsKey,mapTopic,type ApiTopic} from "../work/apiModel";

// Choices are read independently of the conversation sidebar's current scope.
export function DiscussionPicker({value,onChange,onValidityChange}:{value:string;onChange:(id:string)=>void;onValidityChange:(valid:boolean)=>void}){
 const {state,project,mode,t,text}=useWork(),[search,setSearch]=useState(""),[querySearch,setQuery]=useState("");
 useEffect(()=>{const timer=setTimeout(()=>setQuery(search.trim()),200);return()=>clearTimeout(timer);},[search]);
 const prefix=apiWsKey(project.id,state.currentUserId),query=useCollection<ApiTopic>([...prefix,"discussionPicker",querySearch],`/projects/${project.id}/topics?q=${encodeURIComponent(querySearch)}`,mode==="api");
 const selected=useQuery({queryKey:[...prefix,"topic",value],enabled:mode==="api"&&!!value,queryFn:()=>request<ApiTopic>(`/projects/${project.id}/topics/${value}`)});
 const choices=mode==="api"?(query.data?.items??[]).map(v=>mapTopic(v,project.id,[])):state.topics.filter(v=>v.projectId===project.id&&v.kind!=="handoff"&&text(v.title).toLowerCase().includes(querySearch.toLowerCase()));
 const chosen=mode==="api"?(selected.data?mapTopic(selected.data,project.id,[]):undefined):state.topics.find(v=>v.projectId===project.id&&v.id===value&&v.kind!=="handoff");
 const items=[...new Map([...(chosen&&chosen.kind!=="handoff"?[chosen]:[]),...choices].map(v=>[v.id,v])).values()];
 const valid=!!value&&items.some(v=>v.id===value&&v.kind!=="handoff")&&!(mode==="api"&&selected.isError);
 useEffect(()=>onValidityChange(valid),[valid,onValidityChange]);
 return <div className="judex-co-task-facts">
  <UIInput data-testid="suggestion-topic-search" aria-label={t("coSearchExistingTopics")} placeholder={t("coSearchExistingTopics")} value={search} onChange={e=>setSearch(e.target.value)}/>
  {mode==="api"&&query.isPending&&<p role="status">{t("shellLoading")}</p>}
  {mode==="api"&&query.isError&&<UIWarning role="alert">{t("errNetwork")}<Button size="sm" onPress={()=>void query.refetch()}>{t("shellRetry")}</Button></UIWarning>}
  <UISelect aria-label={t("coReplyTarget")} data-testid="suggestion-topic-select" value={value} onChange={e=>onChange(e.target.value)}><UIOption value="">{t("coSelectTopic")}</UIOption>{items.map(v=><UIOption key={v.id} value={v.id}>{text(v.title)}</UIOption>)}</UISelect>
  {mode==="api"&&selected.isError&&<UIWarning role="alert">{t("coSelectedDiscussionUnavailable")}<Button size="sm" onPress={()=>void selected.refetch()}>{t("shellRetry")}</Button></UIWarning>}
  {!items.length&&!(mode==="api"&&(query.isPending||query.isError))&&<p className="judex-co-meta">{t("coNoMatches")}</p>}
  {mode==="api"&&<LoadMore query={query}/>}
 </div>;
}
