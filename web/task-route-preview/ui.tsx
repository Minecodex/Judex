import {Dropdown,Label} from '@heroui/react';
import {ArrowLeft,Check,ChevronDown,Ellipsis,FileText,GitBranch,ListChecks,MessageCircle,Moon,Pencil,RotateCcw,SkipForward,Sun,Trash2} from 'lucide-react';
import type {ReactNode} from 'react';
import {Button} from '../src/components/ui/Button';
import {Brand,PersonAvatar} from '../src/components/ui/Presentation';
import {UIStatus} from '../src/components/ui/FormControls';
import {usePreferences} from '../src/stores/preferences';
import {canEdit,people,projectNames,type Plan,type Section,type Task} from './model';
import {usePreview} from './context';
export {Button,PersonAvatar};
export type MenuItem={id:string;label:string;icon?:ReactNode;danger?:boolean;disabled?:boolean;run:()=>void};
export function Menu({items,label,testId,children}:{items:MenuItem[];label:string;testId?:string;children?:ReactNode}){
 return <Dropdown><Button size="sm" isIconOnly={!children} aria-label={label} data-testid={testId}>{children??<Ellipsis/>}</Button><Dropdown.Popover className="judex-d4-menu"><Dropdown.Menu aria-label={label} disabledKeys={items.filter(v=>v.disabled).map(v=>v.id)} onAction={key=>items.find(v=>v.id===key)?.run()}>{items.map(v=><Dropdown.Item id={v.id} key={v.id} textValue={v.label} aria-label={v.label} variant={v.danger?'danger':undefined}>{v.icon}<Label>{v.label}</Label></Dropdown.Item>)}</Dropdown.Menu></Dropdown.Popover></Dropdown>;
}
export function Header({reset}:{reset:()=>void}){
 const p=usePreview(),prefs=usePreferences(),project=p.page.projectId??p.state.plans.find(v=>v.id===p.page.planId)?.projectId??'product';
 return <header className="judex-d4-header"><div className="judex-d4-header-path"><Button className="judex-d4-brand" onPress={()=>p.go({kind:'projects'})} aria-label="Judex"><Brand small/></Button><span className="judex-d4-header-separator"/><Button size="sm" onPress={()=>p.go({kind:'plans',projectId:project})}>{p.text(projectNames[project as keyof typeof projectNames])}</Button><span className="judex-d4-preview-label">{p.t('d4Preview')}</span></div><div className="judex-d4-header-actions"><Menu label={p.t('d4DraftExample')} testId="d4-examples" items={[{id:'route',label:p.t('d4Route'),icon:<GitBranch/>,run:()=>p.go({kind:'route',planId:'launch'})},{id:'chat',label:p.t('d4ChatExample'),icon:<MessageCircle/>,run:()=>p.go({kind:'chat',planId:'launch',taskId:'review'})},{id:'draft',label:p.t('d4DraftExample'),icon:<Pencil/>,run:()=>p.go({kind:'route',planId:p.state.plans.some(v=>v.id==='quality')?'quality':'launch'})}]}><ListChecks/><ChevronDown/></Menu><Button size="sm" data-testid="d4-language" onPress={()=>prefs.setLocale(p.locale==='en'?'zh-CN':'en')}>{p.locale==='en'?'中文':'EN'}</Button><Button size="sm" isIconOnly aria-label={p.t('d4Theme')} data-testid="d4-theme" onPress={()=>prefs.setTheme(prefs.theme==='dark'?'light':'dark')}>{prefs.theme==='dark'?<Sun/>:<Moon/>}</Button><Menu label={p.t('d4Account')} testId="d4-account" items={[{id:'person',label:p.t(p.state.role==='manager'?'d4SwitchMember':'d4SwitchManager'),icon:<PersonAvatar name={p.text(people[p.state.role==='manager'?'member':'manager'])} small/>,run:()=>p.setState(s=>({...s,role:s.role==='manager'?'member':'manager'}))},{id:'reset',label:p.t('d4Reset'),icon:<RotateCcw/>,run:reset}]}><PersonAvatar name={p.text(people[p.state.role])} small/><span>{p.text(people[p.state.role])}</span><ChevronDown/></Menu></div></header>;
}
export function Status({status}:{status:string}){
 const p=usePreview();const key=({draft:'d4Draft',ready:'d4Ready',working:'d4Working',delivered:'d4Delivered',accepted:'d4Accepted',blocked:'d4Blocked',skipped:'d4Skipped',active:'d4Active',rework:'d4Rework'} as const)[status as 'draft']??'d4Ready';
 return <UIStatus className={'judex-d4-status judex-d4-status-'+status}>{p.t(key)}</UIStatus>;
}
export function Person({id}:{id:string}){const p=usePreview(),name=p.text(people[id as keyof typeof people]??id);return <span className="judex-d4-person"><PersonAvatar name={name} small/><span>{name}</span></span>;}
export function Heading({title,description,back,actions}:{title:string;description?:string;back?:{label:string;run:()=>void};actions?:ReactNode}){
 return <div className="judex-d4-heading">{back&&<Button size="sm" className="judex-d4-back" onPress={back.run}><ArrowLeft/>{back.label}</Button>}<div className="judex-d4-heading-row"><div><h1>{title}</h1>{description&&<p>{description}</p>}</div><div className="judex-d4-actions">{actions}</div></div></div>;
}
export function TaskMenu({task,onSection,readOnly=false}:{task:Task;onSection?:(section:Section)=>void;readOnly?:boolean}){
 const p=usePreview();if(readOnly)return null;
 const open=(section:Section)=>onSection?onSection(section):p.setDrawer({id:task.id,section});
 const items:MenuItem[]=[{id:'records',label:p.t('d4Records'),icon:<FileText/>,run:()=>open('records')},{id:'flow',label:p.t('d4Flow'),icon:<GitBranch/>,run:()=>open('flow')}];
 if(p.state.changes.some(c=>c.type==='task'&&c.id===task.id))items.push({id:'review-change',label:p.t('d4ReviewChange'),icon:<Pencil/>,run:()=>p.setModal({kind:'review',type:'task',id:task.id})});
 if(canEdit(p.state,task)&&task.status!=='accepted'&&!task.skip)items.push({id:'edit',label:p.t(task.status==='draft'?'d4EditTask':'d4ChangeTask'),icon:<Pencil/>,run:()=>p.setModal({kind:'edit',type:'task',id:task.id})});
 if(p.state.role==='manager'&&task.skip)items.push({id:'restore',label:p.t('d4Restore'),icon:<RotateCcw/>,run:()=>p.setModal({kind:'restore',id:task.id})});
 if(p.state.role==='manager'&&!task.skip&&['working','ready','rework'].includes(task.status))items.push({id:'skip',label:p.t('d4Skip'),icon:<SkipForward/>,run:()=>p.setModal({kind:'skip',id:task.id})});
 if(task.status==='draft'&&canEdit(p.state,task))items.push({id:'delete',label:p.t('d4DeleteTask'),icon:<Trash2/>,danger:true,run:()=>p.setModal({kind:'delete',type:'task',id:task.id})});
 return <Menu items={items} label={p.t('d4TaskMenu',{title:p.text(task.title)})} testId={'d4-task-menu-'+task.id}/>;
}
export function PlanMenu({plan}:{plan:Plan}){
 const p=usePreview();if(!canEdit(p.state,plan)||plan.status==='accepted')return null;
 const items:MenuItem[]=[{id:'edit',label:p.t(plan.status==='draft'?'d4EditPlan':'d4ChangePlan'),icon:<Pencil/>,run:()=>p.setModal({kind:'edit',type:'plan',id:plan.id})}];
 if(p.state.changes.some(c=>c.type==='plan'&&c.id===plan.id))items.push({id:'review-change',label:p.t('d4ReviewChange'),icon:<Pencil/>,run:()=>p.setModal({kind:'review',type:'plan',id:plan.id})});
 if(plan.status==='draft')items.push({id:'delete',label:p.t('d4DeletePlan'),icon:<Trash2/>,danger:true,run:()=>p.setModal({kind:'delete',type:'plan',id:plan.id})});
 return <Menu label={p.t('d4PlanMenu',{title:p.text(plan.title)})} items={items} testId={'d4-plan-menu-'+plan.id}/>;
}
export function PendingChange({id,type}:{id:string;type:'task'|'plan'}){const p=usePreview();return p.state.changes.some(c=>c.id===id&&c.type===type)?<Button size="sm" className="judex-d4-pending" onPress={()=>p.setModal({kind:'review',type,id})}><Pencil/>{p.t('d4PendingChange')}</Button>:null;}
