import {request} from "../../lib/api/client";
import type {Change,EvidenceReview} from "../work/ReviewDetails";
type Intent={operation:string;objectId:string;reviewHash:string;payload:Record<string,unknown>};
export type IntentReview={reviewHash:string;changes?:Change[];evidence?:EvidenceReview};
export async function loadIntentReview(project:string,value:Intent,receiptHint:string):Promise<IntentReview>{
 const prefix=`/projects/${project}`;
 if(value.operation.startsWith("proposal."))return request<IntentReview>(`${prefix}/proposals/${value.objectId}/review`);
 if(value.operation==="task.acceptance"||value.operation==="plan.acceptance"){const evidence=await request<EvidenceReview>(`${prefix}/${value.operation.startsWith("task")?"tasks":"plans"}/${value.objectId}/acceptance-review`);return {reviewHash:evidence.reviewHash,evidence};}
 if(value.operation==="workflow.publish"){
  const versions=await request<{items:{draftHash:string;state:string;body:Record<string,unknown>}[]}>(`${prefix}/workflows/${value.objectId}/versions`);
  const draft=versions.items.find((v)=>v.state==="draft");return {reviewHash:draft?.draftHash??"changed",changes:draft?[{operation:value.operation,targetType:"workflow",fields:draft.body}]:[]};
 }
 if(value.operation==="task.reopen"||value.operation==="plan.reopen"){
  const item=await request<Record<string,unknown>>(`${prefix}/${value.operation.startsWith("task")?"tasks":"plans"}/${value.objectId}`);
  const current=item.status==="accepted"&&item.latestAcceptanceId===value.payload.acceptanceId&&item.version===value.payload.expectedVersion;
  return {reviewHash:current?value.reviewHash:"changed",changes:[{operation:value.operation,targetType:value.operation.split('.')[0],fields:{title:item.title,goal:item.goal??item.expectedOutput,acceptanceCriteria:item.acceptanceCriteria,...value.payload}}]};
 }
 if(value.operation.startsWith("handoff.")){
  type Source={id:string;sourceTaskId:string;currentVersion:number|null;currentVersionId:string|null;state:string;summary?:string;evidence?:{reports:NonNullable<EvidenceReview['reports']>}};
  type Handoff={id:string;title:string;receiverDisplayName?:string;sources:Source[]};
  let cursor="";let handoff:Handoff|undefined;let source:Source|undefined;
  do{const page=await request<{items:Handoff[];nextCursor?:string}>(`${prefix}/handoffs?limit=100${cursor?"&cursor="+encodeURIComponent(cursor):""}`);handoff=page.items.find((h)=>h.sources.some((s)=>s.id===value.objectId));source=handoff?.sources.find((s)=>s.id===value.objectId);cursor=page.nextCursor??"";}while(!source&&cursor);
  if(!source||!handoff)throw new Error("Source no longer available");
  const changes=[{operation:value.operation,targetType:"handoff",fields:{title:handoff.title,receiver:handoff.receiverDisplayName,summary:value.payload.summary??source.summary,meaning:receiptHint}}];
  if(value.operation==="handoff.decision")return {reviewHash:source.state==="pending"?source.currentVersionId??"changed":"changed",changes,evidence:{reviewId:source.id,reviewHash:source.currentVersionId??"",targetVersion:source.currentVersion??0,reports:source.evidence?.reports}};
  const task=await request<{latestReportId:string;version:number}>(`${prefix}/tasks/${source.sourceTaskId}`);
  const evidence=await request<EvidenceReview>(`${prefix}/tasks/${source.sourceTaskId}/acceptance-review`);
  const current=task.latestReportId===value.reviewHash&&(source.currentVersion??0)===(value.payload.sourceVersion??0);
  return {reviewHash:current?value.reviewHash:"changed",changes,evidence};
 }
 return {changes:[{operation:value.operation,targetType:value.operation.split('.')[0],targetId:value.objectId,fields:value.payload}],reviewHash:value.reviewHash};
}
