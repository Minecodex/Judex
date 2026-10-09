import {useQuery} from '@tanstack/react-query';
import type {ReactNode} from 'react';
import {Button} from '../../components/ui/Button';
import {UIWarning} from '../../components/ui/FormControls';
import {request} from '../../lib/api/client';
import {useWork, WorkContext} from '../work/store';
import {apiWorkspaceQueries, apiWsKey, mapFlow, mapPlan, mapPosition, mapSeat, type ApiPlan} from '../work/apiModel';
import type {Task} from '../work/types';

// A list preview keeps the underlying route unchanged. Its inspector must load
// its own read context rather than infer missing identities/workflows from the
// hub's deliberately small data projection. Queries and mappers stay shared.
export function TaskInspectorData({task, children}: {task: Task; children: ReactNode}) {
  const p = useWork(), api = p.mode === 'api', q = apiWorkspaceQueries(p.project.id, p.state.currentUserId);
  const positions = useQuery({...q.positions, enabled: api});
  const identities = useQuery({...q.identities, enabled: api});
  const definitions = useQuery({...q.workflows, enabled: api && !!task.flowId});
  const versions = useQuery({...q.workflowVersions(task.flowId), enabled: api && !!task.flowId});
  const knownPlan=p.state.plans.find(v=>v.id===task.planId);
  const plan=useQuery({queryKey:[...apiWsKey(p.project.id,p.state.currentUserId),'plan',task.planId],enabled:api&&!!task.planId&&!knownPlan,queryFn:()=>request<ApiPlan>(`/projects/${p.project.id}/plans/${task.planId}`)});
  const queries = [positions, identities, ...(task.flowId ? [definitions, versions] : []), ...(task.planId&&!knownPlan?[plan]:[])];
  if (!api) return children;
  if (queries.some(v => v.isPending)) return <p role="status">{p.t('shellLoading')}</p>;
  if (queries.some(v => v.isError)) return <UIWarning>{p.t('errNetwork')}<Button onPress={() => queries.forEach(v => void v.refetch())}>{p.t('shellRetry')}</Button></UIWarning>;
  const definition = definitions.data?.find(v => v.id === task.flowId);
  const flow = definition ? mapFlow(definition, versions.data ?? [], p.project.id) : undefined;
  const state = {...p.state,
    plans:plan.data?[...p.state.plans.filter(v=>v.id!==plan.data!.id),mapPlan(plan.data,p.project.id)]:p.state.plans,
    tasks: [...p.state.tasks.filter(v => v.id !== task.id), task],
    positions: [...p.state.positions.filter(v => v.projectId !== p.project.id), ...(positions.data ?? []).filter(v => v.status === 'active').map((v, i) => mapPosition(v, p.project.id, i))],
    seats: [...p.state.seats.filter(v => !identities.data?.some(i => i.id === v.id)), ...(identities.data ?? []).filter(v => v.kind === 'position').map(v => mapSeat(v, positions.data ?? []))],
    flows: flow ? [...p.state.flows.filter(v => v.id !== flow.id), flow] : p.state.flows,
  };
  return <WorkContext.Provider value={{...p, state}}>{children}</WorkContext.Provider>;
}
