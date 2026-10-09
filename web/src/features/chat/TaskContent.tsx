import {Check,CornerDownRight,Lock,ArrowUpRight} from 'lucide-react';
import {useWork} from '../work/store';
import {Person,EvidenceList} from '../work/ui';
import {Button} from '../../components/ui/Button';
import {UIDisclosure} from '../../components/ui/FormControls';
import {TaskFlowRules} from './TaskFlowRules';
import {TaskPlanPreviewButton} from '../cooperation/TaskPlanPreviewButton';
import {useSurfaceNavigation} from '../work/SurfaceNavigation';
import {requirementMet} from '../work/selectors';
import type {Task} from '../work/types';
import {useTaskActivity} from './collaborationData';
export function TaskOverview({task,onRecords,onTask}:{task:Task;onRecords:()=>void;onTask?:(id:string)=>void}){
 const {state,t,text}=useWork(),{go}=useSurfaceNavigation(),records=useTaskActivity(task.id),latest=records.items[0];
 return <div className="judex-task-section" data-testid="task-section-overview"><section><h3>{t('taskDetailsDescription')}</h3><p>{text(task.expected)||t('taskDetailsNoDescription')}</p></section><section><h3>{t('coCompleteCriteria')}</h3><ul className="judex-task-check-list">{task.criteria.map((v,i)=><li key={i}><Check/>{text(v)}</li>)}</ul></section><dl className="judex-task-facts"><div><dt>{t('workReviewer')}</dt><dd><Person seatId={task.reviewerSeatId} small/></dd></div><div><dt>{t('taskDetailsVersion')}</dt><dd>{task.revision}</dd></div></dl>
 {task.requirements.length>0&&<section><h3>{t('taskDetailsPrerequisites')}</h3><div className="judex-task-condition-list">{task.requirements.map((r,i)=><div className="judex-task-condition" key={r.id+':'+i} data-testid={'requirement-'+r.id}>{requirementMet(state,task,r)?<Check/>:<Lock/>}<div><span>{r.kind==='task'?text(state.tasks.find(v=>v.id===r.ref)?.title??r.label):text(r.label)}</span><small>{t(r.at==='start'?'chatBeforeStart':r.at==='accept'?'chatBeforeAccept':'taskDetailsBothPhases')} · {t(r.hard?'taskDetailsHard':'taskDetailsAdvisory')}</small>{r.waived&&<small>{t('taskDetailsWaived')}</small>}{r.inheritedFrom&&<small>{t('taskDetailsVia')}</small>}</div><span className="judex-task-meta">{t(requirementMet(state,task,r)?'workMet':'workUnmet')}</span>{r.kind==='task'&&<Button size="sm" isIconOnly aria-label={t('details')} onPress={()=>onTask?onTask(r.ref):go({view:'task',id:r.ref,taskSection:'overview',activityId:undefined})}><ArrowUpRight/></Button>}</div>)}</div></section>}
 {task.executionException&&<section className="judex-task-note"><h3><CornerDownRight/>{t('taskDetailsExceptionRecord')}</h3><p>{task.executionException.reason}</p></section>}
 {task.bugDetails&&<UIDisclosure title={t('taskDetailsBug')}><dl className="judex-task-bug-facts">{[['taskDetailsEnvironment',task.bugDetails.environment],['taskDetailsSteps',task.bugDetails.steps],['taskDetailsExpected',task.bugDetails.expected],['taskDetailsActual',task.bugDetails.actual],['taskDetailsSeverity',task.bugDetails.severity],['taskDetailsRelease',task.bugDetails.observedReleaseRef]].map(([key,value])=><div key={key}><dt>{t(key as 'taskDetailsEnvironment')}</dt><dd>{String(value??'')}</dd></div>)}</dl></UIDisclosure>}
 {task.files.length>0&&<section><h3>{t('chatMaterials')}</h3><EvidenceList files={task.files}/></section>}
 {latest&&<section><div className="judex-task-section-heading"><h3>{t('taskDetailsLatest')}</h3><Button size="sm" onPress={onRecords}>{t('taskDetailsRecords')}<ArrowUpRight/></Button></div><p className="judex-task-excerpt">{latest.text}</p></section>}
 </div>;
}
export function TaskFlowReference({task,onBrief}:{task:Task;onBrief:()=>void}){
 const {state,t,text,mode}=useWork(),{go}=useSurfaceNavigation();
 const flow=state.flows.find(v=>v.id===task.flowId),published=!!flow?.version&&(mode==='api'?flow.status==='published':flow.status!=='draft');
 const node=flow?.nodes.find(v=>v.id===task.nodeId),bodyNode=flow?.body?.nodes.find(v=>v.id===task.nodeId);
 const positions=bodyNode?.allowedPositionIds.map(id=>text(state.positions.find(v=>v.id===id)?.name??id)).join(' · ');
 const responsibility=bodyNode?.responsibility??(node?.responsibility?text(node.responsibility):''),instructions=flow?.body?.instructions??(flow?.instructions?text(flow.instructions):'');
 const nodes=flow?.body?.nodes.map(v=>({id:v.id,label:v.name}))??flow?.nodes??[];
 return <div className="judex-task-section" data-testid="task-section-flow">
  <section><h3>{t('taskDetailsFlowVersion')}</h3>{published?<div className="judex-task-section-heading"><strong>{text(flow!.name)}</strong><span className="judex-task-meta">v{flow!.version}</span></div>:<p className="judex-task-meta">{t('taskDetailsNoWorkflow')}</p>}</section>
  {published&&<>
   <section><h3>{t('taskDetailsCurrentNode')}</h3><p className="judex-task-current-node">{bodyNode?.name??(node?text(node.label):task.nodeId||t('taskDetailsReferenceUnavailable'))}</p></section>
   {responsibility&&<section><h3>{t('taskDetailsResponsibility')}</h3><p>{responsibility}</p>{positions&&<p className="judex-task-meta">{positions}</p>}</section>}
   {instructions&&<section><h3>{t('taskDetailsWorkflowInstructions')}</h3><p className="judex-task-original">{instructions}</p></section>}
   <TaskFlowRules task={task} rules={flow!.body?.hardRules?.filter(r=>!r.nodeId||r.nodeId===task.nodeId)??[]}/>
   <div className="judex-task-flow-strip" aria-label={t('taskDetailsFlow')}>{nodes.map(v=><span key={v.id} className={v.id===task.nodeId?'judex-task-flow-current':''}>{text(v.label)}</span>)}</div>
  </>}
  <p className="judex-task-meta">{t('taskDetailsFlowHint')}</p>
  <div className="judex-task-section-heading"><TaskPlanPreviewButton task={task}/>{flow&&<><Button size="sm" variant="outline" onPress={onBrief}>{t('workBrief')}</Button><Button size="sm" onPress={()=>go({settingsSection:'flows',settingsItem:flow.id})}>{t('coViewSettings')}<ArrowUpRight/></Button></>}</div>
 </div>;
}
