import { useState } from "react";
import { Button } from "@heroui/react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { FormField, UIInput, UITextArea, UISelect, UIOption, UICheckbox, UIWarning } from "../../components/ui/FormControls";
import { request } from "../../lib/api/client";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import type { Identity } from "../projects/TeamSettings";

export function ArrangementForm({ projectId, onCreated }: { projectId: string; onCreated: () => void }) {
  const { locale } = usePreferences(); const t = (key: Key) => translate(locale, key); const client = useQueryClient();
  const [kind, setKind] = useState("plan"); const [title, setTitle] = useState(""); const [goal, setGoal] = useState(""); const [criteria, setCriteria] = useState("");
  const [owner, setOwner] = useState(""); const [participants, setParticipants] = useState<string[]>([]); const [planId, setPlan] = useState("");
  const [bug,setBug] = useState({environment:"",steps:"",expected:"",actual:"",severity:"medium",observedReleaseRef:""});
  const identities = useQuery({ queryKey: ["identities", projectId], queryFn: () => request<{ items: Identity[] }>(`/projects/${projectId}/identities`) });
  const plans = useQuery({ queryKey: ["plans", projectId], queryFn: () => request<{ items: { id: string; title: string; status: string }[] }>(`/projects/${projectId}/plans`) });
  const [workflowId,setWorkflow]=useState("");const [nodeId,setNode]=useState("");
  const workflows=useQuery({queryKey:["workflows",projectId],queryFn:()=>request<{items:{id:string;name:string;publishedVersionId:string|null}[]}>(`/projects/${projectId}/workflows`)});
  const workflowVersions=useQuery({queryKey:["workflowVersions",projectId,workflowId],enabled:!!workflowId,queryFn:()=>request<{items:{state:string;body:{nodes:{id:string;name:string}[]}}[]}>(`/projects/${projectId}/workflows/${workflowId}/versions`)});
  const workflowNodes=workflowVersions.data?.items?.find((v)=>v.state==="published")?.body.nodes??[];
  const seats = (identities.data?.items ?? []).filter((i) => i.kind === "position" && i.currentBinding);
  const create = useMutation({
    mutationFn: () => request(`/projects/${projectId}/proposals`, { method: "POST", idempotencyKey: crypto.randomUUID(), body: JSON.stringify({ kind: "work_arrangement", changes: [{ operation: `create_${kind === "plan" ? "plan" : "task"}`, targetType: kind === "plan" ? "plan" : "task", clientRef: "work", fields: {
      title, acceptanceCriteria: criteria, ...(workflowId?{workflowId,...(kind!=="plan"&&nodeId?{nodeId}:{})}:{}), ...(kind === "bug" ? {kind:"bug",bugDetails:bug} : {}), ...(kind === "plan" ? { goal, ownerIdentityId: owner } : { expectedOutput: goal, reviewerIdentityId: owner, participantIdentityIds: participants, ...(planId ? { planId } : {}) }),
    } }] }) }),
    onSuccess: () => { void client.invalidateQueries({ queryKey: ["proposals", projectId] }); onCreated(); },
  });
  return <form className="judex-auth-form" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
    <UISelect aria-label={t("wpNewProposal")} value={kind} onChange={(e) => setKind(e.target.value)}><UIOption value="plan">{t("paPlan")}</UIOption><UIOption value="task">{t("paTask")}</UIOption><UIOption value="bug">{t("rdBug")}</UIOption></UISelect>
    <FormField label={t("paName")}><UIInput required value={title} onChange={(e) => setTitle(e.target.value)} /></FormField>
    <FormField label={t("paGoal")}><UITextArea required value={goal} onChange={(e) => setGoal(e.target.value)} /></FormField>
    <FormField label={t("paCriteria")}><UITextArea required value={criteria} onChange={(e) => setCriteria(e.target.value)} /></FormField>
    <FormField label={t(kind === "plan" ? "paOwner" : "paReviewer")}><UISelect value={owner} onChange={(e) => setOwner(e.target.value)}>
      <UIOption value="">{t("paChoose")}</UIOption>{seats.map((i) => <UIOption key={i.id} value={i.id}>{i.positionName} · {i.currentBinding?.displayName}</UIOption>)}
    </UISelect></FormField>
    {kind !== "plan" && <>
      <FormField label={t("paPlan")}><UISelect value={planId} onChange={(e) => setPlan(e.target.value)}><UIOption value="">—</UIOption>{(plans.data?.items ?? []).filter((p) => p.status === "active").map((p) => <UIOption key={p.id} value={p.id}>{p.title}</UIOption>)}</UISelect></FormField>
      <strong>{t("paParticipants")}</strong>{seats.map((i) => <UICheckbox key={i.id} checked={participants.includes(i.id)} onChange={() => setParticipants((values) => values.includes(i.id) ? values.filter((v) => v !== i.id) : [...values, i.id])}>{i.positionName} · {i.currentBinding?.displayName}</UICheckbox>)}
    </>}
    {kind === "bug" && <>
      {([['environment','rdEnvironment'],['steps','rdSteps'],['expected','rdExpected'],['actual','rdActual'],['observedReleaseRef','rdObserved']] as const).map(([key,label]) => <FormField key={key} label={t(label)}><UITextArea required={key !== 'observedReleaseRef'} value={bug[key]} onChange={(e)=>setBug({...bug,[key]:e.target.value})}/></FormField>)}
      <FormField label={t('rdSeverity')}><UISelect value={bug.severity} onChange={(e)=>setBug({...bug,severity:e.target.value})}>{(['low','medium','high','critical'] as const).map((level)=><UIOption key={level} value={level}>{t(({low:'rdLow',medium:'rdMedium',high:'rdHigh',critical:'rdCritical'} as const)[level])}</UIOption>)}</UISelect></FormField>
    </>}
    <FormField label={t("weWorkflow")}><UISelect value={workflowId} onChange={(e)=>{setWorkflow(e.target.value);setNode("");}}><UIOption value="">—</UIOption>{workflows.data?.items?.filter((w)=>w.publishedVersionId).map((w)=><UIOption key={w.id} value={w.id}>{w.name}</UIOption>)}</UISelect></FormField>
    {workflowId&&kind!=="plan"&&<FormField label={t("weNode")}><UISelect value={nodeId} onChange={(e)=>setNode(e.target.value)}><UIOption value="">—</UIOption>{workflowNodes.map((n)=><UIOption key={n.id} value={n.id}>{n.name}</UIOption>)}</UISelect></FormField>}
    {!seats.length && <UIWarning>{t("paNoIdentity")}</UIWarning>}
    <Button type="submit" isPending={create.isPending} isDisabled={!title.trim() || !goal.trim() || !criteria.trim() || !owner || (kind !== "plan" && !participants.length)}>{t("wpNewProposal")}</Button>
    {create.isError && <UIWarning>{create.error.message}</UIWarning>}
  </form>;
}
