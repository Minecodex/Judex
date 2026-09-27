import { useEffect, useRef } from "react";
import { Tabs } from "@heroui/react";
import {
  MessageCircle,
  Inbox,
  X,
  PanelRightClose,
  PanelRightOpen,
} from "lucide-react";
import { Button } from "../../components/ui/Button";
import { useWork } from "../work/store";
import { tabKey } from "./conversationTabs";
import { useConversationTabs } from "./useConversationTabs";
export function ConversationTabBar({
  model,
  panelOpen,
  onTogglePanel,
}: {
  model: ReturnType<typeof useConversationTabs>;
  panelOpen: boolean;
  onTogglePanel: () => void;
}) {
  const { state, t, text } = useWork(),
    ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    ref.current
      ?.querySelector('[aria-selected="true"]')
      ?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [model.active]);
  return (
    <div className="judex-conversation-tabstrip" ref={ref}>
      <Tabs.List
        className="judex-conversation-tabs"
        aria-label={t("chatOpenTabs")}
      >
        {model.tabs.map((tab) => {
          const key = tabKey(tab),
            title =
              tab.kind === "home"
                ? t("chatHome")
                : text(
                    (tab.kind === "topic" ? state.topics : state.handoffs).find(
                      (v) => v.id === tab.id,
                    )!.title,
                  );
          return (
            <Tabs.Tab
              className={
                "judex-conversation-tab" +
                (model.active === key ? " judex-conversation-tab-active" : "")
              }
              key={key}
              id={key}
              aria-label={title}
              data-testid={"tab-" + key}
            >
              {tab.kind === "handoff" ? (
                <Inbox size={14} />
              ) : tab.kind === "home" ? (
                <span>✳</span>
              ) : (
                <MessageCircle size={14} />
              )}
              <span className="judex-conversation-tab-label" title={title}>
                {title}
              </span>
              {tab.kind !== "home" && (
                <Button
                  className="judex-conversation-tab-close"
                  aria-label={t("chatCloseTab") + " · " + title}
                  title={t("chatCloseTabHint")}
                  onPointerDown={(e) => e.stopPropagation()}
                  onClick={(e) => {
                    e.stopPropagation();
                    model.close(key);
                  }}
                >
                  <X size={13} />
                </Button>
              )}
            </Tabs.Tab>
          );
        })}
      </Tabs.List>
      <Button
        data-testid="chat-pane-toggle"
        aria-label={t(panelOpen ? "chatFocus" : "chatExpand")}
        onClick={onTogglePanel}
      >
        {panelOpen ? (
          <PanelRightClose size={17} />
        ) : (
          <PanelRightOpen size={17} />
        )}
      </Button>
    </div>
  );
}
