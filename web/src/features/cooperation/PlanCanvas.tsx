import {taskReadingKey,copyTaskRecordReading} from '../chat/taskReadingPosition';
import {createPortal} from 'react-dom';
import {useEffect, useRef, useState} from 'react';
import {GitBranch, X, ArrowUpRight} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {UIWarning} from '../../components/ui/FormControls';
import {useWork} from '../work/store';
import {SurfaceNavigation} from '../work/SurfaceNavigation';
import {ExecutionMap, type GraphView} from '../chat/ExecutionCanvas';
import {useWorkspaceDraft} from '../chat/useWorkspaceDraft';
import {useDiscussionNavigation} from './useDiscussionNavigation';
import {useReturnFocus} from './useReturnFocus';
import {useRouteData} from './routeData';
import {TaskDrawer} from './TaskDrawer';
import type {Plan, Task, Route} from '../work/types';
import type {TaskSection} from '../work/runtimeTypes';

export function requestRouteFullscreen() {
  if (!document.fullscreenElement && document.fullscreenEnabled) void document.documentElement.requestFullscreen().catch(() => {});
}
export function PlanCanvas({plan, preview = false, focusTaskId, onClose}: {plan: Plan; preview?: boolean; focusTaskId?: string; onClose?: () => void}) {
  const p = useWork(), data = useRouteData(plan.id), nav = useDiscussionNavigation();
  const [full, setFull] = useState(preview), [local, setLocal] = useState<{id: string; section: TaskSection} | null>(focusTaskId ? {id: focusTaskId, section: 'overview'} : null);
  const [focus, setFocus] = useState(focusTaskId), drawerFocus = useReturnFocus(), viewerFocus = useReturnFocus();
  const [view, setView] = useWorkspaceDraft<GraphView>((preview ? 'plan-preview-graph:' : 'plan-graph:') + plan.id, {zoom: 1, left: 0, top: 0});
  const snapshot = useRef<{view: GraphView; readingTop: number; route: Pick<Route, 'view' | 'id' | 'taskSection' | 'activityId' | 'focusTaskId'>} | null>(null);
  const native = useRef(preview && !!document.fullscreenElement), closing = useRef(false), restoreReading = useRef<number | null>(null), active = useRef(full);
  active.current = full;
  const entryRoute = useRef(JSON.stringify(p.route));
  const selected = preview ? local : p.route.view === 'task' && p.route.id ? {id: p.route.id, section: p.route.activityId ? 'records' as const : p.route.taskSection ?? 'overview'} : null;
  const task = selected ? p.state.tasks.find(t => t.id === selected.id) ?? data.tasks.find(t => t.id === selected.id) : undefined;
  const selectTask = (id: string, section: TaskSection = 'overview') => {
    drawerFocus.capture('route-task-' + id, true);
    if (preview) setLocal({id, section});
    else p.go({page: 'route', scopePlanId: plan.id, view: 'task', id, taskSection: section, activityId: undefined});
  };
  const open = (t: Task, section: TaskSection = 'overview') => selectTask(t.id, section);
  const closeDrawer = () => {
    if (preview) setLocal(null);
    else p.go({view: 'plan', id: plan.id, taskSection: undefined, activityId: undefined});
    drawerFocus.restore([...(selected ? ['route-task-' + selected.id] : []), 'route-locate-' + plan.id]);
  };
  const enter = () => {
    viewerFocus.capture('route-fullscreen-' + plan.id);
    snapshot.current = {view: {...view}, readingTop: document.querySelector('.judex-plan-canvas .judex-task-scroll')?.scrollTop ?? 0, route: {view: p.route.view, id: p.route.id, taskSection: p.route.taskSection, activityId: p.route.activityId, focusTaskId: p.route.focusTaskId}};
    closing.current = false;
    if (selected) {
      const user = p.state.currentUserId ?? p.state.currentUser;
      copyTaskRecordReading(p.mode,p.project.id,user,selected.id,'task','fullscreen');
      const top = document.querySelector('.judex-task-scroll')?.scrollTop ?? 0;
      sessionStorage.setItem('judex.chat.reading.' + taskReadingKey(p.mode, p.project.id, user, selected.id, selected.section, 'fullscreen'), JSON.stringify({top}));
    }
    requestRouteFullscreen();
    setFull(true);
  };
  const exit = (restore = true) => {
    if (closing.current) return;
    closing.current = true;
    setFull(false);
    if (restore && snapshot.current) {
      setView(snapshot.current.view);
      restoreReading.current = snapshot.current.readingTop;
      if (p.route.page === 'route' && p.route.scopePlanId === plan.id) p.go(snapshot.current.route);
    }
    snapshot.current = null;
    if (document.fullscreenElement) void document.exitFullscreen().catch(() => {});
    if (preview) onClose?.();
    if (restore) viewerFocus.restore();
  };
  const leave = () => {if (full) exit(false);};
  const locatePlan = (t: Task) => {
    if (t.planId === plan.id) {
      open(t);
      setFocus(t.id);
    } else {
      leave();
      p.go({page: 'route', scopePlanId: t.planId ?? undefined, scopeTaskId: undefined, view: 'task', id: t.id, taskSection: 'overview', activityId: undefined, focusTaskId: t.id});
    }
    return true;
  };
  useEffect(() => {if (preview) viewerFocus.capture();}, []);
  useEffect(() => {
    if (full || restoreReading.current === null) return;
    const top = restoreReading.current;
    let next = 0;
    const first = requestAnimationFrame(() => {next = requestAnimationFrame(() => {
      const el = document.querySelector('.judex-plan-canvas .judex-task-scroll');
      if (el) {el.scrollTop = top; el.dispatchEvent(new Event('scroll'));}
      restoreReading.current = null;
    });});
    return () => {cancelAnimationFrame(first); cancelAnimationFrame(next);};
  }, [full]);
  const latest = useRef(exit);
  latest.current = exit;
  useEffect(() => {
    const change = () => {
      if (!active.current) return;
      if (document.fullscreenElement) native.current = true;
      else if (native.current) {
        native.current = false;
        if (!document.querySelector('[data-slot="modal-dialog"],[role="alertdialog"]')) latest.current();
      }
    };
    document.addEventListener('fullscreenchange', change);
    return () => document.removeEventListener('fullscreenchange', change);
  }, []);
  useEffect(() => {
    if (!full) return;
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !document.querySelector('[data-slot="modal-dialog"],[role="alertdialog"]')) {e.preventDefault(); latest.current();}
    };
    document.addEventListener('keydown', key);
    return () => document.removeEventListener('keydown', key);
  }, [full]);
  useEffect(() => {
    if (full && (preview ? JSON.stringify(p.route) !== entryRoute.current : p.route.page !== 'route' || p.route.scopePlanId !== plan.id || !!p.route.settingsSection)) exit(false);
  }, [p.route]);
  useEffect(() => () => {if (active.current && document.fullscreenElement) void document.exitFullscreen().catch(() => {});}, []);
  const body = <SurfaceNavigation beforeNavigate={leave} locatePlan={full ? locatePlan : undefined}>
    <div className={'judex-plan-canvas' + (full ? ' judex-route-fullscreen' : '')} data-testid={full ? 'route-fullscreen' : 'plan-canvas'}>
      {full && <header className="judex-route-fullscreen-header"><div><GitBranch/><h2>{p.text(plan.title)}</h2>{preview && <span className="judex-task-meta">{p.t('taskDetailsRoutePreview')}</span>}</div><div>
        {preview && <Button size="sm" variant="outline" onPress={() => {exit(false); p.go({page: 'route', scopePlanId: plan.id, scopeTaskId: undefined, view: 'plan', id: plan.id, activityId: undefined});}}>{p.t('taskDetailsEnterPlan')}<ArrowUpRight/></Button>}
        <Button size="sm" data-testid="exit-route-fullscreen" onPress={() => exit()}><X/>{p.t('taskDetailsFullscreenExit')}<kbd>Esc</kbd></Button>
      </div></header>}
      <div className={'judex-plan-canvas-body' + (selected ? ' judex-plan-canvas-drawer' : '')}>
        {data.pending ? <p role="status">{p.t('shellLoading')}</p> : data.error ? <UIWarning>{p.t('errNetwork')}<Button onPress={data.retry}>{p.t('shellRetry')}</Button></UIWarning> :
          <ExecutionMap planId={plan.id} tasks={data.tasks} model={data.map} onOpen={open} selectedTaskId={selected?.id} focusTaskId={focus} onExternal={id => selectTask(id)} onDiscuss={preview ? undefined : t => {leave(); void nav.task(t);}} busyTask={nav.busy} cooperation full={full} onFullscreen={enter} readOnly={preview} view={view} onView={setView}/>
        }
        {selected && <TaskDrawer taskId={selected.id} task={task} pending={p.dataPending} section={selected.section} onSection={section => preview ? setLocal({...selected, section}) : p.go({view: 'task', id: selected.id, taskSection: section, activityId: undefined})} onClose={closeDrawer} readOnly={preview} readingScope={preview ? 'preview' : full ? 'fullscreen' : 'task'} onTask={id => selectTask(id)}/>}
      </div>
    </div>
  </SurfaceNavigation>;
  return full ? createPortal(body, document.body) : body;
}
