import { useEffect, useState } from "react";
import { useWork } from "../work/store";
import {
  HOME_TAB,
  tabKey,
  routeTab,
  openTab,
  closeTab,
  type ConversationTab,
} from "./conversationTabs";
export function useConversationTabs() {
  const { state, project, route, go } = useWork();
  const scope = project.id + ":" + state.currentUser,
    key = "judex.chat.tabs.v1." + scope;
  const valid = (tab: ConversationTab) =>
    tab.kind === "home" ||
    (tab.kind === "topic" ? state.topics : state.handoffs).some(
      (t) => t.projectId === project.id && t.id === tab.id,
    );
  const requested = routeTab(route),
    current = valid(requested) ? requested : HOME_TAB,
    active = tabKey(current);
  const read = () => {
    try {
      const raw = JSON.parse(sessionStorage.getItem(key) || "[]");
      return [
        HOME_TAB,
        ...(Array.isArray(raw) ? raw : []).filter(
          (t) =>
            t &&
            ["topic", "handoff"].includes(t.kind) &&
            typeof t.id === "string" &&
            valid(t),
        ),
      ].reduce<ConversationTab[]>((tabs, tab) => openTab(tabs, tab), []);
    } catch {
      return [HOME_TAB];
    }
  };
  const [stored, setStored] = useState(() => ({
    scope,
    tabs: openTab(read(), current),
  }));
  const tabs = openTab(
    (stored.scope === scope ? stored.tabs : read()).filter(valid),
    current,
  );
  useEffect(() => {
    setStored((prev) => ({
      scope,
      tabs: openTab(prev.scope === scope ? prev.tabs : read(), current),
    }));
  }, [scope, active]);
  useEffect(() => {
    if (stored.scope === scope) {
      try {
        sessionStorage.setItem(key, JSON.stringify(stored.tabs.filter(valid)));
      } catch {}
    }
  }, [stored, key]);
  const open = (tab: ConversationTab) => {
    const destination = valid(tab) ? tab : HOME_TAB;
    setStored({ scope, tabs: openTab(tabs, destination) });
    const conversation =
      destination.kind === "topic"
        ? destination.id
        : destination.kind === "handoff"
          ? "handoff:" + destination.id
          : undefined;
    if (["home", "topic", "handoff", "topics"].includes(route.view))
      go({ view: destination.kind, id: destination.id, conversation });
    else go({ conversation });
  };
  const close = (key: string) => {
    const result = closeTab(tabs, key, active);
    setStored({ scope, tabs: result.tabs });
    if (result.active !== active) {
      const target =
        result.tabs.find((t) => tabKey(t) === result.active) ?? HOME_TAB;
      const conversation =
        target.kind === "topic"
          ? target.id
          : target.kind === "handoff"
            ? "handoff:" + target.id
            : undefined;
      if (["home", "topic", "handoff", "topics"].includes(route.view))
        go({ view: target.kind, id: target.id, conversation });
      else go({ conversation });
    }
  };
  return { tabs, active, current, open, close };
}
