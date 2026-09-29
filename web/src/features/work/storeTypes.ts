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

export type ActResult = { ok: boolean; id?: string };
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
  flowId: string;
  nodeId: string;
};

export interface ActionPayloads {
  sourceDecision: {
    handoffId: string;
    sourceId: string;
    revision: number;
    decision: "accepted" | "rejected";
    reason?: string;
  };
  reviseSource: {
    handoffId: string;
    sourceId: string;
    revision: number;
    summary: string;
    files: Evidence[];
  };
  sendSource: { handoffId: string; sourceId: string; revision: number };
  reportTask: { taskId: string; summary: string; files: Evidence[] };
  taskAction: {
    taskId: string;
    op: "start" | "accept" | "reopen";
    reason?: string;
    revision?: number;
  };
  planAction: {
    planId: string;
    op: "resume" | "activate" | "accept";
    key?: string;
    revision?: number;
  };
  decideDraft: { taskId: string };
  createWork: { projectId: string; draft: WorkDraft };
  topicMessage: { topicId: string; body: string; files?: Evidence[] };
  publishFlow: {
    projectId: string;
    flowId: string;
    version: number;
    instructions: string;
    material: boolean;
  };
  refreshHandoff: { handoffId: string };
  personalPrompt: { projectId: string; prompt: string };
  savePosition: { projectId: string; value: PositionValue };
  invitePerson: { projectId: string; name: string; positionIds: string[] };
  assignPositions: { projectId: string; person: string; ids: string[] };
  acceptInvite: { invitationId: string };
  replaceSeat: { seatId: string; from: string; to: string };
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
    proposalId: string;
    revision: number;
    accept: boolean;
    reason?: string;
  };
  discussTopic: { topicId: string };
  setDiscussionLimit: { projectId: string; limit: number };
  proposeHandoff: {
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
    projectId: string;
    topicId?: string;
    title: string;
    body: string;
    files: Evidence[];
  };
  registerResource: { projectId: string; purpose: string; files: Evidence[] };
  createProject: {
    name: string;
    goal: string;
    kind: Project["kind"];
    maxRounds: number;
  };
}

export interface WorkStore {
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
