import {
  discussTopic,
  discussWorkSubmission,
} from "../chat/discussionPolicy.ts";
import {
  words as W,
  type WorkState,
  type Result,
  type Evidence,
  type Task,
  type Text,
} from "./types.ts";
import { uid } from "./seed.ts";
import {
  blockers,
  canWork,
  canAcceptTask,
  canOwnPlan,
  manage,
  member,
  ownsSeat,
  planTasks,
  planReviewKey,
} from "./selectors.ts";
const fail = (error: Exclude<Result["error"], undefined>): Result => ({
  error,
});
function record(
  s: WorkState,
  projectId: string,
  targetId: string,
  zh: string,
  en: string,
): Result {
  s.events.push({
    id: uid(),
    projectId,
    targetId,
    actor: s.currentUser,
    text: W(zh, en),
    at: Date.now(),
  });
  return { state: s };
}
export function sourceDecision(
  state: WorkState,
  handoffId: string,
  sourceId: string,
  revision: number,
  decision: "accepted" | "rejected",
  reason = "",
): Result {
  const h = state.handoffs.find((v) => v.id === handoffId),
    source = h?.sources.find((v) => v.id === sourceId);
  if (
    !h ||
    !source ||
    !member(state, h.projectId) ||
    !ownsSeat(state, h.receiverSeatId)
  )
    return fail("permission");
  if (h.stale || source.status !== "pending" || source.revision !== revision)
    return fail("stale");
  if (decision === "rejected" && !reason.trim()) return fail("required");
  const s = structuredClone(state),
    next = s.handoffs.find((v) => v.id === h.id)!,
    src = next.sources.find((v) => v.id === source.id)!;
  next.history.push({ source: structuredClone(src), at: Date.now() });
  src.status = decision;
  src.reason = decision === "rejected" ? reason.trim() : undefined;
  src.decidedBy = s.currentUser;
  src.decidedAt = Date.now();
  if (decision === "rejected") {
    const task = s.tasks.find((t) => t.id === src.taskId)!;
    if (task.status !== "accepted") task.status = "rework";
  }
  return record(
    s,
    h.projectId,
    h.id,
    decision === "accepted"
      ? "明确接受来源 v" + revision + "；交接接收不替代任务验收。"
      : "拒收来源 v" + revision + "：" + reason.trim(),
    decision === "accepted"
      ? "Explicitly received source v" +
          revision +
          "; receipt is not final acceptance."
      : "Rejected source v" + revision + ": " + reason.trim(),
  );
}
export function reviseSource(
  state: WorkState,
  handoffId: string,
  sourceId: string,
  revision: number,
  summary: string,
  files: Evidence[],
): Result {
  const h = state.handoffs.find((v) => v.id === handoffId),
    source = h?.sources.find((v) => v.id === sourceId);
  if (
    !h ||
    !source ||
    !member(state, h.projectId) ||
    !ownsSeat(state, source.senderSeatId)
  )
    return fail("permission");
  if (
    source.revision !== revision ||
    !["rejected", "draft"].includes(source.status)
  )
    return fail("stale");
  if (state.tasks.find((t) => t.id === source.taskId)?.status === "accepted")
    return fail("accepted");
  if (!summary.trim()) return fail("required");
  const s = structuredClone(state),
    next = s.handoffs.find((v) => v.id === h.id)!,
    src = next.sources.find((v) => v.id === source.id)!;
  next.history.push({ source: structuredClone(src), at: Date.now() });
  Object.assign(src, {
    revision: src.revision + 1,
    summary: W(summary.trim()),
    files: [...src.files, ...files],
    status: "draft",
    reason: undefined,
    sentBy: undefined,
    sentAt: undefined,
    decidedBy: undefined,
    decidedAt: undefined,
  });
  const task = s.tasks.find((t) => t.id === src.taskId)!;
  task.files.push(...files);
  task.revision++;
  if (task.status !== "accepted") task.status = "delivered";
  const recorded = record(
    s,
    h.projectId,
    h.id,
    "补交已整理为 v" +
      src.revision +
      "，等待本人确认发送；其它来源的结果保留。",
    "Revision v" +
      src.revision +
      " is ready for the sender's confirmation. Other source decisions remain.",
  );
  return recorded.error
    ? recorded
    : discussWorkSubmission(
        recorded.state,
        source.taskId,
        "source:" + sourceId + ":" + src.revision,
        summary,
        files,
      );
}
export function sendSource(
  state: WorkState,
  handoffId: string,
  sourceId: string,
  revision: number,
): Result {
  const h = state.handoffs.find((v) => v.id === handoffId),
    source = h?.sources.find((v) => v.id === sourceId);
  if (
    !h ||
    !source ||
    !member(state, h.projectId) ||
    !ownsSeat(state, source.senderSeatId)
  )
    return fail("permission");
  if (h.stale || source.status !== "draft" || source.revision !== revision)
    return fail("stale");
  const s = structuredClone(state),
    src = s.handoffs
      .find((v) => v.id === h.id)!
      .sources.find((v) => v.id === source.id)!;
  src.status = "pending";
  src.sentBy = s.currentUser;
  src.sentAt = Date.now();
  return record(
    s,
    h.projectId,
    h.id,
    "本人确认发送 v" + revision + "，进入待接收。",
    "Sender confirmed v" + revision + "; awaiting explicit receipt.",
  );
}
export function reportTask(
  state: WorkState,
  taskId: string,
  summary: string,
  files: Evidence[],
): Result {
  const task = state.tasks.find((t) => t.id === taskId);
  if (!task || !canWork(state, task)) return fail("permission");
  if (["draft", "accepted"].includes(task.status)) return fail("stale");
  if (!summary.trim()) return fail("required");
  const s = structuredClone(state),
    next = s.tasks.find((t) => t.id === taskId)!;
  next.files.push(
    {
      id: uid(),
      name: "工作上报 / Work report",
      text: summary.trim(),
      author: s.currentUser,
      at: Date.now(),
    },
    ...files,
  );
  next.status = "delivered";
  next.revision++;
  const recorded = record(
    s,
    task.projectId,
    task.id,
    "本人提交工作成果；最终验收尚未完成。",
    "The assigned person reported delivery; final acceptance is still pending.",
  );
  return recorded.error
    ? recorded
    : discussWorkSubmission(
        recorded.state,
        taskId,
        next.files[next.files.length - files.length - 1].id,
        summary,
        files,
      );
}
export function taskAction(
  state: WorkState,
  taskId: string,
  action: "start" | "accept" | "reopen",
  reason = "",
  expectedRevision?: number,
): Result {
  const task = state.tasks.find((t) => t.id === taskId);
  if (!task || !member(state, task.projectId)) return fail("permission");
  if (action === "start" ? !canWork(state, task) : !canAcceptTask(state, task))
    return fail("permission");
  if (action === "reopen" && (task.status !== "accepted" || !reason.trim()))
    return fail("required");
  if (action === "accept" && task.status !== "delivered") return fail("stale");
  if (action === "accept" && expectedRevision !== task.revision)
    return fail("stale");
  if (action === "start" && !["ready", "rework"].includes(task.status))
    return fail("stale");
  if (action !== "reopen" && blockers(state, task, action).length)
    return fail("blocked");
  if (
    task.planId &&
    state.plans.find((p) => p.id === task.planId)?.status === "draft"
  )
    return fail("blocked");
  const s = structuredClone(state),
    next = s.tasks.find((t) => t.id === taskId)!;
  next.status =
    action === "start"
      ? "working"
      : action === "accept"
        ? "accepted"
        : "rework";
  if (action === "accept") next.acceptedAt = Date.now();
  next.revision++;
  return record(
    s,
    task.projectId,
    task.id,
    action === "start"
      ? "本人明确开始工作。"
      : action === "accept"
        ? "有权人确认任务最终验收通过。"
        : "有权人重新打开任务：" +
          reason.trim() +
          "。原验收记录保留，计划状态由负责人决定。",
    action === "start"
      ? "The assigned person started work."
      : action === "accept"
        ? "The authorized reviewer accepted the task outcome."
        : "Task reopened: " +
          reason.trim() +
          ". Prior acceptance remains in history; the plan owner controls the plan status.",
  );
}
export function planAction(
  state: WorkState,
  planId: string,
  action: "activate" | "accept" | "resume",
  expectedReview?: string,
): Result {
  const plan = state.plans.find((p) => p.id === planId);
  if (!plan || !canOwnPlan(state, plan)) return fail("permission");
  const tasks = planTasks(state, planId);
  if (
    action === "accept" &&
    (!tasks.length ||
      tasks.some((t) => t.status !== "accepted") ||
      plan.status !== "active")
  )
    return fail("blocked");
  if (action === "accept" && expectedReview !== planReviewKey(state, plan))
    return fail("stale");
  if (
    (action === "activate" && plan.status !== "draft") ||
    (action === "resume" && plan.status !== "accepted")
  )
    return fail("stale");
  const s = structuredClone(state),
    next = s.plans.find((p) => p.id === planId)!;
  next.status = action === "accept" ? "accepted" : "active";
  if (action === "accept") next.acceptedAt = Date.now();
  return record(
    s,
    plan.projectId,
    plan.id,
    action === "accept"
      ? "计划负责人确认整体验收并更新计划完成。"
      : "计划负责人明确更新计划为进行中。",
    action === "accept"
      ? "The plan owner accepted the whole plan and marked it complete."
      : "The plan owner explicitly set the plan to active.",
  );
}
export function decideDraft(state: WorkState, taskId: string): Result {
  const task = state.tasks.find((t) => t.id === taskId),
    plan = state.plans.find((p) => p.id === task?.planId);
  if (
    !task ||
    !(plan
      ? canOwnPlan(state, plan)
      : member(state, task.projectId)?.role === "owner")
  )
    return fail("permission");
  if (task.status !== "draft") return fail("stale");
  const s = structuredClone(state);
  s.tasks.find((t) => t.id === taskId)!.status = "ready";
  return record(
    s,
    task.projectId,
    taskId,
    "有权人确认工作草稿与分配。",
    "The authorized person confirmed the work draft and assignment.",
  );
}
export function createWork(
  state: WorkState,
  projectId: string,
  draft: {
    kind: "plan" | "task";
    title: string;
    description: string;
    criteria: string;
    seatId: string;
    flowId: string;
    planId: string | null;
    parentId?: string;
  },
): Result {
  if (!member(state, projectId)) return fail("permission");
  if (
    draft.parentId &&
    (draft.kind !== "task" ||
      !draft.planId ||
      !state.tasks.some(
        (task) =>
          task.id === draft.parentId &&
          task.projectId === projectId &&
          task.planId === draft.planId,
      ))
  )
    return fail("scope");
  const position = state.positions.find(
    (p) =>
      p.id === state.seats.find((seat) => seat.id === draft.seatId)?.positionId,
  );
  if (
    !position ||
    position.projectId !== projectId ||
    !state.flows.some(
      (f) => f.id === draft.flowId && f.projectId === projectId,
    ) ||
    (draft.planId &&
      !state.plans.some(
        (p) => p.id === draft.planId && p.projectId === projectId,
      ))
  )
    return fail("scope");
  if (![draft.title, draft.description, draft.criteria].every((v) => v.trim()))
    return fail("required");
  const s = structuredClone(state),
    id = uid(),
    criteria = draft.criteria
      .split("\n")
      .map((v) => v.trim())
      .filter(Boolean)
      .map((v) => W(v));
  if (draft.kind === "plan")
    s.plans.push({
      id,
      projectId,
      title: W(draft.title),
      goal: W(draft.description),
      criteria,
      ownerSeatId: draft.seatId,
      flowId: draft.flowId,
      status: "draft",
      referenceTaskIds: [],
    });
  else
    s.tasks.push({
      revision: 1,
      id,
      projectId,
      title: W(draft.title),
      expected: W(draft.description),
      criteria,
      seatIds: [draft.seatId],
      reviewerSeatId: draft.planId
        ? state.plans.find((p) => p.id === draft.planId)!.ownerSeatId
        : draft.seatId,
      planId: draft.planId,
      parentId: draft.parentId || undefined,
      flowId: draft.flowId,
      nodeId: state.flows.find((f) => f.id === draft.flowId)!.nodes[0].id,
      status: "draft",
      requirements: [],
      files: [],
    });
  return record(
    s,
    projectId,
    id,
    "已创建工作草稿，等待有权人确认。",
    "Work draft created; awaiting an authorized decision.",
  );
}
export function topicMessage(
  state: WorkState,
  topicId: string,
  body: string,
  files: Evidence[] = [],
): Result {
  const topic = state.topics.find((t) => t.id === topicId);
  if (!topic || !member(state, topic.projectId)) return fail("permission");
  if (!body.trim() && !files.length) return fail("required");
  const s = structuredClone(state),
    next = s.topics.find((t) => t.id === topicId)!;
  next.messages.push(
    {
      id: uid(),
      actor: s.currentUser,
      kind: "person",
      submissionType: files.length ? "material" : "message",
      text: W(body.trim()),
      files,
      at: Date.now(),
    },
    {
      id: uid(),
      actor: "Judex",
      kind: "ai",
      text: W(
        "补充已记录为讨论。涉及正式分配或完成状态，请使用对应的工作与决定入口。此为 Demo 示例回复。",
        "Recorded as discussion. Use the work or decision action for formal changes. This is a simulated demo reply.",
      ),
      at: Date.now(),
    },
  );
  return discussTopic(s, topicId);
}
export function publishFlow(
  state: WorkState,
  flowId: string,
  version: number,
  instructions: string,
  material: boolean,
): Result {
  const flow = state.flows.find((f) => f.id === flowId);
  if (!flow || !manage(state, flow.projectId)) return fail("permission");
  if (flow.version !== version) return fail("stale");
  if (!instructions.trim()) return fail("required");
  const s = structuredClone(state),
    next = s.flows.find((f) => f.id === flowId)!;
  next.history.push({
    version: flow.version,
    instructions: flow.instructions,
    actor: s.currentUser,
  });
  next.instructions = W(instructions.trim());
  next.version++;
  if (material)
    s.handoffs.forEach((h) => {
      if (h.flowId === flowId && h.sources.some((v) => v.status !== "accepted"))
        h.stale = true;
    });
  return record(
    s,
    flow.projectId,
    flowId,
    "人工发布流程 v" +
      next.version +
      "，后续参考直接更新。" +
      (material ? "受影响的在途交接需重核。" : ""),
    "Flow v" +
      next.version +
      " published for subsequent context." +
      (material ? "Affected open handoffs need review." : ""),
  );
}
export function refreshHandoff(state: WorkState, handoffId: string): Result {
  const h = state.handoffs.find((h) => h.id === handoffId);
  if (
    !h ||
    !member(state, h.projectId) ||
    !(
      ownsSeat(state, h.receiverSeatId) ||
      h.sources.some((v) => ownsSeat(state, v.senderSeatId)) ||
      manage(state, h.projectId)
    )
  )
    return fail("permission");
  if (!h.stale) return fail("stale");
  const s = structuredClone(state),
    next = s.handoffs.find((v) => v.id === handoffId)!;
  next.flowVersion = s.flows.find((f) => f.id === next.flowId)!.version;
  next.stale = false;
  next.sources.forEach((src) => {
    if (src.status === "pending") {
      next.history.push({ source: structuredClone(src), at: Date.now() });
      src.status = "draft";
      src.revision++;
      src.sentBy = undefined;
      src.sentAt = undefined;
    }
  });
  return record(
    s,
    h.projectId,
    h.id,
    "按新版整理交接草稿。受影响来源等待各发送人重新确认，原记录保留。",
    "Updated the handoff draft. Affected contributors must reconfirm; previous records remain.",
  );
}
export function personalPrompt(
  state: WorkState,
  projectId: string,
  prompt: string,
): Result {
  if (!member(state, projectId)) return fail("permission");
  const s = structuredClone(state);
  s.preferences = [
    ...s.preferences.filter(
      (p) => p.projectId !== projectId || p.person !== s.currentUser,
    ),
    { projectId, person: s.currentUser, prompt },
  ];
  return { state: s };
}
export function savePosition(
  state: WorkState,
  projectId: string,
  value: {
    id?: string;
    name: string;
    prompt: string;
    flowId: string;
    nodeId: string;
  },
): Result {
  if (!manage(state, projectId)) return fail("permission");
  if (![value.name, value.prompt].every((v) => v.trim()))
    return fail("required");
  if (
    !state.flows.some(
      (f) =>
        f.id === value.flowId &&
        f.projectId === projectId &&
        f.nodes.some((n) => n.id === value.nodeId),
    )
  )
    return fail("scope");
  if (
    value.id &&
    !state.positions.some((p) => p.id === value.id && p.projectId === projectId)
  )
    return fail("scope");
  const s = structuredClone(state),
    id = value.id ?? uid();
  const position = {
    id,
    projectId,
    name: W(value.name.trim()),
    prompt: W(value.prompt.trim()),
    tone: "mint",
    bindings: [{ flowId: value.flowId, nodeId: value.nodeId }],
  };
  if (value.id)
    s.positions = s.positions.map((p) =>
      p.id === id ? { ...p, ...position, tone: p.tone } : p,
    );
  else s.positions.push(position);
  return record(
    s,
    projectId,
    id,
    "管理者更新职位模板与具体节点挂载。",
    "Manager updated the position template and node binding.",
  );
}
export function invitePerson(
  state: WorkState,
  projectId: string,
  person: string,
  positionIds: string[],
): Result {
  if (!manage(state, projectId)) return fail("permission");
  if (
    !person.trim() ||
    !positionIds.length ||
    new Set(positionIds).size !== positionIds.length
  )
    return fail("required");
  if (
    positionIds.some(
      (id) =>
        !state.positions.some((p) => p.id === id && p.projectId === projectId),
    ) ||
    state.projects
      .find((p) => p.id === projectId)!
      .members.some((m) => m.name === person.trim()) ||
    state.invites.some(
      (i) =>
        i.projectId === projectId &&
        i.person === person.trim() &&
        i.status === "pending",
    )
  )
    return fail("invite");
  const s = structuredClone(state);
  s.invites.push({
    id: uid(),
    projectId,
    person: person.trim(),
    positionIds,
    status: "pending",
    sender: s.currentUser,
  });
  return record(
    s,
    projectId,
    projectId,
    "邀请 " + person.trim() + " 承担所选职位；同岗已有成员不影响邀请。",
    "Invited " +
      person.trim() +
      " to selected positions; existing holders do not occupy the template.",
  );
}
export function assignPositions(
  state: WorkState,
  projectId: string,
  person: string,
  positionIds: string[],
): Result {
  if (!manage(state, projectId)) return fail("permission");
  if (
    !positionIds.length ||
    new Set(positionIds).size !== positionIds.length ||
    !state.projects
      .find((p) => p.id === projectId)
      ?.members.some((m) => m.name === person)
  )
    return fail("required");
  if (
    positionIds.some(
      (id) =>
        !state.positions.some((p) => p.id === id && p.projectId === projectId),
    )
  )
    return fail("scope");
  if (
    positionIds.some((id) =>
      state.seats.some(
        (seat) => seat.positionId === id && seat.person === person,
      ),
    )
  )
    return fail("required");
  const s = structuredClone(state);
  for (const positionId of positionIds)
    s.seats.push({
      id: uid(),
      positionId,
      person,
      notes: W(
        "由管理者明确分配的任职身份。",
        "Work identity explicitly assigned by a manager.",
      ),
    });
  return record(
    s,
    projectId,
    projectId,
    "管理者为现有成员 " + person + " 分配职位。",
    "Manager assigned positions to existing member " + person + ".",
  );
}
export function acceptInvite(state: WorkState, invitationId: string): Result {
  const invite = state.invites.find(
    (i) =>
      i.id === invitationId &&
      i.person === state.currentUser &&
      i.status === "pending",
  );
  if (!invite || member(state, invite.projectId)) return fail("invite");
  const s = structuredClone(state);
  s.invites.find((i) => i.id === invite.id)!.status = "accepted";
  s.projects
    .find((p) => p.id === invite.projectId)!
    .members.push({ name: s.currentUser, role: "member" });
  invite.positionIds.forEach((id) =>
    s.seats.push({
      id: uid(),
      positionId: id,
      person: s.currentUser,
      notes: W(
        "新任职身份，等待工作安排。",
        "New work identity; ready for assignments.",
      ),
    }),
  );
  return record(
    s,
    invite.projectId,
    invite.id,
    "本人接受职位邀请；个人提示词由本人设置。",
    "The invitee accepted the positions and can now set personal preferences.",
  );
}
export function replaceSeat(
  state: WorkState,
  seatId: string,
  expectedPerson: string,
  person: string,
): Result {
  const seat = state.seats.find((v) => v.id === seatId),
    position = state.positions.find((p) => p.id === seat?.positionId);
  if (!seat || !position || !manage(state, position.projectId))
    return fail("permission");
  if (seat.person !== expectedPerson) return fail("stale");
  if (
    person === seat.person ||
    !state.projects
      .find((p) => p.id === position.projectId)!
      .members.some((m) => m.name === person)
  )
    return fail("required");
  const s = structuredClone(state);
  s.seats.find((v) => v.id === seatId)!.person = person;
  return record(
    s,
    position.projectId,
    seatId,
    "关联人由 " +
      expectedPerson +
      " 替换为 " +
      person +
      "；工作、历史和原确认保留，个人偏好不转移。",
    "Replaced " +
      expectedPerson +
      " with " +
      person +
      ". Work and decisions remain; personal preferences do not transfer.",
  );
}
export function createFlow(
  state: WorkState,
  projectId: string,
  name: string,
  instructions: string,
  labels: string[],
): Result {
  if (!manage(state, projectId)) return fail("permission");
  if (
    !name.trim() ||
    !instructions.trim() ||
    labels.length < 2 ||
    labels.some((v) => !v.trim())
  )
    return fail("required");
  const s = structuredClone(state),
    id = uid(),
    nodes = labels.map((label, i) => ({
      id: "step" + i,
      label: W(label.trim()),
    }));
  s.flows.push({
    id,
    projectId,
    name: W(name.trim()),
    instructions: W(instructions.trim()),
    version: 1,
    nodes,
    edges: nodes.slice(1).map((node, i) => [nodes[i].id, node.id]),
    history: [],
  });
  return record(
    s,
    projectId,
    id,
    "本人确认新流程草稿，规则与图以 v1 发布。",
    "The person confirmed a new workflow draft; rules and diagram published as v1.",
  );
}
export function remindPending(state: WorkState, projectId: string): Result {
  if (!manage(state, projectId)) return fail("permission");
  const s = structuredClone(state);
  return record(
    s,
    projectId,
    projectId,
    "模拟接收提醒：已提醒待接收成员及计划负责人；不自动接受或改变任务状态。",
    "Simulated receipt reminders to recipients and plan owners. No receipt or task status was changed.",
  );
}
