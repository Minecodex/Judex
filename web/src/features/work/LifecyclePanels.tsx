import { WorkChangeForm } from "./WorkChangeForm";
import { useState } from "react";
import { Button } from "@heroui/react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { FormField, UIInput, UITextArea, UISelect, UIOption, UIWarning } from "../../components/ui/FormControls";
import { request } from "../../lib/api/client";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import type { Identity } from "../projects/TeamSettings";

export type WorkItem = { id: string; title: string; status: string; version: number; acceptanceCriteria?: string; goal?: string; expectedOutput?: string; latestAcceptanceId?: string | null; reviewerIdentityId?: string | null; ownerIdentityId?: string | null };
export function PlanDecisions({ projectId, planId }: { projectId: string; planId: string }) {
  const { locale } = usePreferences(); const t = (key: Key) => translate(locale, key); const client = useQueryClient();
  const prefix = `/projects/${projectId}/plans/${planId}`;
  const plan = useQuery({ queryKey: ["plan", projectId, planId], queryFn: () => request<WorkItem>(prefix) });
  const [reason, setReason] = useState("");
  const [changing, setChanging] = useState(false);
  const [review, setReview] = useState<{ reviewId: string; reviewHash: string; targetVersion: number; taskAcceptanceIds: string[]; manifest: {tasks:{taskId:string;title:string;status:string;acceptanceId:string|null}[]}; blockers: { reason: string }[] } | null>(null);
  const load = useMutation({ mutationFn: () => request<NonNullable<typeof review>>(prefix + "/acceptance-review"), onSuccess: setReview });
  const decide = useMutation({ mutationFn: (reopen: boolean) => request(prefix + (reopen ? "/reopens" : "/acceptances"), {
    method: "POST", idempotencyKey: crypto.randomUUID(), body: JSON.stringify(reopen
      ? { acceptanceId: plan.data?.latestAcceptanceId, expectedVersion:plan.data?.version, reason }
      : { reviewId: review?.reviewId, reviewHash: review?.reviewHash,expectedVersion:review?.targetVersion, decision: "accept" }),
  }), onSuccess: () => { setReview(null); for (const root of ["plan", "plans", "actions"]) void client.invalidateQueries({ queryKey: [root, projectId] }); } });
  return <section className="judex-panel-stack">
    <h4>{plan.data?.title}</h4><p>{plan.data?.goal}</p><p>{plan.data?.acceptanceCriteria}</p>
    {plan.data?.status === "active" && <Button isPending={load.isPending} onPress={() => load.mutate()}>{t("lcPlanAccept")}</Button>}
    {review && <>
      <strong>{t("lcEvidence")}</strong>
      <ul>{review.manifest?.tasks?.map((task) => <li key={task.taskId}>{task.title} · {task.status}<small>{task.acceptanceId}</small></li>)}</ul>
      {review.blockers?.map((b, i) => <UIWarning key={i}>{b.reason}</UIWarning>)}
      <Button isDisabled={!!review.blockers?.length} isPending={decide.isPending} onPress={() => decide.mutate(false)}>{t("accessConfirm")}</Button>
      <Button variant="ghost" onPress={() => setReview(null)}>{t("accessCancel")}</Button>
    </>}
    {plan.data?.status === "accepted" && <>
      <FormField label={t("paReason")}><UIInput value={reason} onChange={(e) => setReason(e.target.value)} /></FormField>
      <Button isDisabled={!reason.trim()} isPending={decide.isPending} onPress={() => decide.mutate(true)}>{t("lcReopen")}</Button>
    </>}
    {plan.data && plan.data.status !== "accepted" && plan.data.status !== "cancelled" && <Button variant="ghost" onPress={() => setChanging(!changing)}>{t("lcChange")}</Button>}
    {changing && plan.data && <WorkChangeForm projectId={projectId} target={plan.data} kind="plan" />}
    {(load.isError || decide.isError) && <UIWarning>{(load.error ?? decide.error)?.message}</UIWarning>}
  </section>;
}

export function ExecutionGraph({ projectId, onTask }: { projectId: string; onTask: (id: string) => void }) {
  const { locale } = usePreferences(); const t = (key: Key, values?: Record<string, number>) => translate(locale, key, values);
  const [open, setOpen] = useState(false);
  const graph = useQuery({ queryKey: ["executionMap", projectId], enabled: open, queryFn: () => request<{ nodes: { taskId: string; title: string; status: string; column: number; blockers: { reason: string }[] }[]; edges: { fromTaskId: string; toTaskId: string; kind: string }[] }>(`/projects/${projectId}/execution-map`) });
  const columns = [...new Set((graph.data?.nodes ?? []).map((n) => n.column))].sort((a, b) => a - b);
  return <section className="judex-panel-stack">
    <Button variant="ghost" onPress={() => setOpen(!open)}>{t("lcGraph")}</Button>
    {open && <div className="judex-execution-columns">{columns.map((column) => <section key={column} className="judex-execution-column">
      <strong>{t("lcStage", { n: column + 1 })}</strong>
      {graph.data?.nodes.filter((node) => node.column === column).map((node) => <article key={node.taskId}>
        <Button variant="secondary" onPress={() => onTask(node.taskId)}>{node.title}</Button>
        {node.blockers?.map((b, i) => <small key={i}>{b.reason}</small>)}
      </article>)}
    </section>)}</div>}
    {graph.isError && <UIWarning>{graph.error.message}</UIWarning>}
  </section>;
}

export function HandoffEditor({ projectId, onOpen }: { projectId: string; onOpen?: (id: string) => void }) {
  const { locale } = usePreferences(); const t = (key: Key) => translate(locale, key); const client = useQueryClient();
  const prefix = `/projects/${projectId}`; const [open, setOpen] = useState(false);
  const [title, setTitle] = useState(""); const [source, setSource] = useState(""); const [target, setTarget] = useState("");
  const [sender, setSender] = useState(""); const [receiver, setReceiver] = useState("");
  const tasks = useQuery({ queryKey: ["tasks", projectId, ""], queryFn: () => request<{ items: WorkItem[] }>(prefix + "/tasks") });
  const identities = useQuery({ queryKey: ["identities", projectId], queryFn: () => request<{ items: Identity[] }>(prefix + "/identities") });
  const create = useMutation({ mutationFn: () => request<{ id: string }>(prefix + "/handoffs", { method: "POST", idempotencyKey: crypto.randomUUID(), body: JSON.stringify({ title, targetTaskId: target, receiverIdentityId: receiver, kind: "dependency", sources: [{ sourceTaskId: source, senderIdentityId: sender }] }) }),
    onSuccess: (h) => { void client.invalidateQueries({ queryKey: ["handoffs", projectId] }); setOpen(false); onOpen?.(h.id); },
  });
  const select = (label: string, value: string, set: (value: string) => void, options: { id: string; label: string }[]) => <FormField label={label}><UISelect value={value} onChange={(e) => set(e.target.value)}><UIOption value="">{t("paChoose")}</UIOption>{options.map((o) => <UIOption key={o.id} value={o.id}>{o.label}</UIOption>)}</UISelect></FormField>;
  const taskOptions = (tasks.data?.items ?? []).map((task) => ({ id: task.id, label: task.title }));
  const identityOptions = (identities.data?.items ?? []).filter((i) => i.currentBinding).map((i) => ({ id: i.id, label: `${i.positionName} · ${i.currentBinding!.displayName}` }));
  return <section className="judex-panel-stack">
    <Button variant="ghost" onPress={() => setOpen(!open)}>{t("lcCreateHandoff")}</Button>
    {open && <form className="judex-auth-form" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
      <FormField label={t("paName")}><UIInput value={title} onChange={(e) => setTitle(e.target.value)} /></FormField>
      {select(t("lcSource"), source, setSource, taskOptions)}{select(t("lcTarget"), target, setTarget, taskOptions)}
      {select(t("lcSender"), sender, setSender, identityOptions)}{select(t("lcReceiver"), receiver, setReceiver, identityOptions)}
      <p>{t("lcReceiptHint")}</p>
      <Button type="submit" isDisabled={!source || !target || !sender || !receiver} isPending={create.isPending}>{t("lcCreateHandoff")}</Button>
      {create.isError && <UIWarning>{create.error.message}</UIWarning>}
    </form>}
  </section>;
}
