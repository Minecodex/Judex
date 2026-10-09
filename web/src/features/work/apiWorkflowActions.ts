import {request} from '../../lib/api/client';
import type {components} from '../../lib/api/schema';
import type {ActionPayloads,ActResult} from './storeTypes';
type Names = 'importWorkflowPresets'|'saveFlowDraft'|'publishFlowDraft';
const write = <T>(path: string, method: 'POST'|'PUT', body: unknown) =>
  request<T>(path,{method,body:JSON.stringify(body),idempotencyKey:crypto.randomUUID()});
export const workflowApiActions: {[K in Names]: (projectId:string,p:ActionPayloads[K])=>Promise<ActResult>} = {
  importWorkflowPresets: async (projectId,p) => {
    const result = await write<components['schemas']['ImportedWorkflowPresets']>('/projects/'+projectId+'/workflows/import-presets','POST',{
      catalogVersion:p.catalogVersion,scenarioId:p.scenarioId,workflowIds:p.workflowIds,locale:p.locale});
    return {ok:true,id:result.items[0]?.id ?? result.skipped[0]?.workflowId,createdCount:result.items.length,skippedCount:result.skipped.length};
  },
  saveFlowDraft: async (projectId,p) => {
    await write('/projects/'+projectId+'/workflows/'+p.flowId+'/draft','PUT',{expectedVersion:p.expectedVersion,body:p.body});
    return {ok:true,id:p.flowId};
  },
  publishFlowDraft: async (projectId,p) => {
    await write('/projects/'+projectId+'/workflows/'+p.flowId+'/publish','POST',{expectedVersion:p.expectedVersion,draftHash:p.draftHash});
    return {ok:true,id:p.flowId};
  },
};
