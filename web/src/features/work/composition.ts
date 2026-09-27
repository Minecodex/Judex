import {
  words as W,
  type WorkState,
  type Result,
  type Handoff,
} from "./types.ts";
import { uid } from "./seed.ts";
import { member } from "./selectors.ts";
export function proposeHandoff(
  s: WorkState,
  projectId: string,
  taskId: string,
  receiverSeatId: string,
  sources: string[],
  kind: Handoff["kind"],
): Result {
  if (!member(s, projectId)) return { error: "permission" };
  const task = s.tasks.find(
    (t) => t.id === taskId && t.projectId === projectId,
  );
  if (
    !task ||
    ![...task.seatIds, task.reviewerSeatId].includes(receiverSeatId) ||
    !sources.length ||
    new Set(sources).size !== sources.length
  )
    return { error: "scope" };
  const tasks = sources.map((id) =>
    s.tasks.find((t) => t.id === id && t.projectId === projectId),
  );
  if (
    tasks.some(
      (t) =>
        !t || !["delivered", "accepted"].includes(t.status) || !t.files.length,
    )
  )
    return { error: "blocked" };
  if (
    (kind === "stage" && (sources.length !== 1 || sources[0] !== taskId)) ||
    (kind === "dependency" && sources.includes(taskId))
  )
    return { error: "scope" };
  if (
    task.status === "draft" ||
    (task.planId &&
      s.plans.find((p) => p.id === task.planId)?.status === "draft")
  )
    return { error: "blocked" };
  const flow = s.flows.find((f) => f.id === task.flowId)!;
  const next = structuredClone(s),
    id = uid();
  next.handoffs.push({
    id,
    projectId,
    taskId,
    receiverSeatId,
    flowId: flow.id,
    flowVersion: flow.version,
    stale: false,
    kind,
    title: W("交接：" + task.title.zh, "Handoff: " + task.title.en),
    history: [],
    sources: tasks.map((t) => ({
      id: uid(),
      taskId: t!.id,
      senderSeatId: t!.seatIds[0],
      revision: 1,
      summary: t!.expected,
      files: structuredClone(t!.files),
      status: "draft",
    })),
  });
  next.events.push({
    id: uid(),
    projectId,
    targetId: id,
    actor: s.currentUser,
    text: W(
      "整理交接草稿，各来源等待本人确认发送；没有修改既有分配。",
      "Prepared a handoff draft. Each source needs its sender's confirmation; assignments stay unchanged.",
    ),
    at: Date.now(),
  });
  return { state: next };
}
export function createDiscussion(
  s: WorkState,
  projectId: string,
  title: string,
  planIds: string[],
  taskIds: string[],
): Result {
  if (!member(s, projectId)) return { error: "permission" };
  if (!title.trim()) return { error: "required" };
  if (
    planIds.some(
      (id) => !s.plans.some((p) => p.id === id && p.projectId === projectId),
    ) ||
    taskIds.some(
      (id) => !s.tasks.some((t) => t.id === id && t.projectId === projectId),
    )
  )
    return { error: "scope" };
  const next = structuredClone(s);
  next.topics.push({
    id: uid(),
    projectId,
    title: W(title.trim()),
    planIds: [...new Set(planIds)],
    taskIds: [...new Set(taskIds)],
    messages: [],
  });
  return { state: next };
}
