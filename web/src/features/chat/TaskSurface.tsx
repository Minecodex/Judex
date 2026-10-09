import {useEffect,useRef,useState} from 'react';
import {Tabs} from '@heroui/react';
import {FileText,MessageCircle,Paperclip,Check} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {UIDisclosure,UIWarning,UIStatus,UISelect,UIOption} from '../../components/ui/FormControls';
import {MaterialReferences} from '../materials/FileCard';
import {useWork} from '../work/store';
import {Person} from '../work/ui';
import {canWork,canAcceptTask,blockers,ownsSeat} from '../work/selectors';
import {workCapabilities,taskDisplayStatus} from '../work/runtime';
import {TaskStartDialog} from '../work/TaskStartDialog';
import {AcceptanceDialog,ReasonDialog,BriefDialog} from '../work/Dialogs';
import {HandoffDialog} from '../work/CompositionDialogs';
import {WorkMutationDialog,type WorkMutation} from '../work/WorkMutationDialog';
import {useDiscussionNavigation} from '../cooperation/useDiscussionNavigation';
import {TaskActivityDialog} from './CollaborationDialogs';
import {useTaskActivity,useDiscussionSuggestions} from './collaborationData';
import {taskReadingKey} from './taskReadingPosition';
import {useReadingPosition} from './useReadingPosition';
import {useTaskRecordReading} from './useTaskRecordReading';
import {AnalysisText,SuggestionCard} from './CollaborationCards';
import {taskStatusKey} from './collaborationPresentation';
import {TaskOverview,TaskFlowReference} from './TaskContent';
import {TaskMore,type TaskOperation} from './TaskActions';
import {useSurfaceNavigation} from '../work/SurfaceNavigation';
import type {Task} from '../work/types';
import type {TaskSection} from '../work/runtimeTypes';
import type {TaskActivity,TaskInputKind} from './collaborationTypes';
export function TaskRecordCard({activity,full=false,originalExpanded,onOriginalChange}:{activity:TaskActivity;full?:boolean;originalExpanded?:boolean;onOriginalChange?:(expanded:boolean)=>void}){
 const {state,route,t,mode,locale,text}=useWork(),{go}=useSurfaceNavigation();
 return <article className="judex-task-record" data-activity={activity.id}><div className="judex-task-record-heading"><strong>{t(activity.kind==='decision'?'taskDetailsDecision':activity.kind==='progress'?'coProgressKind':activity.kind==='delivery'?'coDeliveryKind':activity.kind==='reply'?'coReplyKind':'coQuestionKind')}</strong><time>{new Date(activity.createdAt).toLocaleString(locale,{timeZone:'Asia/Shanghai',month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit'})}</time></div><div className="judex-task-meta">{activity.actorName} · {t(activity.source==='cli'?'coSourceCli':activity.source==='web'?'coSourceWeb':'coSourceUnknown')}</div><p className={full?'judex-task-original':'judex-task-excerpt'}>{activity.text}</p>{mode==='api'?<MaterialReferences materials={activity.materials}/>:<div className="judex-task-materials">{activity.materials.map(m=><span key={m.versionId}><Paperclip/>{m.name}</span>)}</div>}
 {!full&&<UIDisclosure title={t('taskDetailsOriginal')} open={route.activityId===activity.id} isExpanded={originalExpanded} onExpandedChange={onOriginalChange}><p className="judex-task-original">{activity.text}</p></UIDisclosure>}{activity.kind!=='decision'&&<AnalysisText activity={activity}/>}{activity.topicIds.length>0&&<div className="judex-task-record-links">{activity.topicIds.map(id=><Button key={id} size="sm" onPress={()=>go({page:'chat',view:'topic',id,conversation:id,scopePlanId:undefined,scopeTaskId:activity.taskId,taskContextId:activity.taskId})}><MessageCircle/>{text(state.topics.find(v=>v.id===id)?.title??t('coOpenDiscussion'))}</Button>)}</div>}</article>;
}
export function TaskTimeline({task,readingScope='task'}:{task:Task;readingScope?:string}){
 const {route,t,mode}=useWork(),records=useTaskActivity(task.id),area=useRef<HTMLDivElement>(null),[reading,setReading]=useTaskRecordReading(task.id,readingScope),located=useRef<string|undefined>(undefined);
 const activityId=route.view==='task'&&route.id===task.id?route.activityId:undefined,pages=records.data?.pages.length??1;
 const visible=records.items.filter(a=>activityId===a.id||reading.source==='all'||a.source===reading.source);
 useEffect(()=>{if(activityId)setReading(v=>({...v,expanded:true,originals:{...v.originals,[activityId]:true}}));},[activityId]);
 useEffect(()=>{
  if(pages>reading.pages)setReading(v=>({...v,pages}));
  const missingSource=activityId&&!records.items.some(v=>v.id===activityId);
  if(records.hasNextPage&&!records.isFetching&&!records.isError&&(pages<reading.pages||missingSource))void records.fetchNextPage();
 },[pages,reading.pages,activityId,records.items.length,records.hasNextPage,records.isFetching,records.isError]);
 useEffect(()=>{if(!activityId){located.current=undefined;return;}if(located.current===activityId)return;const row=area.current?.querySelector('[data-activity="'+activityId+'"]');if(row){row.scrollIntoView({block:'nearest'});located.current=activityId;}},[activityId,records.items.length,reading.expanded]);
 return <div className="judex-task-section" ref={area} data-testid="task-section-records"><div className="judex-task-section-heading"><h3>{t('coTimeline')}</h3><UISelect className="judex-task-source-filter" aria-label={t('taskDetailsSourceFilter')} data-testid="task-record-source" value={reading.source} onChange={e=>setReading(v=>({...v,source:e.target.value}))}><UIOption value="all">{t('coopAll')}</UIOption><UIOption value="web">{t('coSourceWeb')}</UIOption><UIOption value="cli">{t('coSourceCli')}</UIOption></UISelect></div>{records.pending?<p role="status">{t('shellLoading')}</p>:records.isError&&mode==='api'?<UIWarning>{t('errNetwork')}<Button onPress={()=>void records.refetch()}>{t('shellRetry')}</Button></UIWarning>:<div className="judex-task-timeline">{(reading.expanded?visible:visible.slice(0,3)).map(a=><TaskRecordCard key={a.id} activity={a} originalExpanded={reading.originals[a.id]??false} onOriginalChange={expanded=>setReading(v=>({...v,originals:{...v.originals,[a.id]:expanded}}))}/>)}{!visible.length&&<p className="judex-task-meta">{t('coNoRecords')}</p>}</div>}{(!reading.expanded&&visible.length>3||records.hasNextPage)&&<Button size="sm" variant="outline" onPress={()=>{setReading(v=>({...v,expanded:true}));if(records.hasNextPage)void records.fetchNextPage();}}>{t('coMoreRecords')}</Button>}</div>;
}
export function TaskRecords({task,section='overview',onSection,readOnly=false,navigation=false,readingScope='task',onTask}:{task:Task;section?:TaskSection;onSection?:(s:TaskSection)=>void;readOnly?:boolean;navigation?:boolean;readingScope?:string;onTask?:(id:string)=>void}){
 const p=useWork(),nav=useDiscussionNavigation(),{navigate}=useSurfaceNavigation(),[report,setReport]=useState<{kind:TaskInputKind;local?:boolean}>(),[operation,setOperation]=useState<TaskOperation>(),[mutation,setMutation]=useState<WorkMutation>(),caps=workCapabilities(p.state,task),status=task.businessStatus??task.status,display=taskDisplayStatus(task,p.state),authorized=canWork(p.state,task)&&!task.executionException&&!task.discardedAt;
 const changeSection=(s:TaskSection)=>onSection?onSection(s):p.go({view:'task',id:task.id,taskSection:s,activityId:undefined});
 const run=(op:TaskOperation)=>{if(op==='progress'||op==='delivery'||op==='question')setReport({kind:op});else if(op==='local')setReport({kind:authorized&&['working','rework'].includes(status)?'progress':'question',local:true});else if(op==='approve')void p.act('decideDraft',{taskId:task.id});else if(op==='start'){const own=task.seatIds.filter(id=>ownsSeat(p.state,id));if(own.length===1)void p.act('taskAction',{taskId:task.id,op:'start',identityId:own[0]});else setOperation(op);}else setOperation(op);};
 const plan=p.state.plans.find(v=>v.id===task.planId),reading=useReadingPosition(taskReadingKey(p.mode,p.project.id,p.state.currentUserId??p.state.currentUser,task.id,section,readingScope));
 const content=section==='overview'?<><TaskOverview task={task} onRecords={()=>changeSection('records')} onTask={onTask}/>{!readOnly&&<TaskDiscussionSuggestions taskId={task.id}/>}</>:section==='records'?<TaskTimeline task={task} readingScope={readingScope}/>:<TaskFlowReference task={task} onBrief={()=>setOperation('brief')}/>;
 return <div className="judex-task-surface" data-testid="task-inspector"><header className="judex-task-header"><p className="judex-task-breadcrumb">{p.text(plan?.title??'')}</p><div className="judex-task-title"><h2>{p.text(task.title)}</h2><TaskMore task={task} onMutation={setMutation} onOperation={run} onSection={changeSection} readOnly={readOnly}/></div><div className="judex-task-header-meta"><UIStatus className={'judex-task-status judex-task-status-'+display}>{p.t(display==='skipped'?'taskDetailsSkipped':display==='discarded'?'taskDetailsDiscarded':taskStatusKey(display))}</UIStatus>{task.seatIds.map((id,i)=><Person key={id} name={task.participantNames?.[i]} seatId={id} small/>)}</div></header>
 {navigation?<Tabs variant="secondary" className="judex-task-section-tabs" selectedKey={section} onSelectionChange={key=>changeSection(key as TaskSection)}><Tabs.List aria-label={p.t('coMoreDetails')}>{(['overview','records','flow'] as const).map(s=><Tabs.Tab id={s} key={s}>{p.t(s==='overview'?'taskDetailsOverview':s==='records'?'taskDetailsRecords':'taskDetailsFlow')}<Tabs.Indicator/></Tabs.Tab>)}</Tabs.List><Tabs.Panel id={section} className="judex-task-scroll" ref={reading}>{content}</Tabs.Panel></Tabs>:<div className="judex-task-scroll" ref={reading}>{content}</div>}
 <footer className="judex-task-actions">{!readOnly&&status==='draft'&&caps.editDraft?<Button size="sm" variant="primary" onPress={()=>setMutation({action:'edit',kind:'task',id:task.id})}>{p.t('taskDetailsDraftEdit')}</Button>:!readOnly&&authorized&&status==='ready'?<Button size="sm" variant="primary" data-testid="start-task" disabled={!!blockers(p.state,task,'start').length} onPress={()=>run('start')}>{p.t('workTaskStart')}</Button>:!readOnly&&authorized&&['working','rework'].includes(status)?<Button size="sm" variant="primary" onPress={()=>run('progress')}><FileText/>{p.t('coProgress')}</Button>:!readOnly&&!task.executionException&&status==='delivered'&&canAcceptTask(p.state,task)?<Button size="sm" variant="primary" data-testid="accept-task" onPress={()=>run('accept')}><Check/>{p.t('workTaskAccept')}</Button>:!readOnly&&status==='draft'&&(p.management||canWork(p.state,task))?<Button size="sm" variant="primary" data-testid="decide-draft" onPress={()=>run('approve')}>{p.t('workTaskDraftApprove')}</Button>:null}{!readOnly&&<Button size="sm" variant="outline" isPending={nav.busy===task.id} onPress={()=>navigate(()=>void nav.task(task))}><MessageCircle/>{p.t('coDiscussTask')}</Button>}</footer>
 {report&&<TaskActivityDialog task={task} kind={report.kind} local={report.local} onClose={()=>setReport(undefined)}/>}{mutation&&<WorkMutationDialog mutation={mutation} onClose={()=>setMutation(undefined)}/>}{operation==='start'&&<TaskStartDialog task={task} onClose={()=>setOperation(undefined)}/>}{operation==='accept'&&<AcceptanceDialog task={task} onClose={()=>setOperation(undefined)}/>}{operation==='reopen'&&<ReasonDialog task={task} onClose={()=>setOperation(undefined)}/>}{operation==='handoff'&&<HandoffDialog task={task} onClose={()=>setOperation(undefined)}/>}{operation==='brief'&&<BriefDialog task={task} onClose={()=>setOperation(undefined)}/>}
 </div>;
}

function TaskDiscussionSuggestions({taskId}:{taskId:string}){const {t,route}=useWork(),query=useDiscussionSuggestions(taskId),items=query.items.filter(s=>s.state==='pending');return route.page!=='chat'&&items.length?<section className="judex-task-section judex-task-suggestions"><h3>{t('coopDecisions')}</h3>{items.map(s=><SuggestionCard key={s.id} suggestion={s}/>)}{query.hasNextPage&&<Button size="sm" onPress={()=>void query.fetchNextPage()}>{t('lcLoadMore')}</Button>}</section>:null;}
