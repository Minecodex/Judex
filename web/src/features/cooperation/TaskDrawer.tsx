import {X} from 'lucide-react';
import {useEffect} from 'react';
import {Button} from '../../components/ui/Button';
import {UIWarning} from '../../components/ui/FormControls';
import {useWork} from '../work/store';
import {useTaskDetail} from '../work/useTaskDetail';
import {TaskInspector} from '../chat/TaskInspector';
import type {Task} from '../work/types';
import type {TaskSection} from '../work/runtimeTypes';
export function TaskDrawer({task:initial,taskId,pending=false,section='overview',onSection,onClose,readOnly=false,readingScope,onTask}:{task?:Task;taskId?:string;pending?:boolean;section?:TaskSection;onSection:(s:TaskSection)=>void;onClose:()=>void;readOnly?:boolean;readingScope?:string;onTask?:(id:string)=>void}){
 const detail=useTaskDetail(taskId??initial?.id,initial),task=detail.task;
 useEffect(()=>{const key=(e:KeyboardEvent)=>{if(e.key==='Escape'&&!document.querySelector('[data-slot="modal-dialog"],[role="alertdialog"],.judex-route-fullscreen')){e.preventDefault();onClose();}};document.addEventListener('keydown',key);return()=>document.removeEventListener('keydown',key);},[onClose]);
 const {t}=useWork();return <aside className="judex-task-drawer" aria-label={t('coMoreDetails')} data-testid="task-details-drawer"><div className="judex-task-drawer-top"><span>{t('coMoreDetails')}</span><Button size="sm" isIconOnly data-testid="close-task-drawer" aria-label={t('close')} onPress={onClose}><X/></Button></div>{detail.failed?<UIWarning>{t(detail.unavailable?'coopTaskNotFound':'errNetwork')}<Button size="sm" onPress={detail.retry}>{t('shellRetry')}</Button></UIWarning>:pending||detail.pending?<p role="status">{t('shellLoading')}</p>:task?<TaskInspector task={task} section={section} onSection={onSection} navigation readOnly={readOnly} readingScope={readingScope} onTask={onTask}/>:<UIWarning>{t('coopTaskNotFound')}</UIWarning>}</aside>;
}
