import {useEffect, useState} from 'react';
import {useQueries, useQuery} from '@tanstack/react-query';
import {useWork} from '../work/store';
import {request} from '../../lib/api/client';
import {useCollection, LoadMore} from '../../lib/api/collections';
import {FormField, UIInput, UIOption, UISelect, UIWarning} from '../../components/ui/FormControls';
import {UIDialog} from '../../components/ui/Presentation';
import {Button} from '../../components/ui/Button';
import {mapTopic, type ApiTopic, type ApiPlan, type ApiTask} from '../work/apiModel';
import type {MaterialItem} from './api';
import {storedConversationContext} from '../chat/conversationDraft';

export function ShareMaterialDialog({file, onClose, onStage}: {
  file: MaterialItem; onClose: () => void; onStage: (target: {topicId: string; taskId?: string; planId?: string}) => void;
}) {
  const p = useWork(), conversation = p.route.conversation ?? (p.route.view === 'topic' ? p.route.id : undefined);
  const currentTopic = conversation?.startsWith('handoff:') ? undefined : conversation;
  const [topicId, setTopicId] = useState(currentTopic ?? ''), [search, setSearch] = useState('');
  const [context, setContext] = useState(''), [edited, setEdited] = useState(false);
  const root = '/projects/' + p.project.id, key = ['materials', p.project.id, p.state.currentUserId, 'share-choices'];
  const topics = useCollection<ApiTopic>([...key, 'topics', search], root + '/topics?q=' + encodeURIComponent(search));
  const selected = useQuery({queryKey: [...key, 'topic', topicId], enabled: !!topicId,
    queryFn: () => request<ApiTopic>(root + '/topics/' + topicId)});
  const topic = selected.data ? mapTopic(selected.data, p.project.id, []) : undefined;
  const linkedPlans = [...new Set([...(topic?.planIds ?? []), ...(selected.data?.contextType === 'plan' && selected.data.contextId ? [selected.data.contextId] : [])])];
  const linkedTasks = [...new Set([...(topic?.taskIds ?? []), ...(selected.data?.contextType === 'task' && selected.data.contextId ? [selected.data.contextId] : [])])];
  const taskPages = useCollection<ApiTask>([...key, 'tasks'], root + '/tasks');
  const taskDetails = useQueries({queries: linkedTasks.map(id => ({queryKey: [...key, 'task', id], queryFn: () => request<ApiTask>(root + '/tasks/' + id)}))});
  const planDetails = useQueries({queries: linkedPlans.map(id => ({queryKey: [...key, 'plan', id], queryFn: () => request<ApiPlan>(root + '/plans/' + id)}))});
  const tasks = new Map((taskPages.data?.items ?? []).map(v => [v.id, v]));
  for (const row of taskDetails) if (row.data) tasks.set(row.data.id, row.data);
  const allowedTasks = [...tasks.values()].filter(v => linkedTasks.includes(v.id) || !!v.planId && linkedPlans.includes(v.planId));
  useEffect(() => {
    if (edited || !selected.data) return;
    const saved=storedConversationContext(p.project.id+':'+(p.state.currentUserId??p.state.currentUser)+':'+topicId);
    const task = topicId === currentTopic ? p.route.taskContextId ?? p.route.scopeTaskId : undefined;
    const plan = topicId === currentTopic ? p.route.scopePlanId : undefined;
    setContext(saved ? saved.kind==='topic'?'':saved.kind+':'+saved.id : task ? 'task:' + task : plan ? 'plan:' + plan : selected.data.contextType === 'task' && selected.data.contextId ? 'task:' + selected.data.contextId : selected.data.contextType === 'plan' && selected.data.contextId ? 'plan:' + selected.data.contextId : '');
  }, [topicId, selected.data, edited]);
  const topicItems = [...(topics.data?.items ?? [])].filter(v => v.kind !== 'handoff');
  if (selected.data && !topicItems.some(v => v.id === selected.data!.id)) topicItems.unshift(selected.data);
  const validContext = !context || context.startsWith('task:') && allowedTasks.some(v => 'task:' + v.id === context) || context.startsWith('plan:') && linkedPlans.some(id => 'plan:' + id === context);
  return <UIDialog title={p.t('matSend')} description={p.t('matShareDraftHint')} onClose={onClose} footer={<>
    <Button variant="outline" onPress={onClose}>{p.t('matCancel')}</Button><Button variant="primary" disabled={!topicId || !selected.data || !validContext || selected.isError} onPress={() => onStage({topicId, ...(context.startsWith('task:') ? {taskId: context.slice(5)} : context.startsWith('plan:') ? {planId: context.slice(5)} : {})})}>{p.t('matChooseConfirm')}</Button>
  </>}><strong>{file.title} · v{file.revision}</strong>
    <FormField label={p.t('matTargetDiscussion')}><UIInput aria-label={p.t('matFindDiscussion')} placeholder={p.t('matFindDiscussion')} value={search} onChange={e => setSearch(e.target.value)}/>
      <UISelect aria-label={p.t('matTargetDiscussion')} value={topicId} onChange={e => {setTopicId(e.target.value);setContext('');setEdited(false);}}><UIOption value="">{p.t('matChooseDiscussion')}</UIOption>{topicItems.map(v => <UIOption key={v.id} value={v.id}>{v.title}</UIOption>)}</UISelect><LoadMore query={topics}/>
    </FormField>
    <FormField label={p.t('matWorkScope')}><UISelect aria-label={p.t('matWorkScope')} value={context} onChange={e => {setContext(e.target.value);setEdited(true);}}>
      <UIOption value="">{p.t('coopWholeConversation')}</UIOption>{planDetails.flatMap(q => q.data ? [<UIOption key={q.data.id} value={'plan:' + q.data.id}>{p.t('matPlan')} · {q.data.title}</UIOption>] : [])}{allowedTasks.map(v => <UIOption key={v.id} value={'task:' + v.id}>{p.t('matTask')} · {v.title}</UIOption>)}
    </UISelect><LoadMore query={taskPages} label={p.t('matTask')}/></FormField>
    {(selected.isError || topics.isError || taskDetails.some(q => q.isError) || planDetails.some(q => q.isError)) && <UIWarning>{p.t('matContextUnavailable')}</UIWarning>}
  </UIDialog>;
}
