import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";

export type Change = { operation: string; targetType: string; targetId?: string; clientRef?: string; expectedVersion?: number; fields?: Record<string, unknown> };
export type EvidenceReview = { reviewId: string; reviewHash: string; targetVersion: number; reports?: { reportId: string; text?: string; identityId?: string; materialVersionIds?: string[] }[]; blockers?: { reason: string; phase?: string }[] };

// The full frozen content stays visible beside the decision. Object IDs and
// versions are supporting evidence, never substitutes for the actual text.
export function ReviewDetails({ changes, evidence }: { changes?: Change[]; evidence?: EvidenceReview }) {
  const { locale } = usePreferences(); const t = (key: Key) => translate(locale, key);
  return <section className="judex-panel-stack" data-testid="fixed-review">
    <strong>{t(evidence ? "accessEvidence" : "accessChanges")}</strong>
    <p>{t("accessReviewFirst")}</p>
    {changes?.map((change, index) => <article key={index} className="judex-workspace-item">
      <div className="judex-panel-stack">
        <strong>{change.operation} · {change.targetType}</strong>
        <small>{change.targetId ?? change.clientRef} {change.expectedVersion ? `v${change.expectedVersion}` : ""}</small>
        <dl>{Object.entries(change.fields ?? {}).map(([key, value]) => <div key={key}>
          <dt>{key}</dt><dd className="judex-message-content">{typeof value === "string" ? value : Array.isArray(value) ? value.map((v) => typeof v === "object" ? Object.values(v).join(" · ") : String(v)).join("\n") : JSON.stringify(value, null, 2)}</dd>
        </div>)}</dl>
      </div>
    </article>)}
    {evidence && <>
      <small>v{evidence.targetVersion} · {evidence.reviewHash}</small>
      {evidence.reports?.map((report) => <article key={report.reportId} className="judex-panel-stack">
        <small>{t("accessReportId")}: {report.reportId}</small>
        <p className="judex-message-content">{report.text}</p>
        {!!report.materialVersionIds?.length && <p>{t("accessMaterials")}: {report.materialVersionIds.join(", ")}</p>}
      </article>)}
      {evidence.blockers?.map((b, i) => <p role="alert" key={i}>{b.reason}</p>)}
    </>}
  </section>;
}
