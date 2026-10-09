import type { Route, View, WorkState } from "../work/types.ts";
export type WorkspaceTab = { view: View; id?: string;section?:import('../work/runtimeTypes').TaskSection };
const views: View[] = [
  "plans",
  "plan",
  "task",
  "resources",
];
export function workspaceRoute(
  route: Pick<Route, "view" | "id"> & {taskSection?:import('../work/runtimeTypes').TaskSection;section?:import('../work/runtimeTypes').TaskSection;activityId?:string},
): WorkspaceTab | null {
  const view=route.view==="tasks"||route.view==="overview"?"plans":route.view;
  return views.includes(view)
    ? {view,id:["plan","task"].includes(view)?route.id:undefined,...(view==='task'?{section:route.activityId?'records' as const:route.taskSection??route.section??'overview' as const}:{})}
    : null;
}
export const workspaceKey = (tab: WorkspaceTab) =>
  tab.view === "settings"
    ? "settings:" + (tab.id ?? "preferences")
    : tab.view==='task'? 'task:'+tab.id+':'+(tab.section??'overview'):["plan"].includes(tab.view)
      ? tab.view + ":" + tab.id
      : tab.view;
export function workspaceTabValid(
  s: WorkState,
  projectId: string,
  tab: WorkspaceTab,
) {
  if (!tab || !views.includes(tab.view)) return false;
  if (tab.view === "task")
    return s.tasks.some((v) => v.projectId === projectId && v.id === tab.id)||!!s.currentUserId&&/^[0-9a-f-]{36}$/i.test(tab.id??"");
  if (tab.view === "plan")
    return s.plans.some((v) => v.projectId === projectId && v.id === tab.id)||!!s.currentUserId&&/^[0-9a-f-]{36}$/i.test(tab.id??"");
  if (tab.view === "settings")
    return !tab.id || ["project", "local"].includes(tab.id);
  return true;
}
export function addWorkspaceTab(tabs: WorkspaceTab[], tab: WorkspaceTab) {
  const key = workspaceKey(tab);
  return tabs.some((t) => workspaceKey(t) === key)
    ? tabs.map((t) => (workspaceKey(t) === key ? tab : t))
    : [...tabs, tab];
}
export function removeWorkspaceTab(
  tabs: WorkspaceTab[],
  key: string,
  active: string | null,
) {
  const index = tabs.findIndex((t) => workspaceKey(t) === key),
    next = tabs.filter((t) => workspaceKey(t) !== key);
  return {
    tabs: next,
    active:
      active === key
        ? next[Math.min(index, next.length - 1)]
          ? workspaceKey(next[Math.min(index, next.length - 1)])
          : null
        : active,
  };
}
