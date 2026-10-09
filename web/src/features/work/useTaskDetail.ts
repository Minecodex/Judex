import {useQuery} from '@tanstack/react-query';
import {useWork} from './store';
import {APIError} from '../../lib/api/client';
import {apiWorkspaceQueries, mapTask} from './apiModel';
import type {Task} from './types';

// Selection is identified by object id. A warm route cache is a display
// optimization, not a prerequisite for reading another task in the project.
export function useTaskDetail(id?: string, initial?: Task) {
  const p = useWork(), api = p.mode === 'api';
  const query = useQuery({...apiWorkspaceQueries(p.project.id, p.state.currentUserId).taskDetail(id ?? ''), enabled: api && !!id,
    retry:(attempt,error)=>!(error instanceof APIError&&[403,404].includes(error.status))&&attempt<2});
  const known = initial && initial.id === id && initial.projectId === p.project.id ? initial : p.state.tasks.find(v => v.id === id && v.projectId === p.project.id);
  const task = api && query.data ? mapTask(query.data, p.project.id) : known;
  return {task: api && query.isError ? undefined : task, pending: api && !!id && !task && query.isPending, failed: api && query.isError,
    unavailable:query.error instanceof APIError&&[403,404].includes(query.error.status),retry: () => void query.refetch()};
}
