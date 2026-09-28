import { useMemo, useState, type ReactNode } from "react";
import { Button } from "../../components/ui/Button";
import { Input } from "@heroui/react";
import { ArrowLeft, Check, Inbox, MessageCircle, Plus, Search } from "lucide-react";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import {
  useCreateTopic,
  useHandoffs,
  type ApiTopic,
} from "./api";
import { tabKey, type ConversationTab } from "./tabs";

// 左栏：项目入口 + 新议题 + 会话搜索（议题/交接）+ 待定决定。
// 数据来自真实 topics/handoffs 列表；SSE 变更自动失效刷新。
export function WorkspaceSidebar({
  projectId,
  projectTitle,
  pendingCount,
  topics,
  live,
  activeKey,
  onOpen,
  onExit,
  onOpenPanel,
  account,
}: {
  projectId: string;
  projectTitle: string;
  pendingCount: number;
  topics: ApiTopic[];
  live: boolean;
  activeKey: string;
  onOpen: (tab: ConversationTab) => void;
  onExit: () => void;
  onOpenPanel: (view: "work" | "materials" | "team") => void;
  account?: ReactNode;
}) {
  const { locale } = usePreferences();
  const t = (key: Key, values?: Record<string, string | number>) =>
    translate(locale, key, values);
  const [search, setSearch] = useState("");
  const [creating, setCreating] = useState(false);
  const [title, setTitle] = useState("");
  const create = useCreateTopic(projectId);
  const handoffs = useHandoffs(projectId);

  const visibleTopics = useMemo(
    () =>
      topics.filter((topic) =>
        topic.title.toLowerCase().includes(search.trim().toLowerCase()),
      ),
    [topics, search],
  );
  const visibleHandoffs = useMemo(
    () =>
      (handoffs.data?.items ?? []).filter((handoff) =>
        (handoff.title ?? t("wsHandoffTitle"))
          .toLowerCase()
          .includes(search.trim().toLowerCase()),
      ),
    [handoffs.data, search, t],
  );
  const pendingHandoffs = (handoffs.data?.items ?? []).filter(
    (handoff) => handoff.state !== "accepted",
  );

  const submitTopic = () => {
    const trimmed = title.trim();
    if (!trimmed) return;
    create.mutate(trimmed, {
      onSuccess: (topic) => {
        setTitle("");
        setCreating(false);
        onOpen({ kind: "topic", id: topic.id });
      },
    });
  };

  return (
    <aside className="judex-chat-sidebar">
      <div className="judex-chat-sidebar-head">
        <Button
          variant="ghost"
          aria-label={t("wsBackProjects")}
          title={t("wsBackProjects")}
          onClick={onExit}
          data-testid="ws-back-projects"
        >
          <ArrowLeft size={16} />
        </Button>
        <strong className="judex-chat-project-title" title={projectTitle}>
          {projectTitle}
        </strong>
      </div>
      <Button
        className="judex-chat-new"
        data-testid="ws-new-topic"
        onClick={() => setCreating((value) => !value)}
      >
        <Plus size={17} />
        {t("wsNewDiscussion")}
      </Button>
      {creating && (
        <form
          className="judex-chat-new-form"
          onSubmit={(event) => {
            event.preventDefault();
            submitTopic();
          }}
        >
          <Input
            aria-label={t("wsNewDiscussion")}
            placeholder={t("wsNewDiscussion")}
            value={title}
            autoFocus
            onChange={(event) => setTitle(event.target.value)}
          />
          <Button type="submit" isPending={create.isPending}>
            {create.isPending ? t("submitting") : t("wsSend")}
          </Button>
          {create.isError && (
            <p role="alert">{t("wsTopicCreateFailed")}</p>
          )}
        </form>
      )}
      <div className="judex-chat-search">
        <Search size={14} />
        <Input
          aria-label={t("wsSearch")}
          placeholder={t("wsSearch")}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      <span
        className={"judex-live-dot" + (live ? " judex-live-on" : "")}
        title={live ? t("wsSseConnected") : t("wsSseOffline")}
        data-testid="ws-live"
        data-live={live ? "on" : "off"}
      />
      <div className="judex-chat-section-label">
        <span>{t("wsConversations")}</span>
        <span>{visibleTopics.length + visibleHandoffs.length + 1}</span>
      </div>
      <nav className="judex-chat-conversation-list">
        <Button
          className={activeKey === "home" ? "judex-chat-selected" : ""}
          onClick={() => onOpen({ kind: "home" })}
          data-testid="ws-nav-home"
        >
          <span>✳</span>
          <span>{t("wsHome")}</span>
        </Button>
        {visibleTopics.map((topic) => (
          <Button
            key={topic.id}
            className={
              activeKey === tabKey({ kind: "topic", id: topic.id })
                ? "judex-chat-selected"
                : ""
            }
            onClick={() => onOpen({ kind: "topic", id: topic.id })}
            data-testid={"ws-nav-topic-" + topic.id}
          >
            <MessageCircle size={16} />
            <span>{topic.title}</span>
          </Button>
        ))}
        {visibleHandoffs.map((handoff) => (
          <Button
            key={handoff.id}
            className={
              activeKey === tabKey({ kind: "handoff", id: handoff.id })
                ? "judex-chat-selected"
                : ""
            }
            onClick={() => onOpen({ kind: "handoff", id: handoff.id })}
            data-testid={"ws-nav-handoff-" + handoff.id}
          >
            <Inbox size={16} />
            <span>{handoff.title ?? t("wsHandoffTitle")}</span>
            <small>
              {handoff.sources.filter((s) => s.state === "pending").length || (
                <Check size={12} />
              )}
            </small>
          </Button>
        ))}
        {!visibleTopics.length && !visibleHandoffs.length && (
          <p className="judex-chat-empty-hint">
            {handoffs.isPending && topics.length === 0
              ? t("wsLoading")
              : t("wsTopicsEmpty")}
          </p>
        )}
      </nav>
      <Button
        className="judex-chat-pending"
        data-testid="ws-nav-decisions"
        onClick={() => onOpenPanel("work")}
      >
        <Check size={16} />
        {t("wsDecisions")}
        <b>{pendingCount + pendingHandoffs.length}</b>
      </Button>
      {account}
    </aside>
  );
}
