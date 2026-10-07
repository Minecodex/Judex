// api 模式命名动作执行器（P2）：与 actionRegistry 命名一一对应的真实 REST 写操作。
// 未在 apiActions 中出现的动作在 api 模式仍不可用（调用方提示 shellUnavailable）。
// 约定：写操作带幂等键；需要 reviewHash/expectedVersion 的操作先读冻结引用再提交。
import { APIError, request } from "../../lib/api/client";
import type { components } from "../../lib/api/schema";
import type { ActResult, ActionPayloads } from "./storeTypes";
import type { Evidence, WorkState } from "./types";
import type { ProposalInput } from "../chat/proposalModel";

type Schema = components["schemas"];
type ApiProposal = Schema["Proposal"];
type ApiProposalChange = Schema["ProposalChange"];
type ApiProposalReview = Schema["ProposalReview"];
type ApiAcceptanceReview = Schema["AcceptanceReview"];
type ApiMessage = Schema["Message"];
type ApiTask = Schema["Task"];
type ApiPlan = Schema["Plan"];
type ApiHandoffSource = Schema["HandoffSource"] & { summary?: string };
type ApiHandoff = Omit<Schema["Handoff"], "sources"> & {
  sources: ApiHandoffSource[];
};
type ApiTopic = Schema["Topic"];
type ApiProject = Schema["Project"];
type ApiPosition = Schema["Position"];
type ApiMember = Schema["ProjectMember"];
type ApiIdentity = Schema["Identity"];
type ApiInvitation = Schema["Invitation"];
type ApiPreferences = Schema["PersonalPreferences"];
type ApiWorkflow = Schema["Workflow"];
type ApiWorkflowVersion = Schema["WorkflowVersion"];

// 执行器可读的当前装配状态与当前用户（person 一律 displayName）。
export type ApiActContext = {
  state: WorkState;
  userId: string;
  displayName: string;
};

type Executor<K extends keyof ActionPayloads> = (
  projectId: string,
  payload: ActionPayloads[K],
  ctx: ApiActContext,
) => Promise<ActResult>;

const post = <T>(path: string, body: unknown): Promise<T> =>
  request<T>(path, {
    method: "POST",
    body: JSON.stringify(body),
    idempotencyKey: crypto.randomUUID(),
  });

const put = <T>(path: string, body: unknown): Promise<T> =>
  request<T>(path, {
    method: "PUT",
    body: JSON.stringify(body),
    idempotencyKey: crypto.randomUUID(),
  });

const patch = <T>(path: string, body: unknown): Promise<T> =>
  request<T>(path, {
    method: "PATCH",
    body: JSON.stringify(body),
    idempotencyKey: crypto.randomUUID(),
  });

const listItems = async <T>(path: string): Promise<T[]> =>
  (await request<{ items: T[] }>(path)).items ?? [];

// 发布当前草稿：expectedVersion 必须取调用瞬间的流程头版本（草稿保存会推进头版本）。
async function publishCurrentDraft(projectId: string, workflowId: string) {
  const [heads, versions] = await Promise.all([
    listItems<ApiWorkflow>(`/projects/${projectId}/workflows`),
    listItems<ApiWorkflowVersion>(
      `/projects/${projectId}/workflows/${workflowId}/versions`,
    ),
  ]);
  const head = heads.find((w) => w.id === workflowId);
  const draft = versions.find((v) => v.state === "draft");
  if (!head || !draft) return;
  await post(`/projects/${projectId}/workflows/${workflowId}/publish`, {
    expectedVersion: head.version ?? 1,
    draftHash: draft.draftHash,
  });
}

async function memberUserId(
  projectId: string,
  displayName: string,
): Promise<string> {
  const members = await listItems<ApiMember>(
    `/projects/${projectId}/members?limit=100`,
  );
  const user = members.find(
    (m) => m.displayName === displayName && m.state === "active",
  );
  if (!user) throw new APIError(404, "NOT_FOUND", "member not found");
  return user.userId;
}

// ---- 附件：demo Evidence（dataURL/文本）还原成 File 走分片上传 ----

async function sha256(bytes: ArrayBuffer) {
  const hash = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(hash))
    .map((n) => n.toString(16).padStart(2, "0"))
    .join("");
}

type UploadSession = {
  id: string;
  checksum: string;
  name: string;
  partSize: number;
  partCount: number;
};

async function uploadFile(projectId: string, file: File): Promise<string> {
  const base = `/projects/${projectId}/uploads`;
  const checksum = await sha256(await file.arrayBuffer());
  const open = await request<{ items: UploadSession[] }>(base);
  const upload =
    open.items.find((u) => u.checksum === checksum && u.name === file.name) ??
    (await post<UploadSession>(base, {
      name: file.name,
      size: file.size,
      sha256: checksum,
      mime: file.type || "application/octet-stream",
      kind: "file",
    }));
  for (let part = 0; part < upload.partCount; part++) {
    const bytes = await file
      .slice(part * upload.partSize, (part + 1) * upload.partSize)
      .arrayBuffer();
    await request(`${base}/${upload.id}/parts/${part + 1}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/octet-stream",
        "X-Judex-Part-SHA256": await sha256(bytes),
      },
      body: bytes,
    });
  }
  const version = await post<{ id: string; state: string }>(
    `${base}/${upload.id}/complete`,
    {},
  );
  if (version.state !== "ready")
    throw new APIError(409, "INVALID_TRANSITION", "material not ready");
  return version.id;
}

async function evidenceToFile(evidence: Evidence): Promise<File> {
  if (evidence.data) {
    const blob = await (await fetch(evidence.data)).blob();
    return new File([blob], evidence.name, {
      type: evidence.type || blob.type || "application/octet-stream",
    });
  }
  return new File([evidence.text], evidence.name, {
    type: evidence.type || "text/plain",
  });
}

async function uploadEvidence(
  projectId: string,
  files: Evidence[] | undefined,
): Promise<string[]> {
  const ids: string[] = [];
  for (const evidence of files ?? [])
    ids.push(await uploadFile(projectId, await evidenceToFile(evidence)));
  return ids;
}

// ---- 共享读取与提案流 ----

const fetchTask = (projectId: string, taskId: string) =>
  request<ApiTask>(`/projects/${projectId}/tasks/${taskId}`);
const fetchPlan = (projectId: string, planId: string) =>
  request<ApiPlan>(`/projects/${projectId}/plans/${planId}`);
const fetchReview = (projectId: string, proposalId: string) =>
  request<ApiProposalReview>(
    `/projects/${projectId}/proposals/${proposalId}/review`,
  );

async function findHandoffSource(
  projectId: string,
  handoffId: string,
  sourceId: string,
): Promise<ApiHandoffSource> {
  const page = await request<{ items: ApiHandoff[] }>(
    `/projects/${projectId}/handoffs?limit=100`,
  );
  const source = page.items
    .find((h) => h.id === handoffId)
    ?.sources.find((s) => s.id === sourceId);
  if (!source) throw new APIError(404, "NOT_FOUND", "handoff source not found");
  return source;
}

// 参与者身份：取参与者列表中绑定当前用户的 identity。
function actingIdentity(
  ctx: ApiActContext,
  task: ApiTask,
): string | undefined {
  return task.participants?.find((p) => p.displayName === ctx.displayName)
    ?.identityId;
}

// 提交并（按当前用户可决定的席位）批准一个提案；返回提案 id。
async function submitAndApprove(
  projectId: string,
  changes: ApiProposalChange[],
): Promise<string> {
  const proposal = await post<ApiProposal>(`/projects/${projectId}/proposals`, {
    kind: "work_arrangement",
    changes,
  });
  const draft = await fetchReview(projectId, proposal.id);
  const frozen = await post<ApiProposalReview>(
    `/projects/${projectId}/proposals/${proposal.id}/submit`,
    { expectedVersion: proposal.version, draftHash: draft.reviewHash },
  );
  await decideOnReview(projectId, proposal.id, "approve", frozen);
  return proposal.id;
}

async function decideOnReview(
  projectId: string,
  proposalId: string,
  decision: "approve" | "reject",
  review: ApiProposalReview,
  reason?: string,
) {
  await post(`/projects/${projectId}/proposals/${proposalId}/decisions`, {
    reviewId: review.reviewId,
    reviewHash: review.reviewHash,
    expectedVersion: review.version ?? 1,
    decision,
    slotIds: (review.slots ?? []).filter((s) => s.canDecide).map((s) => s.id),
    actingBindingVersions: (review.slots ?? [])
      .filter((s) => s.canDecide && s.authorityType === "identity")
      .map((s) => ({
        identityId: s.authorityId,
        bindingVersion: s.bindingVersion ?? 1,
      })),
    ...(reason ? { reason } : {}),
  });
}

// demo ProposalInput → work_arrangement changes：计划为 clientRef "plan"，
// 任务依次 "task-N"；after 只能映射为 dependsOn（排序引用，不产生验收硬条件）。
function arrangementChanges(draft: ProposalInput): ApiProposalChange[] {
  return [
    {
      operation: "create_plan",
      targetType: "plan",
      clientRef: "plan",
      fields: {
        title: draft.title,
        goal: draft.goal,
        acceptanceCriteria: draft.criteria,
        ...(draft.ownerSeatId ? { ownerIdentityId: draft.ownerSeatId } : {}),
        ...(draft.flowId ? { workflowId: draft.flowId } : {}),
      },
    },
    ...draft.tasks.map((task, i) => ({
      operation: "create_task" as const,
      targetType: "task",
      clientRef: "task-" + i,
      dependsOn: task.after.map((a) => "task-" + a),
      fields: {
        title: task.title,
        planId: "plan",
        expectedOutput: task.title,
        acceptanceCriteria: draft.criteria,
        participantIdentityIds: task.seatId ? [task.seatId] : [],
        ...(task.reviewerSeatId
          ? { reviewerIdentityId: task.reviewerSeatId }
          : {}),
        ...(draft.flowId ? { workflowId: draft.flowId } : {}),
        ...(task.nodeId ? { nodeId: task.nodeId } : {}),
      },
    })),
  ];
}

async function sendMessage(
  projectId: string,
  topicId: string,
  body: string,
  files: Evidence[] | undefined,
): Promise<void> {
  const materialVersionIds = await uploadEvidence(projectId, files);
  await post(`/projects/${projectId}/submissions`, {
    clientSubmissionId: crypto.randomUUID(),
    purpose: "message",
    topicId,
    // 服务端要求非空 text；纯附件消息退化为首个附件名。
    text: body.trim() || (files?.[0]?.name ?? ""),
    materialVersionIds,
  });
}

export const apiActions: {
  [K in keyof ActionPayloads]?: Executor<K>;
} = {
  topicMessage: async (projectId, p) => {
    await sendMessage(projectId, p.topicId, p.body, p.files);
    return { ok: true, id: p.topicId };
  },
  sendTopicMessage: async (projectId, p, ctx) => {
    if (p.topicId)
      return apiActions.topicMessage!(
        projectId,
        { topicId: p.topicId, body: p.body, files: p.files },
        ctx,
      );
    if (p.files?.length) {
      const topic = await post<ApiTopic>(`/projects/${projectId}/topics`, {
        title: p.title,
      });
      await sendMessage(projectId, topic.id, p.body, p.files);
      return { ok: true, id: topic.id };
    }
    const topic = await post<ApiTopic>(`/projects/${projectId}/topics`, {
      title: p.title,
      ...(p.body.trim() ? { initialMessage: p.body } : {}),
    });
    return { ok: true, id: topic.id };
  },
  createDiscussion: async (projectId, p) => {
    const topic = await post<ApiTopic>(`/projects/${projectId}/topics`, {
      title: p.title,
      links: [
        ...p.planIds.map((id) => ({ objectType: "plan" as const, objectId: id })),
        ...p.taskIds.map((id) => ({ objectType: "task" as const, objectId: id })),
      ],
    });
    return { ok: true, id: topic.id };
  },
  registerResource: async (projectId, p) => {
    const materialVersionIds = await uploadEvidence(projectId, p.files);
    const topic = await post<ApiTopic>(`/projects/${projectId}/topics`, {
      title: p.purpose,
    });
    await post(`/projects/${projectId}/submissions`, {
      clientSubmissionId: crypto.randomUUID(),
      purpose: "material",
      topicId: topic.id,
      text: p.purpose,
      materialVersionIds,
    });
    return { ok: true, id: topic.id };
  },
  discussTopic: async (projectId, p, ctx) => {
    const page = await request<{ items: ApiMessage[] }>(
      `/projects/${projectId}/topics/${p.topicId}/messages?limit=50`,
    );
    const committed = (page.items ?? []).filter((m) => m.state === "committed");
    const source =
      [...committed]
        .reverse()
        .find(
          (m) => m.kind === "human" && m.submissionId && m.authorUserId === ctx.userId,
        ) ??
      [...committed].reverse().find((m) => m.kind === "human" && m.submissionId);
    if (!source?.submissionId)
      throw new APIError(422, "INVALID_TRANSITION", "no submission to analyze");
    await post(`/projects/${projectId}/topics/${p.topicId}/runs`, {
      sourceSubmissionId: source.submissionId,
    });
    return { ok: true, id: p.topicId };
  },
  taskAction: async (projectId, p, ctx) => {
    if (p.op === "start") {
      const task = await fetchTask(projectId, p.taskId);
      await post(`/projects/${projectId}/tasks/${p.taskId}/start`, {
        expectedVersion: task.version,
        identityId: actingIdentity(ctx, task),
      });
      return { ok: true, id: p.taskId };
    }
    if (p.op === "accept") {
      const review = await request<ApiAcceptanceReview>(
        `/projects/${projectId}/tasks/${p.taskId}/acceptance-review`,
      );
      await post(`/projects/${projectId}/tasks/${p.taskId}/acceptances`, {
        reviewId: review.reviewId,
        reviewHash: review.reviewHash,
        expectedVersion: review.targetVersion ?? 1,
        decision: "accept",
      });
      return { ok: true, id: p.taskId };
    }
    const task = await fetchTask(projectId, p.taskId);
    await post(`/projects/${projectId}/tasks/${p.taskId}/reopens`, {
      expectedVersion: task.version,
      ...(task.latestAcceptanceId
        ? { acceptanceId: task.latestAcceptanceId }
        : {}),
      reason: p.reason ?? "",
    });
    return { ok: true, id: p.taskId };
  },
  reportTask: async (projectId, p, ctx) => {
    const task = await fetchTask(projectId, p.taskId);
    const materialVersionIds = await uploadEvidence(projectId, p.files);
    await post(`/projects/${projectId}/tasks/${p.taskId}/reports`, {
      kind: "delivery",
      text: p.summary.trim() || (p.files[0]?.name ?? ""),
      identityId: actingIdentity(ctx, task),
      materialVersionIds,
      expectedTaskVersion: task.version,
    });
    return { ok: true, id: p.taskId };
  },
  decideDraft: async (projectId, p) => {
    const task = await fetchTask(projectId, p.taskId);
    await submitAndApprove(projectId, [
      {
        operation: "activate_object",
        targetType: "task",
        targetId: p.taskId,
        expectedVersion: task.version,
      },
    ]);
    return { ok: true, id: p.taskId };
  },
  planAction: async (projectId, p) => {
    if (p.op === "activate") {
      const plan = await fetchPlan(projectId, p.planId);
      await submitAndApprove(projectId, [
        {
          operation: "activate_object",
          targetType: "plan",
          targetId: p.planId,
          expectedVersion: plan.version,
        },
      ]);
      return { ok: true, id: p.planId };
    }
    if (p.op === "accept") {
      const review = await request<ApiAcceptanceReview>(
        `/projects/${projectId}/plans/${p.planId}/acceptance-review`,
      );
      await post(`/projects/${projectId}/plans/${p.planId}/acceptances`, {
        reviewId: review.reviewId,
        reviewHash: review.reviewHash,
        expectedVersion: review.targetVersion ?? 1,
        decision: "accept",
      });
      return { ok: true, id: p.planId };
    }
    // resume：后端对应"重开已验收计划"（原因必填）。
    const plan = await fetchPlan(projectId, p.planId);
    await post(`/projects/${projectId}/plans/${p.planId}/reopens`, {
      expectedVersion: plan.version,
      ...(plan.latestAcceptanceId
        ? { acceptanceId: plan.latestAcceptanceId }
        : {}),
      reason: "恢复计划执行 / Resume plan",
    });
    return { ok: true, id: p.planId };
  },
  sourceDecision: async (projectId, p) => {
    const source = await findHandoffSource(projectId, p.handoffId, p.sourceId);
    await post(
      `/projects/${projectId}/handoffs/${p.handoffId}/sources/${p.sourceId}/decisions`,
      {
        sourceVersion: source.currentVersion ?? p.revision,
        reviewHash: source.currentVersionId ?? source.id,
        decision: p.decision === "accepted" ? "accept" : "reject",
        ...(p.reason ? { reason: p.reason } : {}),
      },
    );
    return { ok: true, id: p.handoffId };
  },
  reviseSource: async (projectId, p, ctx) => {
    const source = await findHandoffSource(projectId, p.handoffId, p.sourceId);
    const task = await fetchTask(projectId, source.sourceTaskId);
    // 新版本以交付报告为证据载体：有附件先补一条交付报告，否则复用最新报告。
    let reportId = task.latestReportId ?? null;
    if (p.files.length) {
      const report = await post<{ id: string }>(
        `/projects/${projectId}/tasks/${source.sourceTaskId}/reports`,
        {
          kind: "delivery",
          text: p.summary,
          identityId: actingIdentity(ctx, task),
          materialVersionIds: await uploadEvidence(projectId, p.files),
          expectedTaskVersion: task.version,
        },
      );
      reportId = report.id;
    }
    if (!reportId)
      throw new APIError(422, "REQUIREMENT_UNMET", "source task has no delivery report");
    // revisions 与 send 同为 sendAt：summary 必填、reviewHash 为报告 ID。
    await post(
      `/projects/${projectId}/handoffs/${p.handoffId}/sources/${p.sourceId}/revisions`,
      {
        summary: p.summary,
        sourceVersion: (source.currentVersion ?? 0) + 1,
        reviewHash: reportId,
      },
    );
    return { ok: true, id: p.handoffId };
  },
  sendSource: async (projectId, p) => {
    const source = await findHandoffSource(projectId, p.handoffId, p.sourceId);
    const task = await fetchTask(projectId, source.sourceTaskId);
    if (!task.latestReportId)
      throw new APIError(422, "REQUIREMENT_UNMET", "source task has no delivery report");
    await post(
      `/projects/${projectId}/handoffs/${p.handoffId}/sources/${p.sourceId}/send`,
      {
        summary: source.summary || task.expectedOutput || task.title,
        sourceVersion: (source.currentVersion ?? 0) + 1,
        reviewHash: task.latestReportId,
      },
    );
    return { ok: true, id: p.handoffId };
  },
  proposeHandoff: async (projectId, p) => {
    const target = await fetchTask(projectId, p.target);
    const sources: { sourceTaskId: string; senderIdentityId: string }[] = [];
    for (const id of p.ids) {
      const task = id === p.target ? target : await fetchTask(projectId, id);
      const sender = task.participants?.[0]?.identityId;
      if (!sender)
        throw new APIError(422, "VALIDATION_ERROR", "source task has no participant");
      sources.push({ sourceTaskId: id, senderIdentityId: sender });
    }
    const handoff = await post<ApiHandoff>(`/projects/${projectId}/handoffs`, {
      // 注意：handler 严格绑定，没有 workflowId 字段（schema 与实现不一致）。
      title: "交接：" + target.title,
      targetTaskId: p.target,
      receiverIdentityId: p.receiver,
      kind: p.kind,
      sources,
    });
    return { ok: true, id: handoff.id };
  },
  refreshHandoff: async (projectId, p) => {
    const page = await request<{ items: ApiHandoff[] }>(
      `/projects/${projectId}/handoffs?limit=100`,
    );
    const handoff = page.items.find((h) => h.id === p.handoffId);
    if (!handoff) throw new APIError(404, "NOT_FOUND", "handoff not found");
    await post(`/projects/${projectId}/handoffs/${p.handoffId}/reminders`, {
      expectedVersion: handoff.version ?? 1,
    });
    return { ok: true, id: p.handoffId };
  },
  submitProposal: async (projectId, p) => {
    const changes = arrangementChanges(p.draft);
    let proposal: ApiProposal;
    if (p.previousId) {
      const previous = await fetchReview(projectId, p.previousId);
      proposal = await post<ApiProposal>(
        `/projects/${projectId}/proposals/${p.previousId}/revisions`,
        { previousReviewId: previous.reviewId, changes },
      );
    } else {
      proposal = await post<ApiProposal>(`/projects/${projectId}/proposals`, {
        kind: "work_arrangement",
        topicId: p.topicId,
        changes,
      });
    }
    const draft = await fetchReview(projectId, proposal.id);
    await post<ApiProposalReview>(
      `/projects/${projectId}/proposals/${proposal.id}/submit`,
      { expectedVersion: proposal.version, draftHash: draft.reviewHash },
    );
    return { ok: true, id: proposal.id };
  },
  decideProposal: async (projectId, p) => {
    const review = await fetchReview(projectId, p.proposalId);
    await decideOnReview(
      projectId,
      p.proposalId,
      p.accept ? "approve" : "reject",
      review,
      p.reason,
    );
    return { ok: true, id: p.proposalId };
  },
  createWork: async (projectId, p, ctx) => {
    const d = p.draft;
    if (d.kind === "plan") {
      const plan = await post<ApiPlan>(`/projects/${projectId}/plans`, {
        title: d.title,
        goal: d.description,
        acceptanceCriteria: d.criteria,
        ...(d.seatId ? { ownerIdentityId: d.seatId } : {}),
        ...(d.flowId ? { workflowId: d.flowId } : {}),
      });
      return { ok: true, id: plan.id };
    }
    // demo 语义：任务的验收人是计划负责人；独立任务则验收人即执行席位。
    let reviewer = d.seatId;
    if (d.planId) {
      const plan = await fetchPlan(projectId, d.planId);
      reviewer = plan.ownerIdentityId ?? d.seatId;
    }
    const nodeId = ctx.state.flows.find((f) => f.id === d.flowId)?.nodes[0]?.id;
    const task = await post<ApiTask>(`/projects/${projectId}/tasks`, {
      title: d.title,
      ...(d.planId ? { planId: d.planId } : {}),
      ...(d.parentId ? { parentTaskId: d.parentId } : {}),
      expectedOutput: d.description,
      acceptanceCriteria: d.criteria,
      participantIdentityIds: d.seatId ? [d.seatId] : [],
      ...(reviewer ? { reviewerIdentityId: reviewer } : {}),
      ...(d.flowId ? { workflowId: d.flowId } : {}),
      ...(nodeId ? { nodeId } : {}),
    });
    return { ok: true, id: task.id };
  },
  createProject: async (_projectId, p) => {
    const project = await post<ApiProject>("/projects", {
      title: p.name,
      description: p.goal,
      kind: p.kind,
      maxDiscussionRounds: p.maxRounds,
    });
    return { ok: true, id: project.id };
  },
  // demo 语义：新流程直接以 v1 可用 = 后端创建草稿后立即发布。
  createFlow: async (projectId, p) => {
    const workflow = await post<ApiWorkflow>(
      `/projects/${projectId}/workflows`,
      {
        name: p.name,
        instructions: p.instructions,
        nodes: p.labels.map((label, i) => ({
          id: "step" + i,
          name: label,
          responsibility: "",
          allowedPositionIds: [],
          defaultApprovalPolicy: "all",
        })),
        advisoryEdges: p.labels
          .slice(1)
          .map((_, i) => ({ from: "step" + i, to: "step" + (i + 1) })),
      },
    );
    await publishCurrentDraft(projectId, workflow.id);
    return { ok: true, id: workflow.id };
  },
  // demo 的"发布新说明" = 沿用已发布 body、替换 instructions 后重新发布。
  // material 标记在后端没有对应物（不置 stale，详见报告）。
  publishFlow: async (projectId, p) => {
    const heads = await listItems<ApiWorkflow>(
      `/projects/${projectId}/workflows`,
    );
    const head = heads.find((w) => w.id === p.flowId);
    if (!head) throw new APIError(404, "NOT_FOUND", "workflow not found");
    const versions = await listItems<ApiWorkflowVersion>(
      `/projects/${projectId}/workflows/${p.flowId}/versions`,
    );
    const current =
      versions.find((v) => v.id === head.publishedVersionId) ??
      versions.find((v) => v.state === "published") ??
      versions.find((v) => v.state === "draft");
    if (!current?.body)
      throw new APIError(404, "NOT_FOUND", "workflow body missing");
    // OpenAPI 层校验嵌套 body（Go 端匿名字段同键解码）。
    await put(`/projects/${projectId}/workflows/${p.flowId}/draft`, {
      expectedVersion: head.version ?? 1,
      body: { ...current.body, instructions: p.instructions },
    });
    await publishCurrentDraft(projectId, p.flowId);
    return { ok: true, id: p.flowId };
  },
  savePosition: async (projectId, p) => {
    const nodeBindings = p.value.flowId
      ? [{ workflowId: p.value.flowId, nodeId: p.value.nodeId }]
      : [];
    if (p.value.id) {
      const positions = await listItems<ApiPosition>(
        `/projects/${projectId}/positions?limit=100`,
      );
      const current = positions.find((x) => x.id === p.value.id);
      if (!current) throw new APIError(404, "NOT_FOUND", "position not found");
      await patch(`/projects/${projectId}/positions/${p.value.id}`, {
        expectedVersion: current.currentVersion,
        name: p.value.name,
        prompt: p.value.prompt,
        nodeBindings,
      });
      return { ok: true, id: p.value.id };
    }
    const position = await post<ApiPosition>(
      `/projects/${projectId}/positions`,
      { name: p.value.name, prompt: p.value.prompt, nodeBindings },
    );
    return { ok: true, id: position.id };
  },
  // 语义差异：demo 按"姓名"邀请，REST 按 targetEmail（api 模式输入即邮箱）。
  invitePerson: async (projectId, p) => {
    const invitation = await post<ApiInvitation & { inviteUrl?: string }>(
      `/projects/${projectId}/invitations`,
      { targetEmail: p.name, positionIds: p.positionIds },
    );
    return {
      ok: true,
      id: invitation.id,
      inviteUrl: invitation.inviteUrl
        ? new URL(invitation.inviteUrl, location.origin).href
        : undefined,
    };
  },
  // handler 要求 expectedVersion 字段但服务侧不使用（接受邀请不校验版本）。
  acceptInvite: async (_projectId, p) => {
    await post(`/invitations/${p.invitationId}/accept`, { expectedVersion: 1 });
    return { ok: true, id: p.invitationId };
  },
  assignPositions: async (projectId, p) => {
    const userId = await memberUserId(projectId, p.person);
    for (const positionId of p.ids)
      await post(`/projects/${projectId}/identities`, { positionId, userId });
    return { ok: true };
  },
  // demo 无 reason 输入；后端必填，给固定文案。
  replaceSeat: async (projectId, p) => {
    const newUserId = await memberUserId(projectId, p.to);
    const identities = await listItems<ApiIdentity>(
      `/projects/${projectId}/identities?limit=100`,
    );
    const current = identities.find((i) => i.id === p.seatId);
    if (!current) throw new APIError(404, "NOT_FOUND", "identity not found");
    await post(`/projects/${projectId}/identities/${p.seatId}/replace`, {
      expectedBindingVersion: current.currentBindingVersion,
      newUserId,
      reason: "席位替换 / Seat replacement",
    });
    return { ok: true, id: p.seatId };
  },
  personalPrompt: async (projectId, p) => {
    const preferences = await request<ApiPreferences>(
      `/projects/${projectId}/me/preferences`,
    );
    await put(`/projects/${projectId}/me/preferences`, {
      expectedRevision: preferences.revision,
      prompt: p.prompt,
    });
    return { ok: true };
  },
  setDiscussionLimit: async (projectId, p) => {
    const project = await request<ApiProject>(`/projects/${projectId}`);
    await patch(`/projects/${projectId}`, {
      expectedVersion: project.version,
      maxDiscussionRounds: p.limit,
    });
    return { ok: true, id: projectId };
  },
};
