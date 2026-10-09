import {useWorkspaceDraft} from './useWorkspaceDraft';
import {useWork} from '../work/store';
import {emptyTaskRecordReading,taskRecordReadingSlot,type TaskRecordReading} from './taskReadingPosition';

export function useTaskRecordReading(taskId:string,scope='task'){
 const {mode}=useWork();
 return useWorkspaceDraft<TaskRecordReading>(taskRecordReadingSlot(mode,taskId,scope),emptyTaskRecordReading(),'task-record-reading-v1');
}
