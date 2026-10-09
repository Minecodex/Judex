import type {ForkTopicInput,TaskActivityInput,ResolveSuggestionInput} from "../chat/collaborationTypes";
import type { Key, Locale } from "../../i18n";
import type { ProposalInput } from "../chat/proposalModel";
import type {
  Evidence,
  Handoff,
  Member,
  Project,
  Route,
  Text,
  WorkState,
} from "./types";

export type ActResult = { ok: boolean; errorCode?: string; id?: string; inviteUrl?: string; createdCount?: number; skippedCount?: number };
export type ActOptions = { toast?: boolean };
export type ActFn = <K extends keyof ActionPayloads>(
  name: K,
  payload: ActionPayloads[K],
  opts?: ActOptions,
) => Promise<ActResult>;

export type WorkDraft = {
  kind: "plan" | "task";
  title: string;
  description: string;
  criteria: string;
  seatId: string;
  flowId: string;
  planId: string | null;
  parentId?: string;
};
export type PositionValue = {
  id?: string;
  name: string;
  prompt: string;
  publicSummary?: string;
  flowId: string;
  nodeId: string;
};

export interface ActionPayloads {
 updateWorkDraft:{kind:'task'|'plan';id:string;expectedVersion:number;fields:Record<string,unknown>};
 discardWork:{kind:'task'|'plan';id:string;expectedVersion:number;reviewHash:string};
 executionException:{taskId:string;operation:'skip'|'restore';expectedVersion:number;reviewHash:string;reason:string;waivers:import('./runtimeTypes').ExceptionWaiver[];acknowledgeStarted:boolean};
 proposeWorkChange:{kind:'task'|'plan';id:string;expectedVersion:number;fields:Record<string,unknown>;reason:string};
 ensureTaskMainTopic:{taskId:string};
 replaceTopicLinks:{topicId:string;expectedLinksVersion:number;planIds:string[];taskIds:string[]};
 forkTopic:ForkTopicInput;
 recordTaskActivity:TaskActivityInput;
 resolveDiscussionSuggestion:ResolveSuggestionInput;
 retryTaskAnalysis:{analysisId:string};
  sourceDecision: {
    handoffId: string;
    sourceId: string;
    revision: number;
    decision: "accepted" | "rejected";
    reason?: string;
  };
  reviseSource: {
    identityId?:string;
    handoffId: string;
    sourceId: string;
    revision: number;
    summary: string;
    files: Evidence[];
  };
  sendSource: { handoffId: string; sourceId: string; revision: number };
  reportTask: { taskId: string; summary: string; files: Evidence[];identityId?:string };
  taskAction: {
    identityId?:string;
 frozenReview?:{reviewId:string;reviewHash:string;targetVersion:number};decision?:"accept"|"reject";
    taskId: string;
    op: "start" | "accept" | "reopen";
    reason?: string;
    revision?: number;
  };
  planAction: {
 frozenReview?:{reviewId:string;reviewHash:string;targetVersion:number};
    planId: string;
    op: "resume" | "activate" | "accept";
    key?: string;
    revision?: number;
  };
  decideDraft: { taskId: string };
  createWork: { projectId: string; draft: WorkDraft };
  topicMessage: { topicId: string; body: string; files?: Evidence[];taskId?:string;planId?:string };
  publishFlow: {
    projectId: string;
    flowId: string;
    version: number;
    instructions: string;
    material: boolean;
  };
  refreshHandoff: { handoffId: string };
  personalPrompt: { projectId: string; positionId: string; expectedRevision: number; prompt: string };
  savePosition: { projectId: string; value: PositionValue };
  importPositionPresets: {projectId: string; catalogVersion: string; scenarioId: string; roleIds: string[]; locale: Locale};
  importWorkflowPresets: {projectId: string; catalogVersion: string; scenarioId: string; workflowIds: string[]; locale: Locale};
  saveFlowDraft: {projectId: string; flowId: string; expectedVersion: number; body: import('./types').FlowBody};
  publishFlowDraft: {projectId: string; flowId: string; expectedVersion: number; draftHash: string};
  invitePerson: { projectId: string; name: string; positionIds: string[] };
  assignPositions: { projectId: string; person: string; userId?: string; ids: string[] };
  acceptInvite: { invitationId: string };
  replaceSeat: { seatId: string; from: string; to: string; userId?: string };
  createFlow: {
    projectId: string;
    name: string;
    instructions: string;
    labels: string[];
  };
  remindPending: { projectId: string };
  submitProposal: {
    topicId: string;
    draft: ProposalInput;
    previousId?: string;
  };
  decideProposal: {
 frozenReview?:import("../../lib/api/schema").components["schemas"]["ProposalReview"];
    proposalId: string;
    revision: number;
    accept: boolean;
    reason?: string;
  };
  discussTopic: { topicId: string };
  setDiscussionLimit: { projectId: string; limit: number };
  proposeHandoff: {
    senderIdentityIds?:Record<string,string>;
    projectId: string;
    target: string;
    receiver: string;
    ids: string[];
    kind: Handoff["kind"];
  };
  createDiscussion: {
    projectId: string;
    title: string;
    planIds: string[];
    taskIds: string[];
  };
  sendTopicMessage: {
    taskId?:string;planId?:string;
    projectId: string;
    topicId?: string;
    title: string;
    body: string;
    files: Evidence[];
  };
  registerResource: { projectId: string; purpose: string; files: Evidence[];taskId?:string;planId?:string };
  createProject: {
    name: string;
    goal: string;
    kind: Project["kind"];
    maxRounds: number;
  };
}

export interface WorkStore {
 dataPending?:boolean;dataFailed?:boolean;dataRetry?:()=>void;
 mode:"demo"|"api";
  state: WorkState;
  route: Route;
  go(next: Partial<Route>): void;
  locale: Locale;
  setLocale(locale: Locale): void;
  theme: "light" | "dark";
  setTheme(theme: string): void;
  toast: string;
  setToast(value: string): void;
  storageError: boolean;
  text(value: Text | string): string;
  t(key: Key, values?: Record<string, string | number>): string;
  act: ActFn;
  switchPerson(name: string): void;
  reset(): void;
  project: Project;
  membership: Member | undefined;
  management: boolean;
  people: string[];
}
