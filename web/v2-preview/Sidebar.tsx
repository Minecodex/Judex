import { Input } from '@heroui/react';
import { ChevronDown, ChevronRight, ChevronsUpDown, Plus, Search, Settings2 } from 'lucide-react';
import { useState } from 'react';
import type { Group } from './model';
import { usePreview } from './context';
import { Avatar, Button, Empty, GroupIcon, IconButton, Tabs } from './ui';

export function Sidebar() {
 const p = usePreview(); const dm = p.screen === 'chats';
 const [query, setQuery] = useState(''); const [filter, setFilter] = useState('all'); const [collapsed, setCollapsed] = useState<string[]>([]);
 const allGroups = p.state.groups.filter(g => g.projectId === p.project.id && g.members.me);
 const groups = allGroups.filter(g => (filter === 'archive' ? g.archived : !g.archived) && (filter !== 'unread' || g.unread > 0) && (p.text(g.name) + p.text(g.purpose)).toLowerCase().includes(query.toLowerCase()));
 const count = allGroups.filter(g => g.unread > 0 && !g.archived).length;
 const toggle = (id: string) => setCollapsed(s => s.includes(id) ? s.filter(v => v !== id) : [...s, id]);
 const renderGroup = (group: Group, depth = 0): React.ReactNode => {
  const children = groups.filter(g => g.parentId === group.id); const isOpen = !collapsed.includes(group.id);
  return <div key={group.id} className={'judex-v2-tree-branch judex-v2-depth-' + depth}>
   <div className={'judex-v2-group-row' + (p.groupId === group.id ? ' judex-v2-group-selected' : '')}>
    <Button className="judex-v2-group-row-main" aria-label={p.text(group.name)} aria-current={p.groupId === group.id ? 'page' : undefined} data-testid={'group-' + group.id} onPress={() => p.openGroup(group.id)}>
     <GroupIcon icon={group.icon} tone={group.tone}/><span className="judex-v2-group-row-copy"><span><strong>{p.text(group.name)}</strong><small>{group.time === '昨天' ? p.locale === 'en' ? 'Yesterday' : '昨天' : group.time === '周五' ? p.locale === 'en' ? 'Fri' : '周五' : group.time}</small></span><span><p>{p.text(group.preview)}</p>{group.unread > 0 && <b className="judex-v2-unread">{group.unread}</b>}</span></span>
    </Button>{children.length > 0 && <Button isIconOnly className="judex-v2-tree-toggle" aria-label={(isOpen ? '− ' : '+ ') + p.text(group.name)} aria-expanded={isOpen} onPress={() => toggle(group.id)}>{isOpen ? <ChevronDown/> : <ChevronRight/>}</Button>}
   </div>{isOpen && children.map(child => renderGroup(child, depth + 1))}
  </div>;
 };
 // A joined child remains visible as a root if its parent is not in this list.
 const roots = groups.filter(g => !groups.some(parent => parent.id === g.parentId));
 return <aside className="judex-v2-sidebar">
  <header className="judex-v2-sidebar-header"><Button className="judex-v2-project-switch" onPress={() => p.setScreen('projects')}><GroupIcon icon={p.project.icon} tone={p.project.tone}/><span>{dm ? p.t('vpChats') : p.text(p.project.name)}</span><ChevronsUpDown/></Button>{!dm && <IconButton label={p.t('vpProjectSettings')} onPress={() => p.setModal({ name: 'project' })}><Settings2/></IconButton>}<IconButton label={dm ? p.t('vpNewChat') : p.t('vpNewGroup')} onPress={() => p.setModal({ name: dm ? 'dm' : 'create' })}><Plus/></IconButton></header>
  <div className="judex-v2-sidebar-search"><Search/><Input aria-label={dm ? p.t('vpFindPerson') : p.t('vpFindGroup')} placeholder={dm ? p.t('vpFindPerson') : p.t('vpFindGroup')} value={query} onChange={e => setQuery(e.target.value)}/></div>
  {!dm && <Tabs label={p.t('vpGroups')} value={filter} onChange={setFilter} items={[{ id: 'all', label: p.t('vpAll') }, { id: 'unread', label: p.t('vpUnread'), count }, { id: 'archive', label: p.t('vpArchive') }]}/>}
  <div className="judex-v2-list-heading"><span>{dm ? p.t('vpChats') : filter === 'archive' ? p.t('vpArchive') : p.t('vpYourGroups')}</span>{!dm && <IconButton label={p.t('vpGroupSettings')} isDisabled={!p.group} onPress={() => p.setModal({ name: 'group' })}><Settings2/></IconButton>}</div>
  <div className="judex-v2-conversation-list">{dm ? p.state.dms.filter(d => p.text(p.people.find(person => person.id === d.personId)?.name).toLowerCase().includes(query.toLowerCase())).map(d => {
   const person = p.people.find(person => person.id === d.personId);
   return <Button key={d.id} className={'judex-v2-dm-row' + (p.dmId === d.id ? ' judex-v2-group-selected' : '')} aria-label={p.text(person?.name)} onPress={() => { p.setDmId(d.id); p.setState(s => ({ ...s, dms: s.dms.map(v => v.id === d.id ? { ...v, unread: 0 } : v) })); }}><Avatar person={person} online/><span className="judex-v2-group-row-copy"><span><strong>{p.text(person?.name)}</strong><small>{d.time === '昨天' ? p.locale === 'en' ? 'Yesterday' : '昨天' : d.time}</small></span><span><p>{p.text(d.preview)}</p>{d.unread > 0 && <b className="judex-v2-unread">{d.unread}</b>}</span></span></Button>;
  }) : roots.map(g => renderGroup(g))}{!dm && !roots.length && <Empty title={p.t('vpNoResults')} description={p.t('vpNoResultsSub')}/>}</div>
  <footer className="judex-v2-sidebar-footer"><span>{dm ? p.t('vpDMHint') : p.t('vpOnlyJoined')}</span>{!dm && <Button onPress={() => p.setModal({ name: 'push' })} isDisabled={!p.group}><span className="judex-v2-local-status"/><span>{p.t('vpPush')}</span><ChevronRight/></Button>}</footer>
 </aside>;
}
