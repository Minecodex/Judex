import {useCollection,LoadMore} from "../../lib/api/collections";
import { useState } from "react";
import { Button } from "@heroui/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UIFilePicker, UIWarning, UICheckbox } from "../../components/ui/FormControls";
import { apiUrl, request } from "../../lib/api/client";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { uploadFile, type MaterialVersion } from "./upload";

type Material = { id: string; title: string; kind: string; currentVersionId: string };
export function MaterialsBrowser({ projectId }: { projectId: string }) {
  const { locale } = usePreferences(); const t = (key: Key) => translate(locale, key); const client = useQueryClient();
  const [selected, setSelected] = useState<Material | null>(null); const [progress, setProgress] = useState(0); const [bundle, setBundle] = useState(false);
  const materials=useCollection<Material>(["materials",projectId],`/projects/${projectId}/materials`);
  const upload = useMutation({ mutationFn: (file: File) => uploadFile(projectId, file, setProgress, bundle), onSuccess: () => { void client.invalidateQueries({ queryKey: ["materials", projectId] }); } });
  const versions=useCollection<MaterialVersion>(["materialVersions",projectId,selected?.id],`/projects/${projectId}/materials/${selected?.id}/versions`,!!selected);
  const preview = useMutation({ mutationFn: (version: MaterialVersion) => request<{ previewUrl: string }>(`/projects/${projectId}/materials/${version.materialId}/versions/${version.id}/preview-session`, { method: "POST", body: "{}", idempotencyKey: crypto.randomUUID() }) });
  return <div className="judex-panel-stack">
    <h3>{t("wsPanelMaterials")}</h3>
    <UICheckbox checked={bundle} onChange={(e) => setBundle(e.target.checked)}>HTML / ZIP</UICheckbox>
    <UIFilePicker aria-label={t("paUpload")} disabled={upload.isPending} onChange={(e) => { const file = e.target.files?.[0]; if (file) upload.mutate(file); e.target.value = ""; }}>{t("paUpload")}</UIFilePicker>
    {upload.isPending && <p role="status">{progress}%</p>}
    {upload.isError && <UIWarning>{upload.error.message}</UIWarning>}
    {(materials.data?.items ?? []).map((material) => <Button key={material.id} variant="ghost" onPress={() => { setSelected(material); preview.reset(); }}>{material.title}</Button>)}
    <LoadMore query={materials} />
    {selected && <h4>{selected.title} · {t("paVersions")}</h4>}
    {(versions.data?.items ?? []).map((version) => <article key={version.id} className="judex-panel-stack">
      <strong>v{version.revision}</strong><small>{version.sha256} · {version.size}</small>
      {selected?.kind === "html_bundle" ? <>
        <Button onPress={() => preview.mutate(version)} isPending={preview.isPending}>{t("paPreview")}</Button>
        {version.entries?.map((entry) => <a key={entry.relativePath} href={apiUrl(`/projects/${projectId}/materials/${version.materialId}/versions/${version.id}/content?entry=${encodeURIComponent(entry.relativePath)}`)}>{entry.relativePath}</a>)}
      </> : <a href={apiUrl(`/projects/${projectId}/materials/${version.materialId}/versions/${version.id}/content`)}>{t("paDownload")}</a>}
    </article>)}
    {preview.data?.previewUrl && <a href={preview.data.previewUrl} target="_blank" rel="noopener noreferrer">{t("paPreview")}</a>}
    {preview.isError && <UIWarning>{preview.error.message}</UIWarning>}
    {(materials.isError || versions.isError) && <UIWarning>{t("accessUnavailable")}</UIWarning>}
    <LoadMore query={versions} />
  </div>;
}
