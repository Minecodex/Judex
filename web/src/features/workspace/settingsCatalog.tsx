import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {ArrowRight,BriefcaseBusiness,GitBranch,Layers3,Plus,Check} from 'lucide-react';
import {useWorkspace} from './context';
import {request} from '../../lib/api/client';
import {Button,CheckField,Choice,Empty,Field,QueryState} from './ui';
import {BackButton,Badge,SettingsHeading} from './settingsUI';
import {WorkflowDiagram} from './WorkflowDiagram';
import type {WorkflowGraph} from './workflowGraph';
import type {Position,Workflow} from './types';

type Text={zh:string;en:string};
export type PresetRole={id:string;name:Text;summary:Text;prompt:Text};
export type PresetFlow={id:string;name:Text;summary:Text;instructions:Text;complexity:string;roleIds:string[];nodes:{id:string;name:Text;phase:Text;responsibility:Text;kind:string}[];edges:{from:string;to:string;kind:string;label:Text}[]};
export type PresetCatalog={version:string;roles:PresetRole[];workflows:PresetFlow[];scenarios:{id:string;name:Text;description:Text;roleIds:string[];workflowIds:string[]}[]};
export function usePresetCatalog(){const w=useWorkspace();return useQuery({queryKey:['v2',w.user.id,'preset-catalog'],queryFn:()=>request<PresetCatalog>('/template-presets'),staleTime:300000,retry:false});}
const existing=(item:Position|Workflow,preset:PresetRole|PresetFlow)=>item.presetId===preset.id||[preset.name.zh,preset.name.en].some(name=>name.trim().toLowerCase()===item.name.trim().toLowerCase());
export function PresetLibrary({kind,onClose}:{kind:'positions'|'workflows';onClose:()=>void}){
 const w=useWorkspace(),catalog=usePresetCatalog(),[scenario,setScenario]=useState('all'),[search,setSearch]=useState(''),[selected,setSelected]=useState<string[]>([]),[preview,setPreview]=useState<string|null>(null);
 const role=kind==='positions',text=(value?:Text)=>value?.[w.locale==='en'?'en':'zh']||'',items=role?catalog.data?.roles:catalog.data?.workflows,projectItems=(role?w.positions.data:w.workflows.data)||[],active=catalog.data?.scenarios.find(s=>s.id===scenario);
 const added=(p:PresetRole|PresetFlow)=>projectItems.some(item=>existing(item,p));
 const visible=(items||[]).filter(p=>(!active||(role?active.roleIds:active.workflowIds).includes(p.id))&&(text(p.name)+' '+text(p.summary)).toLowerCase().includes(search.toLowerCase()));
 const chosen=selected.filter(id=>items?.some(p=>p.id===id&&!added(p))),detail=items?.find(p=>p.id===preview);
 const toggle=(id:string)=>setSelected(current=>current.includes(id)?current.filter(v=>v!==id):[...current,id]);
 const submit=async()=>{try{const result=await w.execute<{created:number;skipped:number}>(`/projects/${w.projectId}/template-presets`,{catalogVersion:catalog.data!.version,locale:w.locale,positionIds:role?chosen:[],workflowIds:role?[]:chosen});await w.refresh();w.setToast(w.t('stImportDone',result));onClose();}catch{}};
 return <div className={"judex-settings-page-body"+(!detail?" judex-settings-library":"")}><BackButton label={w.t(detail?'stBackLibrary':'stBackList')} onPress={()=>detail?setPreview(null):onClose()}/>
  <SettingsHeading title={detail?text(detail.name):w.t(role?'stBuiltinRoles':'stBuiltinFlows')} description={detail?text(detail.summary):w.t('stLibraryHint')}/>
  <QueryState query={catalog}>
   {detail?<div className="judex-settings-preset-preview">
    <Badge accent>{w.t('stReferenceOnly')}</Badge>
    {'prompt' in detail?<div className="judex-settings-prompt-read">{text(detail.prompt)}</div>:<><p className="judex-settings-note">{text(detail.instructions)}</p><WorkflowDiagram graph={{nodes:detail.nodes.map(n=>({...n,name:text(n.name),phase:text(n.phase),responsibility:text(n.responsibility),kind:(n.kind||'activity') as WorkflowGraph['nodes'][number]['kind']})),edges:detail.edges.map(e=>({...e,label:text(e.label),kind:(e.kind||'sequence') as 'sequence'|'feedback'}))}}/><details><summary>{w.t('wfNodeDetails')}</summary><div className="judex-settings-flow-steps">{detail.nodes.map((node,i)=><article key={node.id}><span className="judex-settings-step-number">{i+1}</span><div>{text(node.phase)&&<small>{text(node.phase)}</small>}<h3>{text(node.name)}</h3><p>{text(node.responsibility)}</p></div></article>)}</div></details><h3>{w.t('stRelationships')}</h3><div className="judex-settings-relationships">{detail.edges.map((edge,i)=><div key={i}><span>{text(detail.nodes.find(n=>n.id===edge.from)?.name)}</span><ArrowRight/><span>{text(detail.nodes.find(n=>n.id===edge.to)?.name)}</span>{text(edge.label)&&<small>{text(edge.label)}</small>}</div>)}</div><h3>{w.t('stRecommended')}</h3><div className="judex-settings-tags">{detail.roleIds?.map(id=><Badge key={id}>{text(catalog.data?.roles.find(r=>r.id===id)?.name)}</Badge>)}</div></>}
    <div className="judex-settings-savebar"><span>{w.t('stReferenceOnly')}</span><Button variant="primary" isDisabled={added(detail)||chosen.includes(detail.id)} onPress={()=>{toggle(detail.id);setPreview(null);}}>{w.t(added(detail)?'stInProject':chosen.includes(detail.id)?'stSelected':'stSelectTemplate',{count:1})}</Button></div>
   </div>:<><div className="judex-settings-library-filters"><Choice label={w.t('stScenario')} value={scenario} onChange={setScenario} items={[{id:'all',name:w.t('stAllScenarios')},...(catalog.data?.scenarios||[]).map(s=>({id:s.id,name:text(s.name)}))]}/><Field label={w.t('stTemplateSearch')} value={search} onChange={setSearch}/></div>
    {active&&<p className="judex-settings-note">{text(active.description)}</p>}
    <div className="judex-settings-toolbar"><span>{visible.length} {w.t(role?'vpRoles':'vpWorkflows')}</span><Button onPress={()=>setSelected(current=>Array.from(new Set([...current,...visible.filter(p=>!added(p)).map(p=>p.id)])))}>{w.t('stSelectAll')}</Button></div>
    <div className="judex-settings-catalog-grid">{visible.map(p=><article key={p.id} className={'judex-settings-catalog-card'+(chosen.includes(p.id)?' judex-settings-card-selected':'')}><div className="judex-settings-card-top"><span className="judex-settings-card-icon">{role?<BriefcaseBusiness/>:<GitBranch/>}</span>{added(p)?<Badge><Check/>{w.t('stInProject')}</Badge>:<CheckField selected={chosen.includes(p.id)} disabled={w.pending} onChange={()=>toggle(p.id)}>{text(p.name)}</CheckField>}</div><h3>{text(p.name)}</h3><p>{text(p.summary)}</p><footer>{'nodes' in p?<Badge>{w.t('stSteps',{count:p.nodes.length})}</Badge>:<Badge>{w.t('stBuiltin')}</Badge>}<Button onPress={()=>setPreview(p.id)} aria-label={w.t('stPreview')+' · '+text(p.name)}>{w.t('stPreview')}<ArrowRight/></Button></footer></article>)}</div>
    {!visible.length&&<Empty title={w.t('stNoMatches')}/>}
    <div className="judex-settings-savebar"><span>{w.t('stSelected',{count:chosen.length})}</span><Button isDisabled={!chosen.length} onPress={()=>setSelected([])}>{w.t('stClearSelection')}</Button><Button variant="primary" isDisabled={!chosen.length||chosen.length>200||w.pending} onPress={()=>void submit()}><Plus/>{w.t('stImport')}</Button></div>
   </>}
  </QueryState>
 </div>;
}
