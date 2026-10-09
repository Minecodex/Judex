import {useQuery} from '@tanstack/react-query';
import {request} from '../../lib/api/client';
import type {components} from '../../lib/api/schema';
import {useWork} from '../work/store';
import {apiWsKey,mapTask,type ApiTask} from '../work/apiModel';
export function useRouteData(planId:string){
 const {state,mode,project}=useWork(),prefix=apiWsKey(project.id,state.currentUserId);
 const map=useQuery({queryKey:[...prefix,'executionMap',planId,'drafts'],enabled:mode==='api',queryFn:()=>request<components['schemas']['ExecutionMap']>(`/projects/${project.id}/execution-map?planId=${planId}&includeDrafts=true&includeReferences=true`)});
 const tasks=useQuery({queryKey:[...prefix,'routeTasks',planId],enabled:mode==='api',queryFn:async()=>{const items:ApiTask[]=[];let cursor='';do{const page=await request<{items:ApiTask[];nextCursor:string|null}>(`/projects/${project.id}/tasks?planId=${planId}&includeReferences=true&limit=100${cursor?'&cursor='+encodeURIComponent(cursor):''}`);items.push(...page.items);cursor=page.nextCursor??'';}while(cursor);return items;}});
 return {tasks:mode==='api'?(tasks.data??[]).map(t=>mapTask(t,project.id)):state.tasks.filter(t=>!t.discardedAt&&(t.planId===planId||state.plans.find(p=>p.id===planId)?.referenceTaskIds.includes(t.id))),map:map.data,pending:mode==='api'&&(map.isPending||tasks.isPending),error:mode==='api'&&(map.isError||tasks.isError),retry:()=>{void map.refetch();void tasks.refetch();}};
}
