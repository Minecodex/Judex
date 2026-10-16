import {useEffect, useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Button} from '../../components/ui/Button';
import {UIInput, UIOption, UISelect, UIWarning} from '../../components/ui/FormControls';
import {useCollection, LoadMore} from '../../lib/api/collections';
import {request, requestAll} from '../../lib/api/client';
import type {components} from '../../lib/api/schema';
import type {MaterialItem} from '../materials/api';
import {useWork} from './store';
import {apiWsKey, type ApiTask, type ApiHandoff} from './apiModel';
import type {FlowBody} from './types';

type Kind = NonNullable<FlowBody['hardRules']>[number]['kind'];
type Choice = {id: string; label: string; materialId?: string};
type Row = ApiTask | ApiHandoff | MaterialItem;

// Object choices use the same scoped, paged APIs as their directory pages.
// Retain an existing fixed reference independently of the current search/page.
export function WorkflowTargetPicker({kind, value, onChange, disabled, testId}: {
  kind: Kind; value: string; onChange: (id: string) => void; disabled: boolean; testId: string;
}) {
  const {project, state, mode, t, text} = useWork();
  const [search, setSearch] = useState(''), [querySearch, setQuerySearch] = useState('');
  useEffect(() => {const timer = setTimeout(() => setQuerySearch(search.trim()), 200); return () => clearTimeout(timer);}, [search]);
  const base = `/projects/${project.id}`, prefix = [...apiWsKey(project.id, state.currentUserId), 'workflowTargets', kind];
  const resource = kind === 'task_acceptance' ? 'tasks' : kind === 'handoff_receipt' ? 'handoffs' : 'materials';
  // Handoffs currently have no server-side text filter; search the loaded pages.
  const url = base + '/' + resource + (kind === 'handoff_receipt' ? '' : '?q=' + encodeURIComponent(querySearch));
  const query = useCollection<Row>([...prefix, kind === 'handoff_receipt' ? '' : querySearch], url, mode === 'api');
  const toChoices = (rows: Row[]): Choice[] => rows.flatMap(row => {
    if (kind === 'task_acceptance') return [{id: row.id, label: row.title ?? row.id}];
    if (kind === 'material_ready') {
      const file = row as MaterialItem;
      return file.originalStatus === 'ready' ? [{id: file.versionId, label: `${file.title} · v${file.revision}`, materialId: file.id}] : [];
    }
    return (row as ApiHandoff).sources.map(source => ({id: source.id,
      label: `${row.title} · ${state.tasks.find(task => task.id === source.sourceTaskId) ? text(state.tasks.find(task => task.id === source.sourceTaskId)!.title) : source.sourceTaskId}`}));
  });
  const selected = useQuery({queryKey: [...prefix, 'selected', value], enabled: mode === 'api' && !!value,
    queryFn: async ({signal}): Promise<Choice | null> => {
      if (kind === 'task_acceptance') {
        const task = await request<ApiTask>(`${base}/tasks/${value}`, {signal});
        return {id: task.id, label: task.title ?? task.id};
      }
      if (kind === 'material_ready') {
        const {library: file} = await request<{library: MaterialItem}>(`${base}/material-versions/${value}`, {signal});
        return {id: value, label: `${file.title} · v${file.revision}`, materialId: file.id};
      }
      const rows = await requestAll<ApiHandoff>(`${base}/handoffs`, {signal});
      return toChoices(rows).find(choice => choice.id === value) ?? null;
    }});
  const demo: Choice[] = kind === 'task_acceptance' ? state.tasks.filter(row => row.projectId === project.id).map(row => ({id: row.id, label: text(row.title)})) :
    kind === 'handoff_receipt' ? state.handoffs.filter(row => row.projectId === project.id).flatMap(row => row.sources.map(source => ({id: source.id, label: text(row.title)}))) :
    state.tasks.filter(row => row.projectId === project.id).flatMap(row => row.files.filter(file => !!file.versionId).map(file => ({id: file.versionId!, label: file.name})));
  const choices: Choice[] = mode === 'api' ? toChoices(query.data?.items ?? []) : demo;
  const filtered = choices.filter(row => row.label.toLowerCase().includes(querySearch.toLowerCase()));
  const items = [...new Map([...(selected.data ? [selected.data] : []), ...filtered].map(row => [row.id, row])).values()];
  const unavailable = !!value && mode === 'api' && (selected.isError || selected.isSuccess && !selected.data);
  const chosen = selected.data ?? items.find(row => row.id === value);
  return <div className="judex-workflow-target-picker">
    <UIInput value={search} disabled={disabled} aria-label={t('workflowHardRuleSearch')} placeholder={t('workflowHardRuleSearch')}
      data-testid={testId + '-search'} onChange={event => setSearch(event.target.value)}/>
    <UISelect value={value} disabled={disabled} aria-label={t('workflowHardRuleTarget')} data-testid={testId} onChange={event => onChange(event.target.value)}>
      <UIOption value="">{t('workflowHardRuleTargetHint')}</UIOption>
      {value && !items.some(row => row.id === value) && <UIOption value={value} disabled>{t('workflowHardRuleUnavailable')}</UIOption>}
      {items.map(row => <UIOption key={row.id} value={row.id}>{row.label}</UIOption>)}
    </UISelect>
    {mode === 'api' && query.isPending && <p role="status">{t('shellLoading')}</p>}
    {mode === 'api' && query.isError && <UIWarning role="alert">{t('errNetwork')}<Button size="sm" onPress={() => void query.refetch()}>{t('shellRetry')}</Button></UIWarning>}
    {unavailable && <UIWarning role="alert">{t('workflowHardRuleUnavailable')}<Button size="sm" onPress={() => void selected.refetch()}>{t('shellRetry')}</Button></UIWarning>}
    {mode === 'api' && <LoadMore query={query}/>}
    {mode === 'api' && kind === 'material_ready' && chosen?.materialId && <MaterialVersions key={chosen.materialId} material={chosen.materialId} value={value} onChange={onChange} disabled={disabled} testId={testId + '-version'}/>}
  </div>;
}

function MaterialVersions({material, value, onChange, disabled, testId}: {material: string; value: string; onChange: (id: string) => void; disabled: boolean; testId: string}) {
  const {project, state, t} = useWork();
  const query = useCollection<components['schemas']['MaterialVersion']>([...apiWsKey(project.id, state.currentUserId), 'materialVersions', material], `/projects/${project.id}/materials/${material}/versions`);
  return <>
    <UISelect aria-label={t('workflowHardRuleVersion')} value={value} disabled={disabled} data-testid={testId} onChange={event => onChange(event.target.value)}>
      {!query.data?.items.some(row => row.id === value) && <UIOption value={value}>{t('workflowHardRuleVersion')}</UIOption>}
      {(query.data?.items ?? []).filter(row => row.state === 'ready').map(row => <UIOption key={row.id} value={row.id}>{`v${row.revision}`}</UIOption>)}
    </UISelect>
    {query.isError && <UIWarning role="alert">{t('errNetwork')}<Button size="sm" onPress={() => void query.refetch()}>{t('shellRetry')}</Button></UIWarning>}
    <LoadMore query={query}/>
  </>;
}
