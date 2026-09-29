import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@heroui/react";
import { UITextArea, UIWarning } from "../../components/ui/FormControls";
import { usePreferences } from "../../stores/preferences";
import { useAuth } from "../auth/AuthProvider";
import { translate, type Key } from "../../i18n";
import { request } from "../../lib/api/client";
import { ReviewDetails, type EvidenceReview } from "../work/ReviewDetails";
import type { Identity } from "../projects/TeamSettings";
import type { ApiHandoffSource } from "./api";

export function HandoffSender({ projectId, handoffId, source }: { projectId: string; handoffId: string; source: ApiHandoffSource }) {
  const { locale } = usePreferences(); const t = (key: Key) => translate(locale, key); const { user } = useAuth(); const client = useQueryClient();
  const [summary, setSummary] = useState("");
  const [snapshot, setSnapshot] = useState<{ reportId: string; sourceVersion: number; review: EvidenceReview } | null>(null);
  const identities = useQuery({ queryKey: ["identities", projectId], queryFn: () => request<{ items: Identity[] }>(`/projects/${projectId}/identities`) });
  const mine = identities.data?.items.find((i) => i.id === source.senderIdentityId)?.currentBinding?.userId === user?.id;
  const load = useMutation({ mutationFn: async () => {
    const task = await request<{ latestReportId: string | null }>(`/projects/${projectId}/tasks/${source.sourceTaskId}`);
    if (!task.latestReportId) throw new Error(t("accessNoChanges"));
    const review = await request<EvidenceReview>(`/projects/${projectId}/tasks/${source.sourceTaskId}/acceptance-review`);
    return { reportId: task.latestReportId, sourceVersion: (source.currentVersion ?? 0) + 1, review };
  }, onSuccess: setSnapshot });
  const send = useMutation({ mutationFn: () => request(`/projects/${projectId}/handoffs/${handoffId}/sources/${source.id}/send`, {
    method: "POST", idempotencyKey: crypto.randomUUID(), body: JSON.stringify({ summary, sourceVersion: snapshot?.sourceVersion, reviewHash: snapshot?.reportId }),
  }), onSuccess: () => { setSnapshot(null); void client.invalidateQueries({ queryKey: ["handoffs", projectId] }); } });
  if (!mine || (source.state !== "draft" && source.state !== "rejected")) return null;
  return <section className="judex-panel-stack">
    <Button variant="secondary" isPending={load.isPending} onPress={() => load.mutate()}>{t("lcEvidence")}</Button>
    {snapshot && <>
      <ReviewDetails evidence={snapshot.review} />
      <UITextArea aria-label={t("lcSummary")} value={summary} onChange={(e) => setSummary(e.target.value)} />
      <Button isDisabled={!summary.trim()} isPending={send.isPending} onPress={() => send.mutate()}>{t("lcSend")}</Button>
    </>}
    {(load.isError || send.isError) && <UIWarning>{(load.error ?? send.error)?.message}</UIWarning>}
  </section>;
}
