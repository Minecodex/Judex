// 三栏工作区会话标签模型（纯函数，供单测）：标签以 kind:id 标识，
// 活动标签同步到 URL ?chat=，标签集合仅存内存（刷新回到当前会话）。
export type ConversationTab =
  | { kind: "home" }
  | { kind: "topic"; id: string }
  | { kind: "handoff"; id: string };

export const tabKey = (tab: ConversationTab): string =>
  tab.kind === "home" ? "home" : `${tab.kind}:${tab.id}`;

export function parseTabKey(key: string | null | undefined): ConversationTab {
  if (!key || key === "home") return { kind: "home" };
  const separator = key.indexOf(":");
  if (separator <= 0) return { kind: "home" };
  const kind = key.slice(0, separator),
    id = key.slice(separator + 1);
  if (!id) return { kind: "home" };
  if (kind === "topic" || kind === "handoff") return { kind, id };
  return { kind: "home" };
}

export type TabState = { tabs: ConversationTab[]; active: ConversationTab };

// open 返回新状态：已存在的标签复用，未知标签追加（上限 8 个，超过丢弃最旧的非活动标签）。
export function openTab(state: TabState, tab: ConversationTab): TabState {
  const key = tabKey(tab);
  const exists = state.tabs.some((item) => tabKey(item) === key);
  let tabs = exists ? state.tabs : [...state.tabs, tab];
  if (!exists && tabs.length > 8) {
    const activeKey = tabKey(state.active);
    tabs = tabs.filter(
      (item, index) => tabKey(item) === activeKey || index > 0 || item.kind === "home",
    );
  }
  return { tabs, active: tab };
}

export function closeTab(state: TabState, key: string): TabState {
  if (key === "home") return state; // 首页标签不可关闭
  const index = state.tabs.findIndex((item) => tabKey(item) === key);
  if (index < 0) return state;
  const tabs = state.tabs.filter((item) => tabKey(item) !== key);
  if (tabKey(state.active) !== key) return { tabs, active: state.active };
  const fallback = tabs[Math.max(0, index - 1)] ?? { kind: "home" } as ConversationTab;
  return { tabs, active: fallback };
}

// 右侧面板视图（URL ?view= 同步；home 视图 = 中央会话模式）。
export type PanelView = "work" | "materials" | "team";
