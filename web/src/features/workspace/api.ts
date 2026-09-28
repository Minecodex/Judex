import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { request } from "../../lib/api/client";

// 三栏工作区的真实 API 资源（openapi.yaml 议题/消息/提交/Agent 运行/交接）。
// 写操作全部带 clientSubmissionId 或 Idempotency-Key（06 §统一入口）。

export type ApiTopic = {
  id: string;
  title: string;
  kind: "project_room" | "discussion" | "handoff";
  contextType?: string | null;
  contextId?: string | null;
  lastMessageSeq: number;
  links: Array<{ objectType: string; objectId: string }>;
  createdAt: string;
};

export type ApiMessage = {
  id: string;
  topicId: string;
  seq: number;
  kind: "human" | "agent" | "system";
  authorUserId?: string | null;
  authorDisplayName?: string | null;
  identityId?: string | null;
  identityName?: string | null;
  source?: "web" | "cli" | "platform" | null;
  submissionId?: string | null;
  runId?: string | null;
  content: string;
  state: "committed" | "superseded";
  materialVersionIds: string[];
  createdAt: string;
};

export type ApiSubmission = {
  id: string;
  clientSubmissionId: string;
  purpose: "message" | "progress" | "delivery" | "material";
  source: "web" | "cli" | "agent";
  status: "preparing" | "ready" | "failed";
  text?: string | null;
  topicId?: string | null;
  taskId?: string | null;
  identityId?: string | null;
  createdAt: string;
};

export type ApiRun = {
  id: string;
  batchId?: string | null;
  roundId?: string | null;
  identityId: string;
  identityName?: string | null;
  state:
    | "queued"
    | "provisioning"
    | "running"
    | "waiting_children"
    | "waiting_human"
    | "waiting_material"
    | "succeeded"
    | "failed"
    | "cancelled";
  budget: Record<string, unknown>;
  waitingFor: string[];
  artifactRefs: Array<Record<string, unknown>>;
  createdAt: string;
};

export type ApiHandoffSource = {
  id: string;
  sourceTaskId: string;
  senderIdentityId: string;
  senderDisplayName?: string | null;
  currentVersionId?: string | null;
  currentVersion?: number | null;
  state: "draft" | "pending" | "accepted" | "rejected";
  reason?: string | null;
  sentAt?: string | null;
  decidedAt?: string | null;
};

export type ApiHandoff = {
  id: string;
  title?: string | null;
  targetTaskId: string;
  receiverIdentityId: string;
  receiverDisplayName?: string | null;
  workflowId?: string | null;
  kind: "dependency" | "stage";
  state: "waiting" | "accepted" | "needs_revision";
  version?: number;
  sources: ApiHandoffSource[];
};

export type ApiBootstrap = {
  project: { id: string; title: string; state?: string };
  identities: Array<{
    id: string;
    kind: "coordinator" | "position";
    status: string;
    positionName?: string | null;
    currentBinding?: { displayName: string; userId: string } | null;
  }>;
  pendingActionsCount: number;
  eventCursor: number;
  summary?: Record<string, unknown>;
};

type List<T> = { items: T[]; nextCursor?: string | null };

export function useBootstrap(projectId: string) {
  return useQuery({
    queryKey: ["bootstrap", projectId],
    queryFn: () => request<ApiBootstrap>(`/projects/${projectId}/bootstrap`),
  });
}

export function useTopics(projectId: string) {
  return useQuery({
    queryKey: ["topics", projectId],
    queryFn: () => request<List<ApiTopic>>(`/projects/${projectId}/topics?limit=100`),
  });
}

export function useMessages(projectId: string, topicId: string | null) {
  return useQuery({
    queryKey: ["messages", projectId, topicId],
    enabled: !!topicId,
    // SSE 为主；轮询兜底（SSE 断连期间仍保持可用）。
    refetchInterval: 5000,
    queryFn: () =>
      request<List<ApiMessage>>(
        `/projects/${projectId}/topics/${topicId}/messages?limit=100`,
      ),
  });
}

export function useHandoffs(projectId: string) {
  return useQuery({
    queryKey: ["handoffs", projectId],
    refetchInterval: 15000,
    queryFn: () => request<List<ApiHandoff>>(`/projects/${projectId}/handoffs`),
  });
}

export function useAgentRun(projectId: string, runId: string | null) {
  return useQuery({
    queryKey: ["runs", projectId, runId],
    enabled: !!runId,
    refetchInterval: (query) => {
      const state = query.state.data?.state;
      return state && ["succeeded", "failed", "cancelled"].includes(state)
        ? false
        : 2000;
    },
    queryFn: () => request<ApiRun>(`/projects/${projectId}/runs/${runId}`),
  });
}

export function useCreateTopic(projectId: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (title: string) =>
      request<ApiTopic>(`/projects/${projectId}/topics`, {
        method: "POST",
        body: JSON.stringify({ title }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["topics", projectId] });
    },
  });
}

// 发消息走统一提交入口：clientSubmissionId 幂等 + Idempotency-Key。
export function useSendMessage(projectId: string, topicId: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (text: string) => {
      const body = JSON.stringify({
        clientSubmissionId: crypto.randomUUID(),
        purpose: "message",
        topicId,
        text,
      });
      return request<ApiSubmission>(`/projects/${projectId}/submissions`, {
        method: "POST",
        body,
        idempotencyKey: crypto.randomUUID(),
      });
    },
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["messages", projectId, topicId] });
      void client.invalidateQueries({ queryKey: ["topics", projectId] });
    },
  });
}

// Agent 分析：以上一条本人提交为来源启动运行（05 §3 批次）。
export function useStartAgentRun(projectId: string, topicId: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (sourceSubmissionId: string) =>
      request<{ batchId: string; runId: string | null }>(
        `/projects/${projectId}/topics/${topicId}/runs`,
        {
          method: "POST",
          body: JSON.stringify({ sourceSubmissionId }),
          idempotencyKey: crypto.randomUUID(),
        },
      ),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["runs", projectId] });
    },
  });
}

// 交接决定（03 §5 正式决定协议）：请求按契约带 sourceVersion/reviewHash
//（以列表呈现的 currentVersion/currentVersionId 为冻结引用），拒收须理由。
export function useHandoffDecision(projectId: string, handoffId: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: {
      source: ApiHandoffSource;
      decision: "accept" | "reject";
      reason?: string;
    }) =>
      request<ApiHandoffSource>(
        `/projects/${projectId}/handoffs/${handoffId}/sources/${input.source.id}/decisions`,
        {
          method: "POST",
          body: JSON.stringify({
            sourceVersion: input.source.currentVersion ?? 1,
            reviewHash: input.source.currentVersionId ?? input.source.id,
            decision: input.decision,
            ...(input.reason ? { reason: input.reason } : {}),
          }),
          idempotencyKey: crypto.randomUUID(),
        },
      ),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["handoffs", projectId] });
      void client.invalidateQueries({ queryKey: ["actions", projectId] });
    },
  });
}
