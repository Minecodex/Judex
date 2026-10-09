import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { request } from "../../lib/api/client";
import { useWork } from "../work/store";
import type { DiscussionSuggestion, PlanDiscussionSummary, TaskActivity } from "./collaborationTypes";
type Page<T>={items:T[];nextCursor:string|null};
export const collaborationKey=(projectId:string,...parts:string[])=>["apiWs",projectId,"collaboration",...parts];
export function useTaskActivity(taskId:string){
  const {state,project,mode}=useWork();
  const query=useInfiniteQuery({
    queryKey:collaborationKey(project.id,state.currentUserId??state.currentUser,"activity",taskId),enabled:mode==="api"&&!!taskId,
    initialPageParam:"",queryFn:({pageParam})=>request<Page<TaskActivity>>(`/projects/${project.id}/tasks/${taskId}/activity?limit=20${pageParam?"&cursor="+encodeURIComponent(pageParam):""}`),
    getNextPageParam:page=>page.nextCursor||undefined,refetchInterval:5000,
  });
  return {...query,items:mode==="demo"?(state.taskActivities??[]).filter(a=>a.taskId===taskId).slice().reverse():query.data?.pages.flatMap(p=>p.items)??[],pending:mode==="api"&&query.isPending};
}
export function usePlanSummary(planId?:string){
  const {state,project,mode}=useWork();
  const query=useQuery({queryKey:collaborationKey(project.id,state.currentUserId??state.currentUser,"summary",planId??""),enabled:mode==="api"&&!!planId,queryFn:()=>request<PlanDiscussionSummary>(`/projects/${project.id}/plans/${planId}/discussion-summary`),refetchInterval:5000});
  if(mode==="demo"){
    const plan=state.plans.find(p=>p.id===planId);
    return {...query,pending:false,data:plan?{planId:plan.id,mainTopicId:plan.mainTopicId??null,tasks:state.tasks.filter(t=>t.planId===plan.id).map(t=>{const entries=(state.taskActivities??[]).filter(a=>a.taskId===t.id);return {taskId:t.id,activityCount:entries.length,latestActivity:entries.at(-1)??null};}),suggestions:(state.discussionSuggestions??[]).filter(s=>s.planId===plan.id&&s.state==="pending")}:undefined};
  }
  return {...query,pending:query.isPending&&!!planId};
}
export function useDiscussionSuggestions(taskId?:string){
  const {state,project,mode}=useWork();
  const query=useInfiniteQuery({queryKey:collaborationKey(project.id,state.currentUserId??state.currentUser,"suggestions",taskId??""),enabled:mode==="api",initialPageParam:"",queryFn:({pageParam})=>request<Page<DiscussionSuggestion>>(`/projects/${project.id}/discussion-suggestions?limit=100${taskId?"&taskId="+taskId:""}${pageParam?"&cursor="+encodeURIComponent(pageParam):""}`),getNextPageParam:p=>p.nextCursor||undefined,refetchInterval:5000});
  return {...query,items:(mode==="demo"?state.discussionSuggestions??[]:query.data?.pages.flatMap(p=>p.items)??[]).filter(s=>!taskId||s.taskId===taskId)};
}
