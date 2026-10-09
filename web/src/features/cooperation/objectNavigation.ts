import {useWork} from '../work/store';
import {useSurfaceNavigation} from '../work/SurfaceNavigation';
import type {Route} from '../work/types';
export function workObjectRoute(current: Route, type: 'task' | 'plan', id: string): Partial<Route> {
  if (current.page === 'chat') return {view: type, id, taskSection: type === 'task' ? 'overview' : undefined, activityId: undefined};
  return type === 'plan'
    ? {page: 'route', view: 'plan', id, scopePlanId: id, scopeTaskId: undefined}
    : {page: 'hub', hubTab: 'deliveries', view: 'task', id, scopePlanId: undefined, scopeTaskId: undefined, taskSection: 'overview', activityId: undefined};
}
export function useWorkObjectNavigation() {
  const {route} = useWork(), {go} = useSurfaceNavigation();
  return (type: 'task' | 'plan', id: string) => go(workObjectRoute(route, type, id));
}
