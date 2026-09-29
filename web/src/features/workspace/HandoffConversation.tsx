import { HandoffSender } from "./HandoffSender";
import { ReviewDetails } from "../work/ReviewDetails";
import { useState } from "react";
import { Button } from "../../components/ui/Button";
import { TextArea } from "@heroui/react";
import { Check, X } from "lucide-react";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { errorText } from "../projects/ProjectWorkspace";
import { useHandoffDecision, useHandoffs, type ApiHandoffSource } from "./api";

// 交接会话：接收方视角的来源列表与接收/拒收决定（03 §5 分来源决定，
// 拒收须理由；请求带 sourceVersion/reviewHash 冻结引用）。
export function HandoffConversation({
  projectId,
  handoffId,
}: {
  projectId: string;
  handoffId: string;
}) {
  const { locale } = usePreferences();
  const t = (key: Key, values?: Record<string, string | number>) =>
    translate(locale, key, values);
  const handoffs = useHandoffs(projectId);
  const decide = useHandoffDecision(projectId, handoffId);
  const [rejecting, setRejecting] = useState<ApiHandoffSource | null>(null);
  const [reason, setReason] = useState("");

  const handoff = (handoffs.data?.items ?? []).find((item) => item.id === handoffId);

  if (handoffs.isPending) {
    return <p className="judex-workspace-status">{t("wsLoading")}</p>;
  }
  if (!handoff) {
    return (
      <div className="judex-chat-empty-hint">
        {t("wsHandoffsEmpty")}{" "}
        <Button variant="ghost" onClick={() => handoffs.refetch()}>
          {t("wsRetry")}
        </Button>
      </div>
    );
  }

  const stateLabel: Record<string, Key> = {
    waiting: "wsHandoffWaiting",
    accepted: "wsHandoffAccepted",
    needs_revision: "wsHandoffNeedsRevision",
  };
  const sourceLabel: Record<string, Key> = {
    draft: "wsHandoffSourceDraft",
    pending: "wsHandoffSourcePending",
    accepted: "wsHandoffSourceAccepted",
    rejected: "wsHandoffSourceRejected",
  };

  return (
    <div className="judex-handoff-conversation" data-testid="ws-handoff">
      <header className="judex-handoff-head">
        <h2>
          {handoff.title ?? t("wsHandoffTitle")}
          <small>
            {t(
              handoff.kind === "dependency"
                ? "wsHandoffKindDependency"
                : "wsHandoffKindStage",
            )}
          </small>
        </h2>
        <span
          className={
            "judex-pill judex-pill-" + handoff.state.replace("_", "-")
          }
          data-testid="ws-handoff-state"
        >
          {t(stateLabel[handoff.state] ?? "wsHandoffWaiting")}
        </span>
        <p>{t("wsReceiver", { name: handoff.receiverDisplayName ?? "—" })}</p>
      </header>
      <ul className="judex-handoff-sources">
        {handoff.sources.map((source) => (
          <li
            key={source.id}
            className="judex-handoff-source"
            data-status={source.state}
            data-testid={"ws-source-" + source.id}
          >
            <header>
              <strong>
                {t("wsSender", { name: source.senderDisplayName ?? "—" })}
              </strong>
              <span className="judex-pill">{t(sourceLabel[source.state])}</span>
            </header>
            {source.summary && <p className="judex-message-content">{source.summary}</p>}
            {source.evidence && <ReviewDetails evidence={{ reviewId: source.currentVersionId ?? source.id, reviewHash: source.currentVersionId ?? source.id, targetVersion: source.currentVersion ?? 1, reports: source.evidence.reports }} />}
            {source.reason && <p className="judex-source-reason">{source.reason}</p>}
            <HandoffSender projectId={projectId} handoffId={handoffId} source={source} />
            {source.state === "pending" && (
              <div className="judex-source-actions">
                <Button
                  data-testid={"ws-accept-" + source.id}
                  isPending={decide.isPending}
                  onClick={() =>
                    decide.mutate({ source, decision: "accept" })
                  }
                >
                  <Check size={15} />
                  {t("wsAccept")}
                </Button>
                <Button
                  variant="secondary"
                  data-testid={"ws-reject-" + source.id}
                  onClick={() => {
                    setRejecting(source);
                    setReason("");
                  }}
                >
                  <X size={15} />
                  {t("wsReject")}
                </Button>
              </div>
            )}
            {source.state === "rejected" && source.reason && (
              <p className="judex-source-rejected-reason">{source.reason}</p>
            )}
          </li>
        ))}
      </ul>
      {decide.isError && (
        <p role="alert">{errorText(locale, decide.error)}</p>
      )}
      {rejecting && (
        <div className="judex-dialog-backdrop" role="dialog" aria-modal>
          <form
            className="judex-dialog judex-reject-dialog"
            onSubmit={(event) => {
              event.preventDefault();
              if (!reason.trim()) return;
              decide.mutate(
                { source: rejecting, decision: "reject", reason: reason.trim() },
                { onSuccess: () => setRejecting(null) },
              );
            }}
          >
            <h3>{t("wsReject")}</h3>
            <TextArea
              aria-label={t("wsRejectReason")}
              placeholder={t("wsRejectReason")}
              value={reason}
              autoFocus
              onChange={(event: React.ChangeEvent<HTMLTextAreaElement>) =>
                setReason(event.target.value)
              }
            />
            <footer>
              <Button type="submit" isPending={decide.isPending} disabled={!reason.trim()}>
                {t("wsRejectSubmit")}
              </Button>
              <Button variant="ghost" onClick={() => setRejecting(null)}>
                {t("wsCancel")}
              </Button>
            </footer>
          </form>
        </div>
      )}
    </div>
  );
}
