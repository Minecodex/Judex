import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {MoreHorizontal,Plus,Shield,Trash2,UserRound,Users,X} from 'lucide-react';
import {useWorkspace} from './context';
import {ActionMenu,Avatar,Button,Empty,Field,IconButton,QueryState} from './ui';
import {Badge,SettingsHeading} from './settingsUI';
import {list} from './api';
import type {Person} from './types';

export function ProjectMembers(){
 const w=useWorkspace(),[search,setSearch]=useState(''),[adding,setAdding]=useState(false),[lookup,setLookup]=useState(''),[removing,setRemoving]=useState<Person|null>(null);
 const manager=w.project?.role!=='member';
 const users=useQuery({queryKey:['v2',w.user.id,'users',lookup],queryFn:()=>list<Person&{id:string}>(`/users?q=${encodeURIComponent(lookup.trim())}`),enabled:adding&&!!lookup.trim(),retry:false});
 const filtered=(w.members.data||[]).filter(p=>(p.displayName+' '+p.email).toLowerCase().includes(search.toLowerCase()));
 const add=async(id:string)=>{try{await w.execute(`/projects/${w.projectId}/members`,{userId:id,role:'member'});setLookup('');setAdding(false);}catch{}};
 return <div className="judex-settings-page-body">
  <SettingsHeading title={w.t('vpMembers')} description={w.t('stPeopleHint')} actions={manager&&<Button variant="primary" onPress={()=>setAdding(!adding)}><Plus/>{w.t('stAddMember')}</Button>}/>
  {adding&&<section className="judex-settings-add-people"><header><div><h3>{w.t('stAddMember')}</h3><p>{w.t('stAddHint')}</p></div><IconButton label={w.t('stDismiss')} onPress={()=>setAdding(false)}><X/></IconButton></header><Field label={w.t('v2UserSearch')} value={lookup} onChange={setLookup}/>
   {lookup.trim()?<QueryState query={users} empty={!users.data?.length} emptyTitle={w.t('stEmptyPeople')}><div className="judex-settings-search-results">{users.data?.map(person=>{const exists=w.members.data?.some(m=>m.userId===person.id);return <div className="judex-settings-person" key={person.id}><Avatar userId={person.id} avatarVersion={person.avatarVersion} name={person.displayName}/><div><strong>{person.displayName}</strong><small>{person.email}</small></div><Button isDisabled={exists||w.pending} onPress={()=>void add(person.id)}>{w.t(exists?'stAdded':'vpInviteAction')}</Button></div>;})}</div></QueryState>:<p className="judex-settings-note">{w.t('stNoSearch')}</p>}
  </section>}
  <div className="judex-settings-toolbar"><Badge><Users/>{w.t('stMemberCount',{count:w.members.data?.length||0})}</Badge><Field label={w.t('stMemberSearch')} placeholder={w.t('stMemberSearch')} value={search} onChange={setSearch}/></div>
  <QueryState query={w.members} empty={!filtered.length} emptyTitle={w.t('stEmptyPeople')}><div className="judex-settings-member-table"><div className="judex-settings-member-header"><span>{w.t('stIdentityColumn')}</span><span>{w.t('stRole')}</span><span>{w.t('stActions')}</span></div>
   {filtered.map(person=><div className="judex-settings-member-entry" key={person.userId}><div className="judex-settings-member-row"><div className="judex-settings-person"><Avatar userId={person.userId} avatarVersion={person.avatarVersion} name={person.displayName}/><div><strong>{person.displayName}{person.userId===w.user.id&&<small className="judex-settings-you">{w.t('vpMe')}</small>}</strong><small>{person.email}</small></div></div><Badge accent={person.role!=='member'}>{person.role==='owner'?<Shield/>:person.role==='admin'?<Shield/>:<UserRound/>}{w.t(person.role==='owner'?'v2Owner':person.role==='admin'?'v2Admin':'stMember')}</Badge><div className="judex-settings-member-menu">{manager&&person.role!=='owner'&&<ActionMenu label={w.t('stMemberActions',{name:person.displayName})} items={[
    {id:'role',icon:<Shield/>,label:w.t(person.role==='admin'?'v2MakeMember':'v2MakeAdmin'),action:()=>{void w.execute(`/projects/${w.projectId}/members`,{userId:person.userId,role:person.role==='admin'?'member':'admin',expectedVersion:person.version}).catch(()=>{});}},
    {id:'remove',icon:<Trash2/>,label:w.t('stRemoveMember'),danger:true,action:()=>setRemoving(person)},
   ]}><MoreHorizontal/></ActionMenu>}</div></div>
   {removing?.userId===person.userId&&<div className="judex-settings-confirm" role="alert"><div><strong>{w.t('stRemoveMember')} · {person.displayName}</strong><p>{w.t('stRemoveConfirm')}</p></div><div><Button onPress={()=>setRemoving(null)}>{w.t('vpCancel')}</Button><Button variant="danger" isDisabled={w.pending} onPress={()=>{void w.execute(`/projects/${w.projectId}/members/${person.userId}`,{expectedVersion:removing.version},'DELETE').then(()=>setRemoving(null)).catch(()=>{});}}>{w.t('stConfirmRemove')}</Button></div></div>}
   </div>)}
  </div></QueryState>
 </div>;
}
