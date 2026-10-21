// One selected batch is one intent. Confirmed items stay attached to that
// intent until every item is confirmed, including when a subset is reselected.
// Binary data is kept in this signed-in workspace's memory, never localStorage.
type Transport = <T>(path: string, init?: RequestInit & {idempotencyKey?: string}) => Promise<T>;
type Update = {fileId: string; version: number};
type Session = {id: string; partSize: number; partCount: number};
type Item = {
  file: File;
  fingerprint: string;
  metadata: {name: string; mime: string; size: number; sha256: string; fileId?: string; expectedVersion?: number};
  createKey: string;
  completeKey: string;
  partKeys: Record<number, string>;
  session?: Session;
  nextPart: number;
  result?: {id: string};
};
type Batch = {id: string; items: Item[]; running: boolean; progress: string};
export class PendingUploadBatch extends Error {}

export function createUploadManager(request: Transport) {
  const batches = new Map<string, Batch>();
  const listeners = new Set<() => void>();
  let revision = 0;
  const changed = () => { revision++; listeners.forEach(listener => listener()); };
  async function prepare(file: File, update?: Update): Promise<Item> {
    if (file.size > 100 * 1024 * 1024) throw new Error('Maximum file size: 100 MiB');
    const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer());
    const metadata = {
      name: file.name, mime: file.type, size: file.size,
      sha256: Array.from(new Uint8Array(digest), b => b.toString(16).padStart(2, '0')).join(''),
      ...(update ? {fileId: update.fileId, expectedVersion: update.version} : {}),
    };
    return {file, metadata, fingerprint: JSON.stringify(metadata), createKey: crypto.randomUUID(),
      completeKey: crypto.randomUUID(), partKeys: {}, nextPart: 1};
  }
  async function run(room: string, batch: Batch, selected: Item[]) {
    if (batch.running) return false;
    batch.running = true;
    changed();
    try {
      for (const item of selected) {
        if (item.result) continue;
        batch.progress = item.file.name;
        changed();
        const base = `/conversations/${room}/uploads`;
        if (!item.session) item.session = await request<Session>(base, {
          method: 'POST', body: JSON.stringify(item.metadata), idempotencyKey: item.createKey,
        });
        const session = item.session, bytes = await item.file.arrayBuffer();
        for (let part = item.nextPart; part <= session.partCount; part++) {
          batch.progress = `${item.file.name} · ${Math.round((part - 1) / session.partCount * 100)}%`;
          changed();
          item.partKeys[part] ??= crypto.randomUUID();
          await request(`${base}/${session.id}/parts/${part}`, {
            method: 'PUT', headers: {'Content-Type': 'application/octet-stream'},
            body: bytes.slice((part - 1) * session.partSize, part * session.partSize),
            idempotencyKey: item.partKeys[part],
          });
          item.nextPart = part + 1;
        }
        const result = await request<{id: string}>(`${base}/${session.id}/complete`, {
          method: 'POST', body: '{}', idempotencyKey: item.completeKey,
        });
        if (!result?.id) throw new Error('Upload result could not be confirmed');
        item.result = result;
        changed();
      }
      const complete = batch.items.every(item => item.result);
      if (complete) batches.delete(room);
      return complete;
    } finally {
      batch.running = false;
      batch.progress = '';
      changed();
    }
  }
  return {
    subscribe(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    revision: () => revision,
    status(room: string) {
      const batch = batches.get(room);
      return batch ? {id: batch.id, running: batch.running, progress: batch.progress,
        items: batch.items.map(item => ({name: item.file.name, confirmed: !!item.result}))} : null;
    },
    async upload(room: string, files: File[], update?: Update) {
      if (!files.length || batches.get(room)?.running) return false;
      // Hash before registering: validation cannot leave half a new batch.
      const prepared = await Promise.all(files.map(file => prepare(file, update)));
      let batch = batches.get(room);
      let selected: Item[];
      if (batch) {
        const available = [...batch.items];
        selected = prepared.map(item => {
          const index = available.findIndex(candidate => candidate.fingerprint === item.fingerprint);
          if (index === -1) throw new PendingUploadBatch('Finish or close the pending batch before selecting different files');
          return available.splice(index, 1)[0];
        });
      } else {
        batch = {id: crypto.randomUUID(), items: prepared, running: false, progress: ''};
        batches.set(room, batch);
        selected = prepared;
      }
      return run(room, batch, selected);
    },
    async resume(room: string) {
      const batch = batches.get(room);
      return batch ? run(room, batch, batch.items) : false;
    },
    close(room: string) {
      if (batches.get(room)?.running) return;
      // Explicitly ending an intent does not delete already committed files.
      // Unknown results remain on the server; a new selection is a new upload.
      batches.delete(room);
      changed();
    },
  };
}
