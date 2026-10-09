import {useEffect,useState} from "react";
import {useWork} from "../work/store";
import {HOME_TAB,tabKey,routeTab,openTab,closeTab,conversationTabDestination,type ConversationTab} from "./conversationTabs";
import {storedConversationContext} from './conversationDraft';
export function useConversationTabs(){
 const {state,project,route,go,mode}=useWork(),actor=state.currentUserId??state.currentUser,scope=project.id+":"+actor,key="judex.chat.tabs.v1."+scope;
 const requested={...routeTab(route),scopePlanId:route.scopePlanId,scopeTaskId:route.scopeTaskId};
 const scoped=!!route.scopePlanId||!!route.scopeTaskId;
 const valid=(tab:ConversationTab)=>tab.kind==="home"?!scoped:!!tab.id&&(mode==="api"|| (tab.kind==="topic"?state.topics:state.handoffs).some(v=>v.projectId===project.id&&v.id===tab.id));
 const current=valid(requested)?requested:HOME_TAB,active=tabKey(current);
 const read=()=>{try{const raw=JSON.parse(sessionStorage.getItem(key)||"[]");return (Array.isArray(raw)?raw:[]).filter(valid).reduce<ConversationTab[]>((a,b)=>openTab(a,b),[]);}catch{return [];}};
 const [stored,setStored]=useState(()=>({scope,tabs:openTab(read(),current)}));
 const tabs=openTab((stored.scope===scope?stored.tabs:read()).filter(valid),current);
 useEffect(()=>{setStored(prev=>({scope,tabs:openTab((prev.scope===scope?prev.tabs:read()).filter(valid),current)}));},[scope,active,route.scopePlanId,route.scopeTaskId]);
 useEffect(()=>{if(stored.scope===scope)try{sessionStorage.setItem(key,JSON.stringify(stored.tabs.filter(valid)));}catch{}},[stored,key]);
 const navigate=(tab:ConversationTab)=>{
  const context=storedConversationContext(scope+':'+tab.id);
  go(conversationTabDestination(tab,route,context?.kind==='task'?context.id:context?undefined:tab.scopeTaskId));
 };
 const open=(tab:ConversationTab)=>{const existing=tabs.find(v=>tabKey(v)===tabKey(tab));const value={...existing,...tab};setStored({scope,tabs:openTab(tabs,value)});navigate(value);};
 const close=(key:string)=>{const result=closeTab(tabs,key,active);setStored({scope,tabs:result.tabs});const target=result.tabs.find(t=>tabKey(t)===result.active);if(target)navigate(target);else go({page:"hub",view:"plans",id:undefined,scopePlanId:undefined,scopeTaskId:undefined});};
 return {tabs,active,current,open,close};
}
