import type {Evidence, Route} from '../work/types';

export type ConversationWorkContext = {kind: 'topic'} | {kind: 'task' | 'plan'; id: string};
export type ConversationDraft = {
  body: string; files: Evidence[]; attachments: boolean;
  context?: ConversationWorkContext;
};
export const conversationContextKey = (key: string) => 'judex.chat.work-context.' + key;
export function parseConversationContext(value: unknown): ConversationWorkContext | undefined {
  if (!value || typeof value !== 'object') return;
  const v = value as {kind?: unknown; id?: unknown};
  if (v.kind === 'topic') return {kind: 'topic'};
  if ((v.kind === 'task' || v.kind === 'plan') && typeof v.id === 'string' && v.id) return {kind: v.kind, id: v.id};
}
export function storedConversationContext(key: string): ConversationWorkContext | undefined {
  try {return parseConversationContext(JSON.parse(sessionStorage.getItem(conversationContextKey(key)) ?? 'null'));} catch {return;}
}
// A discussion's draft owns its explicit work context. Entry scope supplies the
// first default; returning through another tab or entrance cannot replace it.
export function initialConversationContext(key: string, cache: Map<string, ConversationDraft>, route: Pick<Route, 'taskContextId' | 'scopeTaskId' | 'scopePlanId'>): ConversationWorkContext {
  const saved = parseConversationContext(cache.get(key)?.context) ?? storedConversationContext(key);
  if (saved) return saved;
  const task = route.taskContextId ?? route.scopeTaskId;
  return task ? {kind: 'task', id: task} : route.scopePlanId ? {kind: 'plan', id: route.scopePlanId} : {kind: 'topic'};
}
export function conversationContextValue(context: ConversationWorkContext): string {
  return context.kind === 'topic' ? '' : context.kind === 'plan' ? 'plan:' + context.id : context.id;
}
export function conversationContextFromValue(value: string): ConversationWorkContext {
  return value.startsWith('plan:') ? {kind: 'plan', id: value.slice(5)} : value ? {kind: 'task', id: value} : {kind: 'topic'};
}
