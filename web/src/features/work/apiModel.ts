// 真实 API → demo WorkState 的只读映射（P1）：各资源 fetcher、映射函数与装配。
// demo 的 person 一律是 displayName 字符串；REST 字符串统一转 {zh, en} 同值 Text。
import { APIError, request } from "../../lib/api/client";
import type { components } from "../../lib/api/schema";
import {
  words,
  type Audit,
  type Flow,
  type Handoff,
  type Invite,
  type Plan,
  type Position,
  type Project,
  type Seat,
  type Source,
  type Task,
  type Text,
  type Topic,
  type WorkState,
} from "./types";
import type { WorkProposal } from "../chat/proposalModel";

type Schema = components["schemas"];
export type ApiProject = Schema["Project"];
export type ApiMember = Schema["ProjectMember"];
export type ApiPosition = Schema["Position"];
export type ApiIdentity = Schema["Identity"];
export type ApiPlan = Schema["Plan"];
export type ApiTask = Schema["Task"];
export type ApiHandoffSource = Schema["HandoffSource"] & { summary?: string };
export type ApiHandoff = Omit<Schema["Handoff"], "sources"> & {
  sources: ApiHandoffSource[];
};
export type ApiTopic = Schema["Topic"];
export type ApiMessage = Schema["Message"];
export type ApiInvitation = Schema["Invitation"];
export type ApiPreferences = Schema["PersonalPreferences"];
export type ApiAuditEntry = Schema["AuditEntry"];
export type ApiWorkflow = Schema["Workflow"];
export type ApiWorkflowVersion = Schema["WorkflowVersion"];
export type ApiProposal = Schema["Proposal"];
export type ApiProposalReview = Schema["ProposalReview"];
export type ApiBootstrap = Schema["ProjectBootstrap"];

type List<T> = { items: T[]; nextCursor?: string | null };

// 列表读取；invitations/audit 等资源对普通成员可能 403，按空集合降级。
async function list<T>(path: string): Promise<T[]> {
  try {
    const page = await request<List<T>>(
      path + (path.includes("?") ? "&" : "?") + "limit=100",
    );
    return page.items ?? [];
  } catch (error) {
    if (
      error instanceof APIError &&
      (error.status === 403 || error.status === 404)
    )
      return [];
    throw error;
  }
}

async function optional<T>(path: string): Promise<T | null> {
  try {
    return await request<T>(path);
  } catch (error) {
    if (
      error instanceof APIError &&
      (error.status === 403 || error.status === 404)
    )
      return null;
    throw error;
  }
}

export const API_WS_ROOT = "apiWs";
export const apiWsKey = (projectId: string) => [API_WS_ROOT, projectId];

export function apiWorkspaceQueries(projectId: string) {
  const base = `/projects/${projectId}`;
  const key = (...parts: string[]) => [...apiWsKey(projectId), ...parts];
  return {
    bootstrap: {
      queryKey: key("bootstrap"),
      queryFn: () => request<ApiBootstrap>(`${base}/bootstrap`),
    },
    projects: {
      queryKey: key("projects"),
      queryFn: () => list<ApiProject>("/projects"),
    },
    members: {
      queryKey: key("members"),
      queryFn: () => list<ApiMember>(`${base}/members`),
    },
    positions: {
      queryKey: key("positions"),
      queryFn: () => list<ApiPosition>(`${base}/positions`),
    },
    identities: {
      queryKey: key("identities"),
      queryFn: () => list<ApiIdentity>(`${base}/identities`),
    },
    plans: {
      queryKey: key("plans"),
      queryFn: () => list<ApiPlan>(`${base}/plans`),
    },
    tasks: {
      queryKey: key("tasks"),
      queryFn: () => list<ApiTask>(`${base}/tasks`),
    },
    // 任务列表只返回摘要字段（06 §5）；参与者/验收标准等由详情端点补齐。
    taskDetail: (taskId: string) => ({
      queryKey: key("task", taskId),
      queryFn: () => request<ApiTask>(`${base}/tasks/${taskId}`),
    }),
    handoffs: {
      queryKey: key("handoffs"),
      queryFn: () => list<ApiHandoff>(`${base}/handoffs`),
    },
    topics: {
      queryKey: key("topics"),
      queryFn: () => list<ApiTopic>(`${base}/topics`),
    },
    invitations: {
      queryKey: key("invitations"),
      queryFn: () => list<ApiInvitation>(`${base}/invitations`),
    },
    preferences: {
      queryKey: key("preferences"),
      queryFn: () => optional<ApiPreferences>(`${base}/me/preferences`),
    },
    audit: {
      queryKey: key("audit"),
      queryFn: () => list<ApiAuditEntry>(`${base}/audit`),
    },
    workflows: {
      queryKey: key("workflows"),
      queryFn: () => list<ApiWorkflow>(`${base}/workflows`),
    },
    proposals: {
      queryKey: key("proposals"),
      queryFn: () => list<ApiProposal>(`${base}/proposals`),
    },
    workflowVersions: (workflowId: string) => ({
      queryKey: key("workflowVersions", workflowId),
      queryFn: () =>
        list<ApiWorkflowVersion>(`${base}/workflows/${workflowId}/versions`),
    }),
    topicMessages: (topicId: string) => ({
      queryKey: key("topicMessages", topicId),
      queryFn: () =>
        list<ApiMessage>(`${base}/topics/${topicId}/messages?limit=50`),
    }),
    proposalReview: (proposalId: string) => ({
      queryKey: key("proposalReview", proposalId),
      queryFn: () =>
        optional<ApiProposalReview>(`${base}/proposals/${proposalId}/review`),
    }),
  };
}

const text = (value: string | null | undefined): Text => words(value ?? "");
const lines = (value: string | undefined): Text[] =>
  (value ?? "")
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => words(line));
const timestamp = (value?: string | null): number | undefined => {
  if (!value) return undefined;
  const at = Date.parse(value);
  return Number.isFinite(at) ? at : undefined;
};

// 与 seed.ts 一致的四色人物色调，按职位顺序循环分配。
const TONES = ["mint", "blue", "lilac", "peach"];

export function mapPosition(
  p: ApiPosition,
  projectId: string,
  index: number,
): Position {
  return {
    id: p.id,
    projectId,
    name: text(p.name),
    prompt: text(p.prompt),
    tone: TONES[index % TONES.length],
    bindings: (p.nodeBindings ?? []).map((b) => ({
      flowId: b.workflowId,
      nodeId: b.nodeId,
    })),
  };
}

// Identity 的职位关联优先取 templateId（创建时写入的职位 ID），否则按 positionName 匹配。
export function mapSeat(i: ApiIdentity, positions: ApiPosition[]): Seat {
  return {
    id: i.id,
    positionId:
      positions.find((p) => p.id === i.templateId)?.id ??
      positions.find((p) => p.name === i.positionName)?.id ??
      "",
    person: i.currentBinding?.displayName ?? "",
    notes: text(""),
  };
}

export function mapPlan(p: ApiPlan, projectId: string): Plan {
  return {
    id: p.id,
    projectId,
    title: text(p.title),
    goal: text(p.goal),
    criteria: lines(p.acceptanceCriteria),
    ownerSeatId: p.ownerIdentityId ?? "",
    flowId: p.workflowId ?? "",
    status: p.status === "cancelled" ? "draft" : p.status,
    referenceTaskIds: [],
  };
}

const REQUIREMENT_KIND = {
  task_acceptance: "task",
  handoff_receipt: "receipt",
  material_ready: "evidence",
} as const;

export function mapTask(t: ApiTask, projectId: string): Task {
  return {
    revision: t.version ?? 1,
    id: t.id,
    projectId,
    planId: t.planId ?? null,
    parentId: t.parentTaskId ?? undefined,
    title: text(t.title),
    expected: text(t.expectedOutput),
    criteria: lines(t.acceptanceCriteria),
    seatIds: (t.participants ?? []).map((p) => p.identityId),
    reviewerSeatId: t.reviewerIdentityId ?? "",
    flowId: t.workflowId ?? "",
    nodeId: t.nodeId ?? "",
    status: t.status === "cancelled" ? "draft" : t.status,
    requirements: (t.requirements ?? []).map((r) => ({
      id: r.id,
      at: r.phase,
      label: text(r.label ?? r.kind),
      kind: REQUIREMENT_KIND[r.kind],
      ref: r.kind === "material_ready" ? (r.materialVersionId ?? r.targetId) : r.targetId,
      hard: r.hard,
    })),
    files: [],
  };
}

function mapSource(s: ApiHandoffSource): Source {
  return {
    id: s.id,
    taskId: s.sourceTaskId,
    senderSeatId: s.senderIdentityId,
    revision: s.currentVersion ?? 1,
    summary: text(s.summary),
    files: [],
    status: s.state,
    reason: s.reason ?? undefined,
    sentBy: s.senderDisplayName ?? undefined,
    sentAt: timestamp(s.sentAt),
    decidedAt: timestamp(s.decidedAt),
  };
}

// 后端不存交接的 workflowId（schema 与实现不一致）：退回目标任务的流程上下文。
export function mapHandoff(
  h: ApiHandoff,
  projectId: string,
  flows: Flow[],
  tasks: ApiTask[],
): Handoff {
  const flowId =
    h.workflowId ??
    tasks.find((t) => t.id === h.targetTaskId)?.workflowId ??
    "";
  return {
    id: h.id,
    projectId,
    title: text(h.title),
    taskId: h.targetTaskId,
    receiverSeatId: h.receiverIdentityId,
    flowId,
    flowVersion: flows.find((f) => f.id === flowId)?.version ?? 1,
    stale: false,
    kind: h.kind,
    sources: (h.sources ?? []).map(mapSource),
    history: [],
  };
}

export function mapFlow(
  w: ApiWorkflow,
  versions: ApiWorkflowVersion[],
  projectId: string,
): Flow {
  const published =
    versions.find((v) => v.id === w.publishedVersionId) ??
    versions.find((v) => v.state === "published");
  const body = published?.body;
  const nodes = (body?.nodes ?? []).map((n) => ({
    id: n.id,
    label: text(n.name),
  }));
  const edges: [string, string][] = body?.advisoryEdges?.length
    ? body.advisoryEdges.map((e) => [e.from, e.to])
    : nodes.slice(1).map((n, i) => [nodes[i].id, n.id]);
  return {
    id: w.id,
    projectId,
    name: text(w.name),
    version: published?.revision ?? w.version ?? 1,
    instructions: text(body?.instructions),
    nodes,
    edges,
    history: versions
      .filter((v) => v.state === "published")
      .map((v) => ({
        version: v.revision,
        instructions: text(v.body?.instructions),
        actor: "",
      }))
      .sort((a, b) => a.version - b.version),
  };
}

export function mapTopic(
  topic: ApiTopic,
  projectId: string,
  messages: ApiMessage[],
): Topic {
  return {
    id: topic.id,
    projectId,
    title: text(topic.title),
    planIds: (topic.links ?? [])
      .filter((l) => l.objectType === "plan")
      .map((l) => l.objectId),
    taskIds: (topic.links ?? [])
      .filter((l) => l.objectType === "task")
      .map((l) => l.objectId),
    context:
      topic.kind === "handoff" && topic.contextId
        ? { kind: "handoff", id: topic.contextId }
        : undefined,
    messages: messages
      .filter((m) => m.state === "committed")
      .map((m) => ({
        id: m.id,
        actor:
          m.kind === "human"
            ? (m.authorDisplayName ?? "")
            : (m.identityName ?? m.authorDisplayName ?? "Judex"),
        kind: m.kind === "human" ? ("person" as const) : ("ai" as const),
        seatId: m.identityId ?? undefined,
        text: text(m.content),
        at: timestamp(m.createdAt) ?? 0,
        files: [],
      })),
  };
}

// demo 邀请只有 pending/accepted；declined/revoked/expired 没有对应状态，不映射。
export function mapInvite(
  invitation: ApiInvitation,
  projectId: string,
  members: ApiMember[],
): Invite | null {
  if (invitation.state !== "pending" && invitation.state !== "accepted")
    return null;
  return {
    id: invitation.id,
    projectId,
    person: invitation.targetEmail,
    positionIds: (invitation.positions ?? []).map((p) => p.id),
    status: invitation.state,
    sender:
      members.find((m) => m.userId === invitation.inviterUserId)
        ?.displayName ?? invitation.inviterUserId,
  };
}

export function mapAudit(
  entry: ApiAuditEntry,
  projectId: string,
  members: ApiMember[],
  identities: ApiIdentity[],
): Audit {
  const actor =
    members.find((m) => m.userId === entry.actorUserId)?.displayName ??
    identities.find((i) => i.id === entry.identityId)?.currentBinding
      ?.displayName ??
    entry.actorType;
  return {
    id: entry.id,
    projectId,
    targetId: entry.objectId ?? "",
    actor,
    text: text(entry.operation),
    at: timestamp(entry.occurredAt) ?? 0,
  };
}

function changeFields(change: { fields?: { [key: string]: unknown } }) {
  return (change.fields ?? {}) as Record<string, unknown>;
}
const fieldText = (fields: Record<string, unknown>, key: string) =>
  typeof fields[key] === "string" ? (fields[key] as string) : "";

// work_arrangement 提案 → demo WorkProposal：计划/任务来自 review.changes，
// 审批人/票数来自 review.slots。缺 create_plan 变更或标题为空时无法映射。
export function mapProposal(
  proposal: ApiProposal,
  review: ApiProposalReview | null | undefined,
  projectId: string,
  members: ApiMember[],
  flows: Flow[],
): WorkProposal | null {
  if (proposal.kind !== "work_arrangement" || !review) return null;
  const planChange = review.changes.find((c) => c.operation === "create_plan");
  if (!planChange) return null;
  const planFields = changeFields(planChange);
  const title = fieldText(planFields, "title");
  if (!title) return null;
  const taskChanges = review.changes.filter(
    (c) => c.operation === "create_task",
  );
  const indexOf = new Map(
    taskChanges.map((c, i) => [c.clientRef ?? "", i] as const),
  );
  const tasks = taskChanges.map((c) => {
    const fields = changeFields(c);
    const participantIds = Array.isArray(fields.participantIdentityIds)
      ? fields.participantIdentityIds.filter(
          (x): x is string => typeof x === "string",
        )
      : [];
    return {
      title: fieldText(fields, "title"),
      seatId: participantIds[0] ?? "",
      reviewerSeatId: fieldText(fields, "reviewerIdentityId"),
      nodeId: fieldText(fields, "nodeId"),
      after: (c.dependsOn ?? [])
        .map((ref) => indexOf.get(ref))
        .filter((i): i is number => i !== undefined),
    };
  });
  const flowId = fieldText(planFields, "workflowId");
  const slots = review.slots ?? [];
  const votes: Record<string, boolean> = {};
  for (const slot of slots)
    if (slot.displayName && slot.state === "approved")
      votes[slot.displayName] = true;
  return {
    id: proposal.id,
    projectId,
    topicId: proposal.topicId ?? "",
    revision: proposal.revision ?? 1,
    status:
      proposal.status === "approved"
        ? "approved"
        : proposal.status === "cancelled" || proposal.status === "stale"
          ? "rejected"
          : "pending",
    sender:
      members.find((m) => m.userId === proposal.senderUserId)?.displayName ??
      "",
    flowVersion: flows.find((f) => f.id === flowId)?.version ?? 1,
    bindings: slots
      .filter((s) => s.authorityType === "identity")
      .map((s) => ({ seatId: s.authorityId, person: s.displayName ?? "" })),
    approvers: [
      ...new Set(slots.map((s) => s.displayName ?? "").filter(Boolean)),
    ],
    votes,
    title,
    goal: fieldText(planFields, "goal"),
    criteria: fieldText(planFields, "acceptanceCriteria"),
    ownerSeatId: fieldText(planFields, "ownerIdentityId"),
    flowId,
    tasks,
    planId: planChange.clientRef
      ? review.createdIds?.[planChange.clientRef]
      : undefined,
    createdAt: timestamp(proposal.createdAt) ?? 0,
  };
}

export type ApiWorkspaceSnapshot = {
  bootstrap: ApiBootstrap;
  projects: ApiProject[];
  members: ApiMember[];
  positions: ApiPosition[];
  identities: ApiIdentity[];
  plans: ApiPlan[];
  tasks: ApiTask[];
  handoffs: ApiHandoff[];
  topics: ApiTopic[];
  topicMessages: Record<string, ApiMessage[]>;
  invitations: ApiInvitation[];
  preferences: ApiPreferences | null;
  audit: ApiAuditEntry[];
  workflows: ApiWorkflow[];
  workflowVersions: Record<string, ApiWorkflowVersion[]>;
  proposals: ApiProposal[];
  proposalReviews: Record<string, ApiProposalReview | null | undefined>;
  currentUser: string;
};

export function assembleWorkState(
  projectId: string,
  data: ApiWorkspaceSnapshot,
): WorkState {
  const members = data.members
    .filter((m) => m.state === "active")
    .map((m) => ({ name: m.displayName, role: m.role }));
  const positions = data.positions
    .filter((p) => p.status === "active")
    .map((p, i) => mapPosition(p, projectId, i));
  const flows = data.workflows.map((w) =>
    mapFlow(w, data.workflowVersions[w.id] ?? [], projectId),
  );
  const seats = data.identities
    .filter((i) => i.kind === "position")
    .map((i) => mapSeat(i, data.positions));
  const toProject = (p: ApiProject): Project => ({
    id: p.id,
    title: text(p.title),
    description: text(p.description),
    kind: p.kind,
    members:
      p.id === projectId ? members : [{ name: data.currentUser, role: p.role }],
    maxDiscussionRounds: p.maxDiscussionRounds,
  });
  const projects = data.projects.map(toProject);
  if (!projects.some((p) => p.id === projectId))
    projects.unshift(toProject(data.bootstrap.project));
  return {
    schema: 4,
    currentUser: data.currentUser,
    projects,
    positions,
    seats,
    plans: data.plans.map((p) => mapPlan(p, projectId)),
    tasks: data.tasks.map((t) => mapTask(t, projectId)),
    handoffs: data.handoffs.map((h) => mapHandoff(h, projectId, flows, data.tasks)),
    flows,
    topics: data.topics.map((t) =>
      mapTopic(t, projectId, data.topicMessages[t.id] ?? []),
    ),
    invites: data.invitations
      .map((i) => mapInvite(i, projectId, data.members))
      .filter((i): i is Invite => !!i),
    preferences: [
      {
        projectId,
        person: data.currentUser,
        prompt: data.preferences?.prompt ?? "",
      },
    ],
    events: data.audit.map((e) =>
      mapAudit(e, projectId, data.members, data.identities),
    ),
    proposals: data.proposals
      .map((p) =>
        mapProposal(
          p,
          data.proposalReviews[p.id],
          projectId,
          data.members,
          flows,
        ),
      )
      .filter((p): p is WorkProposal => !!p),
  };
}
