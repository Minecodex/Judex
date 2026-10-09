import {useState} from 'react';
import {Card} from '@heroui/react';
import {GitBranch,MessageCircle} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {UIStatus} from '../../components/ui/FormControls';
import {useWork} from '../work/store';
import {Person} from '../work/ui';
import {WorkMutationDialog,type WorkMutation} from '../work/WorkMutationDialog';
import {PlanMore} from '../chat/TaskActions';
import {planStats} from './queries';
import {useDiscussionNavigation} from './useDiscussionNavigation';
import type {Plan} from '../work/types';
export function CooperationPlanCard({plan,preview}:{plan:Plan;preview:()=>void}){
 const p=useWork(),nav=useDiscussionNavigation(),stats=planStats(plan,p.state),status=plan.businessStatus??plan.status,[mutation,setMutation]=useState<WorkMutation>();
 const mine=plan.myTaskCount??p.state.tasks.filter(t=>t.planId===plan.id&&!t.discardedAt&&!t.executionException&&t.seatIds.some(id=>p.state.seats.some(s=>s.id===id&&(p.state.currentUserId?s.userId===p.state.currentUserId:s.person===p.state.currentUser)))&&!['accepted','cancelled'].includes(t.businessStatus??t.status)).length;
 const required=stats.required??stats.total;
 return <><Card className="judex-co-plan-card judex-plan-card" data-testid={'plan-card-'+plan.id}><Button className="judex-co-card-hit" variant="ghost" aria-label={p.t('coopRoute')+' · '+p.text(plan.title)} onPress={()=>p.go({page:'route',scopePlanId:plan.id,scopeTaskId:undefined,view:'plan',id:plan.id,taskSection:undefined,activityId:undefined})}/><Card.Header><div className="judex-co-card-top"><UIStatus>{p.t(status==='draft'?'coDraft':status==='accepted'?'coAccepted':status==='cancelled'?'coopCancel':'coopActive')}</UIStatus><div className="judex-plan-card-menu"><PlanMore plan={plan} onMutation={setMutation}/></div></div><Card.Title>{p.text(plan.title)}</Card.Title><Card.Description>{p.text(plan.goal)}</Card.Description></Card.Header><Card.Content><div className="judex-co-meta"><span>{p.t('coopOwner')}</span><Person name={plan.ownerName} seatId={plan.ownerSeatId} small/></div>{status!=='draft'&&required>0&&<div className="judex-co-progress"><span style={{width:(stats.accepted/required*100)+'%'}}/></div>}<div className="judex-co-card-top judex-co-meta"><span>{status==='draft'?p.t('taskDetailsDraftCount',{count:stats.draft??0}):required>0?p.t('taskDetailsProgress',{accepted:stats.accepted,required}):p.t('taskDetailsEmptyFormal')}</span><span>{stats.total} {p.t('workTasks')}</span></div>{(stats.skipped??0)>0&&<p className="judex-task-meta">{p.t('taskDetailsSkippedCount',{count:stats.skipped!})}</p>}{status!=='draft'&&(stats.draft??0)>0&&<p className="judex-task-meta">{p.t('taskDetailsDraftCount',{count:stats.draft!})}</p>}{mine>0&&<p className="judex-co-my-work">{p.t('coopMyTasks',{count:mine})}</p>}</Card.Content><Card.Footer className="judex-co-card-footer"><Button size="sm" variant="ghost" data-testid={'plan-preview-'+plan.id} onPress={preview}><GitBranch/>{p.t('coopPreviewRoute')}</Button><Button size="sm" variant="outline" disabled={!plan.mainTopicId} data-testid={'plan-discuss-'+plan.id} onPress={()=>nav.plan(plan)}><MessageCircle/>{p.t('coopDiscuss')}</Button></Card.Footer></Card>{mutation&&<WorkMutationDialog mutation={mutation} onClose={()=>setMutation(undefined)}/>}</>;
}
