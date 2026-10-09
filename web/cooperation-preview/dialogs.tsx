import {useState} from "react";
import {Tabs} from "@heroui/react";
import {ArrowRight,Check,FileText,GitBranch,Users} from "lucide-react";
import {UIInput,UIOption,UISelect,UITextArea,UICheckbox} from "../src/components/ui/FormControls";
import {Button,Modal,Status} from "./ui";
import {usePreview,type ModalState} from "./context";
import {linkedPlans,planTasks,type Activity} from "./model";
import {TaskMap} from "./routes";
import {Record,TaskRecords} from "./records";
function Field({label,children}:{label:string;children:React.ReactNode}){return <label className="judex-co-field"><span>{label}</span>{children}</label>;}
function Footer({onClose,save,label,disabled=false}:{onClose:()=>void;save:()=>void;label:string;disabled?:boolean}){const p=usePreview();return <div className="judex-co-dialog-actions"><Button variant="outline" onPress={onClose}>{p.t("cpCancel")}</Button><Button variant="primary" disabled={disabled} data-testid="co-confirm" onPress={save}>{label}</Button></div>;}
export function Dialogs({modal,onClose}:{modal:ModalState;onClose:()=>void}){
 const p=usePreview();
 if(modal.kind==="route"){const plan=p.state.plans.find(v=>v.id===modal.planId)!;return <Modal title={p.text(plan.title)+" · "+p.t("cpTaskRoute")} description={p.t("cpRouteHint")} onClose={onClose}><TaskMap planId={plan.id} compact/><div className="judex-co-dialog-actions"><Button variant="primary" onPress={()=>p.go({page:"route",projectId:plan.projectId,planId:plan.id})}>{p.t("cpTaskRoute")}<ArrowRight size={16}/></Button></div></Modal>;}
 if(modal.kind==="task"){const task=p.state.tasks.find(v=>v.id===modal.taskId)!;return <Modal title={p.t("cpTaskDetails")} onClose={onClose}><TaskRecords task={task}/><div className="judex-co-dialog-actions"><Button variant="primary" onPress={()=>p.taskChat(task)}>{p.t("cpTaskDiscussion")}</Button></div></Modal>;}
 if(modal.kind==="report")return <ReportDialog modal={modal} onClose={onClose}/>;
 if(modal.kind==="topic")return <TopicDialog modal={modal} onClose={onClose}/>;
 if(modal.kind==="suggestion")return <SuggestionDialog modal={modal} onClose={onClose}/>;
 if(modal.kind==="decision")return <DecisionDialog decisionId={modal.decisionId} onClose={onClose}/>;
 if(modal.kind==="receipt")return <ReceiptDialog handoffId={modal.handoffId} onClose={onClose}/>;
 if(modal.kind==="arrangement")return <ArrangementDialog sourceTopicId={modal.sourceTopicId} onClose={onClose}/>;
 if(modal.kind==="invite")return <InviteDialog projectId={modal.projectId} onClose={onClose}/>;
 if(modal.kind==="settings")return <SettingsDialog projectId={modal.projectId} section={modal.section} onClose={onClose}/>;
 if(modal.kind==="material"){const m=p.state.materials.find(v=>v.id===modal.materialId)!;return <Modal title={m.name} description={p.t("cpFixedMaterial")+" "+m.version+" · "+p.person(m.authorId)} onClose={onClose}><pre className="judex-co-material-body">{p.text(m.body)}</pre></Modal>;}
 if(modal.kind==="record"){const record=p.state.activities.find(r=>r.id===modal.recordId)!;return <Modal title={p.t("cpOriginal")} description={p.text(p.state.tasks.find(t=>t.id===record.taskId)!.title)} onClose={onClose}><Record record={record} full/></Modal>;}
 return <ProjectDialog onClose={onClose}/>;
}
function ReportDialog({modal,onClose}:{modal:Extract<ModalState,{kind:"report"}>;onClose:()=>void}){
 const p=usePreview(),task=p.state.tasks.find(t=>t.id===modal.taskId)!,plan=p.state.plans.find(v=>v.id===task.planId)!;
 const [kind,setKind]=useState<Activity["kind"]>(modal.recordKind),[body,setBody]=useState(""),[topicId,setTopicId]=useState("");
 const topics=p.state.topics.filter(t=>t.projectId===task.projectId&&(t.taskIds.includes(task.id)||t.id===plan.mainTopicId));
 const submit=()=>{const result=p.perform({kind:"report",taskId:task.id,recordKind:kind,body,source:modal.local?"cli":"web",topicId:kind==="reply"?topicId:undefined});if(result.state)onClose();};
 return <Modal title={p.t(modal.local?"cpSimulate":"cpRecords")} description={p.t("cpReportHint")} onClose={onClose}>
  <div className="judex-co-context-line">{p.text(plan.title)} <ArrowRight size={14}/>{p.text(task.title)}</div>
  <Field label={p.t("cpType")}><UISelect data-testid="co-record-kind" value={kind} onChange={e=>setKind(e.target.value as Activity["kind"])}>{[["progress","cpProgress"],["delivery","cpDelivery"],["question","cpQuestion"],["reply","cpReply"]].map(([value,key])=><UIOption value={value} key={value}>{p.t(key as "cpProgress")}</UIOption>)}</UISelect></Field>
  {kind==="reply"&&<Field label={p.t("cpChooseTopic")}><UISelect value={topicId} onChange={e=>setTopicId(e.target.value)}><UIOption value="">—</UIOption>{topics.map(t=><UIOption key={t.id} value={t.id}>{p.text(t.title)}</UIOption>)}</UISelect></Field>}
  <Field label={p.t("cpReportBody")}><UITextArea aria-label={p.t("cpReportBody")} data-testid="co-report-body" rows={5} value={body} onChange={e=>setBody(e.target.value)}/></Field>
  <Footer onClose={onClose} save={submit} label={p.t("cpSubmit")} disabled={!body.trim()||kind==="reply"&&!topicId}/>
 </Modal>;
}
function TopicDialog({modal,onClose}:{modal:Extract<ModalState,{kind:"topic"}>;onClose:()=>void}){
 const p=usePreview(),existing=p.state.topics.find(t=>t.id===modal.topicId),parent=p.state.topics.find(t=>t.id===modal.parentId);
 const initialPlan=existing?.planIds??(p.route.planId?[p.route.planId]:p.route.taskId?[p.state.tasks.find(t=>t.id===p.route.taskId)!.planId]:[]);
 const [title,setTitle]=useState(existing?p.text(existing.title):parent?p.text(parent.title)+" · "+p.t("cpSpecial"):""),[planIds,setPlans]=useState(initialPlan),[taskIds,setTasks]=useState(existing?.taskIds??(p.route.taskId?[p.route.taskId]:[]));
 const projectId=p.route.projectId,plans=p.state.plans.filter(v=>v.projectId===projectId),tasks=p.state.tasks.filter(v=>v.projectId===projectId);
 const toggle=(arr:string[],value:string,on:boolean)=>on?[...new Set([...arr,value])]:arr.filter(v=>v!==value);
 const save=()=>{const result=p.perform({kind:"topic",projectId,title,planIds,taskIds,parentId:parent?.id,forkAfterSeq:modal.afterSeq,topicId:existing?.id});if(result.id){const topic=result.state!.topics.find(v=>v.id===result.id)!;p.topicChat(topic);onClose();}};
 return <Modal title={p.t(existing?"cpEditLinks":parent?"cpFork":"cpNewDiscussion")} description={p.t("cpSharedTopicHint")} onClose={onClose}>
  {parent&&<div className="judex-co-context-line">{p.t("cpSource")} · {p.text(parent.title)} · {modal.afterSeq}</div>}
  <Field label={p.t("cpTitle")}><UIInput data-testid="co-topic-title" value={title} onChange={e=>setTitle(e.target.value)} placeholder={p.t("cpTopicPlaceholder")}/></Field>
  <div className="judex-co-object-selection"><strong>{p.t("cpPlans")}</strong>{plans.map(plan=><UICheckbox key={plan.id} appearance="card" data-testid={"co-link-plan-"+plan.id} checked={planIds.includes(plan.id)} onChange={e=>setPlans(toggle(planIds,plan.id,e.target.checked))}>{p.text(plan.title)}<span className="judex-co-meta">{p.text(plan.goal)}</span></UICheckbox>)}</div>
  <div className="judex-co-object-selection"><strong>{p.t("cpTasks")}</strong>{tasks.map(task=><UICheckbox key={task.id} appearance="card" data-testid={"co-link-task-"+task.id} checked={taskIds.includes(task.id)} onChange={e=>{setTasks(toggle(taskIds,task.id,e.target.checked));if(e.target.checked)setPlans(v=>[...new Set([...v,task.planId])]);}}>{p.text(task.title)}<span className="judex-co-meta">{p.text(plans.find(v=>v.id===task.planId)?.title??"")}</span></UICheckbox>)}</div>
  <Footer onClose={onClose} save={save} label={p.t(existing?"cpSave":"cpNewDiscussion")} disabled={!title.trim()||!planIds.length}/>
 </Modal>;
}
function SuggestionDialog({modal,onClose}:{modal:Extract<ModalState,{kind:"suggestion"}>;onClose:()=>void}){
 const p=usePreview(),suggestion=p.state.suggestions.find(s=>s.id===modal.suggestionId)!,task=p.state.tasks.find(t=>t.id===suggestion.taskId)!;
 const [title,setTitle]=useState(p.text(suggestion.title)),[topicId,setTopic]=useState("");
 const save=()=>{const result=p.perform({kind:"suggestion",suggestionId:suggestion.id,mode:modal.mode==="create"?"create":"existing",title,topicId});if(result.id){p.topicChat(result.state!.topics.find(t=>t.id===result.id)!,{taskId:task.id});onClose();}};
 return <Modal title={p.t(modal.mode==="create"?"cpChooseTitle":"cpChooseTopic")} description={p.t("cpSameTopic")} onClose={onClose}><div className="judex-co-context-line">{p.text(p.state.plans.find(v=>v.id===task.planId)!.title)} · {p.text(task.title)}</div>{modal.mode==="create"?<Field label={p.t("cpTitle")}><UIInput value={title} onChange={e=>setTitle(e.target.value)}/></Field>:<Field label={p.t("cpChooseTopic")}><UISelect value={topicId} onChange={e=>setTopic(e.target.value)}><UIOption value="">—</UIOption>{p.state.topics.filter(t=>t.projectId===task.projectId).map(t=><UIOption key={t.id} value={t.id}>{p.text(t.title)}</UIOption>)}</UISelect></Field>}<Footer onClose={onClose} save={save} label={p.t("cpSubmit")} disabled={modal.mode==="create"?!title.trim():!topicId}/></Modal>;
}
function DecisionDialog({decisionId,onClose}:{decisionId:string;onClose:()=>void}){
 const p=usePreview(),d=p.state.decisions.find(v=>v.id===decisionId)!,plan=p.state.plans.find(v=>v.id===d.planId)!,task=p.state.tasks.find(v=>v.id===d.taskId);
 const [reason,setReason]=useState("");
 const decide=(approve:boolean)=>{const result=p.perform({kind:"decision",decisionId:d.id,version:d.version,approve,reason});if(result.state)onClose();};
 const legal=d.actors.includes(p.state.userId)&&!d.approvedBy.includes(p.state.userId)&&d.status==="pending";
 const hint=p.t(d.kind==="task"?"cpTaskAcceptanceHint":d.kind==="plan"?"cpPlanAcceptanceHint":"cpOnlyOwnSlot");
 return <Modal title={p.t(d.kind==="task"?"cpTaskAcceptance":d.kind==="plan"?"cpPlanAcceptance":"cpArrangement")} description={hint} onClose={onClose}><div className="judex-co-review-header"><h3>{p.text(d.title)}</h3><span className="judex-co-tag">{p.t("cpReviewVersion")} {d.version}</span></div><p>{p.text(d.description)}</p><div className="judex-co-review-panel"><strong>{p.t("cpEvidence")}</strong>{(task?task.criteria:plan.criteria).map((v,i)=><p key={i}><Check size={14}/>{p.text(v)}</p>)}{task&&p.state.activities.filter(r=>r.taskId===task.id&&r.kind==="delivery").map(r=><Record key={r.id} record={r} full/>)}{d.kind==="arrangement"&&planTasks(p.state,plan.id).map(t=><div key={t.id} className="judex-co-review-row"><span>{p.text(t.title)}</span><span className="judex-co-meta">{p.person(t.makerId)} · {p.person(t.reviewerId)}</span><Status status={t.status}/></div>)}</div><div className="judex-co-review-panel"><strong>{p.t("cpPendingWho")}</strong>{d.actors.map(who=><div key={who} className="judex-co-review-row"><span>{p.person(who)}</span><span>{d.approvedBy.includes(who)?"✓":who===p.state.userId?p.t("cpDecisions"):"—"}</span></div>)}</div><Field label={p.t("cpReason")}><UITextArea value={reason} onChange={e=>setReason(e.target.value)} placeholder={p.t("cpReasonRequired")}/></Field><div className="judex-co-dialog-actions"><Button variant="outline" onPress={onClose}>{p.t("cpCancel")}</Button><Button variant="danger-soft" disabled={!legal||!reason.trim()} onPress={()=>decide(false)}>{p.t("cpReject")}</Button><Button variant="primary" data-testid="co-approve" disabled={!legal} onPress={()=>decide(true)}>{p.t(d.kind==="task"?"cpAcceptTask":d.kind==="plan"?"cpAcceptPlan":"cpApprove")}</Button></div></Modal>;
}
function ReceiptDialog({handoffId,onClose}:{handoffId:string;onClose:()=>void}){
 const p=usePreview(),h=p.state.handoffs.find(v=>v.id===handoffId)!,source=p.state.tasks.find(t=>t.id===h.sourceTaskId)!,target=p.state.tasks.find(t=>t.id===h.targetTaskId)!;
 const [reason,setReason]=useState(""),legal=h.receiverId===p.state.userId&&h.status==="pending";
 const decide=(accept:boolean)=>{const result=p.perform({kind:"receipt",handoffId:h.id,version:h.version,accept,reason});if(result.state)onClose();};
 return <Modal title={p.t("cpReceipt")} description={p.t("cpReceiptHint")} onClose={onClose}><h3>{p.text(source.title)} → {p.text(target.title)}</h3><div className="judex-co-meta">{p.t("cpSender")} {p.person(h.senderId)} · {p.t("cpReceiver")} {p.person(h.receiverId)} · {p.t("cpSentVersion")} {h.version}</div><div className="judex-co-review-panel">{h.materialIds.map(mid=>{const m=p.state.materials.find(v=>v.id===mid)!;return <Button key={mid} variant="outline" onPress={()=>p.setModal({kind:"material",materialId:mid})}><FileText/>{m.name} · v{m.version}</Button>;})}<div className="judex-co-review-row"><span>{p.text(source.title)}</span><Status status={source.status}/></div></div><Field label={p.t("cpReason")}><UITextArea value={reason} onChange={e=>setReason(e.target.value)} placeholder={p.t("cpReasonRequired")}/></Field><div className="judex-co-dialog-actions"><Button variant="outline" onPress={onClose}>{p.t("cpClose")}</Button><Button variant="danger-soft" disabled={!legal||!reason.trim()} onPress={()=>decide(false)}>{p.t("cpReject")}</Button><Button variant="primary" disabled={!legal} data-testid="co-receive" onPress={()=>decide(true)}>{p.t("cpAcceptReceipt")}</Button></div></Modal>;
}
function ArrangementDialog({sourceTopicId,onClose}:{sourceTopicId:string;onClose:()=>void}){
 const p=usePreview(),[title,setTitle]=useState(""),[goal,setGoal]=useState(""),[taskTitle,setTask]=useState("");
 const save=()=>{const result=p.perform({kind:"arrangement",projectId:p.route.projectId,sourceTopicId,title,goal,taskTitle});if(result.state){onClose();p.go({page:"hub",projectId:p.route.projectId,tab:"decisions"});}};
 return <Modal title={p.t("cpNewArrangement")} description={p.t("cpNewPlanHint")} onClose={onClose}><Field label={p.t("cpTitle")}><UIInput value={title} onChange={e=>setTitle(e.target.value)}/></Field><Field label={p.t("cpGoal")}><UITextArea value={goal} onChange={e=>setGoal(e.target.value)}/></Field><Field label={p.t("cpTasks")}><UIInput value={taskTitle} onChange={e=>setTask(e.target.value)}/></Field><div className="judex-co-info-band">{p.t("cpDraftHint")}</div><Footer onClose={onClose} save={save} label={p.t("cpPropose")} disabled={!title.trim()||!goal.trim()||!taskTitle.trim()}/></Modal>;
}
function ProjectDialog({onClose}:{onClose:()=>void}){
 const p=usePreview(),[title,setTitle]=useState(""),[goal,setGoal]=useState("");
 const save=()=>{const result=p.perform({kind:"project",title,goal});if(result.id)p.go({page:"hub",projectId:result.id,tab:"plans"});};
 return <Modal title={p.t("cpCreateProject")} onClose={onClose}><Field label={p.t("cpTitle")}><UIInput value={title} onChange={e=>setTitle(e.target.value)}/></Field><Field label={p.t("cpGoal")}><UITextArea value={goal} onChange={e=>setGoal(e.target.value)}/></Field><Footer onClose={onClose} save={save} label={p.t("cpCreateProject")} disabled={!title.trim()}/></Modal>;
}
function InviteDialog({projectId,onClose}:{projectId:string;onClose:()=>void}){
 const p=usePreview(),[email,setEmail]=useState(""),[position,setPosition]=useState("dev");
 const save=()=>{const result=p.perform({kind:"invite",projectId,name:email,positionId:position});if(result.state)onClose();};
 return <Modal title={p.t("cpInvite")} description={p.t("cpInviteHint")} onClose={onClose}><Field label={p.t("cpEmail")}><UIInput type="email" value={email} onChange={e=>setEmail(e.target.value)}/></Field><Field label={p.t("cpPositions")}><UISelect value={position} onChange={e=>setPosition(e.target.value)}>{p.state.positions.filter(v=>v.projectId===projectId).map(v=><UIOption key={v.id} value={v.id}>{p.text(v.name)}</UIOption>)}</UISelect></Field><Footer onClose={onClose} save={save} label={p.t("cpInvite")} disabled={!email.includes("@")}/></Modal>;
}
function PositionEditor({positionId}:{positionId:string}){
 const p=usePreview(),position=p.state.positions.find(v=>v.id===positionId)!,[prompt,setPrompt]=useState(p.text(position.prompt)),[privatePrompt,setPrivate]=useState(p.state.preferences[p.state.userId+":"+positionId]??"");
 const owner=p.state.projects.find(v=>v.id===position.projectId)?.ownerId===p.state.userId,own=p.state.people.find(v=>v.id===p.state.userId)?.positionId===position.id;
 return <article className="judex-co-settings-position"><div className="judex-co-card-top"><h3>{p.text(position.name)}</h3><span className="judex-co-meta">{p.t("cpPositionBindings")} · {position.nodes.join(" / ")}</span></div><Field label={p.t("cpPrompt")}><UITextArea value={prompt} onChange={e=>setPrompt(e.target.value)} readOnly={!owner}/></Field>{owner&&<Button size="sm" variant="outline" onPress={()=>{p.perform({kind:"save-position",positionId,prompt});p.notify(p.t("cpSave"));}}>{p.t("cpSave")}</Button>}{own&&<div className="judex-co-private"><Field label={p.t("cpMyPrompt")}><UITextArea value={privatePrompt} onChange={e=>setPrivate(e.target.value)}/></Field><p className="judex-co-meta">{p.t("cpPrivateHint")}</p><Button size="sm" variant="outline" onPress={()=>{p.perform({kind:"preference",positionId,prompt:privatePrompt});p.notify(p.t("cpSave"));}}>{p.t("cpSave")}</Button></div>}</article>;
}
function SettingsDialog({projectId,section,onClose}:{projectId:string;section?:string;onClose:()=>void}){
 const p=usePreview(),project=p.state.projects.find(v=>v.id===projectId)!,[tab,setTab]=useState(section??"general"),[title,setTitle]=useState(p.text(project.title)),[goal,setGoal]=useState(p.text(project.goal));
 const names=[["general","cpGeneral"],["positions","cpPositions"],["flows","cpFlows"],["members","cpMemberList"],["local","cpLocalConnection"]] as const;
 return <Modal title={p.t("cpSettings")+" · "+p.text(project.title)} onClose={onClose}><Tabs selectedKey={tab} onSelectionChange={v=>setTab(String(v))} className="judex-co-settings-tabs"><Tabs.ListContainer><Tabs.List aria-label={p.t("cpSettings")}>{names.map(([id,key])=><Tabs.Tab key={id} id={id}>{p.t(key)}<Tabs.Indicator/></Tabs.Tab>)}</Tabs.List></Tabs.ListContainer><Tabs.Panel id={tab}>
  {tab==="general"?<><Field label={p.t("cpTitle")}><UIInput value={title} onChange={e=>setTitle(e.target.value)} readOnly={project.ownerId!==p.state.userId}/></Field><Field label={p.t("cpGoal")}><UITextArea value={goal} onChange={e=>setGoal(e.target.value)} readOnly={project.ownerId!==p.state.userId}/></Field><Footer onClose={onClose} save={()=>{const result=p.perform({kind:"save-project",projectId,title,goal});if(result.state)onClose();}} label={p.t("cpSave")} disabled={project.ownerId!==p.state.userId}/></>:tab==="positions"?p.state.positions.filter(v=>v.projectId===projectId).map(v=><PositionEditor key={v.id} positionId={v.id}/>):tab==="members"?<><div className="judex-co-actions">{project.ownerId===p.state.userId&&<Button variant="primary" onPress={()=>p.setModal({kind:"invite",projectId})}><Users size={16}/>{p.t("cpInvite")}</Button>}</div>{project.memberIds.map(who=><div className="judex-co-review-row" key={who}><span>{p.person(who)}</span><span>{p.text(p.state.positions.find(v=>v.id===p.state.people.find(v=>v.id===who)?.positionId)?.name??"")}</span></div>)}{p.state.invites?.filter(i=>i.projectId===projectId).map((i,n)=><div className="judex-co-review-row" key={n}><span>{i.email}</span><span className="judex-co-meta">{p.t("cpWaitingInvite")}</span></div>)}</>:tab==="flows"?p.state.flows.filter(f=>f.projectId===projectId).map(f=><article key={f.id} className="judex-co-flow-reference"><div className="judex-co-card-top"><h3>{p.text(f.name)}</h3><span className="judex-co-tag">{p.t("cpPublished")} v{f.version}</span></div><div className="judex-co-flow-nodes">{f.nodes.map((n,i)=><div key={n.id}><span className="judex-co-flow-step">{i+1}</span><div><h4>{p.text(n.name)}</h4><p>{p.text(n.responsibility)}</p><span className="judex-co-meta">{n.positions.map(pid=>p.text(p.state.positions.find(v=>v.id===pid)?.name??"")).join(" · ")}</span></div>{i<f.nodes.length-1&&<ArrowRight size={16}/>}</div>)}</div><p className="judex-co-meta">{p.t("cpWorkflowHint")}</p></article>):<><h3>{p.t("cpLocalConnection")}</h3><p>{p.t("cpLocalHint")}</p><div className="judex-co-review-panel"><code>judex context get TASK</code><br/><code>judex report --task TASK --kind progress --file report.json</code></div></>}
 </Tabs.Panel></Tabs></Modal>;
}
