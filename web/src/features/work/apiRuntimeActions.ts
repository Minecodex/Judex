import {request} from '../../lib/api/client';
import type {ActionPayloads,ActResult} from './storeTypes';
const send=(path:string,method:'POST'|'PUT',body:unknown)=>request(path,{method,headers:{'Idempotency-Key':crypto.randomUUID()},body:JSON.stringify(body)});
export const runtimeApiActions={
 updateWorkDraft:async(project:string,p:ActionPayloads['updateWorkDraft']):Promise<ActResult>=>{await send(`/projects/${project}/${p.kind}s/${p.id}/draft`,'PUT',{expectedVersion:p.expectedVersion,fields:p.fields});return {ok:true,id:p.id};},
 discardWork:async(project:string,p:ActionPayloads['discardWork']):Promise<ActResult>=>{await send(`/projects/${project}/${p.kind}s/${p.id}/discard`,'POST',{expectedVersion:p.expectedVersion,reviewHash:p.reviewHash});return {ok:true,id:p.id};},
 executionException:async(project:string,p:ActionPayloads['executionException']):Promise<ActResult>=>{await send(`/projects/${project}/tasks/${p.taskId}/${p.operation}`,'POST',{expectedVersion:p.expectedVersion,reviewHash:p.reviewHash,reason:p.reason,waivers:p.waivers,acknowledgeStarted:p.acknowledgeStarted});return {ok:true,id:p.taskId};},
};
