import fs from 'node:fs';

// Public actual work snapshot, not a database fixture or locally made proposal.
export function assertAP1Requirements(review){
 const work=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/A-P1-approved.json','utf8'));
 const byCode=code=>{
  const task=work.tasks.find(t=>t.title.startsWith(code+' '));
  if(!task||task.planId!==work.plan.id)throw new Error('Actual planned task missing '+code);
  return task;
 };
 const dependencies={A12:['A11'],A13:['A11'],A14:['A12'],A15:['A12','A13'],A16:['A14','A15']};
 if(review.changes.length!==5)throw new Error('Exactly five existing-task requirements changes required');
 for(const [code,sources] of Object.entries(dependencies)){
  const task=byCode(code),changes=review.changes.filter(c=>c.targetId===task.id);
  if(changes.length!==1)throw new Error('One requirement change per existing task required '+code);
  const c=changes[0],reqs=c.fields.requirements;
  if(c.operation!=='set_requirements'||c.targetType!=='task'||!Array.isArray(reqs)||reqs.length!==sources.length||Object.keys(c.fields).some(k=>k!=='requirements'))throw new Error('Unexpected scope or requirements '+code);
  const expected=sources.map(s=>byCode(s).id).sort();
  if(JSON.stringify(reqs.map(r=>r.targetId).sort())!==JSON.stringify(expected)||reqs.some(r=>r.kind!=='task_acceptance'||r.phase!=='start'||r.hard!==true||!r.label?.trim()))throw new Error('Required accepted-result dependencies differ '+code);
 }
 return work;
}
