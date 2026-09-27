import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { seedWork, WORK_KEY } from "./seed";
import { normalizeWork } from "./normalize";
import { allPeople, manage, member } from "./selectors";
import type { Route, View, Text, WorkState, Result } from "./types";
import { translate, type Key } from "../../i18n";
import { usePreferences } from "../../stores/preferences";
import {
  loadWorkspace,
  persistPreview,
  workspaceKey,
} from "../../lib/api/workspace";
import { dataMode } from "../../lib/api/client";
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
function readRoute(): Route {
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
function initialPreview() {
  try {
    return (
      normalizeWork(JSON.parse(localStorage.getItem(WORK_KEY) || "null")) ||
      seedWork()
    );
  } catch {
    return seedWork();
  }
}
function useWorkbench() {
  const client = useQueryClient();
  const query = useQuery({
    queryKey: workspaceKey,
    queryFn: loadWorkspace,
    initialData: initialPreview,
    staleTime: Infinity,
  });
  const state = query.data;
  const ref = useRef(state);
  ref.current = state;
  const [route, setRoute] = useState(readRoute),
    [toast, setToast] = useState(""),
    [storageError, setStorageError] = useState(false);
  const { locale, setLocale, theme, setTheme } = usePreferences();
  const t = (key: Key, values?: Record<string, string | number>) =>
    translate(locale, key, values);
  const text = (value: Text | string) =>
    typeof value === "string" ? value : locale === "en" ? value.en : value.zh;
  const commit = (next: WorkState) => {
    try {
      persistPreview(next);
      ref.current = next;
      client.setQueryData(workspaceKey, next);
      setStorageError(false);
      return true;
    } catch {
      setStorageError(true);
      return false;
    }
  };
  useEffect(() => {
    document.title = "Judex · " + t("workStudio");
  }, [locale]);
  useEffect(() => {
    if (dataMode === "demo") {
      try {
        if (!localStorage.getItem(WORK_KEY)) persistPreview(ref.current);
      } catch {
        setStorageError(true);
      }
    }
  }, []);
  useEffect(() => {
    const pop = () => setRoute(readRoute());
    const sync = (e: StorageEvent) => {
      if (e.key !== WORK_KEY || !e.newValue) return;
      try {
        const next = normalizeWork(JSON.parse(e.newValue));
        if (next)
          client.setQueryData(workspaceKey, {
            ...next,
            currentUser: ref.current.currentUser,
          });
      } catch {
        /* Ignore invalid preview snapshots. */
      }
    };
    window.addEventListener("popstate", pop);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener("popstate", pop);
      window.removeEventListener("storage", sync);
    };
  }, [client]);
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
  const act = (fn: (s: WorkState) => Result, success = true) => {
    if (dataMode !== "demo") {
      setToast(t("shellUnavailable"));
      return false;
    }
    const result = fn(ref.current);
    if (result.error) {
      setToast(
        t(
          ("workError" +
            result.error[0].toUpperCase() +
            result.error.slice(1)) as Key,
        ),
      );
      return false;
    }
    if (!commit(result.state!)) return false;
    if (success) setToast(t("workSaved"));
    return true;
  };
  const switchPerson = (name: string) => {
    if (commit({ ...ref.current, currentUser: name }))
      go({ view: "home", id: undefined });
  };
  const reset = () => {
    if (commit(seedWork())) {
      go({ projectId: "leaf", view: "home", id: undefined });
      setToast(t("resetDone"));
    }
  };
  const project =
    state.projects.find((p) => p.id === route.projectId) || state.projects[0];
  return {
    state,
    route,
    go,
    locale,
    setLocale,
    theme,
    setTheme,
    toast,
    setToast,
    storageError,
    text,
    t,
    act,
    switchPerson,
    reset,
    project,
    membership: member(state, project.id),
    management: manage(state, project.id),
    people: allPeople(state),
  };
}
const Context = createContext<ReturnType<typeof useWorkbench> | null>(null);
export function WorkProvider({ children }: { children: ReactNode }) {
  return <Context.Provider value={useWorkbench()}>{children}</Context.Provider>;
}
export function useWork() {
  const value = useContext(Context);
  if (!value) throw new Error("WorkProvider required");
  return value;
}

export function WorkView({
  view,
  id,
  children,
}: {
  view: View;
  id?: string;
  children: ReactNode;
}) {
  const base = useWork();
  return (
    <Context.Provider value={{ ...base, route: { ...base.route, view, id } }}>
      {children}
    </Context.Provider>
  );
}
