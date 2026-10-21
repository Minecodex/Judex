import {defaultLayout,useLayoutStore} from './layoutStore';
import {createContext,useContext,useEffect,useMemo,useRef,useState,useSyncExternalStore,type ReactNode} from 'react';
import {useQuery,useInfiniteQuery,useQueryClient,useMutation} from '@tanstack/react-query';
import {useNavigate,useSearchParams} from 'react-router';
import {useAuth} from '../auth/AuthProvider';
import {apiUrl,APIError,request} from '../../lib/api/client';
import {usePreferences} from '../../stores/preferences';
import {translate,type Key} from '../../i18n';
import {command,list} from './api';
import {createUploadManager,PendingUploadBatch} from './uploads';
import type {Project,Person,Group,Position,Workflow,Direct,Message,FileVersion,Discussion,Notice,Modal,Run} from './types';

function useData<T>(scope:string,user:string,path:string,enabled=true){return useQuery({queryKey:['v2',user,scope,path],queryFn:()=>list<T>(path),enabled,retry:false});}
function useWorkspaceValue(){
 const auth=useAuth(),user=auth.user!;const prefs=usePreferences();const client=useQueryClient();const navigate=useNavigate();const [params]=useSearchParams();
 const [messageSearch,setMessageSearch]=useState('');
 const [modal,setModalState]=useState<Modal>(null),[reader,setReaderState]=useState<FileVersion|null>(null),[expanded,setExpanded]=useState(false),[toast,setToast]=useState(''),[connection,setConnection]=useState<'connected'|'reconnecting'>('reconnecting');
 const revealReader=()=>{const store=useLayoutStore.getState(),layout=store.users[user.id]||defaultLayout;if(layout.hidden.reader)store.save(user.id,{...layout,hidden:{...layout.hidden,reader:false}});};
 const setReader=(file:FileVersion|null)=>{if(file){revealReader();setModalState(current=>current?.name==='library'?null:current);}setReaderState(file);};
 const setModal=(value:Modal)=>{if(value?.name==='library')revealReader();setModalState(value);};
 const projectId=params.get('project')||'',screen=params.get('view')==='chats'?'chats':projectId?'groups':'projects',groupId=params.get('group')||'',dmId=params.get('dm')||'',roomId=screen==='chats'?dmId:groupId;
 const uploads=useMemo(()=>createUploadManager(request),[user.id]);
 useSyncExternalStore(uploads.subscribe,uploads.revision);
 const uploadBatch=uploads.status(roomId),uploading=uploadBatch?.running?uploadBatch.progress||'…':'';
 const t=(key:Key,values:Record<string,string|number>={})=>translate(prefs.locale,key,values);
 const projects=useData<Project>('projects',user.id,'/projects');
 const groups=useData<Group>('groups',user.id,`/projects/${projectId}/groups`,!!projectId);
 const members=useData<Person>('members',user.id,`/projects/${projectId}/members`,!!projectId);
 const positions=useData<Position>('positions',user.id,`/projects/${projectId}/positions`,!!projectId);
 const workflows=useData<Workflow>('workflows',user.id,`/projects/${projectId}/workflows`,!!projectId);
 const directs=useData<Direct>('directs',user.id,'/direct-conversations');
 const messagePages=useInfiniteQuery({queryKey:['v2',user.id,'messages',roomId,messageSearch],enabled:!!roomId,initialPageParam:0,queryFn:({pageParam,signal})=>request<{items:Message[]}>(`/conversations/${roomId}/messages?before=${pageParam}&q=${encodeURIComponent(messageSearch)}`,{signal}),getNextPageParam:page=>page.items.length===100?Math.min(...page.items.map(m=>m.seq)):undefined,retry:false});
 const messages={...messagePages,data:messagePages.data?.pages.flatMap(page=>page.items)};
 const files=useData<FileVersion>('files',user.id,`/conversations/${roomId}/files`,!!roomId);
 const discussions=useData<Discussion>('discussions',user.id,`/groups/${groupId}/discussions`,screen==='groups'&&!!groupId);
 const runs=useData<Run>('runs',user.id,`/groups/${groupId}/runs`,screen==='groups'&&!!groupId);
 const notices=useData<Notice>('notices',user.id,'/me/notifications');
 const project=projects.data?.find(p=>p.id===projectId),group=groups.data?.find(g=>g.id===groupId),direct=directs.data?.find(d=>d.id===dmId);
 useEffect(()=>{document.title='Judex · '+(group?.name||direct?.displayName||project?.name||t('vpProjects'));},[group?.name,direct?.displayName,project?.name,prefs.locale]);
 const refresh=()=>client.invalidateQueries({queryKey:['v2',user.id]});
 const mutation=useMutation({mutationFn:async(args:{path:string;body?:unknown;method?:string})=>command(args.path,args.body,args.method),onSuccess:()=>{void refresh();},onError:async error=>{const conflict=error instanceof APIError&&error.code==='VERSION_CONFLICT';setToast(conflict?t('v2Conflict'):error instanceof APIError&&error.code==='INVALID_RESPONSE'?t('v2UncertainResponse'):error.message);if(conflict)await refresh();}});
 const execute=async<T,>(path:string,body:unknown={},method='POST')=>await mutation.mutateAsync({path,body,method}) as T;
 const openProject=(id:string)=>{const first=groups.data?.find(g=>g.projectId===id&&!g.archived);navigate(`/?project=${id}${first?'&group='+first.id:''}`);};
 const openGroup=(id:string)=>navigate(`/?project=${projectId}&group=${id}`);
 const openDM=(id:string)=>navigate(`/?view=chats${projectId?'&project='+projectId:''}&dm=${id}`);
 useEffect(()=>{if(screen==='groups'&&projectId&&!groupId&&groups.data?.length){const g=groups.data.find(g=>!g.archived)||groups.data[0];navigate(`/?project=${projectId}&group=${g.id}`,{replace:true});}},[projectId,groupId,groups.data,screen,navigate]);
 useEffect(()=>{setReader(null);setExpanded(false);setMessageSearch('');},[roomId]);
 useEffect(()=>{if(!toast)return;const id=setTimeout(()=>setToast(''),6000);return()=>clearTimeout(id);},[toast]);
 const cursors=useRef<Record<string,string>>({});
 useEffect(()=>{
  if(!user.id)return;let timer:ReturnType<typeof setTimeout>|undefined;
  const invalidate=()=>{if(timer)return;timer=setTimeout(()=>{timer=undefined;void refresh();},100);};
  const channels=['/me/events',...(roomId?[`/conversations/${roomId}/events`]:[])];
  const events=['profile.changed','message.created','membership.changed','group.changed','group.archived','project.changed','personal-prompt.changed','discussion.changed','discussion.paused','discussion.settled','file.ready','file.removed','notification.created','conversation.read','cursor','conversation.changed','access.changed'];
  const sources=channels.map(path=>{const source=new EventSource(apiUrl(path+(cursors.current[path]?'?after='+cursors.current[path]:'')),{withCredentials:true});source.onopen=()=>setConnection('connected');source.onerror=()=>{setConnection('reconnecting');invalidate();};for(const type of events)source.addEventListener(type,event=>{cursors.current[path]=(event as MessageEvent).lastEventId;if(type!=='cursor')invalidate();if(type==='profile.changed'){try{if(JSON.parse((event as MessageEvent).data).userId===user.id)void auth.refresh();}catch{}}});return source;});
  return()=>{sources.forEach(s=>s.close());if(timer)clearTimeout(timer);};
 },[roomId,user.id]);
 useEffect(()=>{const seq=messages.data?.[0]?.seq;if(seq&&roomId){void command(`/conversations/${roomId}/read`,{seq}).catch(()=>{});}},[messages.data?.[0]?.seq,roomId]);
 useEffect(()=>{if(messages.error instanceof APIError&&[401,403,404].includes(messages.error.status)){setReader(null);setModal(null);client.removeQueries({predicate:q=>q.queryKey[0]==='v2'&&q.queryKey[1]===user.id&&JSON.stringify(q.queryKey).includes(roomId)});navigate('/',{replace:true});}},[messages.error,client,user.id,roomId,navigate]);
 const finishUpload=async(action:()=>Promise<boolean>)=>{try{if(await action())setToast(t('vpUploaded'));}catch(error){setToast(error instanceof PendingUploadBatch?t('v2UploadPendingSelection'):error instanceof APIError&&error.code==='INVALID_RESPONSE'?t('v2UncertainResponse'):(error as Error).message);}finally{await refresh();}};
 const upload=async(files:File[],update?:FileVersion)=>{if(roomId)await finishUpload(()=>uploads.upload(roomId,files,update));};
 const resumeUpload=()=>finishUpload(()=>uploads.resume(roomId));
 const closeUpload=()=>uploads.close(roomId);
 return {messageSearch,setMessageSearch,user,auth,...prefs,t,projectId,project,screen,groupId,group,dmId,direct,roomId,projects,groups,members,positions,workflows,directs,messages,files,discussions,runs,notices,modal,setModal,reader,setReader,expanded,setExpanded,toast,setToast,uploading,upload,uploadBatch,resumeUpload,closeUpload,connection,execute,pending:mutation.isPending,refresh,openProject,openGroup,openDM,navigate,request};
}
type WorkspaceContext=ReturnType<typeof useWorkspaceValue>;
const Context=createContext<WorkspaceContext|null>(null);
export function WorkspaceProvider({children}:{children:ReactNode}){const value=useWorkspaceValue();return <Context.Provider value={value}>{children}</Context.Provider>;}
export function useWorkspace(){const value=useContext(Context);if(!value)throw new Error('WorkspaceProvider required');return value;}
