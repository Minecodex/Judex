import {useState} from "react";
import {Button} from "@heroui/react";
import {useQuery,useMutation,useQueryClient} from "@tanstack/react-query";
import {FormField,UIInput,UITextArea,UISelect,UIOption,UICheckbox,UIWarning} from "../../components/ui/FormControls";
import {request} from "../../lib/api/client";
import {usePreferences} from "../../stores/preferences";
import {translate,type Key} from "../../i18n";
type Node={id:string;name:string;responsibility:string;allowedPositionIds:string[];delegationUserIds?:string[];defaultApprovalPolicy:string};
type Rule={nodeId?:string;kind:string;phase:string;targetId?:string};
type Body={name:string;instructions:string;nodes:Node[];hardRules:Rule[];advisoryEdges:{from:string;to:string}[];approvalPolicies:Record<string,string>};
type Version={id:string;revision:number;state:string;draftHash:string;body:Body};
const empty=():Body=>({name:"",instructions:"",nodes:[],hardRules:[],advisoryEdges:[],approvalPolicies:{}});
export function WorkflowEditor({projectId,manage,owner}:{projectId:string;manage:boolean;owner:boolean}){
 const {locale}=usePreferences();const t=(key:Key)=>translate(locale,key);const client=useQueryClient();const prefix=`/projects/${projectId}/workflows`;
 const [selected,setSelected]=useState("");const [body,setBody]=useState<Body|null>(null);const [snapshot,setSnapshot]=useState<Version|null>(null);
 const workflows=useQuery({queryKey:["workflows",projectId],queryFn:()=>request<{items:{id:string;name:string;version:number}[]}>(prefix)});
 const versions=useQuery({queryKey:["workflowVersions",projectId,selected],enabled:!!selected,queryFn:()=>request<{items:Version[]}>(`${prefix}/${selected}/versions`)});
 const positions=useQuery({queryKey:["positions",projectId],queryFn:()=>request<{items:{id:string;name:string}[]}>(`/projects/${projectId}/positions`)});
 const members=useQuery({queryKey:["members",projectId],queryFn:()=>request<{items:{userId:string;displayName:string}[]}>(`/projects/${projectId}/members`)});
 const definition=workflows.data?.items?.find((w)=>w.id===selected);
 const save=useMutation({mutationFn:()=>request(selected?`${prefix}/${selected}/draft`:prefix,{method:selected?"PUT":"POST",body:JSON.stringify({...body,...(selected?{expectedVersion:definition?.version}:{})})}),onSuccess:()=>{setBody(null);setSnapshot(null);void client.invalidateQueries({queryKey:["workflows",projectId]});void client.invalidateQueries({queryKey:["workflowVersions",projectId]});}});
 const publish=useMutation({mutationFn:()=>request(`${prefix}/${selected}/publish`,{method:"POST",body:JSON.stringify({expectedVersion:definition?.version,draftHash:snapshot?.draftHash})}),onSuccess:()=>{setSnapshot(null);void client.invalidateQueries({queryKey:["workflows",projectId]});void client.invalidateQueries({queryKey:["workflowVersions",projectId]});}});
 const setNode=(index:number,change:Partial<Node>)=>setBody((v)=>v&&({...v,nodes:v.nodes.map((n,i)=>i===index?{...n,...change}:n)}));
 const toggle=(items:string[],id:string)=>items.includes(id)?items.filter((v)=>v!==id):[...items,id];
 return <section className="judex-panel-stack">
  <h3>{t("paWorkflow")}</h3>{manage&&<Button variant="secondary" onPress={()=>{setSelected("");setSnapshot(null);setBody(empty());save.reset();}}>{t("weNew")}</Button>}
  {workflows.data?.items?.map((w)=><Button key={w.id} variant="ghost" onPress={()=>{setSelected(w.id);setSnapshot(null);setBody(null);}}>{w.name}</Button>)}
  {versions.data?.items?.map((v)=><Button key={v.id} variant="secondary" onPress={()=>{setSnapshot(v);setBody(null);}}>{t("weReview")} · v{v.revision} · {v.state}</Button>)}
  {snapshot&&<article className="judex-panel-stack" data-testid="workflow-review"><p>{snapshot.body.instructions}</p>{snapshot.body.nodes.map((n)=><div key={n.id}><strong>{n.name}</strong><p>{n.responsibility}</p><p>{t("wePositions")}: {n.allowedPositionIds.map((id)=>positions.data?.items?.find((p)=>p.id===id)?.name??id).join(" · ")}</p><p>{t("weDelegates")}: {(n.delegationUserIds??[]).map((id)=>members.data?.items?.find((m)=>m.userId===id)?.displayName??id).join(" · ")}</p></div>)}
   <ul>{snapshot.body.hardRules?.map((r,i)=><li key={i}>{r.nodeId} · {r.kind} · {r.phase} · {r.targetId}</li>)}</ul>
   {manage&&<Button variant="secondary" onPress={()=>setBody({...snapshot.body,name:definition?.name??snapshot.body.name})}>{t("weEdit")}</Button>}
   {manage&&snapshot.state==="draft"&&<Button isPending={publish.isPending} onPress={()=>publish.mutate()}>{t("paPublish")}</Button>}
  </article>}
  {body&&<form className="judex-auth-form" data-testid="workflow-editor" onSubmit={(e)=>{e.preventDefault();save.mutate();}}>
   <FormField label={t("paName")}><UIInput required value={body.name} onChange={(e)=>setBody({...body,name:e.target.value})}/></FormField>
   <FormField label={t("paInstructions")}><UITextArea value={body.instructions} onChange={(e)=>setBody({...body,instructions:e.target.value})}/></FormField>
   {body.nodes.map((node,index)=><section key={node.id} className="judex-panel-stack">
    <strong>{t("weNode")} {index+1}</strong><FormField label={t("paName")}><UIInput required value={node.name} onChange={(e)=>setNode(index,{name:e.target.value})}/></FormField>
    <FormField label={t("weDuty")}><UITextArea value={node.responsibility} onChange={(e)=>setNode(index,{responsibility:e.target.value})}/></FormField>
    <FormField label={t("wePolicy")}><UISelect value={body.approvalPolicies[node.id]??node.defaultApprovalPolicy} onChange={(e)=>{setNode(index,{defaultApprovalPolicy:e.target.value});setBody((v)=>v&&({...v,approvalPolicies:{...v.approvalPolicies,[node.id]:e.target.value}}));}}><UIOption value="all">{t("weAll")}</UIOption><UIOption value="none">{t("weNone")}</UIOption></UISelect></FormField>
    <strong>{t("wePositions")}</strong>{positions.data?.items?.map((p)=><UICheckbox key={p.id} checked={node.allowedPositionIds.includes(p.id)} onChange={()=>setNode(index,{allowedPositionIds:toggle(node.allowedPositionIds,p.id)})}>{p.name}</UICheckbox>)}
    <strong>{t("weDelegates")}</strong><small>{t("weOwnerGrant")}</small>{members.data?.items?.map((m)=><UICheckbox key={m.userId} disabled={!owner} checked={(node.delegationUserIds??[]).includes(m.userId)} onChange={()=>setNode(index,{delegationUserIds:toggle(node.delegationUserIds??[],m.userId)})}>{m.displayName}</UICheckbox>)}
   </section>)}
   <Button variant="secondary" onPress={()=>setBody({...body,nodes:[...body.nodes,{id:"node_"+crypto.randomUUID().slice(0,8),name:"",responsibility:"",allowedPositionIds:[],delegationUserIds:[],defaultApprovalPolicy:"all"}]})}>{t("weAddNode")}</Button>
   {body.hardRules.map((rule,index)=><section key={index} className="judex-panel-stack"><strong>{t("weRule")}</strong>
    <UISelect aria-label={t("weNode")} value={rule.nodeId??""} onChange={(e)=>setBody({...body,hardRules:body.hardRules.map((r,i)=>i===index?{...r,nodeId:e.target.value}:r)})}><UIOption value="">—</UIOption>{body.nodes.map((n)=><UIOption key={n.id} value={n.id}>{n.name}</UIOption>)}</UISelect>
    <UISelect aria-label={t("weRule")} value={rule.kind} onChange={(e)=>setBody({...body,hardRules:body.hardRules.map((r,i)=>i===index?{...r,kind:e.target.value}:r)})}><UIOption value="task_acceptance">{t("lcTaskAcceptance")}</UIOption><UIOption value="handoff_receipt">{t("lcReceiptHint")}</UIOption><UIOption value="material_ready">{t("accessMaterials")}</UIOption></UISelect>
    <UISelect aria-label={t("lcPhase")} value={rule.phase} onChange={(e)=>setBody({...body,hardRules:body.hardRules.map((r,i)=>i===index?{...r,phase:e.target.value}:r)})}><UIOption value="start">{t("lcStart")}</UIOption><UIOption value="accept">{t("lcAccept")}</UIOption><UIOption value="both">{t("lcBoth")}</UIOption></UISelect>
    <FormField label={t("weTarget")}><UIInput value={rule.targetId??""} onChange={(e)=>setBody({...body,hardRules:body.hardRules.map((r,i)=>i===index?{...r,targetId:e.target.value}:r)})}/></FormField>
    <Button variant="ghost" onPress={()=>setBody({...body,hardRules:body.hardRules.filter((_,i)=>i!==index)})}>{t("weRemove")}</Button>
   </section>)}
   <Button variant="secondary" onPress={()=>setBody({...body,hardRules:[...body.hardRules,{kind:"task_acceptance",phase:"start"}]})}>{t("weAddRule")}</Button>
   <Button type="submit" isPending={save.isPending} isDisabled={!body.name.trim()||!body.nodes.length||body.nodes.some((n)=>!n.name.trim())}>{t("weSave")}</Button>
  </form>}
  {save.isSuccess&&<p role="status">{t("weSaved")}</p>}{publish.isSuccess&&<p role="status">{t("wePublished")}</p>}
  {[workflows,versions,positions,members,save,publish].map((q,i)=>q.isError&&<UIWarning key={i}>{q.error?.message}</UIWarning>)}
 </section>;
}
