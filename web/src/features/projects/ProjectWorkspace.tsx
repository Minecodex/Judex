import { UIWarning } from "../../components/ui/FormControls";
import { Button, Card, Input } from "@heroui/react";
import { Plus, X } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key, type Locale } from "../../i18n";
import { request, APIError } from "../../lib/api/client";
import { errorKey } from "../auth/LoginPage";

export function errorText(locale: Locale, error: unknown): string {
  const key = errorKey(error);
  return key ? translate(locale, key) : String(error);
}

type Topic = { id: string; title: string; kind: string; lastMessageSeq: number };
type Message = {
  id: string;
  seq: number;
  kind: string;
  authorDisplayName: string | null;
  content: string;
  createdAt: string;
};
type MaterialItem = { id: string; title: string; kind: string };
type Member = { userId: string; displayName: string; email: string; role: string };

// ProjectWorkspace 是 P2-08 的真实数据项目内页面：议题（含消息）、共享
// 资料、团队成员。P3/P5 接入工作/审批/AI 后由完整聊天工作区替代。
export function ProjectWorkspace({ projectId }: { projectId: string }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const [tab, setTab] = useState<"topics" | "materials" | "team">("topics");

  return (
    <div className="judex-project-workspace">
      <nav className="judex-workspace-tabs">
        <Button variant={tab === "topics" ? "primary" : "ghost"} onClick={() => setTab("topics")}>
          {t("pwTabTopics")}
        </Button>
        <Button variant={tab === "materials" ? "primary" : "ghost"} onClick={() => setTab("materials")}>
          {t("pwTabMaterials")}
        </Button>
        <Button variant={tab === "team" ? "primary" : "ghost"} onClick={() => setTab("team")}>
          {t("pwTabTeam")}
        </Button>
      </nav>
      {tab === "topics" && <TopicsPanel projectId={projectId} />}
      {tab === "materials" && <MaterialsPanel projectId={projectId} />}
      {tab === "team" && <TeamPanel projectId={projectId} />}
    </div>
  );
}

function TopicsPanel({ projectId }: { projectId: string }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [title, setTitle] = useState("");
  const [selected, setSelected] = useState<string | null>(null);

  const topics = useQuery({
    queryKey: ["topics", projectId],
    queryFn: () => request<{ items: Topic[] }>(`/projects/${projectId}/topics`),
    enabled: !!projectId,
  });

  const createTopic = useMutation({
    mutationFn: () =>
      request<Topic>(`/projects/${projectId}/topics`, {
        method: "POST",
        body: JSON.stringify({ title: title.trim() }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: (topic) => {
      setTitle("");
      setCreating(false);
      setSelected(topic.id);
      client.invalidateQueries({ queryKey: ["topics", projectId] });
    },
  });

  if (topics.isPending) return <p className="judex-workspace-status">{t("shellLoading")}</p>;
  if (topics.isError) return <UIWarning role="alert">{errorText(locale, topics.error)}</UIWarning>;

  return (
    <div className="judex-panel-stack">
      <div className="judex-workspace-section">
        <h3>{t("pwTopicsTitle")}</h3>
        <Button onClick={() => setCreating((v) => !v)}>
          <Plus size={15} />
          {t("chatNew")}
        </Button>
      </div>
      {creating && (
        <form
          className="judex-inline-form"
          onSubmit={(event) => {
            event.preventDefault();
            if (title.trim()) createTopic.mutate();
          }}
        >
          <Input
            aria-label={t("pwTopicsTitle")}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            autoFocus
          />
          <Button type="submit" isPending={createTopic.isPending}>
            {createTopic.isPending ? t("submitting") : t("pwCreate")}
          </Button>
          <Button variant="ghost" onClick={() => setCreating(false)}>
            <X size={15} />
          </Button>
          {createTopic.isError && <span role="alert">{errorText(locale, createTopic.error)}</span>}
        </form>
      )}
      {topics.data?.items?.length ? (
        <ul className="judex-workspace-list">
          {topics.data.items.map((topic) => (
            <li key={topic.id}>
              <Button
                variant={selected === topic.id ? "secondary" : "ghost"}
                onClick={() => setSelected(topic.id)}
              >
                {topic.title}
              </Button>
            </li>
          ))}
        </ul>
      ) : (
        <div className="judex-workspace-empty">
          <strong>{t("pwNoTopics")}</strong>
        </div>
      )}
      {selected && <MessageThread projectId={projectId} topicId={selected} />}
    </div>
  );
}

function MessageThread({ projectId, topicId }: { projectId: string; topicId: string }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);

  const messages = useQuery({
    queryKey: ["messages", projectId, topicId],
    queryFn: () =>
      request<{ items: Message[] }>(`/projects/${projectId}/topics/${topicId}/messages?limit=50`),
    refetchInterval: 3000,
  });

  const send = async () => {
    if (!draft.trim() || sending) return;
    setSending(true);
    try {
      await request(`/projects/${projectId}/submissions`, {
        method: "POST",
        body: JSON.stringify({
          clientSubmissionId: crypto.randomUUID(),
          purpose: "message",
          topicId,
          text: draft.trim(),
        }),
        idempotencyKey: crypto.randomUUID(),
      });
      setDraft("");
      client.invalidateQueries({ queryKey: ["messages", projectId, topicId] });
    } catch {
      // surfaced via next refetch; keep the draft for retry
    } finally {
      setSending(false);
    }
  };

  return (
    <Card className="judex-thread-card">
      <Card.Content>
        <div className="judex-thread-scroll">
          {messages.isPending && <p className="judex-workspace-status">{t("shellLoading")}</p>}
          {messages.isError && <UIWarning role="alert">{errorText(locale, messages.error)}</UIWarning>}
          {messages.data?.items?.length === 0 && (
            <p className="judex-workspace-status">{t("pwNoMessages")}</p>
          )}
          {(messages.data?.items ?? []).map((message) => (
            <div key={message.id} className="judex-thread-message">
              <strong>{message.authorDisplayName ?? "system"}</strong>
              <span className="judex-thread-content">{message.content}</span>
            </div>
          ))}
        </div>
        <form
          className="judex-inline-form"
          onSubmit={(event) => {
            event.preventDefault();
            void send();
          }}
        >
          <Input
            aria-label={t("pwComposer")}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder={t("pwComposer")}
          />
          <Button type="submit" isPending={sending}>
            {t("pwSend")}
          </Button>
        </form>
      </Card.Content>
    </Card>
  );
}

function MaterialsPanel({ projectId }: { projectId: string }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);

  const materials = useQuery({
    queryKey: ["materials", projectId],
    queryFn: () => request<{ items: MaterialItem[] }>(`/projects/${projectId}/materials`),
    enabled: !!projectId,
  });

  if (materials.isPending) return <p className="judex-workspace-status">{t("shellLoading")}</p>;
  if (materials.isError) return <UIWarning role="alert">{errorText(locale, materials.error)}</UIWarning>;
  return (
    <div className="judex-panel-stack">
      <h3>{t("pwMaterialsTitle")}</h3>
      {materials.data?.items?.length ? (
        <ul className="judex-workspace-list">
          {materials.data.items.map((material) => (
            <li key={material.id} className="judex-workspace-item">
              <strong>{material.title}</strong>
              <small>{material.kind}</small>
            </li>
          ))}
        </ul>
      ) : (
        <div className="judex-workspace-empty">
          <strong>{t("pwNoMaterials")}</strong>
          <span>{t("pwNoMaterialsHint")}</span>
        </div>
      )}
    </div>
  );
}

function TeamPanel({ projectId }: { projectId: string }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);

  const members = useQuery({
    queryKey: ["members", projectId],
    queryFn: () => request<{ items: Member[] }>(`/projects/${projectId}/members`),
    enabled: !!projectId,
  });

  useEffect(() => {
    document.title = "Judex";
  }, [locale]);

  if (members.isPending) return <p className="judex-workspace-status">{t("shellLoading")}</p>;
  if (members.isError) {
    if (members.error instanceof APIError && members.error.status === 404) {
      return <UIWarning role="alert">{t("pwProjectMissing")}</UIWarning>;
    }
    return <UIWarning role="alert">{errorText(locale, members.error)}</UIWarning>;
  }
  return (
    <div className="judex-panel-stack">
      <h3>{t("pwTeamTitle")}</h3>
      <ul className="judex-workspace-list">
        {(members.data?.items ?? []).map((member) => (
          <li key={member.userId} className="judex-workspace-item">
            <strong>{member.displayName}</strong>
            <small>
              {member.role} · {member.email}
            </small>
          </li>
        ))}
      </ul>
    </div>
  );
}
