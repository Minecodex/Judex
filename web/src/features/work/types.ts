import type { TaskActivity, DiscussionSuggestion, SourceRef } from "../chat/collaborationTypes";
export type Text = { zh: string; en: string };
export const words = (zh: string, en = zh): Text => ({ zh, en });
export type Member = { name: string; role: "owner" | "manager" | "member"; userId?: string; email?: string };
export type Project = {
  maxDiscussionRounds?: number;
  id: string;
  title: Text;
  description: Text;
  kind: "software" | "design";
  members: Member[];
};
export type Position = {
  id: string;
  projectId: string;
  presetId?: string;
  publicSummary?: Text;
  name: Text;
  prompt: Text;
  tone: string;
  bindings: { flowId: string; nodeId: string }[];
};
export type Seat = {
  id: string;
  positionId: string;
  person: string;
  userId?: string;
  status?: string;
  notes: Text;
};
export type Plan = {
 capabilities?:import('./runtimeTypes').WorkCapabilities;discardedAt?:string|null;revision?:number;createdBy?:string;
 taskStats?: {total:number;accepted:number;active:number;cancelled:number;required?:number;draft?:number;skipped?:number};
 myTaskCount?:number;ownerName?:string;updatedAt?:number;
 mainTopicId?: string;
 businessStatus?: string;
  id: string;
  projectId: string;
  title: Text;
  goal: Text;
  criteria: Text[];
  ownerSeatId: string;
  flowId: string;
  status: "draft" | "active" | "accepted";
  acceptedAt?: number;
  referenceTaskIds: string[];
};
export type Evidence = {
 versionId?:string;
  id: string;
  name: string;
  text: string;
  author: string;
  at: number;
  data?: string;
  type?: string;
};
export type Requirement = {
 satisfied?:boolean;
 waived?:boolean;inheritedFrom?:string;sourceTaskId?:string;fingerprint?:string;
  at?: "start" | "accept" | "both";
  id: string;
  label: Text;
  kind: "receipt" | "task" | "evidence";
  ref: string;
  hard: boolean;
};
export type Task = {
 kind?:'task'|'bug';bugDetails?:import('../../lib/api/schema').components['schemas']['Task']['bugDetails'];
 capabilities?:import('./runtimeTypes').WorkCapabilities;discardedAt?:string|null;executionException?:import('./runtimeTypes').ExecutionException|null;createdBy?:string;
 mainTopicId?:string;
 participantNames?:string[];
 businessStatus?: string;
  revision: number;
  id: string;
  projectId: string;
  planId: string | null;
  parentId?: string;
  title: Text;
  expected: Text;
  criteria: Text[];
  seatIds: string[];
  reviewerSeatId: string;
  flowId: string;
  nodeId: string;
  status: "draft" | "ready" | "working" | "delivered" | "accepted" | "rework";
  requirements: Requirement[];
  files: Evidence[];
  branch?: string;
  acceptedAt?: number;
};
export type Source = {
  id: string;
  taskId: string;
  senderSeatId: string;
  revision: number;
  summary: Text;
  files: Evidence[];
  status: "draft" | "pending" | "accepted" | "rejected";
  reason?: string;
  sentBy?: string;
  sentAt?: number;
  decidedBy?: string;
  decidedAt?: number;
};
export type Handoff = {
  id: string;
  projectId: string;
  title: Text;
  taskId: string;
  receiverSeatId: string;
  flowId: string;
  flowVersion: number;
  stale: boolean;
  kind: "dependency" | "stage";
  sources: Source[];
  history: { source: Source; at: number }[];
};
export type FlowBody = import('../../lib/api/schema').components['schemas']['WorkflowDraftBody'];
export type Flow = {
  status?: 'draft' | 'published';
  presetId?: string;
  definitionVersion?: number;
  body?: FlowBody;
  draft?: {body: FlowBody; hash: string; revision: number};
  id: string;
  projectId: string;
  name: Text;
  version: number;
  instructions: Text;
  nodes: { id: string; label: Text; responsibility?: Text; kind?: 'activity'|'decision'; phase?: Text }[];
  edges: [string, string][];
  connections?: {from:string;to:string;kind?:'sequence'|'feedback';label?:Text}[];
  history: { version: number; instructions: Text; actor: string }[];
};
export type Topic = {
 linksVersion?:number;
 kind?: "project_room"|"discussion"|"handoff";
 parentTopicId?: string;
 forkAfterSeq?: number;
 lastMessageSeq?: number;
 mainPlanId?: string;
 sourceRefs?: SourceRef[];
  discussionRuns?: import('../chat/discussionPolicy').DiscussionRun[];
  context?: { kind: "handoff"; id: string };
  id: string;
  projectId: string;
  title: Text;
  planIds: string[];
  taskIds: string[];
  messages: {
    seq?:number;
    inherited?:boolean;
    originTopicId?:string;
    taskId?:string;
    sourceRef?:SourceRef;
    materials?:{versionId:string;name:string}[];
    submissionType?: 'message' | 'material' | 'work_report';
    seatId?: string;
    id: string;
    actor: string;
    kind: "person" | "ai";
    text: Text;
    at: number;
    files?: Evidence[];
  }[];
};
export type Invite = {
  id: string;
  projectId: string;
  person: string;
  positionIds: string[];
  status: "pending" | "accepted";
  sender: string;
};
export type Audit = {
  id: string;
  projectId: string;
  targetId: string;
  actor: string;
  text: Text;
  at: number;
};
export type WorkState = {
 taskActivities?: TaskActivity[];
 discussionSuggestions?: DiscussionSuggestion[];
  proposals?: import('../chat/proposalModel').WorkProposal[];
  schema: 4;
  projects: Project[];
  positions: Position[];
  seats: Seat[];
  plans: Plan[];
  tasks: Task[];
  handoffs: Handoff[];
  flows: Flow[];
  topics: Topic[];
  invites: Invite[];
  preferences: { projectId: string; positionId: string; person: string; userId?: string; revision: number; prompt: string }[];
  events: Audit[];
  currentUser: string;
  currentUserId?: string;
};
export type ErrorCode =
  | "discussionLimit"
  | "permission"
  | "required"
  | "stale"
  | "blocked"
  | "scope"
  | "accepted"
  | "invite";
export type Result =
  { state: WorkState; error?: never } | { error: ErrorCode; state?: never };
export type Design = "studio";
export type View =
  | "proposal"
  | "workspace"
  | "overview"
  | "home"
  | "plans"
  | "plan"
  | "tasks"
  | "task"
  | "handoffs"
  | "handoff"
  | "topics"
  | "topic"
  | "team"
  | "flows"
  | "decisions"
  | "settings"
  | "resources";
export type Route = {
 taskSection?:import('./runtimeTypes').TaskSection;
 page?:"projects"|"hub"|"route"|"chat";
 hubTab?:import("../cooperation/routing").HubTab;
 scopePlanId?:string;scopeTaskId?:string;focusTaskId?:string;
 editor?:"plan"|"task"|"topic"|"proposal";proposalTopicId?:string;editTopicId?:string;invite?:boolean;originProjects?:boolean;
 taskContextId?:string;
 activityId?:string;
 sourceId?:string;
 messageSeq?:number;
  settingsSection?: import('../settings/navigation').SettingsSection;
  settingsItem?: string;
  conversation?: string;
  design: Design;
  projectId: string;
  view: View;
  id?: string;
};
