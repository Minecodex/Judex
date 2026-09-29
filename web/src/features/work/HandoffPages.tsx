import {
  UICard,
  UIWarning,
  UIDisclosure,
} from "../../components/ui/FormControls";
import { Button } from "../../components/ui/Button";
import { useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Inbox,
  Mail,
  Send,
  AlertCircle,
  FileText,
} from "lucide-react";
import { useWork } from "./store";
import { Btn, EmptyState, EvidenceList, Heading, Person, Pill } from "./ui";
import { ownsSeat } from "./selectors";
import { ReasonDialog, ReportDialog } from "./Dialogs";
import { Activity } from "./WorkPages";
import type { Handoff, Source } from "./types";
export function HandoffList({ compact = false }: { compact?: boolean }) {
  const { state, project, t, text, go } = useWork(),
    [filter, setFilter] = useState("inbox");
  const items = state.handoffs.filter(
    (h) =>
      h.projectId === project.id &&
      (filter === "all" ||
        (filter === "inbox" &&
          ownsSeat(state, h.receiverSeatId) &&
          (h.stale || h.sources.some((v) => v.status !== "accepted"))) ||
        (filter === "outbox" &&
          h.sources.some((v) => ownsSeat(state, v.senderSeatId)))),
  );
  return (
    <>
      {!compact && (
        <Heading
          eyebrow="WORK, PASSED WITH CARE"
          title={t("workHandoffs")}
          description={t("workPendingHint")}
        />
      )}
      <div className="judex-work-segments judex-work-filter">
        {[
          ["inbox", "workMailInbox"],
          ["outbox", "workMailOutbox"],
          ["all", "workMailAll"],
        ].map(([id, key]) => (
          <Button
            key={id}
            aria-pressed={filter === id}
            onClick={() => setFilter(id)}
          >
            {t(key as Parameters<typeof t>[0])}
          </Button>
        ))}
      </div>
      {items.length ? (
        <div className="judex-handoff-list">
          {items.map((h) => (
            <Button
              key={h.id}
              className="judex-handoff-list-item"
              data-testid={"handoff-list-" + h.id}
              onClick={() =>
                go({
                  view: "handoff",
                  id: h.id,
                })
              }
            >
              <span className="judex-envelope-icon">
                <Mail />
              </span>
              <span>
                <strong>{text(h.title)}</strong>
                <small>
                  {h.sources
                    .map(
                      (s) =>
                        state.seats.find((v) => v.id === s.senderSeatId)
                          ?.person,
                    )
                    .join(" + ")}{" "}
                  → {state.seats.find((v) => v.id === h.receiverSeatId)?.person}
                </small>
              </span>
              <span className="judex-source-dots">
                {h.sources.map((s) => (
                  <i key={s.id} className={"judex-source-dot-" + s.status} />
                ))}
              </span>
              <ArrowRight />
            </Button>
          ))}
        </div>
      ) : (
        <EmptyState text={t("workNoActions")} />
      )}
    </>
  );
}
function Contribution({
  source,
  handoff,
}: {
  source: Source;
  handoff: Handoff;
}) {
  const { state, t, text, act } = useWork(),
    [modal, setModal] = useState<"reject" | "revise" | null>(null);
  const task = state.tasks.find((t) => t.id === source.taskId)!,
    sender = ownsSeat(state, source.senderSeatId),
    receiver = ownsSeat(state, handoff.receiverSeatId);
  return (
    <UICard
      className={"judex-contribution judex-contribution-" + source.status}
      data-testid={"contribution-" + source.id}
      data-status={source.status}
    >
      <div className="judex-contribution-top">
        <Person seatId={source.senderSeatId} />
        <div>
          <span>
            {t("workSourceVersion", {
              version: source.revision,
            })}
          </span>
          <Pill status={source.status} kind="source" />
        </div>
      </div>
      <h3>{text(task.title)}</h3>
      <p>{text(source.summary)}</p>
      <EvidenceList files={source.files} />
      {source.sentBy && (
        <div className="judex-contribution-signed">
          <Send />
          {t("workSender")} · {source.sentBy}
          <span>{new Date(source.sentAt!).toLocaleString()}</span>
        </div>
      )}
      {source.status === "rejected" && (
        <div className="judex-source-feedback">
          <AlertCircle />
          <span>
            <strong>{source.decidedBy}</strong>
            <p>{source.reason}</p>
          </span>
        </div>
      )}
      {source.status === "accepted" && (
        <div className="judex-source-accepted">
          <Check />
          <span>
            {t("workAcceptedSource")} · {source.decidedBy}
          </span>
        </div>
      )}
      {source.status === "rejected" && task.status === "accepted" && (
        <p className="judex-work-small-note">{t("workReopenBeforeRevision")}</p>
      )}
      {!handoff.stale && (
        <div className="judex-contribution-actions">
          {receiver && source.status === "pending" && (
            <>
              <Btn
                secondary
                testId={"reject-" + source.id}
                onClick={() => setModal("reject")}
              >
                {t("workReject")}
              </Btn>
              <Btn
                testId={"receive-" + source.id}
                onClick={() =>
                  void act("sourceDecision", {
                    handoffId: handoff.id,
                    sourceId: source.id,
                    revision: source.revision,
                    decision: "accepted",
                  })
                }
              >
                <Check />
                {t("workReceive")}
              </Btn>
            </>
          )}
          {sender && source.status === "rejected" && (
            <Btn
              testId={"revise-" + source.id}
              disabled={task.status === "accepted"}
              onClick={() => setModal("revise")}
            >
              {t("workRevise")}
            </Btn>
          )}
          {sender && source.status === "draft" && (
            <>
              <Btn secondary onClick={() => setModal("revise")}>
                {t("workEdit")}
              </Btn>
              <Btn
                testId={"send-" + source.id}
                onClick={() =>
                  void act("sendSource", {
                    handoffId: handoff.id,
                    sourceId: source.id,
                    revision: source.revision,
                  })
                }
              >
                <Send />
                {t("workSend")}
              </Btn>
            </>
          )}
        </div>
      )}
      {modal === "reject" && (
        <ReasonDialog
          handoff={handoff}
          source={source}
          onClose={() => setModal(null)}
        />
      )}
      {modal === "revise" && (
        <ReportDialog
          task={task}
          handoff={handoff}
          source={source}
          onClose={() => setModal(null)}
        />
      )}
    </UICard>
  );
}
export function HandoffPage({
  handoff,
  embedded = false,
}: {
  handoff: Handoff;
  embedded?: boolean;
}) {
  const { t, text, go, state, act } = useWork(),
    flow = state.flows.find((f) => f.id === handoff.flowId)!;
  return (
    <div className="judex-handoff-page" data-testid="handoff-detail">
      {!embedded && (
        <Button
          className="judex-work-back"
          onClick={() =>
            go({
              view: "handoffs",
            })
          }
        >
          <ArrowLeft />
          {t("workHandoffs")}
        </Button>
      )}
      <Heading
        eyebrow={t(
          handoff.kind === "dependency"
            ? "workHandoffKindDependency"
            : "workHandoffKindStage",
        )}
        title={text(handoff.title)}
        description={t("workOtherSources")}
      />
      <div className="judex-handoff-address">
        <div>
          <span>{t("workSender")}</span>
          <div>
            {handoff.sources.map((s) => (
              <Person key={s.id} seatId={s.senderSeatId} small />
            ))}
          </div>
        </div>
        <ArrowRight />
        <div>
          <span>{t("workReceiver")}</span>
          <Person seatId={handoff.receiverSeatId} />
        </div>
        <Button
          onClick={() =>
            go({
              view: "flows",
              id: flow.id,
            })
          }
        >
          <FileText />
          <span>
            {t("workBased")}
            <strong>
              {text(flow.name)} · v{handoff.flowVersion}
            </strong>
          </span>
        </Button>
      </div>
      {handoff.stale && (
        <UIWarning className="judex-work-warning" data-testid="handoff-stale">
          <AlertCircle />
          <span>{t("workStale")}</span>
          <Btn
            secondary
            onClick={() => void act("refreshHandoff", { handoffId: handoff.id })}
            testId="refresh-handoff"
          >
            {t("workRefresh")}
          </Btn>
        </UIWarning>
      )}
      <div className="judex-contributions">
        {handoff.sources.map((source) => (
          <Contribution key={source.id} source={source} handoff={handoff} />
        ))}
      </div>
      <p className="judex-work-small-note">{t("workNoReceiptIsAcceptance")}</p>
      <Btn
        secondary
        onClick={() =>
          go({
            view: "task",
            id: handoff.taskId,
          })
        }
      >
        {t("workViewTask")}
        <ArrowRight />
      </Btn>
      <Activity targetId={handoff.id} />
      {!!handoff.history.length && (
        <UIDisclosure
          className="judex-work-records"
          title={
            <>
              {t("deliveryHistory")} · {handoff.history.length}
            </>
          }
        >
          {handoff.history.map((entry, index) => (
            <article key={index}>
              <strong>
                {t("workSourceVersion", {
                  version: entry.source.revision,
                })}{" "}
                ·{" "}
                {entry.source.sentBy ??
                  state.seats.find((v) => v.id === entry.source.senderSeatId)
                    ?.person}
              </strong>
              <p>{text(entry.source.summary)}</p>
              <Pill status={entry.source.status} kind="source" />
              {entry.source.reason && <p>{entry.source.reason}</p>}
            </article>
          ))}
        </UIDisclosure>
      )}
    </div>
  );
}
