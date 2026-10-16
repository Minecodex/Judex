export function overlappingFaults(begin,end,windows){
 if(!Number.isFinite(begin)||!Number.isFinite(end)||end<begin)throw Error('Invalid request interval');
 return windows.filter(w=>{const start=Date.parse(w.startedAt),closed=w.closedAt?Date.parse(w.closedAt):Infinity;if(!Number.isFinite(start)||Number.isNaN(closed)||closed<start)throw Error('Invalid registered fault interval');return start<=end&&closed>=begin;}).map(w=>w.id);
}
