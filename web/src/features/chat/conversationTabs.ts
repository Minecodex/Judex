import type { Route } from "../work/types.ts";
import {workspaceRoute} from './workspaceTabModel.ts';
export type ConversationTab = {
  kind: "home" | "topic" | "handoff";
  id?: string;scopePlanId?:string;scopeTaskId?:string;
};
export const HOME_TAB: ConversationTab = { kind: "home" };
// Conversation and inspector tabs are independent surfaces. Returning to a
// discussion restores its scope without replacing the selected right tool.
export function conversationTabDestination(tab:ConversationTab,route:Pick<Route,'view'|'id'|'taskSection'|'activityId'>,taskContextId?:string):Partial<Route>{
  if(tab.kind==='home')return {page:'chat',view:'home',id:undefined,conversation:undefined,scopePlanId:undefined,scopeTaskId:undefined,taskContextId:undefined};
  const inspector=workspaceRoute(route);
  return {page:'chat',conversation:tab.kind==='handoff'?'handoff:'+tab.id:tab.id,
    view:tab.kind==='handoff'?'handoff':inspector?.view??'topic',id:tab.kind==='handoff'?tab.id:inspector?inspector.id:tab.id,
    taskSection:inspector?.section,activityId:inspector?.id===route.id?route.activityId:undefined,
    scopePlanId:tab.scopePlanId,scopeTaskId:tab.scopeTaskId,taskContextId,messageSeq:undefined};
}
export const tabKey = (tab: ConversationTab) =>
  tab.kind === "home" ? "home" : tab.kind + ":" + tab.id;
export function routeTab(route: Route): ConversationTab {
  if ((route.view === "topic" || route.view === "handoff") && route.id)
    return { kind: route.view, id: route.id };
  if (route.conversation?.startsWith("handoff:"))
    return { kind: "handoff", id: route.conversation.slice(8) };
  return route.conversation
    ? { kind: "topic", id: route.conversation }
    : HOME_TAB;
}
export function openTab(tabs: ConversationTab[], tab: ConversationTab) {
  const current=tabs.find(t=>tabKey(t)===tabKey(tab));
  if(!current)return [...tabs,tab];
  if(("scopePlanId" in tab||"scopeTaskId" in tab)&&(current.scopePlanId!==tab.scopePlanId||current.scopeTaskId!==tab.scopeTaskId))return tabs.map(t=>t===current?{...current,...tab}:t);
  return tabs;
}
export function closeTab(tabs: ConversationTab[], key: string, active: string) {
  const index = tabs.findIndex((t) => tabKey(t) === key);
  if (index < 0 || key === "home") return { tabs, active };
  const next = tabs.filter((t) => tabKey(t) !== key);
  return {
    tabs: next,
    active:
      key === active
        ? tabKey(next[Math.min(index, next.length - 1)] ?? HOME_TAB)
        : active,
  };
}
