import { useState } from "react";
import { Button } from "@heroui/react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { FormField, UIInput, UITextArea, UISelect, UIOption, UICheckbox, UIWarning } from "../../components/ui/FormControls";
import { request } from "../../lib/api/client";
import { translate, type Key } from "../../i18n";
import { usePreferences } from "../../stores/preferences";
import type { Identity } from "../projects/TeamSettings";
import type { WorkItem } from "./LifecyclePanels";

export function WorkChangeForm({ projectId, target, kind }: { projectId: string; target: WorkItem; kind: "plan" | "task" }) {
  const [snapshot] = useState(target); const { locale } = usePreferences(); const t = (key: Key) => translate(locale, key); const client = useQueryClient();
  const [operation, setOperation] = useState("update_scope"); const [title, setTitle] = useState(target.title);
  const [goal, setGoal] = useState(target.goal ?? target.expectedOutput ?? ""); const [criteria, setCriteria] = useState(target.acceptanceCriteria ?? "");
  const [responsible, setResponsible] = useState(""); const [participants, setParticipants] = useState<string[]>([]);
  const [dependency, setDependency] = useState(""); const [phase, setPhase] = useState("start"); const [reason, setReason] = useState("");
  const identities = useQuery({ queryKey: ["identities", projectId], queryFn: () => request<{ items: Identity[] }>(`/projects/${projectId}/identities`) });
  const tasks = useQuery({ queryKey: ["tasks", projectId, ""], queryFn: () => request<{ items: WorkItem[] }>(`/projects/${projectId}/tasks`) });
  const propose = useMutation({ mutationFn: () => {
    const fields = operation === "update_scope" ? { title, [kind === "plan" ? "goal" : "expectedOutput"]: goal, acceptanceCriteria: criteria }
      : operation === "set_assignment" ? { [kind === "plan" ? "ownerIdentityId" : "reviewerIdentityId"]: responsible, ...(kind === "task" ? { participantIdentityIds: participants } : {}) }
      : operation === "set_requirements" ? { requirements: dependency ? [{ phase, kind: "task_acceptance", targetId: dependency, hard: true, label: "" }] : [] }
      : operation === "reference_task" ? { taskId: dependency } : { reason };
    return request(`/projects/${projectId}/proposals`, { method: "POST", idempotencyKey: crypto.randomUUID(), body: JSON.stringify({ kind: "work_change", reason, changes: [{ operation, targetType: kind, targetId: snapshot.id, expectedVersion: snapshot.version, fields }] }) });
  }, onSuccess: () => { void client.invalidateQueries({ queryKey: ["proposals", projectId] }); } });
  const choices: [string, Key][] = [["update_scope", "lcScope"], ["set_assignment", "lcAssign"], [kind === "task" ? "set_requirements" : "reference_task", kind === "task" ? "lcRequirements" : "lcReference"], [`cancel_${kind}`, "lcCancel"]];
  if (snapshot.status === "draft") choices.push(["activate_object", "lcActivate"]);
  return <form className="judex-auth-form" onSubmit={(e) => { e.preventDefault(); propose.mutate(); }}>
    <strong>{t("lcChange")} · {snapshot.title} · v{snapshot.version}</strong>
    <UISelect aria-label={t("lcChange")} value={operation} onChange={(e) => setOperation(e.target.value)}>{choices.map(([value, label]) => <UIOption key={value} value={value}>{t(label)}</UIOption>)}</UISelect>
    {operation === "update_scope" && <>
      <FormField label={t("paName")}><UIInput value={title} onChange={(e) => setTitle(e.target.value)} /></FormField>
      <FormField label={t("paGoal")}><UITextArea value={goal} onChange={(e) => setGoal(e.target.value)} /></FormField>
      <FormField label={t("paCriteria")}><UITextArea value={criteria} onChange={(e) => setCriteria(e.target.value)} /></FormField>
    </>}
    {operation === "set_assignment" && <>
      <FormField label={t(kind === "plan" ? "paOwner" : "paReviewer")}><UISelect value={responsible} onChange={(e) => setResponsible(e.target.value)}><UIOption value="">{t("paChoose")}</UIOption>{identities.data?.items.filter((i) => i.currentBinding).map((i) => <UIOption key={i.id} value={i.id}>{i.positionName} · {i.currentBinding!.displayName}</UIOption>)}</UISelect></FormField>
      {kind === "task" && identities.data?.items.filter((i) => i.currentBinding).map((i) => <UICheckbox key={i.id} checked={participants.includes(i.id)} onChange={() => setParticipants((items) => items.includes(i.id) ? items.filter((id) => id !== i.id) : [...items, i.id])}>{i.positionName} · {i.currentBinding!.displayName}</UICheckbox>)}
    </>}
    {(operation === "set_requirements" || operation === "reference_task") && <>
      <FormField label={t("lcTaskAcceptance")}><UISelect value={dependency} onChange={(e) => setDependency(e.target.value)}><UIOption value="">—</UIOption>{tasks.data?.items.filter((task) => task.id !== snapshot.id).map((task) => <UIOption key={task.id} value={task.id}>{task.title}</UIOption>)}</UISelect></FormField>
      {operation === "set_requirements" && <UISelect aria-label={t("lcPhase")} value={phase} onChange={(e) => setPhase(e.target.value)}><UIOption value="start">{t("lcStart")}</UIOption><UIOption value="accept">{t("lcAccept")}</UIOption><UIOption value="both">{t("lcBoth")}</UIOption></UISelect>}
    </>}
    <FormField label={t("paReason")}><UIInput value={reason} onChange={(e) => setReason(e.target.value)} /></FormField>
    <Button type="submit" isPending={propose.isPending} isDisabled={!reason.trim() || (operation === "set_assignment" && (!responsible || (kind === "task" && !participants.length))) || (operation === "reference_task" && !dependency)}>{t("lcSubmitChange")}</Button>
    {propose.isError && <UIWarning>{propose.error.message}</UIWarning>}
    {propose.isSuccess && <p role="status">{t("lcSuccess")}</p>}
  </form>;
}
