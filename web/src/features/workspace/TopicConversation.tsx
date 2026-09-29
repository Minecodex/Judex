import { UIFilePicker } from "../../components/ui/FormControls";
import { uploadFile } from "../projects/upload";
import { useEffect, useMemo, useRef, useState } from "react";
import { TextArea } from "@heroui/react";
import { Bot, Send, User } from "lucide-react";
import { Button } from "../../components/ui/Button";
import { usePreferences } from "../../stores/preferences";
import { useAuth } from "../auth/AuthProvider";
import { translate, type Key } from "../../i18n";
import {
  useAgentRun,
  useMessages,
  useSendMessage,
  useStartAgentRun,
  type ApiMessage,
} from "./api";

// 议题会话：真实消息流（human/agent/system，来源 web/cli/platform）+
// 统一提交发送（clientSubmissionId 幂等）+ Agent 分析（批次运行状态）。
export function TopicConversation({
  projectId,
  topicId,
  onRefresh,
}: {
  projectId: string;
  topicId: string;
  onRefresh: () => void;
}) {
  const { locale } = usePreferences();
  const { user } = useAuth();
  const t = (key: Key, values?: Record<string, string | number>) =>
    translate(locale, key, values);
  const messages = useMessages(projectId, topicId);
  const send = useSendMessage(projectId, topicId);
  const startRun = useStartAgentRun(projectId, topicId);
  const [runId, setRunId] = useState<string | null>(null);
  const run = useAgentRun(projectId, runId);
  const [draft, setDraft] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState("");
  const draftKey = "judex.ws.draft." + user?.id + ":" + projectId + ":" + topicId;
  const bottom = useRef<HTMLDivElement>(null);

  useEffect(() => {
    try {
      setDraft(sessionStorage.getItem(draftKey) ?? "");
    } catch {
      /* 无存储则不恢复 */
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draftKey]);

  const items = useMemo(
    () => (messages.data?.items ?? []).slice().sort((a, b) => a.seq - b.seq),
    [messages.data],
  );
  const lastOwnSubmission = useMemo(
    () =>
      items
        .filter((m) => m.kind === "human" && m.submissionId)
        .at(-1)?.submissionId ?? null,
    [items],
  );

  useEffect(() => {
    bottom.current?.scrollIntoView({ block: "end" });
  }, [items.length, run.data?.state]);

  const submit = async () => {
    const text = draft.trim();
    if ((!text && !files.length) || send.isPending || uploading) return;
    setUploading(true); setUploadError("");
    try {
      const materialVersionIds: string[] = [];
      for (const file of files) materialVersionIds.push((await uploadFile(projectId, file)).id);
      await send.mutateAsync({ text, materialVersionIds });
      setDraft(""); setFiles([]); sessionStorage.removeItem(draftKey);
    } catch (error) { setUploadError(error instanceof Error ? error.message : String(error)); }
    finally { setUploading(false); }
  };

  const saveDraft = (value: string) => {
    setDraft(value);
    try {
      sessionStorage.setItem(draftKey, value);
    } catch {
      /* 忽略 */
    }
  };

  const runActive =
    !!run.data && !["succeeded", "failed", "cancelled"].includes(run.data.state);

  return (
    <div className="judex-topic-conversation">
      <div className="judex-chat-thread" data-testid="ws-thread">
        {messages.isPending && (
          <p className="judex-workspace-status">{t("wsLoading")}</p>
        )}
        {messages.isError && (
          <p className="judex-workspace-status" role="alert">
            {t("wsLoadFailed")}{" "}
            <Button variant="ghost" onClick={() => messages.refetch()}>
              {t("wsRetry")}
            </Button>
          </p>
        )}
        {messages.isSuccess && !items.length && (
          <div className="judex-chat-empty-hint">{t("wsMessagesEmpty")}</div>
        )}
        {messages.hasNextPage && <Button isPending={messages.isFetchingNextPage} onPress={() => { void messages.fetchNextPage(); }}>{t("paEarlier")}</Button>}
        {items.map((message) => (
          <MessageRow key={message.id} message={message} ownName={user?.displayName} />
        ))}
        {run.data && runActive && (
          <p
            className="judex-run-status"
            role="status"
            data-testid="ws-run-status"
          >
            <Bot size={14} />
            {t("wsRunState", { state: run.data.state })}
          </p>
        )}
        {run.data?.state === "failed" && (
          <p className="judex-run-status judex-run-failed" role="alert">
            {t("wsRunFailed", { reason: (run.data.waitingFor ?? []).join(", ") || "unknown" })}
          </p>
        )}
        <div ref={bottom} />
      </div>
      <div className="judex-chat-composer">
        {lastOwnSubmission && (
          <Button
            variant="secondary"
            data-testid="ws-analyze"
            isPending={startRun.isPending}
            disabled={runActive}
            onClick={() =>
              startRun.mutate(lastOwnSubmission, {
                onSuccess: (result) => setRunId(result.runId),
                onError: () => onRefresh(),
              })
            }
          >
            <Bot size={15} />
            {runActive ? t("wsAnalyzing") : t("wsAnalyze")}
          </Button>
        )}
        {!lastOwnSubmission && (
          <span className="judex-chat-empty-hint">{t("wsNoAnalysisSource")}</span>
        )}
        <UIFilePicker multiple aria-label={t("paAttachments")} onChange={(e) => setFiles(Array.from(e.target.files ?? []).slice(0, 20))}>{t("paAttachments")}</UIFilePicker>
        {files.map((file) => <small key={file.name}>{file.name}</small>)}
        {uploadError && <p role="alert">{uploadError}</p>}
        <TextArea
          aria-label={t("wsComposer")}
          placeholder={t("wsComposer")}
          value={draft}
          data-testid="ws-composer"
          onChange={(event: React.ChangeEvent<HTMLTextAreaElement>) =>
            saveDraft(event.target.value)
          }
          onKeyDown={(event: React.KeyboardEvent<HTMLTextAreaElement>) => {
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault();
              submit();
            }
          }}
        />
        <Button
          data-testid="ws-send"
          isPending={send.isPending || uploading}
          disabled={(!draft.trim() && !files.length) || uploading}
          onClick={submit}
        >
          <Send size={15} />
          {send.isPending ? t("wsSending") : t("wsSend")}
        </Button>
        {send.isError && (
          <p role="alert" className="judex-composer-error">
            {t("wsMessageSendFailed")}
          </p>
        )}
      </div>
    </div>
  );
}

function MessageRow({
  message,
  ownName,
}: {
  message: ApiMessage;
  ownName?: string;
}) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const source =
    message.source && message.source !== "web"
      ? ` · ${t(
          message.source === "cli"
            ? "wsSourceCli"
            : message.source === "platform"
              ? "wsSourcePlatform"
              : "wsSourceWeb",
        )}`
      : "";
  const author =
    message.kind === "agent"
      ? (message.identityName ?? t("wsAgent"))
      : message.kind === "system"
        ? t("wsSystem")
        : (message.authorDisplayName ??
          (ownName && message.authorUserId ? ownName : t("wsYou")));
  return (
    <article
      className={
        "judex-message" +
        (message.kind === "agent" ? " judex-message-agent" : "") +
        (message.kind === "system" ? " judex-message-system" : "")
      }
      data-kind={message.kind}
      data-testid="ws-message"
    >
      <header>
        {message.kind === "agent" ? (
          <Bot size={14} />
        ) : (
          <User size={14} />
        )}
        <strong>{author}</strong>
        {source && <small>{source}</small>}
        <time>{new Date(message.createdAt).toLocaleString()}</time>
      </header>
      <p className="judex-message-content">{message.content}</p>
    </article>
  );
}
