export type Text = { zh: string; en: string };
export const words = (zh: string, en = zh): Text => ({ zh, en });
export type Member = { name: string; role: "owner" | "manager" | "member" };
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
  name: Text;
  prompt: Text;
  tone: string;
  bindings: { flowId: string; nodeId: string }[];
};
export type Seat = {
  id: string;
  positionId: string;
  person: string;
  notes: Text;
};
export type Plan = {
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
  id: string;
  name: string;
  text: string;
  author: string;
  at: number;
  data?: string;
  type?: string;
};
export type Requirement = {
  at?: "start" | "accept" | "both";
  id: string;
  label: Text;
  kind: "receipt" | "task" | "evidence";
  ref: string;
  hard: boolean;
};
export type Task = {
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
export type Flow = {
  id: string;
  projectId: string;
  name: Text;
  version: number;
  instructions: Text;
  nodes: { id: string; label: Text }[];
  edges: [string, string][];
  history: { version: number; instructions: Text; actor: string }[];
};
export type Topic = {
  discussionRuns?: import('../chat/discussionPolicy').DiscussionRun[];
  context?: { kind: "handoff"; id: string };
  id: string;
  projectId: string;
  title: Text;
  planIds: string[];
  taskIds: string[];
  messages: {
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
  preferences: { projectId: string; person: string; prompt: string }[];
  events: Audit[];
  currentUser: string;
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
  settingsSection?: import('../settings/navigation').SettingsSection;
  settingsItem?: string;
  conversation?: string;
  design: Design;
  projectId: string;
  view: View;
  id?: string;
};
