// 事件类型 → 失效查询根键的纯映射（无环境依赖，供 node:test 直测）。
// 服务端既有事件类型来自 internal/* AppendProjectEvent 全集。

// 事件类型 → 需要失效的 query 根键（调用方再拼上 projectId 前缀）。
// 未知事件类型不盲目全量失效（诚实降级：新事件由各自查询的轮询兜底）。
export const PROJECT_EVENT_TYPES = [
  "message.committed",
  "proposal.changed",
  "task.changed",
  "plan.changed",
  "handoff.changed",
  "material.ready",
  "membership.changed",
  "workflow.published",
] as const;

export type ProjectEventType = (typeof PROJECT_EVENT_TYPES)[number];

export const EVENT_INVALIDATION_ROOTS: Record<string, string[]> = {
  "message.committed": ["messages", "topics", "runs", "actions"],
  "proposal.changed": ["proposals", "proposalReview", "actions", "tasks", "plans"],
  "task.changed": ["tasks", "actions"],
  "plan.changed": ["plans", "tasks", "actions"],
  "handoff.changed": ["handoffs", "actions", "topics"],
  "material.ready": ["materials", "actions"],
  "membership.changed": ["members", "bootstrap"],
  "workflow.published": ["workflows", "plans"],
};

export function invalidationRoots(type: string): string[] {
  return EVENT_INVALIDATION_ROOTS[type] ?? [];
}
