import {ownsSeat} from "./selectors.ts";
import type {WorkState,Task} from "./types.ts";

export function handoffResponsibilities(state:WorkState,projectId:string){
 const project=state.projects.find(p=>p.id===projectId);
 return state.seats.filter(seat=>(!seat.status||seat.status==="active")&&state.positions.some(p=>p.id===seat.positionId&&p.projectId===projectId)&&project?.members.some(m=>state.currentUserId?!!seat.userId&&m.userId===seat.userId:!!seat.person&&m.name===seat.person)).map(s=>s.id);
}
export function defaultHandoffSender(state:WorkState,task:Task){
 const valid=handoffResponsibilities(state,task.projectId),participants=task.seatIds.filter(id=>valid.includes(id)),own=participants.filter(id=>ownsSeat(state,id));
 return own.length===1?own[0]:own.length===0&&participants.length===1?participants[0]:"";
}
