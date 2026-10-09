import {ResponsibilityPicker} from "./ResponsibilityPicker";
import {useWork} from "./store";
import {ownsSeat} from "./selectors";
import type {Task} from "./types";

export function TaskResponsibilityPicker({task,value,onChange,testId="task-responsibility"}:{task:Task;value:string;onChange:(id:string)=>void;testId?:string}){
 const {state}=useWork();
 return <ResponsibilityPicker ids={task.seatIds.filter(id=>ownsSeat(state,id))} value={value} onChange={onChange} testId={testId}/>;
}
