import {request} from "../../lib/api/client";
import type {components} from "../../lib/api/schema";
import type {ActResult,ActionPayloads} from "../work/storeTypes";
import type {ApiActContext} from "../work/apiActions";
type Executor<K extends keyof ActionPayloads>=(projectId:string,p:ActionPayloads[K],ctx:ApiActContext)=>Promise<ActResult>;
export const cooperationApiActions:{ensureTaskMainTopic:Executor<"ensureTaskMainTopic">;replaceTopicLinks:Executor<"replaceTopicLinks">}={
 ensureTaskMainTopic:async(projectId,p)=>{const result=await request<components["schemas"]["TaskMainTopicResult"]>(`/projects/${projectId}/tasks/${p.taskId}/main-topic`,{method:"POST",idempotencyKey:crypto.randomUUID()});return {ok:true,id:result.mainTopicId};},
 replaceTopicLinks:async(projectId,p)=>{const result=await request<components["schemas"]["Topic"]>(`/projects/${projectId}/topics/${p.topicId}/links`,{method:"PUT",idempotencyKey:crypto.randomUUID(),body:JSON.stringify({expectedLinksVersion:p.expectedLinksVersion,targetRefs:[...p.planIds.map(objectId=>({objectType:"plan",objectId})),...p.taskIds.map(objectId=>({objectType:"task",objectId}))]})});return {ok:true,id:result.id};},
};
