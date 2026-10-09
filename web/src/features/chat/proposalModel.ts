import { words as W, type WorkState, type Result } from "../work/types.ts";
import { member } from "../work/selectors.ts";
import { uid } from "../work/seed.ts";
import {demoHistory} from "./demoCollaboration.ts";
import {applyDemoWorkFields} from '../work/demoRuntime.ts';
export type ProposedTask = {
  title: string;
  seatId: string;
  reviewerSeatId: string;
  nodeId: string;
  after: number[];
};
export type ProposalInput = {
  title: string;
  goal: string;
  criteria: string;
  ownerSeatId: string;
  flowId: string;
  tasks: ProposedTask[];
};
export type WorkProposal = ProposalInput & {
  workChange?:{kind:'task'|'plan';id:string;expectedVersion:number;fields:Record<string,unknown>};
  id: string;
  projectId: string;
  topicId: string;
  sourceAfterSeq?:number;
  revision: number;
  status: "pending" | "approved" | "rejected";
  sender: string;
  flowVersion: number;
  bindings: { seatId: string; person: string }[];
  approvers: string[];
  votes: Record<string, boolean>;
  reason?: string;
  previousId?: string;
  planId?: string;
  createdAt: number;
};
export function proposalIsCurrent(s: WorkState, p: WorkProposal) {
  if(p.workChange){const c=p.workChange,target=(c.kind==='task'?s.tasks:s.plans).find(v=>v.id===c.id);return !!target&&(target.revision??1)===c.expectedVersion&&p.bindings.every(b=>s.seats.find(v=>v.id===b.seatId)?.person===b.person)&&(!p.flowId||s.flows.find(v=>v.id===p.flowId)?.version===p.flowVersion);}
  return (
    s.flows.find((f) => f.id === p.flowId)?.version === p.flowVersion &&
    p.bindings.every(
      (b) =>
        s.seats.find((x) => x.id === b.seatId)?.person === b.person &&
        s.projects
          .find((x) => x.id === p.projectId)
          ?.members.some((m) => m.name === b.person),
    ) &&
    p.tasks.every((t) =>
      s.positions.some(
        (pos) =>
          pos.projectId === p.projectId &&
          pos.id === s.seats.find((seat) => seat.id === t.seatId)?.positionId &&
          pos.bindings.some(
            (b) => b.flowId === p.flowId && b.nodeId === t.nodeId,
          ),
      ),
    )
  );
}
export function submitProposal(
  s: WorkState,
  topicId: string,
  input: ProposalInput,
  previousId?: string,
): Result {
  const topic = s.topics.find((t) => t.id === topicId),
    flow = s.flows.find((f) => f.id === input.flowId);
  if (!topic || !member(s, topic.projectId)) return { error: "permission" };
  if (!flow || flow.status === 'draft' || flow.projectId !== topic.projectId) return { error: "scope" };
  if (
    !input.title.trim() ||
    !input.goal.trim() ||
    !input.criteria.trim() ||
    !input.tasks.length
  )
    return { error: "required" };
  const seatIds = [
    ...new Set([
      input.ownerSeatId,
      ...input.tasks.flatMap((t) => [t.seatId, t.reviewerSeatId]),
    ]),
  ];
  const bindings = seatIds.map((seatId) => ({
    seatId,
    person: s.seats.find((x) => x.id === seatId)?.person ?? "",
  }));
  if (
    bindings.some(
      (b) =>
        !s.projects
          .find((p) => p.id === topic.projectId)
          ?.members.some((m) => m.name === b.person) ||
        !s.positions.some(
          (p) =>
            p.projectId === topic.projectId &&
            p.id === s.seats.find((x) => x.id === b.seatId)?.positionId,
        ),
    )
  )
    return { error: "scope" };
  if (
    input.tasks.some(
      (t, i) =>
        !t.title.trim() ||
        !flow.nodes.some((n) => n.id === t.nodeId) ||
        t.after.some((a) => a < 0 || a >= i) ||
        !s.positions.some(
          (p) =>
            p.id === s.seats.find((x) => x.id === t.seatId)?.positionId &&
            p.bindings.some(
              (b) => b.flowId === flow.id && b.nodeId === t.nodeId,
            ),
        ),
    )
  )
    return { error: "scope" };
  const old = s.proposals?.find((p) => p.id === previousId);
  if (
    previousId &&
    (!old ||
      old.topicId !== topicId ||
      old.status !== "rejected" ||
      old.sender !== s.currentUser ||
      s.proposals?.some((p) => p.previousId === previousId))
  )
    return { error: "stale" };
  const next = structuredClone(s);
  const p: WorkProposal = {
    ...structuredClone(input),
    id: uid(),
    projectId: topic.projectId,
    topicId,
    sourceAfterSeq:demoHistory(s,topic).at(-1)?.seq??0,
    revision: (old?.revision ?? 0) + 1,
    status: "pending",
    sender: s.currentUser,
    flowVersion: flow.version,
    bindings,
    approvers: [...new Set(bindings.map((b) => b.person))],
    votes: {},
    previousId,
    createdAt: Date.now(),
  };
  next.proposals = [...(next.proposals ?? []), p];
  next.events.push({
    id: uid(),
    projectId: p.projectId,
    targetId: p.id,
    actor: s.currentUser,
    text: W(
      "确认发送工作提案 v" + p.revision,
      "Confirmed proposal v" + p.revision,
    ),
    at: Date.now(),
  });
  return { state: next };
}
export function decideProposal(
  s: WorkState,
  id: string,
  revision: number,
  approve: boolean,
  reason = "",
): Result {
  const p = s.proposals?.find((p) => p.id === id);
  if (
    !p ||
    p.status !== "pending" ||
    p.revision !== revision ||
    p.votes[s.currentUser]
  )
    return { error: "stale" };
  if (!member(s, p.projectId) || !p.approvers.includes(s.currentUser))
    return { error: "permission" };
  if (!approve && !reason.trim()) return { error: "required" };
  if (approve && !proposalIsCurrent(s, p)) return { error: "stale" };
  const next = structuredClone(s),
    draft = next.proposals!.find((x) => x.id === id)!;
  if (!approve) {
    draft.status = "rejected";
    draft.reason = reason.trim();
  } else {
    draft.votes[s.currentUser] = true;
    if (draft.approvers.every((person) => draft.votes[person])) {
      if(draft.workChange){const c=draft.workChange;applyDemoWorkFields(next,c.kind,c.id,c.fields);draft.status='approved';next.events.push({id:uid(),projectId:p.projectId,targetId:c.id,actor:s.currentUser,text:W('安排变更已按职责确认','Arrangement changes confirmed'),at:Date.now()});return {state:next};}
      draft.status = "approved";
      draft.planId = uid();
      const taskIds = draft.tasks.map(() => uid());
      next.plans.push({
        id: draft.planId,
        mainTopicId:"main:"+draft.planId,
        projectId: p.projectId,
        title: W(p.title),
        goal: W(p.goal),
        criteria: p.criteria
          .split("\n")
          .filter(Boolean)
          .map((v) => W(v)),
        ownerSeatId: p.ownerSeatId,
        flowId: p.flowId,
        status: "active",
        referenceTaskIds: [],
      });
      draft.tasks.forEach((t, i) =>
        next.tasks.push({
          id: taskIds[i],
          revision: 1,
          projectId: p.projectId,
          planId: draft.planId!,
          title: W(t.title),
          expected: W(t.title),
          criteria: p.criteria
            .split("\n")
            .filter(Boolean)
            .map((v) => W(v)),
          seatIds: [t.seatId],
          reviewerSeatId: t.reviewerSeatId,
          flowId: p.flowId,
          nodeId: t.nodeId,
          status: "ready",
          requirements: t.after.map((a) => ({
            id: uid(),
            at: "both",
            label: W(
              "前置任务验收：" + draft.tasks[a].title,
              "Predecessor acceptance: " + draft.tasks[a].title,
            ),
            kind: "task",
            ref: taskIds[a],
            hard: true,
          })),
          files: [],
        }),
      );
      const topic = next.topics.find((t) => t.id === p.topicId)!;
      next.topics.push({id:"main:"+draft.planId,projectId:p.projectId,title:W(p.title),kind:"discussion",mainPlanId:draft.planId,parentTopicId:p.topicId,forkAfterSeq:p.sourceAfterSeq??demoHistory(s,topic).at(-1)?.seq??0,planIds:[draft.planId],taskIds:[],messages:[]});
      topic.planIds.push(draft.planId);
      topic.taskIds.push(...taskIds);
    }
  }
  next.events.push({
    id: uid(),
    projectId: p.projectId,
    targetId: p.id,
    actor: s.currentUser,
    text: W(
      approve ? "同意工作提案" : "退回工作提案：" + reason,
      approve ? "Approved work proposal" : "Returned work proposal: " + reason,
    ),
    at: Date.now(),
  });
  return { state: next };
}
export { discussTopic } from "./discussionPolicy.ts";
