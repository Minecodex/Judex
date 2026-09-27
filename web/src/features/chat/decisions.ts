import type { WorkState } from "../work/types.ts";
import {
  member,
  canOwnPlan,
  canAcceptTask,
  planTasks,
  sourceActions,
} from "../work/selectors.ts";
export function pendingDecisionCount(s: WorkState, projectId: string) {
  if (!member(s, projectId)) return 0;
  const proposals = (s.proposals ?? []).filter(
    (p) =>
      p.projectId === projectId &&
      p.status === "pending" &&
      p.approvers.includes(s.currentUser) &&
      !p.votes[s.currentUser],
  );
  const tasks = s.tasks.filter(
    (t) =>
      t.projectId === projectId &&
      ((t.status === "delivered" && canAcceptTask(s, t)) ||
        (t.status === "draft" &&
          (t.planId
            ? s.plans.some((p) => p.id === t.planId && canOwnPlan(s, p))
            : member(s, projectId)?.role === "owner"))),
  );
  const plans = s.plans.filter(
    (p) =>
      p.projectId === projectId &&
      canOwnPlan(s, p) &&
      (p.status === "draft" ||
        (p.status === "active" &&
          planTasks(s, p.id).length > 0 &&
          planTasks(s, p.id).every((t) => t.status === "accepted"))),
  );
  return (
    sourceActions(s, projectId).length +
    proposals.length +
    tasks.length +
    plans.length
  );
}
