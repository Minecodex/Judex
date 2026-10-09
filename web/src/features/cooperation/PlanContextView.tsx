import {useState} from 'react';
import {Check,GitBranch,ChevronRight} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {UIStatus} from '../../components/ui/FormControls';
import {useWork} from '../work/store';
import {Person} from '../work/ui';
import {taskDisplayStatus} from '../work/runtime';
import {taskStatusKey} from '../chat/collaborationPresentation';
import {planStats} from './queries';
import {RoutePreview} from './PlanRoute';
import {requestRouteFullscreen} from './PlanCanvas';
import type {Plan} from '../work/types';
export function PlanContext({plan}:{plan:Plan}){
 const p=useWork(),stats=planStats(plan,p.state),tasks=p.state.tasks.filter(t=>!t.discardedAt&&(t.planId===plan.id||plan.referenceTaskIds.includes(t.id))),[preview,setPreview]=useState(false),required=stats.required??stats.total;
 return <section className="judex-plan-context"><div className="judex-task-section-heading"><h2>{p.text(plan.title)}</h2><UIStatus>{p.t(plan.businessStatus==='cancelled'?'coopCancel':plan.status==='draft'?'coDraft':plan.status==='accepted'?'coAccepted':'coopActive')}</UIStatus></div><p>{p.text(plan.goal)}</p><div className="judex-task-header-meta"><span className="judex-task-meta">{p.t('coopOwner')}</span><Person name={plan.ownerName} seatId={plan.ownerSeatId} small/></div><section><h3>{p.t('workCriteria')}</h3><ul className="judex-task-check-list">{plan.criteria.map((c,i)=><li key={i}><Check/>{p.text(c)}</li>)}</ul></section><div className="judex-task-section-heading"><strong>{plan.status==='draft'?p.t('taskDetailsDraftCount',{count:stats.draft??0}):required>0?p.t('taskDetailsProgress',{accepted:stats.accepted,required}):p.t('taskDetailsEmptyFormal')}</strong><Button size="sm" onPress={()=>{requestRouteFullscreen();setPreview(true);}}><GitBranch/>{p.t('coopPreviewRoute')}</Button></div>{(stats.skipped??0)>0&&<p className="judex-task-meta">{p.t('taskDetailsSkippedCount',{count:stats.skipped!})}</p>}<section><h3>{p.t('taskDetailsPlanTasks')}</h3><div className="judex-plan-context-tasks">{tasks.map(task=>{const status=taskDisplayStatus(task,p.state);return <Button key={task.id} className="judex-co-context-task" variant="ghost" onPress={()=>p.go({view:'task',id:task.id,taskSection:'overview',activityId:undefined})}><div><strong>{p.text(task.title)}</strong><span>{task.participantNames?.join(' · ')||task.seatIds.map(id=>p.state.seats.find(v=>v.id===id)?.person).filter(Boolean).join(' · ')}</span></div><UIStatus className={'judex-task-status judex-task-status-'+status}>{p.t(status==='skipped'?'taskDetailsSkipped':taskStatusKey(status))}</UIStatus><ChevronRight/></Button>;})}</div></section>{preview&&<RoutePreview plan={plan} onClose={()=>setPreview(false)}/>}</section>;
}
