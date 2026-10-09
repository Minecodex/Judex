import {Card} from "@heroui/react";
import {useEffect,useRef,useState} from "react";
import {GitBranch,MessageCircle,ArrowRight} from "lucide-react";
import {Button,PageTitle,PersonAvatar,Status} from "./ui";
import {usePreview} from "./context";
import {planTasks,type Task} from "./model";
export function TaskCard({task}:{task:Task}){
 const p=usePreview();
 return <Card className="judex-co-task-card" data-testid={"co-task-"+task.id}>
  <Button className="judex-co-card-hit" aria-label={p.t("cpTaskDetails")+" · "+p.text(task.title)} onPress={()=>p.setModal({kind:"task",taskId:task.id})}/>
  <Card.Header><div className="judex-co-card-top"><span className="judex-co-meta">{p.text(p.state.positions.find(v=>v.id===task.positionId)?.name??"")}</span><Status status={task.status}/></div><Card.Title>{p.text(task.title)}</Card.Title><Card.Description>{p.text(task.goal)}</Card.Description></Card.Header><Card.Content><div className="judex-co-task-person"><PersonAvatar name={p.person(task.makerId)} small/><span>{p.person(task.makerId)}</span></div>{task.dependsOn.length>0&&<div className="judex-co-meta">{task.dependsOn.map(id=>p.text(p.state.tasks.find(t=>t.id===id)?.title??"")).join(" · ")}</div>}</Card.Content><Card.Footer className="judex-co-task-footer"><span className="judex-co-meta">{p.state.activities.filter(r=>r.taskId===task.id).length} {p.t("cpReports")}</span><Button variant="outline" size="sm" data-testid={"co-task-discuss-"+task.id} onPress={()=>p.taskChat(task)}><MessageCircle size={15}/>{p.t("cpDiscuss")}</Button></Card.Footer>
 </Card>;
}
export function TaskMap({planId,compact=false}:{planId:string;compact?:boolean}){
 const p=usePreview(),tasks=planTasks(p.state,planId),cache=new Map<string,number>();
 const root=useRef<HTMLDivElement>(null),[edges,setEdges]=useState<{id:string;path:string}[]>([]);
 useEffect(()=>{
  const el=root.current;if(!el)return;
  const measure=()=>{
   const box=el.getBoundingClientRect(),out:{id:string;path:string}[]=[];
   for(const task of tasks)for(const parent of task.dependsOn){
    const a=el.querySelector<HTMLElement>('[data-testid="co-task-'+parent+'"]'),b=el.querySelector<HTMLElement>('[data-testid="co-task-'+task.id+'"]');
    if(!a||!b)continue;const left=a.getBoundingClientRect(),right=b.getBoundingClientRect();
    const x1=left.right-box.left,y1=left.top-box.top+left.height/2,x2=right.left-box.left,y2=right.top-box.top+right.height/2,mid=(x1+x2)/2;
    out.push({id:parent+":"+task.id,path:`M ${x1} ${y1} C ${mid} ${y1},${mid} ${y2},${x2} ${y2}`});
   }setEdges(out);
  };
  const observer=new ResizeObserver(measure);observer.observe(el);measure();return()=>observer.disconnect();
 },[planId,tasks.map(t=>t.id+":"+t.status).join("|"),p.locale]);
 function rank(task:Task,seen=new Set<string>()):number{
  if(cache.has(task.id))return cache.get(task.id)!;if(seen.has(task.id))return 0;
  const next=new Set(seen);next.add(task.id);const earlier=task.dependsOn.map(id=>tasks.find(t=>t.id===id)).filter((v):v is Task=>!!v);
  const value=earlier.length?Math.max(...earlier.map(t=>rank(t,next)))+1:0;cache.set(task.id,value);return value;
 }
 for(const t of tasks)rank(t);const levels=[...new Set(cache.values())].sort((a,b)=>a-b);
 return <div className={"judex-co-route-scroll"+(compact?" judex-co-route-compact":"")} data-testid="co-task-map"><div className="judex-co-route-columns" ref={root}><svg className="judex-co-route-lines" aria-hidden="true"><defs><marker id={"co-arrow-"+planId} markerWidth="7" markerHeight="7" refX="7" refY="3.5" orient="auto"><path d="M0 0 L7 3.5 L0 7z"/></marker></defs>{edges.map(edge=><path key={edge.id} d={edge.path} markerEnd={"url(#co-arrow-"+planId+")"}/>)}</svg>{levels.map((level,i)=><div className="judex-co-route-stage" key={level}><div className="judex-co-route-label"><span className="judex-co-stage-dot"/>{level+1}{i<levels.length-1&&<ArrowRight/>}</div><div className="judex-co-route-nodes">{tasks.filter(t=>cache.get(t.id)===level).map(task=><TaskCard key={task.id} task={task}/>)}</div></div>)}</div></div>;
}
export function PlanRoute(){
 const p=usePreview(),plan=p.state.plans.find(v=>v.id===p.route.planId)!;
 if(!plan)return null;
 return <main className="judex-co-main judex-co-route-page"><PageTitle title={p.text(plan.title)} hint={p.text(plan.goal)} back={{label:p.t("cpBackHub"),run:()=>p.go({page:"hub",projectId:plan.projectId,tab:"plans"})}} actions={<><Status status={plan.status}/><Button variant="primary" onPress={()=>p.planChat(plan)}><MessageCircle/>{p.t("cpPlanDiscussion")}</Button></>}/>
  <div className="judex-co-route-heading"><span><GitBranch size={17}/>{p.t("cpTaskRoute")}</span><p>{p.t("cpRouteHint")}</p></div>{plan.status==="draft"&&<div className="judex-co-info-band">{p.t("cpDraftHint")}</div>}<TaskMap planId={plan.id}/>
  <div className="judex-co-route-context"><span>{p.t("cpOwner")} · {p.person(plan.ownerId)}</span><span>{p.t("cpSourceFlow")} · {p.text(p.state.flows.find(f=>f.id===plan.workflowId)?.name??"")}</span></div>
 </main>;
}
