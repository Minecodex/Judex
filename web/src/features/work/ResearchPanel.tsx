import { useState } from "react";
import { Button } from "@heroui/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FormField, UIInput, UISelect, UIOption, UICheckbox, UIWarning } from "../../components/ui/FormControls";
import { request } from "../../lib/api/client";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";

export function ResearchPanel({ projectId }: { projectId: string }) {
  const { locale } = usePreferences(); const t = (key: Key) => translate(locale, key); const client = useQueryClient();
  const prefix = `/projects/${projectId}`;
  const [open, setOpen] = useState(false);
  const [versionLabel, setVersion] = useState(""); const [environment, setEnvironment] = useState("");
  const [url, setURL] = useState(""); const [status, setStatus] = useState("success");
  const [bug, setBug] = useState(""); const [targets, setTargets] = useState<string[]>([]);
  const [repoName, setRepoName] = useState(""); const [repoURL, setRepoURL] = useState("");
  const [repositoryId, setRepository] = useState(""); const [commitSHA, setCommit] = useState("");
  const releases = useQuery({ queryKey: ["releases", projectId], enabled: open, queryFn: () => request<{ items: { id: string; versionLabel: string; environment: string; status: string }[] }>(prefix + "/release-reports") });
  const tasks = useQuery({ queryKey: ["tasks", projectId, ""], enabled: open, queryFn: () => request<{ items: { id: string; title: string; kind: string }[] }>(prefix + "/tasks") });
  const repos = useQuery({ queryKey: ["repositories", projectId], enabled: open, queryFn: () => request<{ items: { id: string; displayName: string }[] }>(prefix + "/repositories") });
  const post = (path: string, data: unknown) => request(prefix + path, { method: "POST", body: JSON.stringify(data), idempotencyKey: crypto.randomUUID() });
  const release = useMutation({ mutationFn: () => post("/release-reports", { versionLabel, environment, url, status, repositoryCommits: commitSHA ? [{ repositoryId, commit: commitSHA, verification: "reported" }] : [] }), onSuccess: () => { setVersion(""); void client.invalidateQueries({ queryKey: ["releases", projectId] }); } });
  const repository = useMutation({ mutationFn: () => post("/repositories", { displayName: repoName, url: repoURL }), onSuccess: () => { setRepoName(""); setRepoURL(""); void client.invalidateQueries({ queryKey: ["repositories", projectId] }); } });
  const fix = useMutation({ mutationFn: () => post(`/tasks/${bug}/fix-propagations`, { targetReleaseRefs: targets }), onSuccess: () => { setTargets([]); void client.invalidateQueries({ queryKey: ["tasks", projectId] }); } });
  return <section className="judex-panel-stack">
    <Button variant="ghost" onPress={() => setOpen(!open)}>{t("rdTitle")}</Button>
    {open && <>
      <p>{t("rdFactHint")}</p>
      <form className="judex-auth-form" onSubmit={(e) => { e.preventDefault(); repository.mutate(); }}>
        <FormField label={t("rdRepository")}><UIInput required value={repoName} onChange={(e) => setRepoName(e.target.value)} /></FormField>
        <FormField label={t("rdRepoURL")}><UIInput required type="url" value={repoURL} onChange={(e) => setRepoURL(e.target.value)} /></FormField>
        <Button type="submit" isPending={repository.isPending}>{t("rdRepoAdd")}</Button>
      </form>
      <form className="judex-auth-form" onSubmit={(e) => { e.preventDefault(); release.mutate(); }}>
        <FormField label={t("rdVersion")}><UIInput required value={versionLabel} onChange={(e) => setVersion(e.target.value)} /></FormField>
        <FormField label={t("rdEnvironment")}><UIInput required value={environment} onChange={(e) => setEnvironment(e.target.value)} /></FormField>
        <FormField label={t("rdURL")}><UIInput type="url" value={url} onChange={(e) => setURL(e.target.value)} /></FormField>
        <UISelect aria-label={t("rdRelease")} value={status} onChange={(e) => setStatus(e.target.value)}>{([['success', 'rdSuccess'], ['partial', 'rdPartial'], ['failed', 'rdFailed']] as const).map(([value, key]) => <UIOption key={value} value={value}>{t(key)}</UIOption>)}</UISelect>
        <FormField label={t("rdRepository")}><UISelect value={repositoryId} onChange={(e) => setRepository(e.target.value)}><UIOption value="">—</UIOption>{repos.data?.items?.map((r) => <UIOption key={r.id} value={r.id}>{r.displayName}</UIOption>)}</UISelect></FormField>
        <FormField label={t("rdCommit")}><UIInput value={commitSHA} pattern="[a-fA-F0-9]{40}|[a-fA-F0-9]{64}" onChange={(e) => setCommit(e.target.value)} /></FormField>
        <Button type="submit" isDisabled={!!commitSHA && !repositoryId} isPending={release.isPending}>{t("rdRecord")}</Button>
      </form>
      <ul>{releases.data?.items?.map((r) => <li key={r.id}>{r.versionLabel} · {r.environment} · {t(({ success: "rdSuccess", partial: "rdPartial", failed: "rdFailed" } as const)[r.status as "success"] ?? "rdRelease")}</li>)}</ul>
      <FormField label={t("rdBug")}><UISelect value={bug} onChange={(e) => { setBug(e.target.value); fix.reset(); }}><UIOption value="">—</UIOption>{tasks.data?.items?.filter((task) => task.kind === "bug").map((task) => <UIOption key={task.id} value={task.id}>{task.title}</UIOption>)}</UISelect></FormField>
      <strong>{t("rdTargets")}</strong>
      {releases.data?.items?.map((r) => <UICheckbox key={r.id} checked={targets.includes(r.id)} onChange={() => setTargets((values) => values.includes(r.id) ? values.filter((id) => id !== r.id) : [...values, r.id])}>{r.versionLabel} · {r.environment}</UICheckbox>)}
      <Button isDisabled={!bug || !targets.length} isPending={fix.isPending} onPress={() => fix.mutate()}>{t("rdFix")}</Button>
      {fix.isSuccess && <p role="status">{t("rdCreated")}</p>}
      {[releases, repos, tasks, repository, release, fix].map((q, i) => q.isError && <UIWarning key={i}>{q.error?.message}</UIWarning>)}
    </>}
  </section>;
}
