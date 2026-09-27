import {
  words as W,
  type WorkState,
  type Result,
  type Project,
  type Evidence,
} from "../work/types.ts";
import { member, manage } from "../work/selectors.ts";
import { uid } from "../work/seed.ts";
export const DEFAULT_DISCUSSION_ROUNDS = 3;
export const MAX_DISCUSSION_ROUNDS = 100;
export const validRoundLimit = (value: number) =>
  Number.isInteger(value) && value >= 1 && value <= MAX_DISCUSSION_ROUNDS;
export const roundLimit = (project?: Project) =>
  validRoundLimit(project?.maxDiscussionRounds ?? NaN)
    ? project!.maxDiscussionRounds!
    : DEFAULT_DISCUSSION_ROUNDS;
export type DiscussionRun = {
  id: string;
  sourceId: string;
  trigger: "message" | "material" | "work_report" | "manual";
  maxRounds: number;
  rounds: number;
  status: "waiting_human" | "limit";
  startedAt: number;
};
export function setDiscussionLimit(
  s: WorkState,
  projectId: string,
  value: number,
): Result {
  if (!manage(s, projectId)) return { error: "permission" };
  if (!validRoundLimit(value)) return { error: "required" };
  const next = structuredClone(s);
  next.projects.find((p) => p.id === projectId)!.maxDiscussionRounds = value;
  next.events.push({
    id: uid(),
    projectId,
    targetId: projectId,
    actor: s.currentUser,
    text: W(
      "每次新提交的最大讨论轮次设为 " + value,
      "Maximum discussion rounds per new submission set to " + value,
    ),
    at: Date.now(),
  });
  return { state: next };
}
export function latestDiscussionRun(s: WorkState, topicId: string) {
  return s.topics.find((t) => t.id === topicId)?.discussionRuns?.at(-1);
}
export function discussTopic(s: WorkState, id: string): Result {
  const topic = s.topics.find((t) => t.id === id);
  if (!topic || !member(s, topic.projectId)) return { error: "permission" };
  const source = topic.messages.filter((m) => m.kind === "person").at(-1),
    sourceId = source?.id ?? "manual:" + topic.id;
  const latest = topic.discussionRuns?.at(-1);
  if (latest?.sourceId === sourceId && latest.rounds >= latest.maxRounds)
    return { error: "discussionLimit" };
  const next = structuredClone(s),
    target = next.topics.find((t) => t.id === id)!;
  target.discussionRuns ??= [];
  if (!latest || latest.sourceId !== sourceId)
    target.discussionRuns.push({
      id: uid(),
      sourceId,
      trigger: source?.submissionType ?? (source ? "message" : "manual"),
      maxRounds: roundLimit(
        next.projects.find((p) => p.id === topic.projectId),
      ),
      rounds: 0,
      status: "waiting_human",
      startedAt: Date.now(),
    });
  const run = target.discussionRuns.at(-1)!;
  run.rounds++;
  run.status = run.rounds >= run.maxRounds ? "limit" : "waiting_human";
  const assigned = new Set(
    s.tasks
      .filter((t) => topic.taskIds.includes(t.id))
      .flatMap((t) => [...t.seatIds, t.reviewerSeatId]),
  );
  const seats = s.seats
    .filter(
      (seat) =>
        s.positions.some(
          (p) => p.id === seat.positionId && p.projectId === topic.projectId,
        ) &&
        (assigned.size === 0 || assigned.has(seat.id)),
    )
    .slice(0, 3);
  for (const seat of seats) {
    const position = s.positions.find((p) => p.id === seat.positionId)!;
    target.messages.push({
      id: uid(),
      actor: seat.person,
      seatId: seat.id,
      kind: "ai",
      text: W(
        "第 " +
          run.rounds +
          " 轮 · 从" +
          position.name.zh +
          "的职责出发：" +
          position.prompt.zh +
          "。本议题关联 " +
          topic.taskIds.length +
          " 项任务；先核对交付范围与验收依据，再确认正式安排。",
        "Round " +
          run.rounds +
          " · From the " +
          position.name.en +
          " perspective: " +
          position.prompt.en +
          ". This conversation links " +
          topic.taskIds.length +
          " tasks. Check scope and acceptance evidence before confirming work.",
      ),
      at: Date.now(),
    });
  }
  target.messages.push({
    id: uid(),
    actor: "Judex",
    kind: "ai",
    text: W(
      "本轮已整理职责意见。" +
        (run.status === "limit"
          ? "本次提交已达到 " +
            run.maxRounds +
            " 轮上限，自动讨论结束；可以继续发送新的材料或处理已有提案。"
          : "当前结论等待人工判断，无需跑满上限；可以补充材料或整理为工作提案。"),
      "Role reviews are ready. " +
        (run.status === "limit"
          ? "This submission reached its " +
            run.maxRounds +
            "-round limit. Automatic discussion has ended; you can submit new material or review existing proposals."
          : "The current conclusion needs human judgment; the round limit need not be exhausted. Add context or prepare a work proposal."),
    ),
    at: Date.now(),
  });
  return { state: next };
}
export function discussWorkSubmission(
  s: WorkState,
  taskId: string,
  sourceId: string,
  summary: string,
  files: Evidence[],
): Result {
  const task = s.tasks.find((t) => t.id === taskId);
  if (!task || !member(s, task.projectId)) return { error: "permission" };
  const next = structuredClone(s);
  let topic = next.topics.find(
    (t) => t.projectId === task.projectId && t.taskIds.includes(taskId),
  );
  if (!topic) {
    topic = {
      id: uid(),
      projectId: task.projectId,
      title: W("工作上报：" + task.title.zh, "Work report: " + task.title.en),
      planIds: task.planId ? [task.planId] : [],
      taskIds: [taskId],
      messages: [],
    };
    next.topics.push(topic);
  }
  if (topic.messages.some((m) => m.id === sourceId)) return { state: s };
  topic.messages.push({
    id: sourceId,
    actor: s.currentUser,
    kind: "person",
    submissionType: "work_report",
    text: W(summary),
    files,
    at: Date.now(),
  });
  return discussTopic(next, topic.id);
}
