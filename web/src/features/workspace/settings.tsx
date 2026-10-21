import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {BriefcaseBusiness,GitBranch,LockKeyhole,Settings2,SlidersHorizontal,Users} from 'lucide-react';
import {useWorkspace} from './context';
import {Button,Choice,ConfigChange,Dialog,Empty,Field,QueryState} from './ui';
import {useConfigDraft} from './configDraft';
import {list} from './api';
import {ProjectMembers} from './settingsMembers';
import {ProjectTemplates,useModels} from './settingsTemplates';
import {Badge,SettingsHeading,SettingsSection} from './settingsUI';
import type {Position,PersonalPrompt,Project} from './types';

export function NewProject(){
 const w=useWorkspace(),models=useModels(),[name,setName]=useState(''),[description,setDescription]=useState(''),[modelId,setModel]=useState('');
 return <Dialog title={w.t('vpNewProject')} description={w.t('stNewProjectConfig')} footer={<><Button onPress={()=>w.setModal(null)}>{w.t('vpCancel')}</Button><Button variant="primary" isDisabled={!name.trim()||w.pending||models.isPending||models.isError} onPress={()=>{void w.execute<Project>('/projects',{name,description,loopLimit:10,defaultModelId:modelId||models.data?.find(m=>m.enabled)?.id||null}).then(p=>{w.openProject(p.id);w.setModal(null);}).catch(()=>{});}}>{w.t('vpCreateProject')}</Button></>}><div className="judex-v2-form-stack"><Field label={w.t('vpName')} value={name} onChange={setName}/><Field label={w.t('vpDescription')} value={description} onChange={setDescription} multiline/><QueryState query={models}><div className="judex-settings-labeled-select"><label>{w.t('stWorkingModel')}</label><Choice label={w.t('v2Model')} value={modelId||models.data?.find(m=>m.enabled)?.id||''} onChange={setModel} items={(models.data||[]).filter(m=>m.enabled).map(m=>({id:m.id,name:m.displayName}))}/></div></QueryState></div></Dialog>;
}
function General(){
 const w=useWorkspace(),project=w.project!,models=useModels(),manager=project.role!=='member';
 const config=useConfigDraft({name:project.name,description:project.description,limit:String(project.loopLimit),modelId:project.defaultModelId||''},project.version),{name,description,limit,modelId}=config.draft;
 const save=async()=>{try{const out=await w.execute<Project>(`/projects/${project.id}`,{name,description,defaultModelId:modelId||null,loopLimit:Number(limit),expectedVersion:config.baseVersion},'PATCH');config.replace({name:out.name,description:out.description,modelId:out.defaultModelId||'',limit:String(out.loopLimit)},out.version);w.setToast(w.t('v2Saved'));}catch{}};
 return <div className="judex-settings-page-body"><SettingsHeading title={w.t('vpGeneral')} description={w.t('stGeneralHint')}/><ConfigChange stale={config.stale} onReload={config.reload}/>
  <SettingsSection title={w.t('stIdentity')}><Field label={w.t('vpName')} value={name} onChange={name=>config.update({name})} disabled={!manager}/><Field label={w.t('vpDescription')} value={description} onChange={description=>config.update({description})} multiline rows={4} disabled={!manager}/></SettingsSection>
  <SettingsSection title={w.t('stAI')} description={w.t('stAIHint')}><QueryState query={models}><div className="judex-settings-labeled-select"><label>{w.t('stWorkingModel')}</label><Choice disabled={!manager} label={w.t('v2Model')} value={modelId} onChange={modelId=>config.update({modelId})} items={(models.data||[]).filter(m=>m.enabled).map(m=>({id:m.id,name:m.displayName}))}/><small>{w.t('stModelHint')}</small></div></QueryState><Field label={w.t('vpLimit')} value={limit} onChange={limit=>config.update({limit})} type="number" hint={w.t('vpLimitHint')} disabled={!manager}/></SettingsSection>
  <div className="judex-settings-savebar"><span>{w.t(manager?'stSaveHint':'stReadOnly')}</span><Button variant="primary" isDisabled={!manager||!name.trim()||w.pending||!Number.isInteger(Number(limit))||Number(limit)<1||Number(limit)>100} onPress={()=>void save()}>{w.t('vpSave')}</Button></div>
 </div>;
}
function Personal(){
 const w=useWorkspace(),[selected,setSelected]=useState(''),prompts=useQuery({queryKey:['v2',w.user.id,'personal-prompts',w.projectId],queryFn:()=>list<PersonalPrompt>(`/projects/${w.projectId}/personal-prompts`),retry:false});
 const position=w.positions.data?.find(p=>p.id===selected)||w.positions.data?.[0];
 return <div className="judex-settings-page-body"><SettingsHeading title={w.t('vpPersonalPrompt')} description={w.t('stPersonalHint')} actions={<Badge><LockKeyhole/>{w.t('stPrivate')}</Badge>}/><QueryState query={w.positions}><QueryState query={prompts}>{position?<><Choice label={w.t('stPersonalRole')} value={position.id} onChange={setSelected} items={(w.positions.data||[]).map(p=>({id:p.id,name:p.name}))}/><PersonalEditor key={position.id} position={position} value={prompts.data?.find(p=>p.positionId===position.id)}/></>:<Empty title={w.t('stPersonalEmpty')}/>}</QueryState></QueryState>
  <section className="judex-settings-layers"><h3>{w.t('stPromptLayers')}</h3>{(['stLayerProject','stLayerFlow','stLayerRole','stLayerPersonal','stLayerKnowledge'] as const).map((label,i)=><div key={label}><span>{i+1}</span><p>{w.t(label)}</p>{label==='stLayerPersonal'&&<LockKeyhole/>}</div>)}<small>{w.t('stPersonalNote')}</small></section>
 </div>;
}
function PersonalEditor({position,value}:{position:Position;value?:PersonalPrompt}){
 const w=useWorkspace(),config=useConfigDraft({prompt:value?.prompt||''},value?.version||0);
 const save=async()=>{try{await w.execute(`/projects/${w.projectId}/personal-prompts/${position.id}`,{prompt:config.draft.prompt,expectedVersion:config.baseVersion},'PUT');config.replace(config.draft,config.baseVersion+1);w.setToast(w.t('v2Saved'));}catch{}};
 return <><ConfigChange stale={config.stale} onReload={config.reload}/><Field label={position.name+' · '+w.t('vpPersonalPrompt')} value={config.draft.prompt} onChange={prompt=>config.update({prompt})} multiline rows={7}/><div className="judex-settings-savebar"><span>{w.t('stSaveHint')}</span><Button variant="primary" isDisabled={w.pending} onPress={()=>void save()}>{w.t('vpSave')}</Button></div></>;
}
export function ProjectSettings(){
 const w=useWorkspace(),[tab,setTab]=useState('general');
 const sections=[{id:'general',label:w.t('vpGeneral'),icon:Settings2},{id:'members',label:w.t('vpMembers'),icon:Users},{id:'positions',label:w.t('vpRoles'),icon:BriefcaseBusiness},{id:'workflows',label:w.t('vpWorkflows'),icon:GitBranch},{id:'personal',label:w.t('vpPersonalPrompt'),icon:SlidersHorizontal}];
 return <Dialog wide className="judex-settings-modal" title={w.t('vpProjectSettings')+' · '+w.project?.name}><div className="judex-settings-layout"><aside className="judex-settings-nav"><div><span className="judex-settings-card-icon"><Settings2/></span><strong>{w.project?.name}</strong><small>{w.t('stWorkspace')}</small></div><nav aria-label={w.t('vpProjectSettings')}>{sections.map(section=><Button key={section.id} aria-current={tab===section.id?'page':undefined} className={tab===section.id?'judex-settings-nav-active':''} onPress={()=>setTab(section.id)}><section.icon/>{section.label}</Button>)}</nav><p>{w.t('stProjectHint')}</p></aside><div className="judex-settings-content">{tab==='general'?<General/>:tab==='members'?<ProjectMembers/>:tab==='personal'?<Personal/>:<ProjectTemplates key={tab} kind={tab as 'positions'|'workflows'}/>}</div></div></Dialog>;
}
