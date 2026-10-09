import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {ArrowLeft,Check,MessageCircle,Plus} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {UIWarning,UIStatus,UIDisclosure} from '../../components/ui/FormControls';
import {request} from '../../lib/api/client';
import {useWork} from '../work/store';
import {apiWsKey,mapPlan,type ApiPlan} from '../work/apiModel';
import {AcceptanceDialog} from '../work/Dialogs';
import {Dialog,Person} from '../work/ui';
import {WorkMutationDialog,type WorkMutation} from '../work/WorkMutationDialog';
import {canOwnPlan,needsReview} from '../work/selectors';
import {PlanMore} from '../chat/TaskActions';
import {PlanCanvas,requestRouteFullscreen} from './PlanCanvas';
import {planStats} from './queries';
import {useDiscussionNavigation} from './useDiscussionNavigation';
import {useReadingPosition} from '../chat/useReadingPosition';
import type {Plan} from '../work/types';
export function PlanRoute(){
 const p=useWork(),plan=p.state.plans.find(v=>v.id===p.route.scopePlanId),nav=useDiscussionNavigation(),[accepting,setAccepting]=useState(false),[mutation,setMutation]=useState<WorkMutation>(),reading=useReadingPosition('route:'+(p.state.currentUserId??p.state.currentUser)+':'+p.route.scopePlanId);
 const stats=plan?planStats(plan,p.state):undefined;
 return <div className="judex-co-scroll" ref={reading}><main className="judex-co-main judex-co-route-main"><Button variant="ghost" className="judex-co-back" onPress={()=>p.go({page:'hub',hubTab:'plans',view:'plans',id:undefined,scopePlanId:undefined,scopeTaskId:undefined,taskSection:undefined,activityId:undefined})}><ArrowLeft/>{p.t('coopBackHub')}</Button>{p.dataPending?<p role="status">{p.t('shellLoading')}</p>:p.dataFailed||!plan?<UIWarning>{p.t('errNetwork')}</UIWarning>:<>
 <div className="judex-co-page-heading"><div><div><h1>{p.text(plan.title)}</h1><p>{p.text(plan.goal)}</p></div><div className="judex-co-actions"><UIStatus>{p.t(plan.businessStatus==='cancelled'?'coopCancel':plan.status==='draft'?'coDraft':plan.status==='accepted'?'coAccepted':'coopActive')}</UIStatus><Button variant="outline" onPress={()=>nav.plan(plan)} disabled={!plan.mainTopicId}><MessageCircle/>{p.t('coopDiscuss')}</Button><Button variant="primary" disabled={!!plan.discardedAt||['accepted','cancelled'].includes(plan.businessStatus??plan.status)} data-testid="new-plan-task" onPress={()=>p.go({editor:'task'})}><Plus/>{p.t('workNewTask')}</Button><PlanMore plan={plan} onMutation={setMutation}/></div></div></div>
 <div className="judex-route-summary"><div className="judex-route-summary-owner"><span className="judex-task-meta">{p.t('coopOwner')}</span><Person name={plan.ownerName} seatId={plan.ownerSeatId} small/></div><div className="judex-route-summary-criteria"><span className="judex-task-meta">{p.t('workCriteria')}</span>{plan.criteria[0]&&<span><Check/>{p.text(plan.criteria[0])}</span>}{plan.criteria.length>1&&<UIDisclosure title={p.t('taskDetailsAdditionalCriteria',{count:plan.criteria.length-1})}><ul className="judex-task-check-list">{plan.criteria.slice(1).map((v,i)=><li key={i}><Check/>{p.text(v)}</li>)}</ul></UIDisclosure>}</div><div className="judex-route-summary-progress"><span>{plan.status==='draft'?p.t('taskDetailsDraftCount',{count:stats?.draft??0}):(stats?.required??stats?.total??0)>0?p.t('taskDetailsProgress',{accepted:stats?.accepted??0,required:stats?.required??stats?.total??0}):p.t('taskDetailsEmptyFormal')}</span>{(stats?.skipped??0)>0&&<small>{p.t('taskDetailsSkippedCount',{count:stats!.skipped!})}</small>}{plan.status!=='draft'&&(stats?.draft??0)>0&&<small>{p.t('taskDetailsDraftCount',{count:stats!.draft!})}</small>}</div><div className="judex-route-summary-operation">{plan.status==='draft'&&canOwnPlan(p.state,plan)&&<Button size="sm" data-testid="activate-plan" variant="outline" onPress={()=>void p.act('planAction',{planId:plan.id,op:'activate'})}>{p.t('workActivate')}</Button>}{plan.status==='active'&&canOwnPlan(p.state,plan)&&<Button size="sm" variant="outline" data-testid="accept-plan" onPress={()=>setAccepting(true)}>{p.t('workPlanAccept')}</Button>}</div></div>
 {needsReview(p.state,plan)&&<UIWarning data-testid="plan-reopened-warning">{p.t('workReopenedPlan')}{canOwnPlan(p.state,plan)&&<Button data-testid="resume-plan" onPress={()=>void p.act('planAction',{planId:plan.id,op:'resume'})}>{p.t('workPlanResume')}</Button>}</UIWarning>}
 <PlanCanvas plan={plan}/>{accepting&&<AcceptanceDialog plan={plan} onClose={()=>setAccepting(false)}/>}{mutation&&<WorkMutationDialog mutation={mutation} onClose={()=>setMutation(undefined)}/>}</>}</main></div>;
}
export function RoutePreview({plan,onClose}:{plan:Plan;onClose:()=>void}){return <PlanCanvas plan={plan} preview onClose={onClose}/>;}
export function RemotePlanDecision({id,onClose}:{id:string;onClose:()=>void}){const p=useWork(),query=useQuery({queryKey:[...apiWsKey(p.project.id,p.state.currentUserId),'plan',id],enabled:p.mode==='api',queryFn:()=>request<ApiPlan>(`/projects/${p.project.id}/plans/${id}`)});return query.data?<AcceptanceDialog plan={mapPlan(query.data,p.project.id)} onClose={onClose}/>:<Dialog title={p.t('coopPlanAcceptance')} onClose={onClose}>{query.isError?<UIWarning>{p.t('errNetwork')}</UIWarning>:<p role="status">{p.t('shellLoading')}</p>}</Dialog>;}
