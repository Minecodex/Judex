import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {GitBranch} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {useWork} from '../work/store';
import {useSurfaceNavigation} from '../work/SurfaceNavigation';
import {request} from '../../lib/api/client';
import {apiWsKey, mapPlan, type ApiPlan} from '../work/apiModel';
import {PlanCanvas, requestRouteFullscreen} from './PlanCanvas';
import type {Task} from '../work/types';

export function TaskPlanPreviewButton({task}: {task: Task}) {
  const p = useWork(), navigation = useSurfaceNavigation(), [preview, setPreview] = useState(false);
  const known = p.state.plans.find(v => v.id === task.planId);
  const query = useQuery({queryKey: [...apiWsKey(p.project.id, p.state.currentUserId), 'plan', task.planId], enabled: p.mode === 'api' && !!task.planId && !known, queryFn: () => request<ApiPlan>(`/projects/${p.project.id}/plans/${task.planId}`)});
  const plan = known ?? (query.data ? mapPlan(query.data, p.project.id) : undefined);
  if (!task.planId) return null;
  return <>
    <Button size="sm" variant="outline" data-focus-key={'locate-plan-' + task.id} data-testid="locate-task-plan" disabled={!plan} onPress={() => {
      if (navigation.locatePlan?.(task)) return;
      requestRouteFullscreen();
      setPreview(true);
    }}><GitBranch/>{p.t('taskDetailsLocatePlan')}</Button>
    {query.isError && <Button size="sm" onPress={() => void query.refetch()}>{p.t('shellRetry')}</Button>}
    {preview && plan && <PlanCanvas plan={plan} preview focusTaskId={task.id} onClose={() => setPreview(false)}/>}
  </>;
}
