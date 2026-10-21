import { ArrowRight, ArrowUpRight, Bell, Check, FileText, MonitorUp, RotateCcw, Search, Share2, Sparkles } from 'lucide-react';
import { Input } from '@heroui/react';
import { useState } from 'react';
import { usePreview } from './context';
import { Avatar, Brand, Button, CheckField, Choice, Dialog, Empty, Field, GroupIcon } from './ui';
import { GroupSettings, ProjectSettings } from './settings';
import { FileLibrary } from './Files';
import { Knowledge } from './Knowledge';
import { c, now, uid } from './model';

function ShareDialog() {
 const p = usePreview(); const reference = p.modal.file; const file = p.state.files.find(f => f.id === reference?.fileId);
 const targets = p.state.groups.filter(g => g.projectId === p.project.id && g.members.me && g.id !== p.roomId);
 const [target, setTarget] = useState(targets[0]?.id || ''); const [note, setNote] = useState('');
 if (!file || !reference) return null;
 const share = () => { if (!target) return; p.setState(s => ({ ...s, messages: [...s.messages, { id: uid(), roomId: target, authorId: 'me', text: note.trim() ? c(note, note) : c('分享了一份文件，供大家参考。', 'Shared a file for everyone to explore.'), time: now(), files: [reference] }], groups: s.groups.map(g => g.id === target ? { ...g, unread: g.unread + 1, preview: c('分享了一份文件', 'A file was shared'), time: now() } : g), hiddenFiles: { ...s.hiddenFiles, [target]: (s.hiddenFiles[target] || []).filter(id => id !== file.id) } })); p.setModal({ name: null }); p.toast(p.t('vpShared')); };
 return <Dialog title={p.t('vpShare')} description={p.t('vpShareExplain')} footer={<><Button onPress={() => p.setModal({ name: null })}>{p.t('vpCancel')}</Button><Button variant="primary" isDisabled={!target} onPress={share}><Share2/>{p.t('vpShare')}</Button></>}><div className="judex-v2-share-file"><FileText/><div><strong>{p.text(file.name)}</strong><small>{p.t('vpVersion')} {reference.version}</small></div></div><div className="judex-v2-field"><label>{p.t('vpShareTarget')}</label><Choice label={p.t('vpShareTarget')} value={target} onChange={setTarget} items={targets.map(g => ({ id: g.id, name: p.text(g.name) }))}/></div><Field label={p.t('vpShareNote')} value={note} onChange={setNote} multiline/></Dialog>;
}
function PushDialog() {
 const p = usePreview(); const [content, setContent] = useState(p.locale === 'en' ? 'I’ve refined the chat and file reading interactions locally. The reader opens on demand, with exact versions preserved in messages. Please review from your role’s perspective.' : '我在本地整理了聊天与文件阅读的交互细节：阅读区按需展开，消息保留具体版本。请从各自岗位看看，有没有需要补充的地方。');
 const [recordOnly, setRecordOnly] = useState(false); const [includeFile, setIncludeFile] = useState(true);
 return <Dialog title={p.t('vpPushTitle')} description={p.t('vpPushDescription')} footer={<><Button onPress={() => p.setModal({ name: null })}>{p.t('vpCancel')}</Button><Button variant="primary" isDisabled={!content.trim() || !p.group} onPress={() => p.push(content.trim(), recordOnly, includeFile)}><MonitorUp/>{p.t('vpPushAction')}</Button></>}><div className="judex-v2-push-target"><GroupIcon icon={p.group?.icon}/><span>{p.text(p.group?.name)}</span><small>{p.locale === 'en' ? 'Current group' : '当前群聊'}</small></div><Field label={p.t('vpPushText')} value={content} onChange={setContent} multiline/><CheckField selected={includeFile} onChange={setIncludeFile}>{p.t('vpPushFile')}</CheckField><div className="judex-v2-push-options"><CheckField selected={recordOnly} onChange={setRecordOnly}><span><strong>{p.t('vpRecordOnly')}</strong><small>{p.t('vpRecordHint')}</small></span></CheckField></div><div className="judex-v2-doc-note"><Sparkles/><p>{p.t('vpPushHint')}</p></div></Dialog>;
}
function ChatDialog() {
 const p = usePreview(); const [search, setSearch] = useState('');
 const start = (personId: string) => { const existing = p.state.dms.find(d => d.personId === personId); const id = existing?.id || uid(); if (!existing) p.setState(s => ({ ...s, dms: [...s.dms, { id, personId, unread: 0, preview: c('开始一段新的对话', 'Start a new conversation'), time: now() }] })); p.setDmId(id); p.setScreen('chats'); p.setModal({ name: null }); };
 const people = p.people.filter(person => person.id !== 'me' && p.text(person.name).toLowerCase().includes(search.toLowerCase()));
 return <Dialog title={p.t('vpNewChat')} description={p.t('vpDMHint')}><div className="judex-v2-dialog-search"><Search/><Input aria-label={p.t('vpSearchPeople')} placeholder={p.t('vpSearchPeople')} value={search} onChange={e => setSearch(e.target.value)}/></div><div className="judex-v2-people-results">{people.map(person => <Button key={person.id} onPress={() => start(person.id)} aria-label={p.t('vpStartChat') + ' · ' + p.text(person.name)}><Avatar person={person} online/><span><strong>{p.text(person.name)}</strong><small>{p.text(person.subtitle)}</small></span><ArrowRight/></Button>)}</div>{!people.length && <Empty title={p.t('vpNoResults')}/>}</Dialog>;
}
function Notifications() {
 const p = usePreview();
 return <Dialog title={p.t('vpNotifications')} footer={<Button onPress={() => p.setState(s => ({ ...s, notices: s.notices.map(n => ({ ...n, read: true })) }))}><Check/>{p.t('vpMarkRead')}</Button>}><div className="judex-v2-notice-list">{p.state.notices.map(notice => <Button key={notice.id} className={notice.read ? '' : 'judex-v2-notice-unread'} onPress={() => { p.setState(s => ({ ...s, notices: s.notices.map(n => n.id === notice.id ? { ...n, read: true } : n) })); if (notice.roomId) p.openGroup(notice.roomId); p.setModal({ name: null }); }}><span><Bell/></span><p>{p.text(notice.text)}</p><ArrowUpRight/></Button>)}</div>{!p.state.notices.length && <Empty title={p.t('vpNoNotifications')}/>}</Dialog>;
}
function NewProject() {
 const p = usePreview(); const [name, setName] = useState(''); const [description, setDescription] = useState('');
 const create = () => { const id = uid(); p.setState(s => ({ ...s, projects: [...s.projects, { id, name: c(name, name), description: c(description, description), members: ['me'], roles: structuredClone(p.project.roles), workflows: structuredClone(p.project.workflows), tone: 'olive', icon: 'flower', limit: 10 }] })); p.setModal({ name: null }); p.setScreen('projects'); p.toast(p.t('vpProjectCreated')); };
 return <Dialog title={p.t('vpNewProject')} description={p.t('vpNewProjectHint')} footer={<><Button onPress={() => p.setModal({ name: null })}>{p.t('vpCancel')}</Button><Button variant="primary" isDisabled={!name.trim()} onPress={create}>{p.t('vpCreateProject')}<ArrowRight/></Button></>}><Field label={p.t('vpName')} value={name} onChange={setName} placeholder={p.t('vpNamePlaceholder')}/><Field label={p.t('vpDescription')} value={description} onChange={setDescription} multiline/></Dialog>;
}
function Help() {
 const p = usePreview();
 return <Dialog title={p.t('vpHelp')} description="Judex V2 · A space to work together." footer={<><Button onPress={p.reset}><RotateCcw/>{p.t('vpReset')}</Button><Button variant="primary" onPress={() => p.setModal({ name: null })}>{p.t('vpClose')}</Button></>}><div className="judex-v2-about"><Brand/><p>{p.t('vpDemoHint')}</p></div><div className="judex-v2-demo-journeys"><Button onPress={() => { p.openProject('judex'); p.setModal({ name: null }); }}><GroupIcon icon="layout"/><span><strong>{p.locale === 'en' ? 'Explore the workspace' : '看看新的工作空间'}</strong><small>{p.locale === 'en' ? 'Groups, conversations and a reader that opens on demand.' : '群聊、子群与按需展开的文件阅读。'}</small></span><ArrowRight/></Button><Button aria-label={p.t('vpTryLoop')} onPress={() => { p.openGroup('agents'); p.setState(s => ({ ...s, discussions: s.discussions.map(d => d.id === 'loop-demo' ? { ...d, status: 'paused', feedback: '' } : d) })); p.setModal({ name: null }); }}><GroupIcon icon="sparkle" tone="peach"/><span><strong>{p.t('vpTryLoop')}</strong><small>{p.locale === 'en' ? 'Give direction, continue freely, or stop this discussion.' : '给出意见继续，直接继续，或终止本次讨论。'}</small></span><ArrowRight/></Button><Button onPress={() => { p.openProject('judex'); p.setModal({ name: 'push' }); }}><GroupIcon icon="file" tone="lilac"/><span><strong>{p.t('vpPush')}</strong><small>{p.locale === 'en' ? 'See your roles respond. Try the record-only option.' : '查看本人岗位如何响应，或选择本次只记录。'}</small></span><ArrowRight/></Button></div><small className="judex-v2-inline-hint">{p.locale === 'en' ? 'Inspired by Kylon’s visual language. This is an independent Judex prototype, with no external services connected.' : '参考 Kylon 的视觉语言，按 Judex 已确认规则独立制作；没有连接任何外部业务服务。'}</small></Dialog>;
}
export function Dialogs() {
 const p = usePreview();
 switch (p.modal.name) {
  case 'create': return <GroupSettings key={'create:' + p.modal.parentId} create/>;
  case 'group': return p.group ? <GroupSettings key={'edit:' + p.group.id}/> : null;
  case 'project': return <ProjectSettings key={p.project.id}/>;
  case 'library': return <FileLibrary/>;
  case 'knowledge': return <Knowledge/>;
  case 'share': return <ShareDialog/>;
  case 'push': return <PushDialog/>;
  case 'dm': return <ChatDialog/>;
  case 'notifications': return <Notifications/>;
  case 'new-project': return <NewProject/>;
  case 'help': return <Help/>;
  default: return null;
 }
}
