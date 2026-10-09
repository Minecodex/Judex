import {demoTaskMain,demoReplaceLinks} from "../cooperation/demo";
import {updateDemoDraft,discardDemoWork,changeDemoExecution,proposeDemoWorkChange} from './demoRuntime.ts';
import {demoFork,demoTaskActivity,demoResolve} from "../chat/demoCollaboration";
import {
  acceptInvite,
  assignPositions,
  createFlow,
  createWork,
  decideDraft,
  invitePerson,
  personalPrompt,
  planAction,
  publishFlow,
  refreshHandoff,
  remindPending,
  replaceSeat,
  reportTask,
  reviseSource,
  savePosition,
  sendSource,
  sourceDecision,
  taskAction,
  topicMessage,
} from "./actions";
import { createDiscussion, proposeHandoff } from "./composition";
import { decideProposal, submitProposal } from "../chat/proposalModel";
import {
  discussTopic,
  setDiscussionLimit,
  validRoundLimit,
} from "../chat/discussionPolicy";
import { uid } from "./seed";
import { words, type Result, type WorkState } from "./types";
import type { ActionPayloads } from "./storeTypes";
import { importDemoPositionPresets } from "./positionPresets";
import {importDemoWorkflowPresets,saveDemoFlowDraft,publishDemoFlowDraft} from './workflowPresets';

export type DemoFn<K extends keyof ActionPayloads> = (
  s: WorkState,
  p: ActionPayloads[K],
) => Result & { id?: string; createdCount?: number; skippedCount?: number };

export const demoActions: { [K in keyof ActionPayloads]: DemoFn<K> } = {
 updateWorkDraft:updateDemoDraft,discardWork:discardDemoWork,executionException:changeDemoExecution,proposeWorkChange:proposeDemoWorkChange,
 ensureTaskMainTopic:(s,p)=>demoTaskMain(s,p.taskId),
 replaceTopicLinks:(s,p)=>demoReplaceLinks(s,p),
 forkTopic:(s,p)=>demoFork(s,p),
 recordTaskActivity:(s,p)=>demoTaskActivity(s,p),
 resolveDiscussionSuggestion:(s,p)=>demoResolve(s,p),
 retryTaskAnalysis:(s)=>({state:s}),
  importPositionPresets: (s, p) => importDemoPositionPresets(s, p.projectId, p),
  importWorkflowPresets: (s,p) => importDemoWorkflowPresets(s,p.projectId,p),
  saveFlowDraft: (s,p) => saveDemoFlowDraft(s,p.projectId,p.flowId,p.expectedVersion,p.body),
  publishFlowDraft: (s,p) => publishDemoFlowDraft(s,p.projectId,p.flowId,p.expectedVersion,p.draftHash),
  sourceDecision: (s, p) =>
    sourceDecision(
      s,
      p.handoffId,
      p.sourceId,
      p.revision,
      p.decision,
      p.reason,
    ),
  reviseSource: (s, p) =>
    reviseSource(s, p.handoffId, p.sourceId, p.revision, p.summary, p.files),
  sendSource: (s, p) => sendSource(s, p.handoffId, p.sourceId, p.revision),
  reportTask: (s, p) => reportTask(s, p.taskId, p.summary, p.files),
  taskAction: (s, p) => taskAction(s, p.taskId, p.op, p.reason, p.revision, p.identityId),
  planAction: (s, p) => planAction(s, p.planId, p.op, p.key),
  decideDraft: (s, p) => decideDraft(s, p.taskId),
  createWork: (s, p) => {
    const r = createWork(s, p.projectId, p.draft);
    return r.error
      ? r
      : {
          ...r,
          id:
            p.draft.kind === "plan"
              ? r.state.plans.at(-1)!.id
              : r.state.tasks.at(-1)!.id,
        };
  },
  topicMessage: (s, p) => p.taskId?demoTaskActivity(s,{taskId:p.taskId,kind:"reply",topicId:p.topicId,body:p.body,files:p.files??[]}):topicMessage(s,p.topicId,p.body,p.files),
  publishFlow: (s, p) =>
    publishFlow(s, p.flowId, p.version, p.instructions, p.material),
  refreshHandoff: (s, p) => refreshHandoff(s, p.handoffId),
  personalPrompt: (s, p) => personalPrompt(s, p.projectId, p.positionId, p.expectedRevision, p.prompt),
  savePosition: (s, p) => savePosition(s, p.projectId, p.value),
  invitePerson: (s, p) => invitePerson(s, p.projectId, p.name, p.positionIds),
  assignPositions: (s, p) => assignPositions(s, p.projectId, p.person, p.ids),
  acceptInvite: (s, p) => acceptInvite(s, p.invitationId),
  replaceSeat: (s, p) => replaceSeat(s, p.seatId, p.from, p.to),
  createFlow: (s, p) =>
    createFlow(s, p.projectId, p.name, p.instructions, p.labels),
  remindPending: (s, p) => remindPending(s, p.projectId),
  submitProposal: (s, p) => submitProposal(s, p.topicId, p.draft, p.previousId),
  decideProposal: (s, p) =>
    decideProposal(s, p.proposalId, p.revision, p.accept, p.reason),
  discussTopic: (s, p) => discussTopic(s, p.topicId),
  setDiscussionLimit: (s, p) => setDiscussionLimit(s, p.projectId, p.limit),
  proposeHandoff: (s, p) => {
    const r = proposeHandoff(
      s,
      p.projectId,
      p.target,
      p.receiver,
      p.ids,
      p.kind,
      p.senderIdentityIds,
    );
    return r.error ? r : { ...r, id: r.state.handoffs.at(-1)!.id };
  },
  createDiscussion: (s, p) => {
    const r = createDiscussion(s, p.projectId, p.title, p.planIds, p.taskIds);
    return r.error ? r : { ...r, id: r.state.topics.at(-1)!.id };
  },
  sendTopicMessage: (s, p) => {
    if (p.topicId) {
      const r = p.taskId?demoTaskActivity(s,{taskId:p.taskId,kind:"reply",topicId:p.topicId,body:p.body,files:p.files??[]}):topicMessage(s, p.topicId, p.body, p.files);
      return r.error ? r : { ...r, id: p.topicId };
    }
    const created = createDiscussion(s, p.projectId, p.title, [], []);
    if (created.error) return created;
    const id = created.state.topics.at(-1)!.id;
    const sent = topicMessage(created.state, id, p.body, p.files);
    return sent.error ? sent : { ...sent, id };
  },
  registerResource: (s, p) => {
    const created = createDiscussion(s, p.projectId, p.purpose, [], []);
    if (created.error) return created;
    const id = created.state.topics.at(-1)!.id;
    const sent = topicMessage(created.state, id, p.purpose, p.files);
    return sent.error ? sent : { ...sent, id };
  },
  createProject: (s, p) => {
    if (!p.name.trim() || !p.goal.trim() || !validRoundLimit(p.maxRounds))
      return { error: "required" };
    const id = uid();
    return {
      state: {
        ...s,
        projects: [
          ...s.projects,
          {
            id,
            title: words(p.name.trim()),
            description: words(p.goal.trim()),
            kind: p.kind,
            maxDiscussionRounds: p.maxRounds,
            members: [{ name: s.currentUser, role: "owner" }],
          },
        ],
      },
      id,
    };
  },
};
