import {Button} from "../../components/ui/Button";
import {EmptyState} from "../../components/ui/Presentation";
import {useWork} from "../work/store";
import {PlanContext} from "./PlanContext";
import {TaskInspector} from "../chat/TaskInspector";
export function ConversationContext(){
 const {state,route,t,text,go}=useWork();
 const topic=state.topics.find(v=>v.id===route.conversation);
 const task=state.tasks.find(v=>v.id===route.scopeTaskId);
 const plan=state.plans.find(v=>v.id===route.scopePlanId);
 if(task)return <TaskInspector key={task.id} task={task}/>;
 if(plan)return <PlanContext plan={plan}/>;
 const plans=state.plans.filter(v=>topic?.planIds.includes(v.id)),tasks=state.tasks.filter(v=>topic?.taskIds.includes(v.id));
 return <section className="judex-collab-inspector-content"><h3>{t("coopLinkObjects")}</h3>{plans.map(v=><Button key={v.id} variant="outline" onPress={()=>go({view:"plan",id:v.id})}>{text(v.title)}</Button>)}{tasks.map(v=><Button key={v.id} variant="outline" onPress={()=>go({view:"task",id:v.id})}>{text(v.title)}</Button>)}{!plans.length&&!tasks.length&&<EmptyState title={t("coopNoContext")}><Button variant="outline" onPress={()=>go({page:"hub",hubTab:"plans",view:"plans",id:undefined})}>{t("coopBackHub")}</Button></EmptyState>}</section>;
}
