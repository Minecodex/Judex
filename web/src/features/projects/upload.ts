import { request } from "../../lib/api/client";

type Upload = { id: string; checksum: string; name: string; partSize: number; partCount: number; state: string };
export type MaterialVersion = { id: string; materialId: string; revision: number; state: string; sha256: string; size: number; entries: { relativePath: string; mime: string }[] };
async function sha256(bytes: ArrayBuffer) {
  const hash = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(hash)).map((n) => n.toString(16).padStart(2, "0")).join("");
}
export async function uploadFile(project: string, file: File, progress: (value: number) => void = () => {}, bundle = false, entrypoint = "index.html"): Promise<MaterialVersion> {
  const base = `/projects/${project}/uploads`;
  const checksum = await sha256(await file.arrayBuffer());
  const open = await request<{ items: Upload[] }>(base);
  const upload = open.items.find((u) => u.checksum === checksum && u.name === file.name) ?? await request<Upload>(base, {
    method: "POST", idempotencyKey: crypto.randomUUID(), body: JSON.stringify({ name: file.name, size: file.size, sha256: checksum, mime: file.type || "application/octet-stream", kind: bundle ? "html_bundle" : "file", ...(bundle ? { entrypoint } : {}) }),
  });
  for (let part = 0; part < upload.partCount; part++) {
    const bytes = await file.slice(part * upload.partSize, (part + 1) * upload.partSize).arrayBuffer();
    await request(`${base}/${upload.id}/parts/${part + 1}`, { method: "PUT", headers: { "Content-Type": "application/octet-stream", "X-Judex-Part-SHA256": await sha256(bytes) }, body: bytes });
    progress(Math.round((part + 1) / upload.partCount * 100));
  }
  const version = await request<MaterialVersion>(`${base}/${upload.id}/complete`, { method: "POST", idempotencyKey: crypto.randomUUID(), body: "{}" });
  if (version.state !== "ready") throw new Error("Material is not ready");
  return version;
}
