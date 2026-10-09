import {TaskRecords} from "./TaskRecords";
import {TaskInspectorData} from './TaskInspectorData';
import {useWork} from "../work/store";
import type {Task} from "../work/types";
import type {TaskSection} from '../work/runtimeTypes';

export function TaskInspector({task,section,onSection,readOnly=false,navigation=false,readingScope,onTask}:{task:Task;section?:TaskSection;onSection?:(s:TaskSection)=>void;readOnly?:boolean;navigation?:boolean;readingScope?:string;onTask?:(id:string)=>void}){
 const {mode,project,state,route}=useWork();
 const scope=JSON.stringify([mode,project.id,state.currentUserId??state.currentUser,task.id]);
 return <TaskInspectorData key={scope} task={task}><TaskRecords task={task} section={section??(route.activityId?'records':route.taskSection??'overview')} onSection={onSection} readOnly={readOnly} navigation={navigation} readingScope={readingScope} onTask={onTask}/></TaskInspectorData>;
}
