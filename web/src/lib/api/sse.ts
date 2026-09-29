import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { apiUrl, request } from "./client";
import {
  PROJECT_EVENT_TYPES,
  invalidationRoots,
} from "./eventMap";

export { EVENT_INVALIDATION_ROOTS, invalidationRoots } from "./eventMap";

// 项目事件流（06 §4 / plans v1 08）：SSE 按 seq 有序，15s 心跳；
// 游标过期 409 时连接失败——重建连接从 0 全量重放，以失效代替增量。
export type ProjectEventRow = {
  eventId: string;
  projectId: string;
  seq: number;
  type: string;
  objectType: string;
  objectId: string;
  version: number | null;
  occurredAt: string;
};

// useProjectEvents 订阅一个项目的事件流并失效相关查询。
// cursor：起始游标（来自 bootstrap.eventCursor）；连接失败超过阈值后从 0 重放。
export function useProjectEvents(projectId: string | null, cursor: number | undefined) {
  const client = useQueryClient();
  const cursorRef = useRef<number | undefined>(cursor);
  const projectRef = useRef(projectId);
  if (projectRef.current !== projectId) { projectRef.current = projectId; cursorRef.current = cursor; }
  if (cursorRef.current === undefined && cursor !== undefined) cursorRef.current = cursor;
  const [connected, setConnected] = useState(false);
  const [generation, setGeneration] = useState(0);
  const failures = useRef(0);

  useEffect(() => {
    if (!projectId) return;
    const after = cursorRef.current;
    const url =
      apiUrl(`/projects/${projectId}/events`) +
      (after && after > 0 ? `?after=${after}` : "");
    const source = new EventSource(url, { withCredentials: true });
    let lastSeq = after ?? 0;
    let disposed = false;
    const recovery = new AbortController();

    const handle = (event: MessageEvent<string>) => {
      failures.current = 0;
      setConnected(true);
      let row: ProjectEventRow | null = null;
      try {
        row = JSON.parse(event.data) as ProjectEventRow;
      } catch {
        return; // 无效帧按未知事件处理，不失效
      }
      if (row.projectId !== projectId || row.seq <= lastSeq) return;
      lastSeq = row.seq;
      cursorRef.current = lastSeq;
      for (const root of invalidationRoots(row.type)) {
        void client.invalidateQueries({ queryKey: [root, projectId] });
      }
    };

    const listeners: Array<[string, EventListener]> = PROJECT_EVENT_TYPES.map(
      (type) => [type, (event: Event) => handle(event as MessageEvent<string>)],
    );
    for (const [type, listener] of listeners) source.addEventListener(type, listener);

    source.onopen = () => {
      failures.current = 0;
      setConnected(true);
    };
    source.onerror = () => {
      setConnected(false);
      // 浏览器会自动重连（带 Last-Event-ID）；连续失败说明游标可能过期，
      // 主动重建：从 0 重放全量失效。
      failures.current += 1;
      if (failures.current >= 5) {
        failures.current = 0;
        source.close();
        void request<{ eventCursor: number }>(`/projects/${projectId}/bootstrap`, { signal: recovery.signal })
          .then((snapshot) => {
            if (disposed) return;
            cursorRef.current = snapshot.eventCursor;
            void client.invalidateQueries({ predicate: (query) => query.queryKey[1] === projectId });
          }).catch(() => undefined).finally(() => { if (!disposed) setGeneration((value) => value + 1); });
      }
    };
    return () => {
      disposed = true;
      recovery.abort();
      source.close();
      setConnected(false);
      for (const [type, listener] of listeners)
        source.removeEventListener(type, listener);
    };
  }, [projectId, client, generation, cursor]);

  return connected;
}
