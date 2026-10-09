import {useRef,useState} from "react";
import {useWork} from "../work/store";
import type {Plan,Task} from "../work/types";
export function useDiscussionNavigation(){
 const {act,go}=useWork(),request=useRef(0),[busy,setBusy]=useState<string>();
 const task=async(value:Task)=>{const version=++request.current;setBusy(value.id);try{const r=await act("ensureTaskMainTopic",{taskId:value.id},{toast:false});if(r.ok&&r.id&&version===request.current)go({page:"chat",view:"task",id:value.id,conversation:r.id,scopeTaskId:value.id,scopePlanId:undefined,taskContextId:value.id,settingsSection:undefined});}finally{if(version===request.current)setBusy(undefined);}};
 const plan=(value:Plan)=>{if(value.mainTopicId)go({page:"chat",view:"plan",id:value.id,conversation:value.mainTopicId,scopePlanId:value.id,scopeTaskId:undefined,taskContextId:undefined,settingsSection:undefined});};
 return {task,plan,busy};
}
