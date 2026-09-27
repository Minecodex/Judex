import type { Route } from "../work/types.ts";
export type ConversationTab = {
  kind: "home" | "topic" | "handoff";
  id?: string;
};
export const HOME_TAB: ConversationTab = { kind: "home" };
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
  return tabs.some((t) => tabKey(t) === tabKey(tab)) ? tabs : [...tabs, tab];
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
