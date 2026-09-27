import { UIWarning, UINotice } from "../../components/ui/FormControls";
import { useEffect, useState, useRef } from "react";
import { Input, Tabs } from "@heroui/react";
import { Plus, MessageCircle, Search, Check, X, Inbox } from "lucide-react";
import { useWork, WorkProvider } from "../work/store";
import { Button } from "../../components/ui/Button";
import { ProjectDialog } from "../work/ProjectPages";
import { DiscussionDialog } from "../work/CompositionDialogs";
import { InvitePage } from "../work/TeamPages";
import { HandoffPage } from "../work/HandoffPages";
import { ProjectSwitcher } from "./ProjectSwitcher";
import { useSplitLayout } from "./useSplitLayout";
import type { View } from "../work/types";
import { Conversation, type ConversationDraft } from "./Conversation";
import { WorkspaceTabs } from "./WorkspaceTabs";
import { pendingDecisionCount } from "./decisions";
import { useConversationTabs } from "./useConversationTabs";
import { ConversationTabBar } from "./ConversationTabBar";
import { useReadingPosition } from "./useReadingPosition";
import { AccountMenu } from "../settings/AccountMenu";
import { SettingsScreen } from "../settings/SettingsScreen";
function ChatShell({ onLogout }: { onLogout: () => Promise<void> }) {
  const {
    state,
    project,
    route,
    go,
    text,
    t,
    locale,
    theme,
    membership,
    storageError,
  } = useWork();
  const [newProject, setNewProject] = useState(false),
    [newTopic, setNewTopic] = useState(false),
    [search, setSearch] = useState(""),
    [panel, setPanel] = useState(true);
  const split = useSplitLayout(panel && !!membership);
  const topics = state.topics.filter((topic) => topic.projectId === project.id),
    handoffs = state.handoffs.filter((h) => h.projectId === project.id);
  const conversations = useConversationTabs();
  useEffect(() => {
    split.restore();
  }, [project.id, state.currentUser, conversations.active]);
  const cache = useRef(new Map<string, ConversationDraft>());
  const topic =
    conversations.current.kind === "topic"
      ? topics.find((t) => t.id === conversations.current.id)
      : undefined;
  const handoff =
    conversations.current.kind === "handoff"
      ? handoffs.find((h) => h.id === conversations.current.id)
      : undefined;
  useEffect(() => {
    if (!["home", "topic", "handoff", "topics"].includes(route.view))
      setPanel(true);
  }, [route.view, route.id]);
  useEffect(() => {
    document.title = "Judex · " + text(project.title);
  }, [project.id, locale]);
  const openPanel = (view: View, id?: string) => {
    setPanel(true);
    go({
      view,
      id,
    });
  };
  const pending = pendingDecisionCount(state, project.id);
  return (
    <div
      className={
        "judex-chat-app" +
        (!panel ? " judex-chat-focused" : "") +
        (split.expanded && panel ? " judex-chat-expanded" : "")
      }
      data-theme={theme}
      ref={split.ref}
      data-resizing={split.resizing}
      style={{
        gridTemplateColumns: split.columns,
      }}
    >
      <aside className="judex-chat-sidebar">
        <ProjectSwitcher
          onCreate={() => setNewProject(true)}
          onSwitch={() => setSearch("")}
        />
        {membership && (
          <>
            <Button
              className="judex-chat-new"
              data-testid="new-discussion"
              onClick={() => {
                split.restore();
                setNewTopic(true);
              }}
            >
              <Plus size={17} />
              {t("chatNew")}
            </Button>
            <div className="judex-chat-search">
              <Search size={14} />
              <Input
                aria-label={t("chatSearch")}
                placeholder={t("chatSearch")}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <div className="judex-chat-section-label">
              <span>{t("chatConversations")}</span>
              <span>{topics.length + handoffs.length + 1}</span>
            </div>
            <nav className="judex-chat-conversation-list">
              <Button
                className={!topic && !handoff ? "judex-chat-selected" : ""}
                onClick={() => {
                  split.restore();
                  conversations.open({
                    kind: "home",
                  });
                }}
              >
                <span>✳</span>
                <span>{t("chatHome")}</span>
              </Button>
              {topics
                .filter((v) =>
                  text(v.title).toLowerCase().includes(search.toLowerCase()),
                )
                .map((v) => (
                  <Button
                    key={v.id}
                    className={
                      topic?.id === v.id && !handoff
                        ? "judex-chat-selected"
                        : ""
                    }
                    onClick={() => (
                      split.restore(),
                      conversations.open({
                        kind: "topic",
                        id: v.id,
                      })
                    )}
                  >
                    <MessageCircle size={16} />
                    <span>{text(v.title)}</span>
                  </Button>
                ))}
              {handoffs
                .filter((h) =>
                  text(h.title).toLowerCase().includes(search.toLowerCase()),
                )
                .map((h) => (
                  <Button
                    key={h.id}
                    className={
                      handoff?.id === h.id ? "judex-chat-selected" : ""
                    }
                    onClick={() => (
                      split.restore(),
                      conversations.open({
                        kind: "handoff",
                        id: h.id,
                      })
                    )}
                  >
                    <Inbox size={16} />
                    <span>{text(h.title)}</span>
                    <small>
                      {h.sources.filter((s) => s.status !== "accepted")
                        .length || <Check size={12} />}
                    </small>
                  </Button>
                ))}
            </nav>
            <Button
              className="judex-chat-pending"
              data-testid="work-nav-decisions"
              onClick={() => openPanel("decisions")}
            >
              <Check size={16} />
              {t("chatDecisions")}
              <b>{pending}</b>
            </Button>
          </>
        )}
        <footer className="judex-chat-account">
          <AccountMenu onLogout={onLogout} />
        </footer>
      </aside>
      {membership && split.divider("left")}
      {!membership ? (
        <main className="judex-chat-invitation">
          <InvitePage />
        </main>
      ) : (
        <>
          <main className="judex-chat-main" hidden={split.expanded}>
            <Tabs
              className="judex-conversation-root"
              selectedKey={conversations.active}
              onSelectionChange={(key) => {
                const tab = conversations.tabs.find(
                  (t) =>
                    (t.kind === "home" ? "home" : t.kind + ":" + t.id) === key,
                );
                if (tab) conversations.open(tab);
              }}
            >
              <ConversationTabBar
                model={conversations}
                panelOpen={panel}
                onTogglePanel={() => setPanel(!panel)}
              />
              {storageError && (
                <UIWarning role="alert">{t("storageError")}</UIWarning>
              )}
              <Tabs.Panel
                className="judex-active-tab-panel"
                id={conversations.active}
              >
                {handoff ? (
                  <HandoffTab
                    key={project.id + state.currentUser + handoff.id}
                    handoff={handoff}
                  />
                ) : (
                  <Conversation
                    key={project.id + state.currentUser + conversations.active}
                    topic={topic}
                    cache={cache.current}
                  />
                )}
              </Tabs.Panel>
            </Tabs>
          </main>
          {panel && !split.expanded && split.divider("right")}
          {panel && (
            <aside className="judex-chat-inspector">
              <WorkspaceTabs
                expanded={split.expanded}
                onExpand={split.toggleWide}
                onClose={() => setPanel(false)}
              />
            </aside>
          )}
        </>
      )}
      {newProject && <ProjectDialog onClose={() => setNewProject(false)} />}
      {newTopic && <DiscussionDialog onClose={() => setNewTopic(false)} />}
    </div>
  );
}
export default function ChatWorkspace({
  onLogout,
}: {
  onLogout: () => Promise<void>;
}) {
  return (
    <WorkProvider>
      <ChatApplication onLogout={onLogout} />
    </WorkProvider>
  );
}
function ChatApplication({ onLogout }: { onLogout: () => Promise<void> }) {
  const { route, toast, t, setToast } = useWork();
  return (
    <>
      <div
        hidden={!!route.settingsSection}
        className="judex-chat-app-container"
      >
        <ChatShell onLogout={onLogout} />
      </div>
      {route.settingsSection && <SettingsScreen />}
      {toast && (
        <UINotice className="judex-toast" role="status">
          <Check size={16} />
          {toast}
          <Button aria-label={t("close")} onClick={() => setToast("")}>
            <X size={15} />
          </Button>
        </UINotice>
      )}
    </>
  );
}
function HandoffTab({ handoff }: { handoff: import("../work/types").Handoff }) {
  const { state } = useWork();
  const ref = useReadingPosition(
    handoff.projectId + ":" + state.currentUser + ":handoff:" + handoff.id,
  );
  return (
    <div className="judex-chat-handoff" ref={ref}>
      <HandoffPage handoff={handoff} />
    </div>
  );
}
