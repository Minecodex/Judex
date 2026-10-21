import {LibraryFileCard} from './LibraryFileCard';
import {ReaderSurface} from './readers/ReaderSurface';
import {PaneControls} from './PaneLayout';
import '../../styles/workspace-readers.css';
import {ArrowLeft,ArrowDownToLine,ArrowUpRight,Search,FileText,History,Maximize2,Minimize2,Plus,Share2,Upload,X} from 'lucide-react';
import {Input} from '@heroui/react';
import {useLayoutEffect,useRef,useState,type RefObject} from 'react';
import {useQuery} from '@tanstack/react-query';
import {apiUrl,request} from '../../lib/api/client';
import {useWorkspace} from './context';
import {Button,Empty,IconButton,QueryState} from './ui';
import {list} from './api';
import type {FileVersion} from './types';

export function FileCard({file}:{file:FileVersion}){const w=useWorkspace();return <Button className="judex-v2-file-card judex-v2-file-card-text" aria-label={w.t('vpOpenFile')+' · '+file.name+' v'+file.revision} onPress={()=>w.setReader(file)}><span className="judex-v2-file-card-top"><span><FileText/><strong>{file.name}</strong><small>v{file.revision}</small></span><ArrowUpRight/></span><span className="judex-v2-file-simple"><span className="judex-v2-file-extension">{file.name.split('.').at(-1)?.slice(0,5).toUpperCase()}</span><span>{w.t('vpDocument')}<small>{Math.ceil(file.size/1024)} KB · v{file.revision}</small></span></span><span className="judex-v2-file-card-bottom"><span>{w.t('vpDocument')}</span><span>{w.t('vpOpenFile')}<ArrowUpRight/></span></span></Button>;}
function HTMLFile({file}:{file:FileVersion}){const w=useWorkspace();const session=useQuery({queryKey:['v2',w.user.id,'html-preview',w.roomId,file.id],queryFn:()=>request<{previewUrl:string}>(`/conversations/${w.roomId}/file-versions/${file.id}/preview-session`,{method:'POST',body:'{}'}),staleTime:300000,retry:false});return session.data?<iframe className="judex-v2-html-preview" title={file.name} sandbox="allow-scripts" src={session.data.previewUrl}/>:<p>{w.t(session.isError?'v2Failed':'v2Loading')}</p>;}
export function ReaderPane(){
 const w=useWorkspace(),[search,setSearch]=useState(''),scroll=useRef(0);
 return w.modal?.name==='library'?<FileLibrary search={search} setSearch={setSearch} scroll={scroll}/>:<Reader/>;
}
function Reader(){const w=useWorkspace(),[history,setHistory]=useState(false);const selected=w.reader;const meta=useQuery({queryKey:['v2',w.user.id,'file-version',w.roomId,selected?.id],queryFn:()=>request<FileVersion>(`/conversations/${w.roomId}/file-versions/${selected!.id}`),enabled:!!selected,refetchInterval:q=>q.state.data?.previewState==='pending'?2000:false,retry:false});
 const versions=useQuery({queryKey:['v2',w.user.id,'file-versions',w.roomId,selected?.fileId],queryFn:()=>list<FileVersion>(`/conversations/${w.roomId}/files/${selected!.fileId}/versions`),enabled:!!selected&&history});
 if(!selected)return null;const file=meta.data||selected,url=apiUrl(`/conversations/${w.roomId}/file-versions/${file.id}/preview`),download=apiUrl(`/conversations/${w.roomId}/file-versions/${file.id}/content`),kind=file.previewData?.kind;
 return <aside className={'judex-v2-reader'+(w.expanded?' judex-v2-reader-expanded':'')} aria-label={w.t('vpPreview')}><header className="judex-v2-reader-header"><span><IconButton label={w.t('readerBackToFiles')} onPress={()=>{w.setExpanded(false);w.setModal({name:'library'});}}><ArrowLeft/></IconButton><FileText/><strong>{file.name}</strong><small>v{file.revision}</small></span><div><IconButton label={w.t('vpVersionHistory')} onPress={()=>setHistory(!history)}><History/></IconButton>{w.screen==='groups'&&<IconButton label={w.t('vpShare')} onPress={()=>w.setModal({name:'share',file})}><Share2/></IconButton>}<a className="judex-v2-reader-download" aria-label={w.t('vpDownload')} href={download} download={file.name}><ArrowDownToLine/></a><IconButton label={w.t(w.expanded?'vpCollapse':'vpExpand')} onPress={()=>w.setExpanded(!w.expanded)}>{w.expanded?<Minimize2/>:<Maximize2/>}</IconButton><PaneControls pane="reader"/></div></header>
 {history&&<div className="judex-v2-version-history"><strong>{w.t('vpVersionHistory')}</strong><QueryState query={versions} empty={!versions.data?.length}>{versions.data?.map(v=><Button key={v.id} aria-pressed={v.id===file.id} onPress={()=>{w.setReader(v);setHistory(false);}}><span>v{v.revision}</span><small>{new Date(v.createdAt).toLocaleString(w.locale)}</small></Button>)}</QueryState></div>}
 <div className="judex-v2-reader-paper">{meta.isPending?<Empty title={w.t('v2Loading')}/>:meta.isError?<Empty title={w.t('v2Failed')}/>:file.previewState==='failed'?<Empty title={w.t('v2Failed')}><Button onPress={()=>void w.execute(`/conversations/${w.roomId}/file-versions/${file.id}/retry-preview`).catch(()=>{})}>{w.t('v2Retry')}</Button></Empty>:file.previewState==='pending'?<Empty title={w.t('v2PreviewPending')}/>:<article className="judex-v2-document judex-reader-document"><ReaderSurface key={file.id} file={file} url={url} original={download} html={kind==='html'?<HTMLFile file={file}/>:undefined}/></article>}</div></aside>;
}
function FileLibrary({search,setSearch,scroll}:{search:string;setSearch:(value:string)=>void;scroll:RefObject<number>}){
 const w=useWorkspace(),input=useRef<HTMLInputElement>(null),versionInput=useRef<HTMLInputElement>(null),[update,setUpdate]=useState<FileVersion>(),content=useRef<HTMLDivElement>(null);
 useLayoutEffect(()=>{if(content.current)content.current.scrollTop=scroll.current;},[scroll]);
 const files=w.files.data?.slice().sort((a,b)=>b.createdAt.localeCompare(a.createdAt)||a.fileId.localeCompare(b.fileId)).filter(file=>file.name.toLocaleLowerCase().includes(search.toLocaleLowerCase()));
 return <aside className="judex-v2-resource-panel" aria-label={w.t(w.screen==='chats'?'vpPrivateFiles':'vpFiles')}>
  <header><h2>{w.t(w.screen==='chats'?'vpPrivateFiles':'vpFiles')}</h2><PaneControls pane="reader"/></header>
  <div className="judex-v2-resource-toolbar"><div className="judex-v2-dialog-search"><Search/><Input aria-label={w.t('v2FindFiles')} placeholder={w.t('v2FindFiles')} value={search} onChange={e=>setSearch(e.target.value)}/></div><IconButton label={w.t('vpAttach')} isDisabled={!!w.uploading} onPress={()=>{setUpdate(undefined);input.current?.click();}}><Upload/></IconButton></div>
  <input ref={input} type="file" multiple hidden onChange={e=>{void w.upload(Array.from(e.target.files||[]));e.target.value='';}}/><input ref={versionInput} type="file" hidden onChange={e=>{void w.upload(Array.from(e.target.files||[]),update);e.target.value='';}}/>
  <div ref={content} onScroll={e=>{scroll.current=e.currentTarget.scrollTop;}} className="judex-v2-resource-content"><QueryState query={w.files} empty={!files?.length} emptyTitle={search?w.t('vpNoResults'):w.t('vpEmptyFiles')}><div className="judex-library-grid">{files?.map(file=><LibraryFileCard key={file.id} latest={file} onUpdate={()=>{setUpdate(file);versionInput.current?.click();}}/>)}</div></QueryState></div>
  <footer>{w.t('vpFileHint')}</footer>
 </aside>;
}
