import {ProjectListPage} from "../cooperation/ProjectListPage";
import { useCollection, LoadMore } from '../../lib/api/collections';
import { Card, Input, Label, TextField, TextArea } from '@heroui/react';
import { Button } from '../../components/ui/Button';
import { Brand, EmptyState, PageHeading, PersonAvatar, PreferenceControls, UIDialog } from '../../components/ui/Presentation';
import { ArrowRight, ArrowUpRight, Command, Folder, FolderPlus, Layers, LogOut, MessageCircle, Plus, Search, Settings, Sparkles } from 'lucide-react';
import { UIStatus, UIWarning } from '../../components/ui/FormControls';
import { AccountSecurity } from '../settings/AccountSecurity';
import CooperationWorkspace from '../cooperation/CooperationShell';
import {projectFromURL} from '../cooperation/routing';
import {PortalHeader} from '../cooperation/PortalHeader';
import {ProjectCards} from '../cooperation/ProjectCards';
import type { ConversationDraft } from '../chat/Conversation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { useNavigate, useLocation } from 'react-router';
import { usePreferences } from '../../stores/preferences';
import { translate, type Key } from '../../i18n';
import { request } from '../../lib/api/client';
import { logout, type Project } from './api';
import { useAuth } from './AuthProvider';
import { errorKey } from './errors';
import type { components } from '../../lib/api/schema';

type RecentTopic = components['schemas']['RecentTopic'];
export function WorkspaceShell() {
  const { locale } = usePreferences();
  const t = (key:Key, values?:Record<string,string|number>) => translate(locale,key,values);
  const { user, signOut } = useAuth();
  const navigate = useNavigate(), route = useLocation(), client = useQueryClient();
  const [creating,setCreating] = useState(false), [title,setTitle] = useState(''), [description,setDescription] = useState('');
  const [tab,setTab] = useState<'projects'|'security'>('projects');
  const [search,setSearch] = useState(''), [query,setQuery] = useState(''), [ownership,setOwnership] = useState<'all'|'owned'|'joined'>('all');
  const activeProject = projectFromURL(route.pathname,route.search);
  const originInvite=new URLSearchParams(route.search).get('invite')==='1'&&new URLSearchParams(route.search).get('origin')==='projects';
  // Keep unsent attachments in this authenticated shell while visiting the project list.
  // Conversation keys already include the project, member and topic, so drafts stay scoped.
  const conversationDrafts = useRef(new Map<string, ConversationDraft>());
  const visible = !activeProject || originInvite || tab === 'security';
  useEffect(() => { const timer = setTimeout(() => setQuery(search.trim()),200); return () => clearTimeout(timer); },[search]);
  const parameters = new URLSearchParams({ include:'summary', ownership });
  if (query) parameters.set('q',query);
  const projects = useCollection<Project>(['projectLanding',user?.id,query,ownership],'/projects?'+parameters,visible);
  const recent = useQuery({ queryKey:['projectRecent',user?.id],enabled:visible,queryFn:()=>request<{items:RecentTopic[]}>('/me/recent-topics?limit=3'),refetchOnMount:'always',refetchInterval:30000 });
  const createProject = useMutation({
    mutationFn:()=>request<Project>('/projects',{method:'POST',body:JSON.stringify({title:title.trim(),description:description.trim()}),idempotencyKey:crypto.randomUUID()}),
    onSuccess:(created)=>{
      setTitle('');setDescription('');setCreating(false);
      void client.invalidateQueries({queryKey:['projectLanding']});void client.invalidateQueries({queryKey:['projects']});void client.invalidateQueries({queryKey:['projectRecent']});
      navigate('/projects/'+created.id);
    },
  });
  useEffect(()=>{ if (visible) document.title='Judex · '+t('portalProjects'); },[locale,visible]);
  const signOutAndRedirect = async () => { await logout().catch(()=>null);await signOut();navigate('/login',{replace:true}); };
  const openProject = (projectId:string,topicId?:string)=>navigate('/projects/'+projectId+(topicId?'/chat/'+topicId:''));
  if (activeProject && !originInvite && tab!=='security') return <CooperationWorkspace key={activeProject} mode="api" projectId={activeProject} onLogout={signOutAndRedirect} onBackToProjects={()=>navigate('/')} draftCache={conversationDrafts.current}/>;
  const tone = (index:number) => (['green','violet','sand'] as const)[index%3];
  const items = projects.data?.items ?? [];
  return <div className="judex-workspace-shell judex-co-app">
    <PortalHeader name={user?.displayName??"Judex"} onLogout={signOutAndRedirect} onProjects={()=>{setTab("projects");navigate("/");}} onSecurity={()=>setTab(tab==="security"?"projects":"security")}/>
    <main className="judex-co-page">
      {tab==='security'?<><Button onPress={()=>setTab('projects')} className="judex-text-link">{t('portalProjects')}</Button><AccountSecurity/></>:<>
        <ProjectListPage items={items.map(p=>({id:p.id,title:p.title,description:p.description??'',role:p.role,memberCount:p.summary?.memberCount??0,topicCount:p.summary?.topicCount??0,members:(p.summary?.memberPreview??[]).map(m=>({id:m.userId,name:m.displayName}))}))} recent={recent.data?.items} pending={projects.isPending} error={projects.isError?t(errorKey(projects.error)??'errNetwork'):undefined} recentPending={recent.isPending} recentError={recent.isError?t(errorKey(recent.error)??'errNetwork'):undefined} onRetry={()=>void projects.refetch()} onRecentRetry={()=>void recent.refetch()} search={search} setSearch={setSearch} ownership={ownership} setOwnership={setOwnership} onEnter={openProject} onInvite={id=>navigate('/projects/'+id+'?invite=1&origin=projects')} onSettings={id=>navigate('/projects/'+id+'/settings/project?returnPage=projects')} onCreate={()=>setCreating(true)} more={<LoadMore query={projects}/>}/>
      </>}
    </main>
    {originInvite&&activeProject&&<CooperationWorkspace mode="api" projectId={activeProject} onLogout={signOutAndRedirect} onBackToProjects={()=>navigate("/")} dialogOnly/>}
    {creating && <UIDialog title={t('workspaceNewProject')} description={t('portalNewCardHint')} onClose={()=>{if(!createProject.isPending){setCreating(false);createProject.reset();}}}><form className="judex-auth-form" onSubmit={(event)=>{event.preventDefault();if(title.trim()&&!createProject.isPending)createProject.mutate();}}><TextField isRequired name="title" value={title} onChange={setTitle}><Label>{t('portalProjectName')}</Label><Input data-testid="new-project-title" autoFocus placeholder={t('portalProjectPlaceholder')} maxLength={200}/></TextField><TextField name="description" value={description} onChange={setDescription}><Label>{t('portalDescription')}</Label><TextArea rows={3} maxLength={20000} placeholder={t('portalDescriptionPlaceholder')}/></TextField>{createProject.isError && <UIWarning role="alert">{t(errorKey(createProject.error)??'errNetwork')}</UIWarning>}<div className="judex-modal-actions"><Button disabled={createProject.isPending} onPress={()=>{setCreating(false);createProject.reset();}}>{t('cancel')}</Button><Button type="submit" variant="primary" disabled={!title.trim()} isPending={createProject.isPending}><Plus/>{t('workspaceNewProject')}</Button></div></form></UIDialog>}
  </div>;
}
