import {parseCooperationRoute,cooperationURL} from "../cooperation/routing";
import { useEffect, useState,useRef } from "react";
import type { Route, Text, View } from "./types";
import { translate, type Key } from "../../i18n";
import { usePreferences } from "../../stores/preferences";
import {
  settingsDestination,
  settingsSections,
  type SettingsSection,
} from "../settings/navigation";

const views: View[] = [
  "proposal",
  "workspace",
  "overview",
  "home",
  "plans",
  "plan",
  "tasks",
  "task",
  "handoffs",
  "handoff",
  "topics",
  "topic",
  "team",
  "flows",
  "decisions",
  "settings",
  "resources",
];

export function readRoute(): Route {
  const q = new URLSearchParams(location.search);
  const view = views.includes(q.get("view") as View)
    ? (q.get("view") as View)
    : "home";
  const destination = settingsDestination(view, q.get("item") || undefined);
  return {
    design: "studio",
    projectId: q.get("project") || "leaf",
    view: destination ? "home" : view,
    id: destination ? undefined : q.get("item") || undefined,
    conversation: q.get("chat")?.replace(/^topic:/,"") || undefined,
 taskContextId:q.get("taskContext")||undefined,
 activityId:q.get("activity")||undefined,
 messageSeq:q.has("message")?Number(q.get("message")):undefined,
    settingsSection:
      destination?.section ??
      (settingsSections.includes(q.get("settings") as SettingsSection)
        ? (q.get("settings") as SettingsSection)
        : undefined),
    settingsItem: destination?.item ?? q.get("settingsItem") ?? undefined,
    ...parseCooperationRoute(location.pathname,location.search),
  };
}

// 与数据源无关的工作台外壳：路由、toast、语言/主题与文案助手，demo/api 共用。
export function useWorkbenchBase() {
  const [route, setRoute] = useState(readRoute),
    [toast, setToast] = useState("");
  const routeRef=useRef(route);routeRef.current=route;
  const { locale, setLocale, theme, setTheme } = usePreferences();
  const t = (key: Key, values?: Record<string, string | number>) =>
    translate(locale, key, values);
  const text = (value: Text | string) =>
    typeof value === "string" ? value : locale === "en" ? value.en : value.zh;
  useEffect(() => {
    document.title = "Judex · " + t("workStudio");
  }, [locale]);
  useEffect(() => {
    const pop = () => setRoute(readRoute());
    window.addEventListener("popstate", pop);
    return () => window.removeEventListener("popstate", pop);
  }, []);
  useEffect(() => {
    if (!toast) return;
    const timeout = setTimeout(() => setToast(""), 4200);
    return () => clearTimeout(timeout);
  }, [toast]);
  const go = (next: Partial<Route>) => {
    const current=routeRef.current;
    if(["decisions","handoffs","handoff","overview"].includes(next.view??"")&&!next.page)next={...next,page:"hub",hubTab:next.view==="handoff"||next.view==="handoffs"?"deliveries":"decisions",scopePlanId:undefined,scopeTaskId:undefined,conversation:undefined};
    const destination = settingsDestination(next.view, next.id);
    if (destination)
      next = {
        ...next,
        view: undefined,
        id: current.id,
        settingsSection: destination.section,
        settingsItem: destination.item,
      };
    if (next.view === undefined) delete next.view;
    const value = {
      ...current,
      ...next,
      id:
        "id" in next
          ? next.id
          : next.view && next.view !== current.view
            ? undefined
            : current.id,
    };
    if (next.view && next.view !== 'handoff' && !('sourceId' in next)) value.sourceId = undefined;
    if(next.view==="topic"&&next.id!==current.conversation&&!("taskContextId" in next))value.taskContextId=undefined;
 if(next.view==="task"&&next.id!==current.id&&!("activityId" in next))value.activityId=undefined;
 if(next.view==='task'&&next.id!==current.id&&!("taskSection" in next))value.taskSection='overview';
 if(next.activityId)value.taskSection='records';
 if (next.projectId && next.projectId !== current.projectId)
      value.conversation = undefined;
    else if (next.view === "topic") value.conversation = next.id;
    else if (next.view === "handoff") value.conversation = "handoff:" + next.id;
    else if (!("conversation" in next) && current.view === "topic")
      value.conversation = current.id;
    else if (!("conversation" in next) && current.view === "handoff")
      value.conversation = "handoff:" + current.id;
    if(next.view==="plan"&&!next.page&&current.page!=="chat"){value.page="route";value.scopePlanId=next.id;}
    if(next.page&&next.page!=="chat"){value.conversation=undefined;value.taskContextId=undefined;value.messageSeq=undefined;value.activityId=undefined;}
    if(next.view==="topic"||next.conversation){value.page="chat";}
    if(next.projectId&&next.projectId!==current.projectId){value.scopePlanId=undefined;value.scopeTaskId=undefined;value.page=next.page??"hub";}
    routeRef.current=value;
    history.pushState(null,"",cooperationURL(value));window.dispatchEvent(new PopStateEvent("popstate"));
    setRoute(value);
  };
  return {
    route,
    go,
    toast,
    setToast,
    locale,
    setLocale,
    theme,
    setTheme,
    t,
    text,
  };
}
