import {useState} from "react";
import {Button} from "../../components/ui/Button";
import {useWork} from "./store";
import {ownsSeat} from "./selectors";
import {Dialog,Field} from "./ui";
import {TaskResponsibilityPicker} from "./TaskResponsibilityPicker";
import type {Task} from "./types";

export function TaskStartDialog({task,onClose}:{task:Task;onClose:()=>void}){
 const {state,t,text,act}=useWork(),[identityId,setIdentity]=useState(""),[busy,setBusy]=useState(false);
 const valid=task.seatIds.includes(identityId)&&ownsSeat(state,identityId);
 const start=async()=>{if(!valid||busy)return;setBusy(true);try{const result=await act("taskAction",{taskId:task.id,op:"start",identityId});if(result.ok)onClose();}finally{setBusy(false);}};
 return <Dialog title={t("workTaskStart")} onClose={onClose}><p>{text(task.title)}</p><p className="judex-modal-description">{t("coChooseStartResponsibility")}</p><Field label={t("workChooseSeat")}><TaskResponsibilityPicker task={task} value={identityId} onChange={setIdentity} testId="start-task-identity"/></Field><div className="judex-modal-actions"><Button variant="outline" onPress={onClose}>{t("cancel")}</Button><Button variant="primary" data-testid="confirm-start-task" disabled={!valid} isPending={busy} onPress={()=>void start()}>{t("workTaskStart")}</Button></div></Dialog>;
}
