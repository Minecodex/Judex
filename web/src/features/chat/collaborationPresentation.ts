export function taskStatusKey(status:string){
 if(status==='blocked')return 'taskDetailsBlocked';if(status==='draft')return 'taskDetailsDraft';
 return status==="ready"?"coReady":status==="working"?"coWorking":status==="accepted"?"coAccepted":status==="rework"?"coRework":status==="delivered"?"coAwaitAcceptance":status==="cancelled"?"coCancelled":"coDraft";
}
