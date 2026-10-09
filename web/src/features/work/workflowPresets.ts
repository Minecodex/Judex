import workflowCatalog from '../../../../internal/project/catalog/workflow-presets.json' with {type: 'json'};
import {builtinPositionPresets} from './positionPresets.ts';
import type {components} from '../../lib/api/schema';
import type {Flow, FlowBody, Result, WorkState} from './types.ts';
import {words} from './types.ts';
import {manage} from './selectors.ts';
import {uid} from './seed.ts';

export type WorkflowPresetCatalog = components['schemas']['WorkflowPresetCatalog'];
export type WorkflowPreset = components['schemas']['WorkflowPreset'];
export type WorkflowImportRequest = components['schemas']['ImportWorkflowPresetsRequest'];
export const builtinWorkflowPresets: WorkflowPresetCatalog = {
  ...workflowCatalog as WorkflowPresetCatalog,
  scenarios: builtinPositionPresets.scenarios.map(s => ({id:s.id,name:s.name,description:s.description,
    workflowIds:workflowCatalog.workflows.filter(p => p.scenarioIds.includes(s.id)).map(p => p.id)})),
  roles: builtinPositionPresets.roles.map(r => ({id:r.id,name:r.name})),
};
const normalized = (value: string) => value.trim().toLowerCase();
export function existingPresetWorkflow(flows: Flow[], preset: WorkflowPreset) {
  const names = [normalized(preset.name.zh),normalized(preset.name.en)];
  return flows.find(f => f.presetId === preset.id ||
    [f.name.zh,f.name.en].some(name => names.includes(normalized(name))));
}
export function workflowPresetBody(preset: WorkflowPreset, locale: 'zh-CN'|'en'): FlowBody {
  const language = locale === 'en' ? 'en' : 'zh';
  return {name:preset.name[language],instructions:preset.instructions[language],
    nodes:preset.nodes.map(n => ({id:n.id,name:n.name[language],responsibility:n.responsibility[language],
      ...(n.kind ? {kind:n.kind}:{}),...(n.phase ? {phase:n.phase[language]}:{}),allowedPositionIds:[],defaultApprovalPolicy:'all'})),
    advisoryEdges:preset.edges.map(e => ({from:e.from,to:e.to,...(e.kind ? {kind:e.kind}:{}),...(e.label ? {label:e.label[language]}:{})})),
    hardRules:[],approvalPolicies:{}};
}
export function diagramFromBody(body: FlowBody): Pick<Flow,'nodes'|'edges'|'connections'> {
  return {nodes:body.nodes.map(n => ({id:n.id,label:words(n.name,n.name),responsibility:words(n.responsibility,n.responsibility),
      ...(n.kind ? {kind:n.kind}:{}),...(n.phase ? {phase:words(n.phase,n.phase)}:{})})),
    edges:(body.advisoryEdges ?? []).filter(e => e.kind !== 'feedback').map(e => [e.from,e.to]),
    connections:(body.advisoryEdges ?? []).map(e => ({from:e.from,to:e.to,kind:e.kind,label:e.label ? words(e.label,e.label):undefined}))};
}
export function bodyFromFlow(flow: Flow, english: boolean): FlowBody {
  if (flow.draft) return structuredClone(flow.draft.body);
  if (flow.body) return structuredClone(flow.body);
  const language = english ? 'en' : 'zh';
  return {name:flow.name[language],instructions:flow.instructions[language],
    nodes:flow.nodes.map(n => ({id:n.id,name:n.label[language],responsibility:n.responsibility?.[language] ?? '',allowedPositionIds:[],defaultApprovalPolicy:'all'})),
    advisoryEdges:flow.edges.map(([from,to]) => ({from,to})),hardRules:[],approvalPolicies:{}};
}
// Shared by the explicit demo and editor; the backend remains authoritative.
export function validFlowBody(body: FlowBody) {
  const ids = new Set(body.nodes.map(n => n.id));
  if (!body.name.trim() || Array.from(body.name.trim()).length > 200 || !body.nodes.length ||
    ids.size !== body.nodes.length || body.nodes.some(n => !n.id || /[\s/]/.test(n.id) || !n.name.trim() ||
      !['activity','decision'].includes(n.kind ?? 'activity') || Array.from(n.phase ?? '').length>120)) return false;
  const edges = body.advisoryEdges ?? [];
  if (edges.some(e => !ids.has(e.from) || !ids.has(e.to) || e.from === e.to ||
    !['sequence','feedback'].includes(e.kind ?? 'sequence') || Array.from(e.label ?? '').length>200)) return false;
  const forward = edges.filter(e => e.kind !== 'feedback');
  const visiting = new Set<string>(), visited = new Set<string>();
  const visit = (id: string): boolean => {
    if (visiting.has(id)) return false;
    if (visited.has(id)) return true;
    visiting.add(id);
    if (forward.filter(e => e.from === id).some(e => !visit(e.to))) return false;
    visiting.delete(id); visited.add(id); return true;
  };
  return [...ids].every(visit) && edges.filter(e => e.kind === 'feedback').every(e =>
    body.nodes.find(n => n.id === e.from)?.kind === 'decision' && !!e.label?.trim() && feedbackCandidates(body,e.from).some(n => n.id === e.to));
}
export function feedbackCandidates(body: FlowBody, from: string) {
  const edges = (body.advisoryEdges ?? []).filter(e => e.kind !== 'feedback');
  const predecessors = new Set<string>(), pending = [from];
  while(pending.length) {
    const target = pending.pop()!;
    for(const edge of edges.filter(e => e.to === target)) if(edge.from !== from && !predecessors.has(edge.from)){
      predecessors.add(edge.from);pending.push(edge.from);
    }
  }
  return body.nodes.filter(n => predecessors.has(n.id));
}
export function importDemoWorkflowPresets(state: WorkState, projectId: string, request: WorkflowImportRequest): Result & {id?:string;createdCount?:number;skippedCount?:number} {
  if (!manage(state,projectId)) return {error:'permission'};
  if (request.catalogVersion !== builtinWorkflowPresets.version) return {error:'stale'};
  const scenario = builtinWorkflowPresets.scenarios.find(s => s.id === request.scenarioId);
  if (!scenario || !['zh-CN','en'].includes(request.locale) || !request.workflowIds.length || request.workflowIds.length > 32 ||
    new Set(request.workflowIds).size !== request.workflowIds.length || request.workflowIds.some(id => !scenario.workflowIds.includes(id))) return {error:'scope'};
  const next = structuredClone(state); let createdCount = 0, skippedCount = 0, firstId: string|undefined;
  for (const id of request.workflowIds) {
    const preset = builtinWorkflowPresets.workflows.find(p => p.id === id)!;
    const existing = existingPresetWorkflow(next.flows.filter(f => f.projectId === projectId),preset);
    if (existing) {skippedCount++;firstId ??= existing.id;continue;}
    const body = workflowPresetBody(preset,request.locale);
    const flow: Flow = {id:uid(),projectId,presetId:id,status:'draft',definitionVersion:1,version:1,
      name:words(body.name,body.name),instructions:words(body.instructions ?? '',body.instructions ?? ''),
      ...diagramFromBody(body),body,draft:{body,hash:JSON.stringify(body),revision:1},history:[]};
    next.flows.push(flow); createdCount++; firstId ??= flow.id;
  }
  return {state:next,id:firstId,createdCount,skippedCount};
}
export function saveDemoFlowDraft(state: WorkState, projectId: string, flowId: string, expectedVersion: number, body: FlowBody): Result {
  if (!manage(state,projectId)) return {error:'permission'};
  const flow = state.flows.find(f => f.id === flowId && f.projectId === projectId);
  if (!flow || !validFlowBody(body)) return {error:'scope'};
  if ((flow.definitionVersion ?? flow.version) !== expectedVersion) return {error:'stale'};
  const next = structuredClone(state), target = next.flows.find(f => f.id === flowId)!;
  target.definitionVersion = expectedVersion + 1;
  target.draft = {body:structuredClone(body),hash:JSON.stringify(body),revision:flow.draft?.revision ?? flow.version + 1};
  if (target.status === 'draft') {
    target.body = structuredClone(body);target.name = words(body.name,body.name);
    target.instructions = words(body.instructions ?? '',body.instructions ?? '');Object.assign(target,diagramFromBody(body));
  }
  return {state:next};
}
export function publishDemoFlowDraft(state: WorkState, projectId: string, flowId: string, expectedVersion: number, draftHash: string): Result {
  if (!manage(state,projectId)) return {error:'permission'};
  const flow = state.flows.find(f => f.id === flowId && f.projectId === projectId);
  if (!flow?.draft) return {error:'scope'};
  if ((flow.definitionVersion ?? flow.version) !== expectedVersion || flow.draft.hash !== draftHash) return {error:'stale'};
  const next = structuredClone(state), target = next.flows.find(f => f.id === flowId)!, body = target.draft!.body;
  target.version = target.draft!.revision; target.definitionVersion = expectedVersion + 1; target.status = 'published';
  target.body = structuredClone(body); target.name = words(body.name,body.name);
  target.instructions = words(body.instructions ?? '',body.instructions ?? ''); Object.assign(target,diagramFromBody(body));
  target.history.push({version:target.version,instructions:target.instructions,actor:state.currentUser}); target.draft = undefined;
  return {state:next};
}
