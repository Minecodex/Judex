import type { WorkState, Task, Plan, Source } from "./types.ts";
export const seatPerson = (s: WorkState, id: string) =>
  s.seats.find((v) => v.id === id)?.person ?? "";
export const member = (s: WorkState, projectId: string) =>
  s.projects
    .find((p) => p.id === projectId)
    ?.members.find((m) => s.currentUserId?m.userId===s.currentUserId:m.name === s.currentUser);
export const manage = (s: WorkState, projectId: string) =>
  ["owner", "manager"].includes(member(s, projectId)?.role ?? "");
export const ownsSeat = (s: WorkState, seatId: string) =>
  s.seats.some(v=>v.id===seatId&&(!v.status||v.status==="active")&&(s.currentUserId?v.userId===s.currentUserId:v.person===s.currentUser));
export const canWork = (s: WorkState, task: Task) =>
  !!member(s, task.projectId) && task.seatIds.some((id) => ownsSeat(s, id));
export const canAcceptTask = (s: WorkState, task: Task) =>
  !!member(s, task.projectId) && ownsSeat(s, task.reviewerSeatId);
export const canOwnPlan = (s: WorkState, plan: Plan) =>
  !!member(s, plan.projectId) && ownsSeat(s, plan.ownerSeatId);
export const planTasks = (s: WorkState, planId: string,includeReferences=false) =>
  s.tasks.filter((t) => !t.discardedAt && (t.planId === planId || includeReferences&&s.plans.find(p=>p.id===planId)?.referenceTaskIds.includes(t.id)));
export const planReviewKey = (s: WorkState, plan: Plan) =>
  JSON.stringify({
    goal: plan.goal,
    criteria: plan.criteria,
    version: plan.revision,
    tasks: planTasks(s, plan.id,true).map((t) => [
      t.id,
      t.revision,
      t.status,
      t.acceptedAt,
      t.executionException,
    ]),
  });
export function requirementMet(
  s: WorkState,
  task: Task,
  requirement: Task["requirements"][number],
) {
  if(requirement.satisfied!==undefined)return requirement.satisfied;
  if(requirement.waived)return true;
  if (requirement.kind === "task")
    return s.tasks.some(
      (t) => t.id === requirement.ref && t.status === "accepted",
    );
  if (requirement.kind === "evidence") return task.files.length > 0;
  const h = s.handoffs.find((h) => h.id === requirement.ref);
  return (
    !!h &&
    !h.stale &&
    h.sources.length > 0 &&
    h.sources.every((v) => v.status === "accepted")
  );
}
export const blockers = (
  s: WorkState,
  task: Task,
  action: "start" | "accept" = "accept",
) =>
  task.requirements.filter(
    (r) =>
      r.hard &&
      (!r.at || r.at === "both" || r.at === action) &&
      !requirementMet(s, task, r),
  );
export const needsReview = (s: WorkState, plan: Plan) =>
  plan.status === "accepted" &&
  planTasks(s, plan.id,true).some((t) => !["accepted","cancelled"].includes(t.businessStatus??t.status) && !(t.planId===plan.id&&(t.executionException || (t.businessStatus??t.status)==="draft")));
export const canSendSource = (s: WorkState, source: Source) =>
  ownsSeat(s, source.senderSeatId) && source.status === "draft";
export function sourceActions(s: WorkState, projectId: string) {
  return s.handoffs
    .filter((h) => h.projectId === projectId)
    .flatMap((handoff) =>
      handoff.sources
        .filter(
          (source) =>
            (ownsSeat(s, handoff.receiverSeatId) &&
              source.status === "pending") ||
            (ownsSeat(s, source.senderSeatId) &&
              ["draft", "rejected"].includes(source.status)) ||
            (handoff.stale && ownsSeat(s, source.senderSeatId)),
        )
        .map((source) => ({ handoff, source })),
    );
}
export const allPeople = (s: WorkState) => [
  ...new Set([
    ...s.projects.flatMap((p) => p.members.map((m) => m.name)),
    ...s.invites.map((i) => i.person),
  ]),
];
