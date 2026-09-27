import { useEffect, useRef, useState } from "react";
import {
  ArrowUp,
  ArrowUpRight,
  Paperclip,
  Sparkles,
  GitBranch,
} from "lucide-react";
import { TextArea } from "@heroui/react";
import { useWork } from "../work/store";
import { Button } from "../../components/ui/Button";
import { Person, Upload, EvidenceList } from "../work/ui";
import { topicMessage } from "../work/actions";
import { createDiscussion } from "../work/composition";
import type { Topic, Evidence } from "../work/types";
import { discussTopic, latestDiscussionRun } from "./discussionPolicy";
import { ProposalCard } from "./ProposalCard";
import { ProposalDialog } from "./ProposalDialog";
import { useReadingPosition } from "./useReadingPosition";
export type ConversationDraft = {
  body: string;
  files: Evidence[];
  attachments: boolean;
};
export function Conversation({
  topic,
  cache,
}: {
  topic?: Topic;
  cache: Map<string, ConversationDraft>;
}) {
  const { state, project, t, text, act, go } = useWork();
  const key =
    project.id + ":" + state.currentUser + ":" + (topic?.id ?? "home");
  const [body, setBody] = useState(
      () =>
        cache.get(key)?.body ??
        sessionStorage.getItem("judex.chat.draft." + key) ??
        "",
    ),
    [files, setFiles] = useState<Evidence[]>(() => cache.get(key)?.files ?? []),
    [attachments, setAttachments] = useState(
      () => cache.get(key)?.attachments ?? false,
    ),
    [proposal, setProposal] = useState(false);
  const tail = useRef<HTMLDivElement>(null);
  const reading = useReadingPosition(key);
  const count =
    (topic?.messages.length ?? 0) +
    (state.proposals?.filter((p) => p.topicId === topic?.id).length ?? 0);
  const previousCount = useRef(count);
  useEffect(() => {
    cache.set(key, { body, files, attachments });
  }, [key, body, files, attachments, cache]);
  useEffect(() => {
    sessionStorage.setItem("judex.chat.draft." + key, body);
  }, [key, body]);
  useEffect(() => {
    if (previousCount.current !== count)
      tail.current?.scrollIntoView({ block: "nearest" });
    previousCount.current = count;
  }, [count]);
  const ensure = (
    fn: (id: string, s: typeof state) => ReturnType<typeof topicMessage>,
  ) => {
    let id = topic?.id;
    const ok = act((s) => {
      if (id) return fn(id, s);
      const r = createDiscussion(
        s,
        project.id,
        body.trim().slice(0, 48) || t("chatHome"),
        [],
        [],
      );
      if (r.error) return r;
      id = r.state.topics.at(-1)!.id;
      return fn(id!, r.state);
    }, false);
    if (ok && id && id !== topic?.id) go({ view: "topic", id });
    return ok;
  };
  const send = () => {
    if (
      ensure((id, s) => {
        return topicMessage(s, id, body, files);
      })
    ) {
      sessionStorage.removeItem("judex.chat.draft." + key);
      setBody("");
      setFiles([]);
      setAttachments(false);
    }
  };
  const run = topic ? latestDiscussionRun(state, topic.id) : undefined;
  const atLimit = !!run && run.rounds >= run.maxRounds;
  const proposals = (state.proposals ?? []).filter(
    (p) => p.topicId === topic?.id,
  );
  return (
    <section className="judex-chat-conversation">
      <div
        className="judex-chat-thread"
        data-testid="chat-thread"
        ref={reading}
      >
        <div className="judex-chat-intro">
          <span className="judex-chat-orbit">✳</span>
          <h2>{t("chatWelcome")}</h2>
          <p>{topic ? t("chatShared") : text(project.description)}</p>
          <small>{t("chatSim")}</small>
        </div>
        {topic?.messages.map((m) => (
          <article
            className={"judex-chat-message judex-chat-message-" + m.kind}
            key={m.id}
          >
            <div className="judex-chat-message-by">
              <Person name={m.actor} seatId={m.seatId} small />
              {m.seatId && (
                <span>
                  {text(
                    state.positions.find(
                      (p) =>
                        p.id ===
                        state.seats.find((s) => s.id === m.seatId)?.positionId,
                    )?.name ?? "",
                  )}
                </span>
              )}
              <span>
                {m.kind === "ai"
                  ? t("workAI")
                  : new Date(m.at).toLocaleTimeString([], {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
              </span>
            </div>
            <div className="judex-chat-message-body">{text(m.text)}</div>
            {!!m.files?.length && <EvidenceList files={m.files} />}
          </article>
        ))}
        {proposals.map((p) => (
          <ProposalCard key={p.id} proposal={p} />
        ))}
        {!!topic?.planIds.length && (
          <div className="judex-chat-linked">
            {topic.planIds.map((id) => {
              const plan = state.plans.find((p) => p.id === id);
              return (
                plan && (
                  <Button key={id} onClick={() => go({ view: "plan", id })}>
                    <GitBranch size={14} />
                    {text(plan.title)}
                    <ArrowUpRight size={13} />
                  </Button>
                )
              );
            })}
          </div>
        )}
        <div ref={tail} />
      </div>
      <div className="judex-chat-compose">
        {run && (
          <div
            className="judex-discussion-budget"
            role="status"
            data-testid="discussion-budget"
          >
            <strong>
              {t("chatRoundCount", { used: run.rounds, max: run.maxRounds })}
            </strong>
            <span>
              {t(atLimit ? "chatRoundLimitReached" : "chatRoundWaiting")}
            </span>
          </div>
        )}
        {topic && (
          <div className="judex-chat-suggestions">
            <Button
              disabled={atLimit}
              data-testid="chat-discuss"
              onClick={() => act((s) => discussTopic(s, topic.id), false)}
            >
              <Sparkles size={14} />
              {t(run ? "chatNextRound" : "chatAnalyze")}
            </Button>
            <Button
              data-testid="chat-propose"
              onClick={() => setProposal(true)}
            >
              <GitBranch size={14} />
              {t("chatPropose")}
            </Button>
          </div>
        )}
        <div className="judex-chat-input-wrap">
          <TextArea
            data-testid="work-discussion-input"
            aria-label={t("chatMessage")}
            value={body}
            placeholder={t("chatMessage")}
            onChange={(e) => setBody(e.target.value)}
            onKeyDown={(e) => {
              if (
                e.key === "Enter" &&
                (e.ctrlKey || e.metaKey) &&
                (body.trim() || files.length)
              ) {
                e.preventDefault();
                send();
              }
            }}
          />
          <div>
            <Button
              aria-label={t("chatAttach")}
              onClick={() => setAttachments(!attachments)}
            >
              <Paperclip size={18} />
            </Button>
            <Button
              className="judex-chat-send"
              data-testid="send-work-message"
              aria-label={t("chatSend")}
              disabled={!body.trim() && !files.length}
              onClick={send}
            >
              <ArrowUp size={19} />
            </Button>
          </div>
        </div>
        {attachments && <Upload files={files} onChange={setFiles} />}
        <small>{t("chatDraftSaved")}</small>
      </div>
      {proposal && topic && (
        <ProposalDialog topicId={topic.id} onClose={() => setProposal(false)} />
      )}
    </section>
  );
}
