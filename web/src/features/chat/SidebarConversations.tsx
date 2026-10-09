import {useState,useEffect} from "react";
import {Input} from "@heroui/react";
import {GitFork,MessageCircle,Search} from "lucide-react";
import {Button} from "../../components/ui/Button";
import {UIWarning} from "../../components/ui/FormControls";
import {useWork} from "../work/store";
import {apiWsKey,mapTopic,type ApiTopic} from "../work/apiModel";
import {useCollection,LoadMore} from "../../lib/api/collections";
import {topicInScope,scopeMain} from "../cooperation/scope";
export function SidebarConversations({restore}:{restore:()=>void}){
 const {state,project,route,mode,go,t,text}=useWork(),key="judex.cooperation.search."+(state.currentUserId??state.currentUser)+":"+project.id+":"+(route.scopePlanId??route.scopeTaskId??"project");
 const [saved,setSaved]=useState(()=>({key,value:sessionStorage.getItem(key)??""})),search=saved.key===key?saved.value:sessionStorage.getItem(key)??"",[querySearch,setQuery]=useState(search);
 useEffect(()=>{const timer=setTimeout(()=>setQuery(search.trim()),200);return()=>clearTimeout(timer);},[search]);
 const parameters=new URLSearchParams();if(route.scopeTaskId)parameters.set("taskId",route.scopeTaskId);else if(route.scopePlanId)parameters.set("planId",route.scopePlanId);if(querySearch)parameters.set("q",querySearch);
 const query=useCollection<ApiTopic>([...apiWsKey(project.id,state.currentUserId),"topics",route.scopePlanId??"",route.scopeTaskId??"",querySearch],`/projects/${project.id}/topics?`+parameters,mode==="api");
 const raw=mode==="api"?(query.data?.items??[]).map(v=>mapTopic(v,project.id,[])):state.topics.filter(v=>v.projectId===project.id&&v.kind!=="handoff"&&topicInScope(v,route,state.tasks)&&text(v.title).toLowerCase().includes(querySearch.toLowerCase()));
 const main=scopeMain(route,state.plans,state.tasks),pinned=state.topics.find(v=>v.id===main&&text(v.title).toLowerCase().includes(querySearch.toLowerCase()));
 const items=[...new Map([...(pinned?[pinned]:[]),...raw].map(t=>[t.id,t])).values()].sort((a,b)=>(a.id===main?-1:b.id===main?1:0));
 return <><div className="judex-chat-search judex-collab-search"><Search/><Input aria-label={t("coSearch")} placeholder={t("coSearch")} value={search} onChange={e=>{setSaved({key,value:e.target.value});sessionStorage.setItem(key,e.target.value);}}/></div><div className="judex-chat-section-label"><span>{t("chatConversations")}</span>{mode==="demo"?<span>{items.length}</span>:query.data?.totalCount!==undefined?<span>{query.data.totalCount}</span>:null}</div><nav className="judex-chat-conversation-list judex-collab-nav">
{mode==="api"&&query.isError?<UIWarning>{t("errNetwork")}<Button onPress={()=>void query.refetch()}>{t("shellRetry")}</Button></UIWarning>:items.map(topic=><Button key={topic.id} className={"judex-collab-nav-row"+(route.conversation===topic.id?" judex-chat-selected":"")} data-testid={"scoped-topic-"+topic.id} onPress={()=>{restore();go({page:"chat",view:"topic",id:topic.id,conversation:topic.id,taskContextId:route.scopeTaskId});}} title={text(topic.title)}>{topic.parentTopicId?<GitFork size={16}/>:<MessageCircle size={16}/>}<span>{text(topic.title)}</span>{topic.id===main&&<small>{t("coopMain")}</small>}</Button>)}
{!items.length&&!(mode==="api"&&query.isPending)&&<p className="judex-collab-empty">{t("coopNoConversations")}</p>}{mode==="api"&&<LoadMore query={query}/>}</nav></>;
}