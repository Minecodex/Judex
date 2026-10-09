import type { Task, Handoff } from "../work/types.ts";
export const GRAPH = {
  width: 236,
  height: 164,
  column: 304,
  row: 206,
  pad: 24,
};
export function executionLayout(tasks: Task[], handoffs: Handoff[], dimensions=GRAPH) {
  const ids = new Set(tasks.map((t) => t.id));
  const edges: { from: string; to: string; hard: boolean; at: string }[] = [];
  const external = new Map<string, string[]>();
  for (const task of tasks)
    for (const r of task.requirements) {
      const sources =
        r.kind === "task"
          ? [r.ref]
          : r.kind === "receipt"
            ? (handoffs
                .find((h) => h.id === r.ref)
                ?.sources.map((s) => s.taskId) ?? [])
            : [];
      if (r.kind === "receipt" && !sources.length)
        external.set(task.id, [...(external.get(task.id) ?? []), r.ref]);
      for (const from of new Set(sources)) {
        if (!ids.has(from)) {
          external.set(task.id, [...(external.get(task.id) ?? []), from]);
          continue;
        }
        const existing = edges.find((e) => e.from === from && e.to === task.id);
        if (existing) {
          existing.hard ||= r.hard;
          if (existing.at !== (r.at ?? "both")) existing.at = "both";
        } else
          edges.push({ from, to: task.id, hard: r.hard, at: r.at ?? "both" });
      }
    }
  const incoming = new Map(
    tasks.map((t) => [t.id, edges.filter((e) => e.to === t.id).length]),
  );
  const rank = new Map(tasks.map((t) => [t.id, 0]));
  const queue = tasks.filter((t) => !incoming.get(t.id)).map((t) => t.id),
    visited = new Set<string>();
  while (queue.length) {
    const id = queue.shift()!;
    visited.add(id);
    for (const edge of edges.filter((e) => e.from === id)) {
      rank.set(edge.to, Math.max(rank.get(edge.to)!, rank.get(id)! + 1));
      incoming.set(edge.to, incoming.get(edge.to)! - 1);
      if (!incoming.get(edge.to)) queue.push(edge.to);
    }
  }
  const invalid = tasks.filter((t) => !visited.has(t.id)).map((t) => t.id);
  const columns = new Map<number, Task[]>();
  tasks.forEach((t) => {
    const col = invalid.includes(t.id) ? 0 : rank.get(t.id)!;
    columns.set(col, [...(columns.get(col) ?? []), t]);
  });
  const positions = new Map<string, { x: number; y: number; rank: number }>();
  columns.forEach((items, col) =>
    items.forEach((t, row) =>
      positions.set(t.id, {
        x: dimensions.pad + col * dimensions.column,
        y: dimensions.pad + row * dimensions.row,
        rank: col,
      }),
    ),
  );
  return {
    edges,
    external,
    invalid,
    positions,
    width:
      dimensions.pad * 2 +
      Math.max(0, ...columns.keys()) * dimensions.column +
      dimensions.width,
    height:
      dimensions.pad * 2 +
      Math.max(1, ...[...columns.values()].map((v) => v.length)) * dimensions.row,
  };
}
