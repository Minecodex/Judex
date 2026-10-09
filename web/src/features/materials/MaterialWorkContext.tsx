import {useEffect, useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {useWork} from '../work/store';
import {request} from '../../lib/api/client';
import {useCollection, LoadMore} from '../../lib/api/collections';
import {FormField, UIInput, UISelect, UIOption, UIWarning} from '../../components/ui/FormControls';
import type {ApiPlan, ApiTask} from '../work/apiModel';

export function MaterialWorkContext({plan, task, setPlan, setTask, disabled}: {
  plan: string; task: string; setPlan: (id: string) => void; setTask: (id: string) => void; disabled: boolean;
}) {
  const p = useWork(), [search, setSearch] = useState('');
  const key = ['materials', p.project.id, p.state.currentUserId, 'work-choices'];
  const root = '/projects/' + p.project.id;
  const selectedTask = useQuery({queryKey: [...key, 'task', task], enabled: !!task,
    queryFn: () => request<ApiTask>(root + '/tasks/' + task)});
  const knownTask = p.state.tasks.find(v => v.id === task && v.projectId === p.project.id);
  const taskPlan = selectedTask.data ? selectedTask.data.planId : knownTask?.planId;
  const displayedPlan = task ? taskPlan === undefined ? plan : taskPlan ?? '' : plan;
  useEffect(() => {if (task && taskPlan !== undefined && (taskPlan ?? '') !== plan) setPlan(taskPlan ?? '');}, [task, taskPlan, plan]);
  const plans = useCollection<ApiPlan>([...key, 'plans', search], root + '/plans?q=' + encodeURIComponent(search));
  const tasks = useCollection<ApiTask>([...key, 'tasks', displayedPlan], root + '/tasks' + (displayedPlan ? '?planId=' + displayedPlan : ''));
  const selectedPlan = useQuery({queryKey: [...key, 'plan', displayedPlan], enabled: !!displayedPlan,
    queryFn: () => request<ApiPlan>(root + '/plans/' + displayedPlan)});
  const planItems = [...(plans.data?.items ?? [])];
  if (selectedPlan.data && !planItems.some(v => v.id === selectedPlan.data!.id)) planItems.unshift(selectedPlan.data);
  const taskItems = [...(tasks.data?.items ?? [])];
  if (selectedTask.data && !taskItems.some(v => v.id === selectedTask.data!.id)) taskItems.unshift(selectedTask.data);
  return <div className="judex-material-work-context">
    <FormField label={p.t('matPlan')}>
      <UIInput aria-label={p.t('matFindPlan')} placeholder={p.t('matFindPlan')} value={search} disabled={disabled} onChange={e => setSearch(e.target.value)}/>
      <UISelect aria-label={p.t('matPlan')} disabled={disabled} value={displayedPlan ?? ''} onChange={e => {setTask(''); setPlan(e.target.value);}}>
        <UIOption value="">{p.t('matNone')}</UIOption>{planItems.map(v => <UIOption key={v.id} value={v.id}>{v.title}</UIOption>)}
      </UISelect><LoadMore query={plans} label={p.t('matPlan')}/>
    </FormField>
    <FormField label={p.t('matTask')}><UISelect aria-label={p.t('matTask')} disabled={disabled} value={task} onChange={e => setTask(e.target.value)}>
      <UIOption value="">{p.t(displayedPlan ? 'matPlanOnly' : 'matNoTask')}</UIOption>{taskItems.map(v => <UIOption key={v.id} value={v.id}>{v.title}</UIOption>)}
    </UISelect><LoadMore query={tasks} label={p.t('matTask')}/></FormField>
    {task && taskPlan && <p className="judex-material-context-note">{p.t('matPlanResolved')}</p>}
    {(selectedTask.isError || selectedPlan.isError || plans.isError || tasks.isError) && <UIWarning>{p.t('matContextUnavailable')}</UIWarning>}
  </div>;
}
