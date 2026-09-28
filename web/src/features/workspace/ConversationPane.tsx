import { Inbox, MessageCircle, PanelRightClose, PanelRightOpen, X } from "lucide-react";
import { Tabs } from "@heroui/react";
import { Button } from "../../components/ui/Button";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { tabKey, type TabState } from "./tabs";
import { TopicConversation } from "./TopicConversation";
import { HandoffConversation } from "./HandoffConversation";
import { HomeConversation } from "./HomeConversation";

// 中央会话栏：会话标签条 + 当前会话（概览 / 议题 / 交接）。
export function ConversationPane({
  projectId,
  tabs,
  topicTitles,
  panelOpen,
  onTogglePanel,
  onOpen,
  onClose,
  onRefresh,
}: {
  projectId: string;
  tabs: TabState;
  topicTitles: Map<string, string>;
  panelOpen: boolean;
  onTogglePanel: () => void;
  onOpen: (tab: TabState["active"]) => void;
  onClose: (key: string) => void;
  onRefresh: () => void;
}) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const active = tabKey(tabs.active);

  const label = (key: string): string => {
    const tab = key === "home" ? null : key;
    if (!tab) return t("wsHome");
    const separator = tab.indexOf(":");
    const kind = tab.slice(0, separator),
      id = tab.slice(separator + 1);
    if (kind === "topic") return topicTitles.get(id) ?? t("wsLoading");
    return t("wsHandoffTitle");
  };

  return (
    <main className="judex-chat-main" hidden={false}>
      <Tabs
        className="judex-conversation-root"
        selectedKey={active}
        onSelectionChange={(key) => {
          const tab = tabs.tabs.find((item) => tabKey(item) === key);
          if (tab) onOpen(tab);
        }}
      >
      <div className="judex-conversation-tabstrip">
        <Tabs.List className="judex-conversation-tabs" aria-label={t("wsConversations")}>
          {tabs.tabs.map((tab) => {
            const key = tabKey(tab);
            return (
              <Tabs.Tab
                className={
                  "judex-conversation-tab" +
                  (active === key ? " judex-conversation-tab-active" : "")
                }
                key={key}
                id={key}
                aria-label={label(key)}
                data-testid={"ws-tab-" + key}
              >
                {tab.kind === "handoff" ? (
                  <Inbox size={14} />
                ) : tab.kind === "home" ? (
                  <span>✳</span>
                ) : (
                  <MessageCircle size={14} />
                )}
                <span className="judex-conversation-tab-label">{label(key)}</span>
                {tab.kind !== "home" && (
                  <span
                    role="button"
                    tabIndex={0}
                    aria-label={t("wsTabClose")}
                    className="judex-conversation-tab-close"
                    data-testid={"ws-close-" + key}
                    onClick={(event) => {
                      event.stopPropagation();
                      onClose(key);
                    }}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        onClose(key);
                      }
                    }}
                  >
                    <X size={12} />
                  </span>
                )}
              </Tabs.Tab>
            );
          })}
        </Tabs.List>
        <Button
          variant="ghost"
          className="judex-chat-panel-toggle"
          aria-label={panelOpen ? t("wsClosePanel") : t("wsShowPanel")}
          onClick={onTogglePanel}
          data-testid="ws-toggle-panel"
        >
          {panelOpen ? <PanelRightClose size={16} /> : <PanelRightOpen size={16} />}
        </Button>
      </div>
      <Tabs.Panel className="judex-active-tab-panel" id={active}>
        <div className="judex-workspace-pane">
          {tabs.active.kind === "home" && (
            <HomeConversation projectId={projectId} onOpenTopic={onOpen} />
          )}
          {tabs.active.kind === "topic" && (
            <TopicConversation
              key={projectId + ":" + tabs.active.id}
              projectId={projectId}
              topicId={tabs.active.id}
              onRefresh={onRefresh}
            />
          )}
          {tabs.active.kind === "handoff" && (
            <HandoffConversation
              key={projectId + ":" + tabs.active.id}
              projectId={projectId}
              handoffId={tabs.active.id}
            />
          )}
        </div>
      </Tabs.Panel>
      </Tabs>
    </main>
  );
}
