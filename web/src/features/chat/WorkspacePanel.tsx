import {PlanContext} from '../cooperation/PlanContext';
import {ConversationContext} from '../cooperation/ConversationContext';
import {TaskInspector} from './TaskInspector';
import {UIWarning} from '../../components/ui/FormControls';
import {Button} from '../../components/ui/Button';
import {useWork} from '../work/store';
import {ResourcesPage} from '../work/ProjectPages';
export function WorkspacePanel(){
 const {state,route,project,t,dataPending,dataRetry}=useWork();
 if(dataPending)return <p role="status">{t('shellLoading')}</p>;
 if(route.view==='resources')return <ResourcesPage/>;
 if(route.view==='task'||route.view==='plan'){
  const task=state.tasks.find(v=>v.id===route.id&&v.projectId===project.id),plan=state.plans.find(v=>v.id===route.id&&v.projectId===project.id);
  return route.view==='task'&&task?<TaskInspector key={task.id} task={task}/>:route.view==='plan'&&plan?<PlanContext plan={plan}/>:<UIWarning>{t('coopTaskNotFound')}{dataRetry&&<Button onPress={dataRetry}>{t('shellRetry')}</Button>}</UIWarning>;
 }
 return <ConversationContext/>;
}
