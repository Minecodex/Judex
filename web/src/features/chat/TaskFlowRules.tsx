import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {ArrowUpRight} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {useWork} from '../work/store';
import {useSurfaceNavigation} from '../work/SurfaceNavigation';
import {useWorkObjectNavigation} from '../cooperation/objectNavigation';
import {apiWorkspaceQueries} from '../work/apiModel';
import {useMaterialVersion} from '../materials/api';
import {MaterialDialog} from '../materials/MaterialDialog';
import type {FlowBody, Task} from '../work/types';

type Rule = NonNullable<FlowBody['hardRules']>[number];
export function TaskFlowRules({task, rules}: {task: Task; rules: Rule[]}) {
  const {t} = useWork();
  if (!rules.length) return null;
  return <section><h3>{t('taskDetailsWorkflowRules')}</h3><div className="judex-task-flow-rules">
    {rules.map((rule, i) => <div key={i} className="judex-task-flow-rule" data-rule-kind={rule.kind} data-rule-target={rule.targetId}>
      <div className="judex-task-section-heading"><strong>{t(rule.kind === 'material_ready' ? 'coCheckMaterial' : rule.kind === 'task_acceptance' ? 'coCheckTask' : 'coCheckHandoff')}</strong><span className="judex-task-meta">{t(rule.phase === 'start' ? 'chatBeforeStart' : rule.phase === 'accept' ? 'chatBeforeAccept' : rule.phase === 'both' ? 'taskDetailsBothPhases' : 'taskDetailsPhaseUnspecified')}</span></div>
      {!rule.targetId ? <Unavailable id=""/> : rule.kind === 'task_acceptance' ? <TaskTarget target={rule.targetId}/> : rule.kind === 'material_ready' ? <MaterialTarget target={rule.targetId} task={task}/> : <HandoffTarget target={rule.targetId}/>}
    </div>)}
  </div></section>;
}
function Unavailable({id, pending, retry}: {id: string; pending?: boolean; retry?: () => void}) {
  const {t} = useWork();
  return <div className="judex-task-rule-target"><p className="judex-task-meta" role={pending ? 'status' : undefined}>{t(pending ? 'shellLoading' : 'taskDetailsReferenceUnavailable')}</p><small className="judex-task-reference-id">{id}</small>{retry && <Button size="sm" onPress={retry}>{t('shellRetry')}</Button>}</div>;
}
function TaskTarget({target}: {target: string}) {
  const p = useWork(), openObject = useWorkObjectNavigation(), cached = p.state.tasks.find(v => v.id === target);
  const query = useQuery({...apiWorkspaceQueries(p.project.id, p.state.currentUserId).taskDetail(target), enabled: p.mode === 'api' && !cached});
  const title = cached ? p.text(cached.title) : query.data?.title;
  if (!title) return <Unavailable id={target} pending={p.mode === 'api' && query.isPending} retry={query.isError ? () => void query.refetch() : undefined}/>;
  return <div className="judex-task-rule-target"><p>{title}</p><Button size="sm" isIconOnly aria-label={p.t('details') + ' · ' + title} onPress={() => openObject('task', target)}><ArrowUpRight/></Button></div>;
}
function MaterialTarget({target, task}: {target: string; task: Task}) {
  const p = useWork(), query = useMaterialVersion(target), [open, setOpen] = useState(false), file = query.data;
  const demo = p.mode === 'demo' ? task.files.find(v => (v.versionId ?? v.id) === target) : undefined;
  if (!file && !demo) return <Unavailable id={target} pending={p.mode === 'api' && query.isPending} retry={query.isError ? () => void query.refetch() : undefined}/>;
  return <><div className="judex-task-rule-target"><div><p>{file?.title ?? demo?.name}</p><small className="judex-task-meta">{p.t('taskDetailsFixedMaterial')}{file ? ' · v' + file.revision : ''}{file?.deletedAt ? ' · ' + p.t('matDeleted') : ''}</small></div>{file && <Button size="sm" isIconOnly aria-label={p.t('matOrigin') + ' · ' + file.title} onPress={() => setOpen(true)}><ArrowUpRight/></Button>}</div>
    {open && file && <MaterialDialog file={file} initial="details" onClose={() => setOpen(false)}/>}
  </>;
}
function HandoffTarget({target}: {target: string}) {
  const p = useWork(), {go} = useSurfaceNavigation();
  const query = useQuery({...apiWorkspaceQueries(p.project.id, p.state.currentUserId).handoffs, enabled: p.mode === 'api'});
  const raw = query.data?.find(v => v.sources.some(s => s.id === target)), source = raw?.sources.find(s => s.id === target);
  const demo = p.mode === 'demo' ? p.state.handoffs.find(v => v.sources.some(s => s.id === target)) : undefined, demoSource = demo?.sources.find(v => v.id === target);
  const taskId = source?.sourceTaskId ?? demoSource?.taskId ?? '', knownTask = p.state.tasks.find(v => v.id === taskId);
  const taskQuery = useQuery({...apiWorkspaceQueries(p.project.id, p.state.currentUserId).taskDetail(taskId), enabled: p.mode === 'api' && !!taskId && !knownTask});
  const handoffId = raw?.id ?? demo?.id, title = raw?.title ?? (demo ? p.text(demo.title) : undefined);
  if (!handoffId || !title || (!source && !demoSource)) return <Unavailable id={target} pending={p.mode === 'api' && query.isPending} retry={query.isError ? () => void query.refetch() : undefined}/>;
  const revision = source?.currentVersion ?? demoSource?.revision, state = source?.state ?? demoSource!.status;
  const taskTitle = knownTask ? p.text(knownTask.title) : taskQuery.data?.title;
  const sender = source?.senderDisplayName ?? p.state.seats.find(v => v.id === demoSource?.senderSeatId)?.person;
  const receiver = raw?.receiverDisplayName ?? p.state.seats.find(v => v.id === demo?.receiverSeatId)?.person;
  return <div className="judex-task-rule-target"><div><p>{title}</p>{taskTitle && <p className="judex-task-meta">{taskTitle}</p>}{(sender || receiver) && <p className="judex-task-meta">{sender ?? p.t('matUnrecorded')} → {receiver ?? p.t('matUnrecorded')}</p>}<small className="judex-task-meta">{revision ? p.t('workSourceVersion', {version: revision}) + ' · ' : ''}{p.t(state === 'accepted' ? 'workSourceReceived' : state === 'rejected' ? 'workStatusRejected' : state === 'draft' ? 'workDraftSource' : 'workStatusPending')}</small></div><Button size="sm" isIconOnly aria-label={p.t('details') + ' · ' + title} onPress={() => go({page: 'hub', hubTab: 'deliveries', view: 'handoff', id: handoffId, sourceId: target, scopePlanId: undefined, scopeTaskId: undefined})}><ArrowUpRight/></Button></div>;
}
