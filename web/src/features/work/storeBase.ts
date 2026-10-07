import { useEffect, useState } from "react";
import type { Route, Text, View } from "./types";
import { translate, type Key } from "../../i18n";
import { usePreferences } from "../../stores/preferences";
import {
  settingsDestination,
  settingsSections,
  type SettingsSection,
} from "../settings/navigation";

const views: View[] = [
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
    conversation: q.get("chat") || undefined,
    settingsSection:
      destination?.section ??
      (settingsSections.includes(q.get("settings") as SettingsSection)
        ? (q.get("settings") as SettingsSection)
        : undefined),
    settingsItem: destination?.item ?? q.get("settingsItem") ?? undefined,
  };
}

// 与数据源无关的工作台外壳：路由、toast、语言/主题与文案助手，demo/api 共用。
export function useWorkbenchBase() {
  const [route, setRoute] = useState(readRoute),
    [toast, setToast] = useState("");
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
    const destination = settingsDestination(next.view, next.id);
    if (destination)
      next = {
        ...next,
        view: undefined,
        id: route.id,
        settingsSection: destination.section,
        settingsItem: destination.item,
      };
    if (next.view === undefined) delete next.view;
    const value = {
      ...route,
      ...next,
      id:
        "id" in next
          ? next.id
          : next.view && next.view !== route.view
            ? undefined
            : route.id,
    };
    if (next.projectId && next.projectId !== route.projectId)
      value.conversation = undefined;
    else if (next.view === "topic") value.conversation = next.id;
    else if (next.view === "handoff") value.conversation = "handoff:" + next.id;
    else if (!("conversation" in next) && route.view === "topic")
      value.conversation = route.id;
    else if (!("conversation" in next) && route.view === "handoff")
      value.conversation = "handoff:" + route.id;
    const q = new URLSearchParams({ project: value.projectId });
    if (value.view !== "home") q.set("view", value.view);
    if (value.id) q.set("item", value.id);
    if (value.conversation) q.set("chat", value.conversation);
    if (value.settingsSection) q.set("settings", value.settingsSection);
    if (value.settingsItem) q.set("settingsItem", value.settingsItem);
    history.pushState(null, "", "/?" + q);
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
