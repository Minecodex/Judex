import type { Route, View, WorkState } from "../work/types.ts";
export type WorkspaceTab = { view: View; id?: string };
const views: View[] = [
  "overview",
  "plans",
  "tasks",
  "plan",
  "task",
  "resources",
  "decisions",
  "handoffs",
];
export function workspaceRoute(
  route: Pick<Route, "view" | "id">,
): WorkspaceTab | null {
  return views.includes(route.view)
    ? { view: route.view === "tasks" ? "plans" : route.view, id: route.id }
    : null;
}
export const workspaceKey = (tab: WorkspaceTab) =>
  tab.view === "settings"
    ? "settings:" + (tab.id ?? "preferences")
    : ["task", "plan"].includes(tab.view)
      ? tab.view + ":" + tab.id
      : tab.view;
export function workspaceTabValid(
  s: WorkState,
  projectId: string,
  tab: WorkspaceTab,
) {
  if (!tab || !views.includes(tab.view)) return false;
  if (tab.view === "task")
    return s.tasks.some((v) => v.projectId === projectId && v.id === tab.id);
  if (tab.view === "plan")
    return s.plans.some((v) => v.projectId === projectId && v.id === tab.id);
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
