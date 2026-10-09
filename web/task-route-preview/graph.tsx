import {Card} from '@heroui/react';
import {useId,useLayoutEffect,useRef} from 'react';
import {CircleCheck,Clock,CornerDownRight,Lock,Maximize2,Minus,Plus,Scan,LocateFixed,GitBranch} from 'lucide-react';
import {usePreview} from './context';
import {effectivePredecessors,taskStage,unmet,type Plan,type Task} from './model';
import {Button,Person,Status,TaskMenu} from './ui';
function layout(tasks:Task[]){
 const css=getComputedStyle(document.documentElement),size=(name:string)=>parseFloat(css.getPropertyValue('--d4-graph-'+name));
 const width=size('width'),height=size('height'),column=size('column'),row=size('row'),pad=size('pad'),heading=size('heading'),ranks=new Map<string,number>();
 const rank=(t:Task,seen=new Set<string>()):number=>{if(ranks.has(t.id))return ranks.get(t.id)!;if(seen.has(t.id))return 0;seen.add(t.id);const r=Math.max(-1,...t.before.map(id=>{const prev=tasks.find(v=>v.id===id);return prev?rank(prev,new Set(seen)):-1;}))+1;ranks.set(t.id,r);return r;};
 const rows=new Map<number,number>(),positions=new Map<string,{x:number;y:number;rank:number}>();
 tasks.forEach(t=>{const r=rank(t),n=rows.get(r)??0;rows.set(r,n+1);positions.set(t.id,{x:pad+r*column,y:pad+heading+n*row,rank:r});});
 return {width,height,pad,positions,canvasWidth:Math.max(1,...rows.keys())*column+width+pad*2,canvasHeight:Math.max(1,...rows.values())*row+pad+heading};
}
export function TaskCard({task,readOnly=false}:{task:Task;readOnly?:boolean}){
 const p=usePreview(),status=taskStage(p.state,task),waiting=unmet(p.state,task),hint=task.skip?p.t('d4SkippedHint'):status==='blocked'?p.t('d4WaitFor',{title:p.text(waiting[0])}):p.t(status==='accepted'?'d4AcceptedHint':status==='draft'?'d4DraftHint':status==='working'?'d4WorkingHint':'d4CanStart');
 const Icon=task.skip?CornerDownRight:status==='accepted'?CircleCheck:status==='blocked'?Lock:Clock;
 return <Card className={'judex-d4-task-card'+(task.skip?' judex-d4-task-skipped':'')+(p.drawer?.id===task.id?' judex-d4-task-selected':'')} data-testid={'d4-task-'+task.id} data-stage={status}>
  <div className="judex-d4-card-top"><Status status={status}/><TaskMenu task={task} readOnly={readOnly}/></div>
  <Button variant="ghost" className="judex-d4-card-body" aria-label={p.t('d4ViewTask',{title:p.text(task.title)})} onPress={()=>p.setDrawer({id:task.id,section:'overview'})}><strong>{p.text(task.title)}</strong><Person id={task.owner}/><p className="judex-d4-card-description">{p.text(task.description)||p.t('d4NoDescription')}</p><span className="judex-d4-card-hint"><Icon/>{hint}</span></Button>
  <div className="judex-d4-card-footer"><Button size="sm" variant="ghost" onPress={()=>p.discuss(task)}><MessageIcon/>{p.t('d4TaskDiscuss')}</Button></div>
 </Card>;
}
import {MessageCircle as MessageIcon} from 'lucide-react';
export function Graph({plan,full=false,readOnly=false}:{plan:Plan;full?:boolean;readOnly?:boolean}){
 const p=usePreview(),tasks=p.state.tasks.filter(t=>t.planId===plan.id),g=layout(tasks),v=p.views[plan.id]??{zoom:1,left:0,top:0},viewport=useRef<HTMLDivElement>(null),arrow='arrow-'+useId().replaceAll(':',''),drag=useRef<{x:number;y:number;left:number;top:number;pointer:number}|null>(null);
 useLayoutEffect(()=>{const el=viewport.current;if(el){el.scrollLeft=v.left;el.scrollTop=v.top;}},[plan.id,full]);
 useLayoutEffect(()=>{const target=p.viewer?.focus;if(target)viewport.current?.querySelector('[data-testid="d4-task-'+target+'"]')?.scrollIntoView({block:'nearest',inline:'center'});},[p.viewer?.focus]);
 const fit=()=>{const el=viewport.current;if(el)p.setView(plan.id,{zoom:Math.min(1,(el.clientWidth-g.pad*2)/g.canvasWidth,(el.clientHeight-g.pad)/g.canvasHeight),left:0,top:0});};
 const locate=()=>{const id=p.drawer?.id??tasks.find(t=>!t.skip&&t.status==='working')?.id??tasks[0]?.id;if(id)viewport.current?.querySelector('[data-testid="d4-task-'+id+'"]')?.scrollIntoView({block:'center',inline:'center',behavior:'smooth'});};
 const originals=tasks.flatMap(t=>t.before.map(id=>({from:id,to:t.id,original:true}))).filter(e=>tasks.find(t=>t.id===e.from)?.skip||tasks.find(t=>t.id===e.to)?.skip);
 const edges=[...tasks.filter(t=>!t.skip).flatMap(t=>effectivePredecessors(p.state,t).map(id=>({from:id,to:t.id,original:false}))),...originals];
 return <section className="judex-d4-graph" aria-label={p.t('d4Route')}><div className="judex-d4-graph-toolbar"><span><GitBranch/>{p.t('d4Route')}</span><div className="judex-d4-actions"><Button size="sm" isIconOnly aria-label={p.t('d4ZoomOut')} onPress={()=>p.setView(plan.id,{zoom:Math.max(.35,v.zoom-.1)})}><Minus/></Button><span className="judex-d4-zoom">{Math.round(v.zoom*100)}%</span><Button size="sm" isIconOnly aria-label={p.t('d4ZoomIn')} onPress={()=>p.setView(plan.id,{zoom:Math.min(1.2,v.zoom+.1)})}><Plus/></Button><Button size="sm" onPress={fit}><Scan/>{p.t('d4Fit')}</Button><Button size="sm" isIconOnly aria-label={p.t('d4Locate')} onPress={locate}><LocateFixed/></Button>{!full&&<Button size="sm" variant="outline" data-testid="d4-fullscreen" onPress={()=>p.openViewer(plan.id,false)}><Maximize2/>{p.t('d4FullScreen')}</Button>}</div></div>
  <div className="judex-d4-graph-viewport" data-testid="d4-graph-viewport" ref={viewport} onScroll={e=>{const el=e.currentTarget;p.setView(plan.id,{left:el.scrollLeft,top:el.scrollTop});}} onPointerDown={e=>{if((e.target as Element).closest('button,article,.card'))return;const el=e.currentTarget;drag.current={x:e.clientX,y:e.clientY,left:el.scrollLeft,top:el.scrollTop,pointer:e.pointerId};el.setPointerCapture(e.pointerId);el.classList.add('judex-d4-dragging');}} onPointerMove={e=>{if(drag.current){e.currentTarget.scrollLeft=drag.current.left+drag.current.x-e.clientX;e.currentTarget.scrollTop=drag.current.top+drag.current.y-e.clientY;}}} onPointerUp={e=>{if(drag.current){e.currentTarget.releasePointerCapture(drag.current.pointer);drag.current=null;e.currentTarget.classList.remove('judex-d4-dragging');}}}>
   <div style={{width:g.canvasWidth*v.zoom,height:g.canvasHeight*v.zoom}}><div className="judex-d4-graph-canvas" style={{width:g.canvasWidth,height:g.canvasHeight,transform:`scale(${v.zoom})`}}>
    {[...new Set([...g.positions.values()].map(v=>v.rank))].map(rank=><span key={rank} className="judex-d4-stage" style={{left:g.pad+rank*parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--d4-graph-column'))}}>{p.t('d4Stage',{n:rank+1})}</span>)}
    <svg className="judex-d4-edges" width={g.canvasWidth} height={g.canvasHeight} aria-hidden="true"><defs><marker id={arrow} viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto"><path d="M 0 0 L 10 5 L 0 10 z"/></marker></defs>{edges.map(e=>{const a=g.positions.get(e.from),b=g.positions.get(e.to);if(!a||!b)return null;const x=a.x+g.width,y=a.y+g.height/2,end=b.y+g.height/2,bypass=!e.original&&b.rank-a.rank>1,path=bypass?`M ${x} ${y} C ${x+20} ${y}, ${x+20} ${g.pad}, ${x+44} ${g.pad} L ${b.x-44} ${g.pad} C ${b.x-20} ${g.pad}, ${b.x-20} ${end}, ${b.x} ${end}`:`M ${x} ${y} C ${x+24} ${y},${b.x-24} ${end},${b.x} ${end}`;return <path key={e.from+e.to+String(e.original)} data-from={e.from} data-to={e.to} data-original={e.original} className={e.original?'judex-d4-edge-original':''} d={path} markerEnd={`url(#${arrow})`}/>;})}</svg>
    {tasks.map(task=>{const pos=g.positions.get(task.id)!;return <div className="judex-d4-node" key={task.id} style={{left:pos.x,top:pos.y,width:g.width,height:g.height}}><TaskCard task={task} readOnly={readOnly}/></div>;})}
   </div></div>
  </div><div className="judex-d4-graph-footnote"><span>{p.t('d4GraphHint')}</span><span>{tasks.some(t=>t.skip)?p.t('d4OriginalEdges'):p.t('d4Pan')}</span></div>
 </section>;
}
