import { useCallback, useEffect, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { usePreferences } from "../../stores/preferences";
import { Button } from "../../components/ui/Button";
import { translate, type Key } from "../../i18n";
import { useProjectEvents } from "../../lib/api/sse";
import { useSplitLayoutBase } from "../chat/useSplitLayout";
import { WorkspaceSidebar } from "./Sidebar";
import { ConversationPane } from "./ConversationPane";
import { Inspector } from "./Inspector";
import {
  closeTab,
  openTab,
  parseTabKey,
  tabKey,
  type ConversationTab,
  type PanelView,
  type TabState,
} from "./tabs";
import { useBootstrap, useTopics } from "./api";

// 三栏聊天工作区（真实 API 版）：左侧项目/会话，中央会话标签，
// 右侧工作/资料/团队面板。布局复用 demo 聊天树的样式与拖拽逻辑，
// 数据全部来自 REST + SSE（plans v1 08 §5）。
export function WorkspaceApp({
  projectId,
  onExit,
  onLogout,
}: {
  projectId: string;
  onExit: () => void;
  onLogout: () => void;
}) {
  const { locale, theme, setLocale, setTheme } = usePreferences();
  const t = useCallback(
    (key: Key, values?: Record<string, string | number>) =>
      translate(locale, key, values),
    [locale],
  );
  const splitT = useCallback(
    (key: "chatResizeLeft" | "chatResizeRight" | "chatResizeHint") =>
      translate(locale, key),
    [locale],
  );

  const bootstrap = useBootstrap(projectId);
  const topics = useTopics(projectId);
  const live = useProjectEvents(projectId, bootstrap.data?.eventCursor);
  const client = useQueryClient();

  const [tabsState, setTabsState] = useState<TabState>(() => {
    const parsed = parseTabKey(new URLSearchParams(location.search).get("chat"));
    return { tabs: [{ kind: "home" }, ...(parsed.kind === "home" ? [] : [parsed])], active: parsed };
  });
  const [panelOpen, setPanelOpen] = useState(true);
  const [panel, setPanel] = useState<PanelView>(() => {
    const value = new URLSearchParams(location.search).get("panel");
    return value === "materials" || value === "team" ? value : "work";
  });
  const split = useSplitLayoutBase(panelOpen, splitT);

  const openConversation = useCallback((tab: ConversationTab) => {
    split.restore();
    setTabsState((state) => openTab(state, tab));
  }, [split]);

  const closeConversation = useCallback((key: string) => {
    setTabsState((state) => {
      const next = closeTab(state, key);
      return next;
    });
  }, []);

  // 标签与面板状态同步到 URL（可刷新恢复当前会话）。
  useEffect(() => {
    const q = new URLSearchParams(location.search);
    q.set("project", projectId);
    q.set("chat", tabKey(tabsState.active));
    if (panel !== "work") q.set("panel", panel);
    else q.delete("panel");
    history.replaceState(null, "", "/?" + q.toString());
  }, [projectId, tabsState.active, panel]);

  useEffect(() => {
    document.title =
      "Judex · " + (bootstrap.data?.project.title ?? t("wsWorkspace"));
  }, [bootstrap.data, locale, t]);

  const openPanel = useCallback((view: PanelView) => {
    setPanel(view);
    setPanelOpen(true);
  }, []);

  const refreshAll = useCallback(() => {
    void client.invalidateQueries({ queryKey: ["topics", projectId] });
    void client.invalidateQueries({ queryKey: ["messages", projectId] });
    void client.invalidateQueries({ queryKey: ["handoffs", projectId] });
  }, [client, projectId]);

  const topicTitles = useMemo(() => {
    const map = new Map<string, string>();
    for (const topic of topics.data?.items ?? []) map.set(topic.id, topic.title);
    return map;
  }, [topics.data]);

  return (
    <div
      className={
        "judex-chat-app" +
        (!panelOpen ? " judex-chat-focused" : "") +
        (split.expanded && panelOpen ? " judex-chat-expanded" : "")
      }
      data-theme={theme}
      ref={split.ref}
      data-resizing={split.resizing}
      style={{ gridTemplateColumns: split.columns }}
    >
      <WorkspaceSidebar
        projectId={projectId}
        projectTitle={bootstrap.data?.project.title ?? t("shellLoading")}
        pendingCount={bootstrap.data?.pendingActionsCount ?? 0}
        topics={topics.data?.items ?? []}
        live={live}
        activeKey={tabKey(tabsState.active)}
        onOpen={openConversation}
        onExit={onExit}
        onOpenPanel={openPanel}
        account={
          <AccountFooter
            displayName={""}
            onLogout={onLogout}
            locale={locale}
            setLocale={setLocale}
            theme={theme}
            setTheme={setTheme}
          />
        }
      />
      {split.divider("left")}
      <ConversationPane
        projectId={projectId}
        tabs={tabsState}
        topicTitles={topicTitles}
        panelOpen={panelOpen}
        onTogglePanel={() => setPanelOpen((value) => !value)}
        onOpen={openConversation}
        onClose={closeConversation}
        onRefresh={refreshAll}
      />
      {panelOpen && !split.expanded && split.divider("right")}
      {panelOpen && (
        <Inspector
          projectId={projectId}
          view={panel}
          onView={setPanel}
          expanded={split.expanded}
          onExpand={split.toggleWide}
          onClose={() => setPanelOpen(false)}
        />
      )}
    </div>
  );
}

// 左栏底部账户行：语言/主题切换 + 登出（与壳层一致的能力，全屏后不丢失）。
function AccountFooter({
  displayName,
  onLogout,
  locale,
  setLocale,
  theme,
  setTheme,
}: {
  displayName: string;
  onLogout: () => void;
  locale: string;
  setLocale: (locale: "zh-CN" | "en") => void;
  theme: string;
  setTheme: (theme: "dark" | "light") => void;
}) {
  const t = (key: Key) => translate(locale as "zh-CN" | "en", key);
  return (
    <footer className="judex-chat-account">
      <span className="judex-chat-account-user" title={displayName}>
        {displayName || "—"}
      </span>
      <div className="judex-chat-account-actions">
        <Button
          variant="ghost"
          aria-label={locale === "en" ? "中文" : "EN"}
          onClick={() => setLocale(locale === "en" ? "zh-CN" : "en")}
        >
          {locale === "en" ? "中文" : "EN"}
        </Button>
        <Button
          variant="ghost"
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
        >
          {theme === "dark" ? "☀" : "☾"}
        </Button>
        <Button variant="ghost" onClick={onLogout}>
          {t("logout")}
        </Button>
      </div>
    </footer>
  );
}
