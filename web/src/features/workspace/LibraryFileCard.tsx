import {Card,Dropdown,Label} from '@heroui/react';
import {ArrowUpRight,ChevronDown,Copy,History,MoreHorizontal,Upload,X} from 'lucide-react';
import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {useWorkspace} from './context';
import {ActionMenu,Avatar,Button} from './ui';
import {list} from './api';
import type {FileVersion} from './types';

// A library selection never rewrites immutable message attachments. Authorization
// and the list of readable revisions always come from the conversation endpoint.
export function LibraryFileCard({latest,onUpdate}:{latest:FileVersion;onUpdate:()=>void}){
 const w=useWorkspace(),[selected,setSelected]=useState<FileVersion>(),[open,setOpen]=useState(false);
 const versions=useQuery({queryKey:['v2',w.user.id,'file-versions',w.roomId,latest.fileId],queryFn:()=>list<FileVersion>(`/conversations/${w.roomId}/files/${latest.fileId}/versions`),enabled:open,retry:false});
 const file=selected||latest,stamp=new Date(file.createdAt).toLocaleString(w.locale),label=w.t('libVersion')+' · '+file.name;
 const actions=[...(latest.owned?[{id:'update',label:w.t('vpNewVersion'),icon:<Upload/>,action:onUpdate}]:[{id:'copy',label:w.t('vpSaveCopy'),icon:<Copy/>,action:()=>{void w.execute(`/conversations/${w.roomId}/file-versions/${file.id}/copy`,{name:file.name}).catch(()=>{});}}]),{id:'remove',label:w.t('vpRemoveFile'),icon:<X/>,danger:true,action:()=>{void w.execute(`/conversations/${w.roomId}/files/${latest.fileId}`,{expectedVersion:latest.version},'DELETE').catch(()=>{});}}];
 return <Card className="judex-library-card" data-file-id={latest.fileId}>
  <Card.Header className="judex-library-card-header"><span className="judex-library-extension">{file.name.includes('.')?file.name.split('.').at(-1)?.slice(0,8).toUpperCase():'FILE'}</span><span>{w.t(selected&&selected.id!==latest.id?'libHistorical':'libLatest')}</span><ActionMenu label={w.t('libActions')+' · '+file.name} items={actions}><MoreHorizontal/></ActionMenu></Card.Header>
  <Card.Content><Button className="judex-library-open" aria-label={w.t('vpOpenFile')+' · '+file.name+' v'+file.revision} onPress={()=>{w.setReader(file);w.setModal(null);}}><strong>{file.name}</strong><ArrowUpRight/><small>{file.size<1024?file.size+' B':Math.ceil(file.size/1024)+' KB'} · {w.t('vpDocument')}</small></Button></Card.Content>
  <Card.Footer className="judex-library-card-footer"><div className="judex-library-author"><Avatar small name={file.authorName||w.t('libUnknownAuthor')} userId={file.authorUserId} avatarVersion={file.authorAvatarVersion}/><span><strong>{file.authorName||w.t('libUnknownAuthor')}</strong><time dateTime={file.createdAt} title={stamp}>{stamp}</time></span></div>
   <Dropdown isOpen={open} onOpenChange={setOpen}><Button className="judex-library-version" aria-label={label}><History/><span>v{file.revision}</span><ChevronDown/></Button><Dropdown.Popover placement="bottom end" className="judex-v2-action-popover judex-library-versions"><Dropdown.Menu aria-label={label} selectionMode="single" selectedKeys={[file.id]} onAction={id=>{if(id==='retry'){void versions.refetch();return;}const found=versions.data?.find(v=>v.id===id);if(found)setSelected(found.id===latest.id?undefined:found);}}>
    {versions.isPending?<Dropdown.Item id="loading" isDisabled textValue={w.t('v2Loading')}><Label>{w.t('v2Loading')}</Label></Dropdown.Item>:versions.isError?<Dropdown.Item id="retry" textValue={w.t('v2Retry')}><Label>{w.t('v2Failed')} · {w.t('v2Retry')}</Label></Dropdown.Item>:(versions.data||[]).map(v=><Dropdown.Item id={v.id} key={v.id} textValue={'v'+v.revision}><span className="judex-library-version-info"><Label>v{v.revision}{v.id===latest.id?' · '+w.t('libLatest'):''}</Label><small>{v.authorName} · {new Date(v.createdAt).toLocaleString(w.locale)}</small></span><Dropdown.ItemIndicator/></Dropdown.Item>)}
   </Dropdown.Menu></Dropdown.Popover></Dropdown>
  </Card.Footer>
 </Card>;
}
