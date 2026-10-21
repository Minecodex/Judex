import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {ArrowRight,BriefcaseBusiness,Clock,GitBranch,Layers3,Plus} from 'lucide-react';
import {useWorkspace} from './context';
import {Button,Choice,ConfigChange,Empty,Field,QueryState,Tabs} from './ui';
import {useConfigDraft} from './configDraft';
import {list} from './api';
import {BackButton,Badge,SettingsHeading} from './settingsUI';
import {PresetLibrary} from './settingsCatalog';
import {MarkdownReader} from '../../components/readers/MarkdownReader';
import {WorkflowEditor} from './WorkflowEditor';
import {WorkflowDiagram} from './WorkflowDiagram';
import {emptyGraph,type WorkflowGraph} from './workflowGraph';
import type {Model,Position,Workflow} from './types';

export function useModels(){const w=useWorkspace();return useQuery({queryKey:['v2',w.user.id,'models'],queryFn:()=>list<Model>('/models'),retry:false});}
export function ProjectTemplates({kind}:{kind:'positions'|'workflows'}){
 const w=useWorkspace(),[view,setView]=useState<'list'|'library'|'editor'>('list'),[selected,setSelected]=useState<string|null>(null),[search,setSearch]=useState('');
 const role=kind==='positions',query=role?w.positions:w.workflows,items=query.data||[],manager=w.project?.role!=='member',item=items.find(p=>p.id===selected),visible=items.filter(p=>(p.name+' '+('prompt' in p?p.prompt:p.content)).toLowerCase().includes(search.toLowerCase()));
 if(view==='library')return <PresetLibrary kind={kind} onClose={()=>setView('list')}/>;
 if(view==='editor')return <TemplateEditor key={selected||'new'} kind={kind} item={item} onBack={()=>setView('list')} onCreated={id=>setSelected(id)}/>;
 return <div className="judex-settings-page-body"><SettingsHeading title={w.t(role?'vpRoles':'vpWorkflows')} description={w.t(role?'stRoleHint':'stFlowHint')} actions={manager&&<><Button variant="outline" onPress={()=>setView('library')}><Layers3/>{w.t(role?'stBuiltinRoles':'stBuiltinFlows')}</Button><Button variant="primary" onPress={()=>{setSelected(null);setView('editor');}}><Plus/>{w.t(role?'stNewRole':'stNewFlow')}</Button></>}/>
  <QueryState query={query}>{items.length>0?<><Field label={w.t('stTemplateSearch')} value={search} onChange={setSearch}/><div className="judex-settings-template-grid">{visible.map(p=><article className="judex-settings-template-card" key={p.id}><header><span className="judex-settings-card-icon">{role?<BriefcaseBusiness/>:<GitBranch/>}</span><Badge>{w.t(p.presetId?'stBuiltin':'stCustom')}</Badge></header><h3>{p.name}</h3><p>{('prompt' in p?p.prompt:p.content)||w.t('stNoDescription')}</p><footer><small>{w.t('stRevision',{revision:p.revision})}</small><Button onPress={()=>{setSelected(p.id);setView('editor');}} aria-label={w.t(manager?'stEdit':'stPreview')+' · '+p.name}>{w.t(manager?'stEdit':'stPreview')}<ArrowRight/></Button></footer></article>)}</div>{!visible.length&&<Empty title={w.t('stNoMatches')}/>}</>:<div className="judex-settings-empty-library"><span className="judex-settings-card-icon">{role?<BriefcaseBusiness/>:<GitBranch/>}</span><h3>{w.t(role?'stNoRoles':'stNoFlows')}</h3><p>{w.t(role?'stNoRolesHint':'stNoFlowsHint')}</p>{manager&&<Button variant="primary" onPress={()=>setView('library')}><Layers3/>{w.t('stBrowseLibrary')}</Button>}</div>}</QueryState>
 </div>;
}
function TemplateHistory({kind,id}:{kind:string;id:string}){
 const w=useWorkspace(),versions=useQuery({queryKey:['v2',w.user.id,'template-versions',kind,id],queryFn:()=>list<{revision:number;prompt?:string;content?:string;graph?:WorkflowGraph;createdAt:string}>(`/projects/${w.projectId}/${kind}/${id}/versions`),retry:false});
 return <QueryState query={versions} empty={!versions.data?.length} emptyTitle={w.t('stNoHistory')}><p className="judex-settings-note">{w.t('stVersionHint')}</p><div className="judex-settings-history">{versions.data?.map(v=><details key={v.revision}><summary><Clock/><strong>{w.t('stRevision',{revision:v.revision})}</strong><small>{new Date(v.createdAt).toLocaleString(w.locale)}</small></summary><>{v.graph&&<WorkflowDiagram graph={v.graph}/>}<div className="judex-settings-prompt-read">{v.prompt||v.content}</div></></details>)}</div></QueryState>;
}
function TemplateEditor({kind,item,onBack,onCreated}:{kind:'positions'|'workflows';item?:Position|Workflow;onBack:()=>void;onCreated:(id:string)=>void}){
 const w=useWorkspace(),models=useModels(),[tab,setTab]=useState('edit'),role=kind==='positions',manager=w.project?.role!=='member';
 const config=useConfigDraft({graph:item&&'graph'in item?item.graph||emptyGraph():emptyGraph(),name:item?.name||'',content:item?('prompt'in item?item.prompt:item.content):'',modelId:item&&'modelId'in item?item.modelId||'project':'project'},item?.version||0);
 const {name,content,modelId,graph}=config.draft;
 const save=async()=>{try{const out=await w.execute<{id:string}>(`/projects/${w.projectId}/${kind}${item?'/'+item.id:''}`,{name,...(role?{prompt:content,modelId:modelId==='project'?null:modelId}:{content,graph}),...(item?{expectedVersion:config.baseVersion}:{})},item?'PATCH':'POST');if(item)config.replace(config.draft,config.baseVersion+1);else{await w.refresh();onCreated(out.id);}w.setToast(w.t('v2Saved'));}catch{}};
 return <div className="judex-settings-page-body"><BackButton label={w.t('stBackList')} onPress={onBack}/><SettingsHeading title={item?.name||w.t(role?'stNewRole':'stNewFlow')} description={w.t(role?'stPromptGuide':'stWorkflowGuide')}/><ConfigChange stale={config.stale} onReload={config.reload}/>
  <Tabs label={w.t(role?'vpRoles':'vpWorkflows')} value={tab} onChange={setTab} items={[{id:'edit',label:w.t('stEdit')},{id:'preview',label:w.t('stPreview')},...(item?[{id:'history',label:w.t('stHistory')}]:[])]}/>
  {tab==='history'&&item?<TemplateHistory kind={kind} id={item.id}/>:tab==='preview'?<div className="judex-settings-document">{!role&&<WorkflowDiagram graph={graph}/>}<MarkdownReader text={content||w.t('stNoDescription')}/></div>:!role?<WorkflowEditor templateKey={item?.id||'new'} value={{name,content,graph}} onChange={next=>config.update(next)} disabled={!manager}/>:<div className="judex-v2-form-stack"><Field label={w.t('vpName')} value={name} onChange={name=>config.update({name})} disabled={!manager}/><Field label={w.t(role?'vpPrompt':'stWorkflowContent')} value={content} onChange={content=>config.update({content})} multiline rows={12} disabled={!manager}/>{role&&<QueryState query={models}><div className="judex-settings-labeled-select"><label>{w.t('stWorkingModel')}</label><Choice disabled={!manager} label={w.t('v2Model')} value={modelId} onChange={modelId=>config.update({modelId})} items={[{id:'project',name:w.t('v2ProjectModel')},...(models.data||[]).filter(m=>m.enabled).map(m=>({id:m.id,name:m.displayName}))]}/></div></QueryState>}</div>}
  {tab!=='history'&&<div className="judex-settings-savebar"><span>{w.t('stSaveHint')}</span>{manager&&<Button variant="primary" isDisabled={!name.trim()||(!role&&!graph.nodes.length)||w.pending} onPress={()=>void save()}>{w.t('vpSave')}</Button>}</div>}
 </div>;
}
