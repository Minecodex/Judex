import type { WorkState } from "./types.ts";
export function normalizeWork(value: unknown): WorkState | null {
  const state = value as WorkState;
  if (
    state?.schema !== 4 ||
    ![
      "projects",
      "positions",
      "seats",
      "plans",
      "tasks",
      "handoffs",
      "flows",
      "topics",
      "events",
      "invites",
      "preferences",
    ].every((key) => Array.isArray(state[key as keyof WorkState]))
  )
    return null;
  return {
    ...state,
    proposals: state.proposals ?? [],
    tasks: state.tasks.map((task) => ({
      ...task,
      revision: Number.isFinite(task.revision) ? task.revision : 1,
      requirements: task.requirements.map((r) =>
        task.id === "package" && r.id === "r-evidence" && !r.at
          ? { ...r, at: "accept" as const }
          : r,
      ),
    })),
  };
}
