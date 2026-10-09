export function taskReadingKey(mode:string,project:string,user:string,task:string,section:string,scope='task'){
 return [scope,mode,project,user,task,section].join(':');
}

export type TaskRecordReading = {expanded:boolean;pages:number;source:string;originals:Record<string,boolean>};
export const emptyTaskRecordReading = ():TaskRecordReading => ({expanded:false,pages:1,source:'all',originals:{}});
export const taskRecordReadingSlot = (mode:string,task:string,scope='task') => 'task-record-reading:'+mode+':'+scope+':'+task;
export const taskRecordReadingStorage = (mode:string,project:string,user:string,task:string,scope='task') =>
 'judex.workspace.draft.'+project+':'+user+':'+taskRecordReadingSlot(mode,task,scope);

// A viewer receives a copy; edits inside it cannot overwrite the original surface.
export function copyTaskRecordReading(mode:string,project:string,user:string,task:string,from:string,to:string){
 try {
  const value=sessionStorage.getItem(taskRecordReadingStorage(mode,project,user,task,from));
  const key=taskRecordReadingStorage(mode,project,user,task,to);
  if(value)sessionStorage.setItem(key,value);else sessionStorage.removeItem(key);
 }catch{}
}
