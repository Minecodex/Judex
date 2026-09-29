import {WorkflowEditor} from "./WorkflowEditor";
import { useState } from "react";
import { Button } from "@heroui/react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { FormField, UIInput, UISelect, UIOption, UIWarning } from "../../components/ui/FormControls";
import { request } from "../../lib/api/client";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";

export function ProjectConfiguration({ projectId }: { projectId: string }) {
  const { locale } = usePreferences(); const t = (key: Key) => translate(locale, key); const client = useQueryClient(); const prefix = `/projects/${projectId}`;
  const project = useQuery({ queryKey: ["project", projectId], queryFn: () => request<{ version: number; role: string; viewerRole?: string; status: string; defaultModelId: string | null; maxDiscussionRounds: number; approvalTimeoutSeconds: number }>(prefix) });
  const models = useQuery({ queryKey: ["models"], queryFn: () => request<{ items: { id: string; displayName: string }[] }>("/models") });
  const [modelId, setModel] = useState<string | null>(null); const [rounds, setRounds] = useState<string | null>(null); const [timeout, setTimeoutValue] = useState<string | null>(null); const [reason, setReason] = useState("");
  const save = useMutation({ mutationFn: (archive?: boolean) => request(prefix + (archive === undefined ? "" : archive ? "/archive" : "/restore"), {
    method: archive === undefined ? "PATCH" : "POST", idempotencyKey: crypto.randomUUID(), body: JSON.stringify(archive === undefined
      ? { expectedVersion: project.data!.version, defaultModelId: modelId ?? project.data!.defaultModelId ?? "", maxDiscussionRounds: Number(rounds ?? project.data!.maxDiscussionRounds), approvalTimeoutSeconds: Number(timeout ?? project.data!.approvalTimeoutSeconds) }
      : { expectedVersion: project.data!.version, reason }),
  }), onSuccess: () => { void client.invalidateQueries({ queryKey: ["project", projectId] }); void client.invalidateQueries({ queryKey: ["bootstrap", projectId] }); } });
  const manage = ["owner", "manager"].includes(project.data?.role ?? project.data?.viewerRole ?? "");
  return <div className="judex-panel-stack">
    <h3>{t("paSettings")}</h3>
    <FormField label={t("paModel")}><UISelect disabled={!manage} value={modelId ?? project.data?.defaultModelId ?? ""} onChange={(e) => setModel(e.target.value)}><UIOption value="">—</UIOption>{(models.data?.items ?? []).map((m) => <UIOption key={m.id} value={m.id}>{m.displayName}</UIOption>)}</UISelect></FormField>
    <FormField label={t("paRounds")}><UIInput type="number" min={1} max={100} value={rounds ?? String(project.data?.maxDiscussionRounds ?? 3)} onChange={(e) => setRounds(e.target.value)} /></FormField>
    <FormField label={t("paTimeout")}><UIInput type="number" min={60} value={timeout ?? String(project.data?.approvalTimeoutSeconds ?? 86400)} onChange={(e) => setTimeoutValue(e.target.value)} /></FormField>
    <Button isDisabled={!manage || !project.data} isPending={save.isPending} onPress={() => save.mutate(undefined)}>{t("paSave")}</Button>
    {project.data?.role === "owner" && <>
      <FormField label={t("paReason")}><UIInput value={reason} onChange={(e) => setReason(e.target.value)} /></FormField>
      <Button variant="danger" isDisabled={!reason.trim()} onPress={() => save.mutate(project.data?.status !== "archived")}>{t(project.data.status === "archived" ? "paRestore" : "paArchive")}</Button>
    </>}
    {save.isError && <UIWarning>{save.error.message}</UIWarning>}
    <WorkflowEditor projectId={projectId} manage={manage} owner={(project.data?.role??project.data?.viewerRole)==="owner"} />
  </div>;
}
