import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "@heroui/react";
import { FormField, UIInput, UITextArea, UISelect, UIOption, UIWarning } from "../../components/ui/FormControls";
import { request } from "../../lib/api/client";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { useAuth } from "../auth/AuthProvider";

export type Identity = { id: string; positionName: string; currentBindingVersion: number; currentBinding: { userId: string; displayName: string } | null; kind: string };
export function TeamSettings({ projectId }: { projectId: string }) {
  const { user } = useAuth(); const { locale } = usePreferences(); const t = (key: Key) => translate(locale, key);
  const client = useQueryClient(); const prefix = `/projects/${projectId}`;
  const project = useQuery({ queryKey: ["project", projectId], queryFn: () => request<{ role: string; viewerRole?: string }>(prefix) });
  const positions = useQuery({ queryKey: ["positions", projectId], queryFn: () => request<{ items: { id: string; name: string; prompt: string; currentVersion: number }[] }>(prefix + "/positions") });
  const members = useQuery({ queryKey: ["members", projectId], queryFn: () => request<{ items: { userId: string; displayName: string }[] }>(prefix + "/members") });
  const identities = useQuery({ queryKey: ["identities", projectId], queryFn: () => request<{ items: Identity[] }>(prefix + "/identities") });
  const preferences = useQuery({ queryKey: ["preferences", projectId, user?.id], queryFn: () => request<{ revision: number; prompt: string }>(prefix + "/me/preferences") });
  const [name, setName] = useState(""); const [prompt, setPrompt] = useState("");
  const [positionId, setPosition] = useState(""); const [memberId, setMember] = useState("");
  const [identityId, setIdentity] = useState(""); const [reason, setReason] = useState("");
  const [email, setEmail] = useState(""); const [link, setLink] = useState(""); const [personal, setPersonal] = useState<string | null>(null);
  const canManage = ["owner", "manager"].includes(project.data?.role ?? project.data?.viewerRole ?? "");
  const command = useMutation({
    mutationFn: ({ path, body, method = "POST" }: { path: string; body: unknown; method?: string }) => request<{ inviteUrl?: string }>(prefix + path, { method, body: JSON.stringify(body), idempotencyKey: crypto.randomUUID() }),
    onSuccess: (result) => { if (result.inviteUrl) setLink(new URL(result.inviteUrl, location.origin).href); for (const root of ["positions", "members", "identities", "preferences"]) void client.invalidateQueries({ queryKey: [root, projectId] }); },
  });
  const select = (label: string, value: string, onChange: (value: string) => void, items: { id: string; name: string }[]) =>
    <FormField label={label}><UISelect value={value} onChange={(e) => onChange(e.target.value)}><UIOption value="">{t("paChoose")}</UIOption>{items.map((item) => <UIOption key={item.id} value={item.id}>{item.name}</UIOption>)}</UISelect></FormField>;
  return <div className="judex-panel-stack">
    <h3>{t("paPositions")}</h3>
    {(identities.data?.items ?? []).map((identity) => <p key={identity.id}>{identity.positionName} · {identity.currentBinding?.displayName ?? "—"}</p>)}
    {canManage ? <>
      <FormField label={t("paName")}><UIInput value={name} onChange={(e) => setName(e.target.value)} /></FormField>
      <FormField label={t("paPrompt")}><UITextArea value={prompt} onChange={(e) => setPrompt(e.target.value)} /></FormField>
      <Button isDisabled={!name.trim()} isPending={command.isPending} onPress={() => command.mutate({ path: "/positions", body: { name, prompt } })}>{t("paCreate")}</Button>
      {select(t("paPosition"), positionId, setPosition, (positions.data?.items ?? []).map((p) => ({ id: p.id, name: p.name })))}
      {select(t("paMember"), memberId, setMember, (members.data?.items ?? []).map((m) => ({ id: m.userId, name: m.displayName })))}
      <Button isDisabled={!positionId || !memberId} isPending={command.isPending} onPress={() => command.mutate({ path: "/identities", body: { positionId, userId: memberId } })}>{t("paAssign")}</Button>
      {select(t("paIdentity"), identityId, setIdentity, (identities.data?.items ?? []).filter((i) => i.kind === "position").map((i) => ({ id: i.id, name: `${i.positionName} · ${i.currentBinding?.displayName ?? "—"}` })))}
      <FormField label={t("paReason")}><UIInput value={reason} onChange={(e) => setReason(e.target.value)} /></FormField>
      <Button isDisabled={!identityId || !memberId || !reason.trim()} onPress={() => command.mutate({ path: `/identities/${identityId}/replace`, body: { newUserId: memberId, expectedBindingVersion: identities.data?.items.find((i) => i.id === identityId)?.currentBindingVersion, reason } })}>{t("paReplace")}</Button>
      <h3>{t("paInvite")}</h3>
      <FormField label={t("paEmail")}><UIInput type="email" value={email} onChange={(e) => setEmail(e.target.value)} /></FormField>
      <Button isDisabled={!email.includes("@") || !positionId} onPress={() => command.mutate({ path: "/invitations", body: { targetEmail: email, positionIds: [positionId] } })}>{t("paInvite")}</Button>
      {link && <FormField label={t("paLink")}><UIInput readOnly value={link} /></FormField>}
    </> : <p>{t("paReadOnly")}</p>}
    <h3>{t("paPreferences")}</h3>
    <UITextArea aria-label={t("paPreferences")} value={personal ?? preferences.data?.prompt ?? ""} onChange={(e) => setPersonal(e.target.value)} />
    <Button isPending={command.isPending} isDisabled={!preferences.data} onPress={() => command.mutate({ path: "/me/preferences", method: "PUT", body: { prompt: personal ?? preferences.data?.prompt ?? "", expectedRevision: preferences.data?.revision ?? 0 } })}>{t("paSave")}</Button>
    {command.isError && <UIWarning>{command.error.message}</UIWarning>}
    {(positions.isError || identities.isError || preferences.isError) && <UIWarning>{t("accessUnavailable")}</UIWarning>}
  </div>;
}
