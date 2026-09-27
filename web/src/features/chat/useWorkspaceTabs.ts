import { useEffect, useState } from "react";
import { useWork } from "../work/store";
import {
  workspaceRoute,
  workspaceKey,
  workspaceTabValid,
  addWorkspaceTab,
  removeWorkspaceTab,
  type WorkspaceTab,
} from "./workspaceTabModel";
type Session = { scope: string; tabs: WorkspaceTab[]; active: string | null };
export function useWorkspaceTabs() {
  const { state, project, route, go } = useWork(),
    scope = project.id + ":" + state.currentUser,
    key = "judex.workspace.tabs.v1." + scope;
  const valid = (tab: WorkspaceTab) =>
    workspaceTabValid(state, project.id, tab);
  const incoming = workspaceRoute(route),
    target = incoming && valid(incoming) ? incoming : null;
  const signature = route.view + ":" + (route.id ?? "");
  const read = (): Session => {
    try {
      const raw = JSON.parse(sessionStorage.getItem(key) || "{}");
      const tabs = (Array.isArray(raw.tabs) ? raw.tabs : [])
        .filter(valid)
        .reduce(
          (result: WorkspaceTab[], t: WorkspaceTab) =>
            addWorkspaceTab(result, t),
          [],
        );
      return {
        scope,
        tabs,
        active: tabs.some((t: WorkspaceTab) => workspaceKey(t) === raw.active)
          ? raw.active
          : null,
      };
    } catch {
      return { scope, tabs: [], active: null };
    }
  };
  const fromRoute = (session: Session): Session =>
    target
      ? {
          ...session,
          tabs: addWorkspaceTab(session.tabs, target),
          active: workspaceKey(target),
        }
      : route.view === "workspace"
        ? { ...session, active: null }
        : session;
  const [stored, setStored] = useState(() => fromRoute(read()));
  const session = stored.scope === scope ? stored : fromRoute(read());
  const tabs = session.tabs.filter(valid),
    active = tabs.some((t) => workspaceKey(t) === session.active)
      ? session.active
      : null;
  useEffect(() => {
    setStored((prev) => fromRoute(prev.scope === scope ? prev : read()));
  }, [scope, signature]);
  useEffect(() => {
    if (stored.scope === scope)
      try {
        sessionStorage.setItem(key, JSON.stringify(stored));
      } catch {}
  }, [stored, key]);
  const open = (tab: WorkspaceTab) => {
    const value = workspaceRoute(tab);
    if (!value || !valid(value)) return;
    setStored({
      scope,
      tabs: addWorkspaceTab(tabs, value),
      active: workspaceKey(value),
    });
    go({ view: value.view, id: value.id });
  };
  const launcher = () => {
    setStored({ scope, tabs, active: null });
    go({ view: "workspace", id: undefined });
  };
  const close = (key: string) => {
    const next = removeWorkspaceTab(tabs, key, active);
    setStored({ scope, ...next });
    if (next.active !== active) {
      const target = next.tabs.find((t) => workspaceKey(t) === next.active);
      if (target) go({ view: target.view, id: target.id });
      else go({ view: "workspace", id: undefined });
    }
  };
  return {
    scope,
    tabs,
    active,
    open,
    close,
    launcher,
    current: tabs.find((t) => workspaceKey(t) === active),
  };
}
