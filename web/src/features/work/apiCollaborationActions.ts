import { APIError,request } from "../../lib/api/client";
import type { components } from "../../lib/api/schema";
import type { ActionPayloads, ActResult } from "./storeTypes";
import type { ApiActContext } from "./apiActions";
import { actingIdentity, uploadEvidence } from "./apiActions";
type Schema=components["schemas"];
type Executor<K extends keyof ActionPayloads>=(projectId:string,p:ActionPayloads[K],ctx:ApiActContext)=>Promise<ActResult>;
const post=<T>(path:string,body:unknown)=>request<T>(path,{method:"POST",body:JSON.stringify(body),idempotencyKey:crypto.randomUUID()});
const links=(planIds:string[],taskIds:string[])=>[...planIds.map(objectId=>({objectType:"plan",objectId})),...taskIds.map(objectId=>({objectType:"task",objectId}))];
export const apiCollaborationActions:{
  forkTopic:Executor<"forkTopic">;
  recordTaskActivity:Executor<"recordTaskActivity">;
  resolveDiscussionSuggestion:Executor<"resolveDiscussionSuggestion">;
  retryTaskAnalysis:Executor<"retryTaskAnalysis">;
}={
  forkTopic:async(projectId,p)=>{
    const topic=await post<Schema["Topic"]>(`/projects/${projectId}/topics/${p.topicId}/fork`,{title:p.title,forkAfterSeq:p.forkAfterSeq,links:links(p.planIds,p.taskIds),sourceRefs:p.sourceRefs??[]});
    return {ok:true,id:topic.id};
  },
  recordTaskActivity:async(projectId,p,ctx)=>{
    const materialVersionIds=await uploadEvidence(projectId,p.files);
    const text=p.body.trim()||p.files[0]?.name||"";
    if(p.kind==="progress"||p.kind==="delivery"){
      const task=await request<Schema["Task"]>(`/projects/${projectId}/tasks/${p.taskId}`);
      if(p.expectedTaskVersion!==undefined&&task.version!==p.expectedTaskVersion)throw new APIError(409,"VERSION_CONFLICT","task agreement changed; review the task before reporting");
      const report=await post<Schema["WorkReport"]>(`/projects/${projectId}/tasks/${p.taskId}/reports`,{kind:p.kind,text,identityId:actingIdentity(ctx,task,p.identityId),materialVersionIds,expectedTaskVersion:task.version});
      return {ok:true,id:report.id};
    }
    const submission=await post<Schema["Submission"]>(`/projects/${projectId}/submissions`,{clientSubmissionId:crypto.randomUUID(),purpose:"message",discussionIntent:p.kind,text,taskId:p.taskId,...(p.topicId?{topicId:p.topicId}:{}),materialVersionIds});
    return {ok:true,id:submission.id};
  },
  resolveDiscussionSuggestion:async(projectId,p)=>{
    const result=await post<Schema["DiscussionSuggestion"]>(`/projects/${projectId}/discussion-suggestions/${p.suggestionId}`,{expectedVersion:p.expectedVersion,mode:p.mode,title:p.title,topicId:p.topicId,links:links(p.planIds??[],p.taskIds??[])});
    return {ok:true,id:result.resultTopicId??undefined};
  },
  retryTaskAnalysis:async(projectId,p)=>{
    await post(`/projects/${projectId}/task-analyses/${p.analysisId}/retry`,{});
    return {ok:true,id:p.analysisId};
  },
};
