import {
  UIDisclosure,
  UIWarning,
  UITextArea,
  UICard,
} from "../../components/ui/FormControls";
import { useState } from "react";
import { Check, ArrowUpRight, GitBranch, Send } from "lucide-react";
import { useWork } from "../work/store";
import { Btn, Dialog, Field, Person } from "../work/ui";
import {
  decideProposal,
  proposalIsCurrent,
  type WorkProposal,
} from "./proposalModel";
import { ProposalDialog } from "./ProposalDialog";
export function ProposalCard({ proposal: p }: { proposal: WorkProposal }) {
  const { state, t, act, go, text } = useWork();
  const [reject, setReject] = useState(false),
    [reason, setReason] = useState(""),
    [revise, setRevise] = useState(false);
  const stale = !proposalIsCurrent(state, p);
  const mine =
    p.status === "pending" &&
    p.approvers.includes(state.currentUser) &&
    !p.votes[state.currentUser];
  return (
    <UICard className="judex-chat-proposal" data-testid={"proposal-" + p.id}>
      <header>
        <span>
          <GitBranch size={16} />
          {t("chatProposal")} · v{p.revision}
        </span>
        <b>
          {t(
            p.status === "approved"
              ? "chatAccepted"
              : p.status === "rejected"
                ? "chatRejected"
                : "chatWaiting",
          )}
        </b>
      </header>
      <h3>{p.title}</h3>
      <p>{p.goal}</p>
      <div className="judex-chat-proposal-meta">
        <span>
          {state.flows.find((f) => f.id === p.flowId) &&
            text(state.flows.find((f) => f.id === p.flowId)!.name)}{" "}
          · v{p.flowVersion}
        </span>
        <span>
          <Send size={12} />
          {p.sender}
        </span>
      </div>
      <ol>
        {p.tasks.map((task, i) => (
          <li key={i}>
            <strong>{task.title}</strong>
            <small>
              {state.seats.find((s) => s.id === task.seatId)?.person} →{" "}
              {state.seats.find((s) => s.id === task.reviewerSeatId)?.person}
            </small>
            <span>
              {task.after.length
                ? "← " + task.after.map((a) => p.tasks[a]?.title).join(" + ")
                : t("chatParallel")}
            </span>
          </li>
        ))}
      </ol>
      <UIDisclosure title={<>{t("chatCriteria")}</>}>
        <p>{p.criteria}</p>
      </UIDisclosure>
      <div className="judex-chat-voters">
        <span>
          {t("chatApprovers")} · {Object.keys(p.votes).length}/
          {p.approvers.length}
        </span>
        {p.approvers.map((person) => (
          <div key={person}>
            <Person name={person} small />
            {p.votes[person] ? (
              <Check size={16} />
            ) : (
              <small>{t("chatWaiting")}</small>
            )}
          </div>
        ))}
      </div>
      {p.status === "pending" && stale && (
        <UIWarning className="judex-work-warning">{t("chatStale")}</UIWarning>
      )}
      {mine && (
        <footer>
          <Btn
            testId={"approve-proposal-" + p.id}
            disabled={stale}
            onClick={() =>
              act((s) => decideProposal(s, p.id, p.revision, true))
            }
          >
            {t("chatApprove")}
          </Btn>
          <Btn secondary onClick={() => setReject(true)}>
            {t("chatReject")}
          </Btn>
        </footer>
      )}
      {p.status === "pending" && p.votes[state.currentUser] && (
        <p>{t("chatAlreadyVoted")}</p>
      )}
      {p.status === "rejected" && (
        <UIWarning className="judex-work-warning">{p.reason}</UIWarning>
      )}
      {p.status === "rejected" &&
        p.sender === state.currentUser &&
        !state.proposals?.some((v) => v.previousId === p.id) && (
          <Btn secondary onClick={() => setRevise(true)}>
            {t("chatRevise")}
          </Btn>
        )}
      {p.planId && (
        <Btn
          secondary
          onClick={() =>
            go({
              view: "plan",
              id: p.planId,
            })
          }
        >
          {t("chatApplied")}
          <ArrowUpRight size={15} />
        </Btn>
      )}
      <small className="judex-chat-footnote">{t("chatVoteHint")}</small>
      {reject && (
        <Dialog title={t("chatReject")} onClose={() => setReject(false)}>
          <Field label={t("chatReason")}>
            <UITextArea
              className="judex-input"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
          </Field>
          <Btn
            disabled={!reason.trim()}
            onClick={() => {
              if (
                act((s) => decideProposal(s, p.id, p.revision, false, reason))
              )
                setReject(false);
            }}
          >
            {t("chatReject")}
          </Btn>
        </Dialog>
      )}
      {revise && (
        <ProposalDialog
          topicId={p.topicId}
          previous={p}
          onClose={() => setRevise(false)}
        />
      )}
    </UICard>
  );
}
