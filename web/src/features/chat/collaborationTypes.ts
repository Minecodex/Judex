import type { components } from "../../lib/api/schema";
import type { Evidence } from "../work/types";
export type TaskActivity = components["schemas"]["TaskActivity"];
export type TaskAnalysis = components["schemas"]["TaskAnalysis"];
export type DiscussionSuggestion = components["schemas"]["DiscussionSuggestion"];
export type PlanDiscussionSummary = components["schemas"]["PlanDiscussionSummary"];
export type SourceRef = components["schemas"]["ConversationSourceRef"];
export type TaskInputKind = "progress" | "delivery" | "question" | "reply";
export type TaskActivityInput = {
 identityId?:string;
 expectedTaskVersion?:number;
  taskId: string; kind: TaskInputKind; body: string; files: Evidence[];
  topicId?: string; simulatedLocal?: boolean;
};
export type ForkTopicInput = {
  topicId: string; title: string; forkAfterSeq?: number;
  planIds: string[]; taskIds: string[]; sourceRefs?: SourceRef[];
};
export type ResolveSuggestionInput = {
  suggestionId: string; expectedVersion: number;
  mode: "create" | "main" | "link" | "dismiss";
  title?: string; topicId?: string; planIds?: string[]; taskIds?: string[];
};
