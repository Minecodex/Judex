import { TextArea } from '@heroui/react';
import { Archive, ArrowUp, AtSign, Check, ChevronDown, ChevronRight, CircleStop, Ellipsis, FolderOpen, Layers3, Monitor, Paperclip, Play, Plus, Sparkles, Users, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { usePreview } from './context';
import { Avatar, Button, Empty, GroupIcon, IconButton, Tabs } from './ui';
import { FileCard } from './Files';
import type { Discussion, Message } from './model';

function MessageText({ value }: { value: string }) {
 return <>{value.split('\n').map((line, i) => <p key={i}>{line.split(/(@[^\s，。！？]+(?: AI)?)/g).map((part, j) => part.startsWith('@') ? <span className="judex-v2-mention-text" key={j}>{part}</span> : part)}</p>)}</>;
}
function ChatMessage({ message }: { message: Message }) {
 const p = usePreview(); const author = p.people.find(person => person.id === message.authorId);
 if (message.kind === 'system') return <div className="judex-v2-system-message"><Check/><span><MessageText value={p.text(message.text)}/></span><time>{message.time}</time></div>;
 const role = p.project.roles.find(r => r.id === message.roleId);
 return <div className={'judex-v2-message' + (message.roleId ? ' judex-v2-message-ai' : '')} data-testid={message.roleId ? 'ai-message' : 'human-message'}>
  <Avatar person={author} ai={!!message.roleId}/><div className="judex-v2-message-main"><div className="judex-v2-message-author"><strong>{p.text(author?.name)}</strong>{message.roleId ? <span className="judex-v2-role-label">{p.text(role?.name)}<small>AI</small></span> : message.authorId === 'me' && <small>{p.t('vpMe')}</small>}<time>{message.time}</time>{message.kind === 'local' && <span className="judex-v2-local-tag"><Monitor/>{p.t('vpLocal')}</span>}</div>
   <div className="judex-v2-message-bubble"><MessageText value={p.text(message.text)}/></div>{message.files?.map((ref, i) => <FileCard key={ref.fileId + ':' + ref.version + ':' + i} reference={ref}/>)}
   {message.tool && <details className="judex-v2-tool-result"><summary><Sparkles/><span>{p.locale === 'en' ? 'Source & tool result' : '依据与工具结果'}</span><ChevronDown/></summary><p>{p.text(message.tool)}</p></details>}
  </div>
 </div>;
}
function DiscussionCard({ discussion }: { discussion: Discussion }) {
 const p = usePreview(); const [feedback, setFeedback] = useState('');
 if (discussion.status !== 'paused') return <div className="judex-v2-discussion-result"><span>{discussion.status === 'stopped' ? <CircleStop/> : <Play/>}</span><div><strong>{p.t(discussion.status === 'stopped' ? 'vpStopped' : 'vpContinued')}</strong><p>{p.t(discussion.status === 'stopped' ? 'vpStoppedHint' : 'vpContinuedHint')}</p>{discussion.status === 'continued' && <Button onPress={() => p.resolveDiscussion(discussion.id, 'stopped', '')}>{p.t('vpStop')}</Button>}</div></div>;
 return <div className="judex-v2-discussion-card" data-testid="discussion-card"><div className="judex-v2-discussion-card-top"><span><Sparkles/>{p.locale === 'en' ? 'A LITTLE HUMAN DIRECTION' : '让人来定方向'}</span><b>{discussion.loops} / {discussion.limit}</b></div><h3>{p.t('vpPauseTitle')}</h3><p>{p.t('vpPauseDescription', { n: discussion.loops })}</p><TextArea aria-label={p.t('vpFeedback')} placeholder={p.t('vpFeedback')} value={feedback} onChange={e => setFeedback(e.target.value)} rows={2}/><div className="judex-v2-discussion-actions"><Button variant="primary" onPress={() => p.resolveDiscussion(discussion.id, 'continued', feedback)}><Play/>{p.t('vpContinue')}</Button><Button onPress={() => p.resolveDiscussion(discussion.id, 'stopped', feedback)}>{p.t('vpStop')}</Button></div><div className="judex-v2-discussion-notice"><div className="judex-v2-avatar-stack">{discussion.people.map(id => <Avatar key={id} person={p.people.find(v => v.id === id)} small/>)}</div><small>{p.t('vpNotified')}</small></div></div>;
}
function Composer() {
 const p = usePreview(); const [drafts, setDrafts] = useState<Record<string, string>>({}); const draft = drafts[p.roomId] || '';
 const [mentionOpen, setMentionOpen] = useState(false); const [mentionType, setMentionType] = useState('ai');
 const input = useRef<HTMLInputElement>(null); const textarea = useRef<HTMLTextAreaElement>(null); const composing = useRef(false);
 useEffect(() => { setMentionOpen(false); }, [p.roomId]);
 const update = (value: string) => setDrafts(s => ({ ...s, [p.roomId]: value }));
 const send = () => { if (draft.trim()) { p.send(draft); update(''); setMentionOpen(false); textarea.current?.focus(); } };
 const mention = (label: string) => { update(draft.replace(/@$/, '') + '@' + label + ' '); setMentionOpen(false); textarea.current?.focus(); };
 const members = p.screen === 'chats' ? p.people.filter(person => person.id === p.state.dms.find(d => d.id === p.dmId)?.personId) : p.people.filter(person => p.group?.members[person.id]);
 return <div className="judex-v2-composer-area"><div className="judex-v2-composer">
  {mentionOpen && <div className="judex-v2-mention-picker"><header><strong>{p.t('vpChooseMention')}</strong><IconButton label={p.t('vpClose')} onPress={() => setMentionOpen(false)}><X/></IconButton></header>{p.screen !== 'chats' && <Tabs label={p.t('vpMention')} value={mentionType} onChange={setMentionType} items={[{ id: 'ai', label: p.t('vpAI') }, { id: 'human', label: p.t('vpHuman') }]}/>}
   <div>{members.flatMap(person => mentionType === 'ai' && p.screen !== 'chats' ? (p.group?.members[person.id] || []).map(roleId => <Button className="judex-v2-mention-option" key={person.id + roleId} onPress={() => mention(p.roleLabel(person.id, roleId))}><Avatar person={person} ai small/><span><strong>{p.text(p.project.roles.find(r => r.id === roleId)?.name)} AI</strong><small>{p.text(person.name)} · {p.t('vpAI')}</small></span><AtSign/></Button>) : [<Button className="judex-v2-mention-option" key={person.id} onPress={() => mention(p.text(person.name))}><Avatar person={person} small/><span><strong>{p.text(person.name)}</strong><small>{p.text(person.subtitle)}</small></span><AtSign/></Button>])}</div>
  </div>}
  <TextArea ref={textarea} aria-label={p.t(p.screen === 'chats' ? 'vpDMComposer' : 'vpComposer')} placeholder={p.t(p.screen === 'chats' ? 'vpDMComposer' : 'vpComposer')} value={draft} onChange={e => { update(e.target.value); if (e.target.value.endsWith('@')) setMentionOpen(true); }} onCompositionStart={() => { composing.current = true; }} onCompositionEnd={() => { composing.current = false; }} onKeyDown={e => { if (e.key === 'Escape') setMentionOpen(false); if (e.key === 'Enter' && !e.shiftKey && !composing.current && !e.nativeEvent.isComposing) { e.preventDefault(); send(); } }} rows={2}/>
  <div className="judex-v2-composer-actions"><input ref={input} type="file" multiple hidden onChange={e => { void p.upload(e.target.files); e.target.value = ''; }}/><IconButton label={p.t('vpAttach')} onPress={() => input.current?.click()}><Plus/></IconButton><IconButton label={p.t('vpMention')} onPress={() => setMentionOpen(!mentionOpen)}><AtSign/></IconButton><IconButton label={p.t(p.screen === 'chats' ? 'vpPrivateFiles' : 'vpFiles')} onPress={() => p.setModal({ name: 'library' })}><Paperclip/></IconButton>{p.screen !== 'chats' && <IconButton label={p.t('vpKnowledge')} onPress={() => p.setModal({ name: 'knowledge' })}><Layers3/></IconButton>}<span/><small>{p.t('vpEnter')}</small><Button isIconOnly className="judex-v2-send" variant="primary" aria-label={p.t('vpSend')} isDisabled={!draft.trim()} onPress={send}><ArrowUp/></Button></div>
 </div><div className="judex-v2-composer-hint">{p.screen === 'chats' ? <Users/> : <Sparkles/>}{p.t(p.screen === 'chats' ? 'vpHumanHint' : 'vpMentionHint')}</div></div>;
}
export function Chat() {
 const p = usePreview(); const dm = p.screen === 'chats'; const conversation = p.state.dms.find(d => d.id === p.dmId); const person = p.people.find(v => v.id === conversation?.personId);
 const messages = p.state.messages.filter(m => m.roomId === p.roomId); const end = useRef<HTMLDivElement>(null); const feed = useRef<HTMLDivElement>(null); const previousRoom = useRef(''); const previousCount = useRef(0);
 useEffect(() => { if (previousRoom.current === p.roomId && messages.length > previousCount.current) end.current?.scrollIntoView({ behavior: 'smooth', block: 'end' }); else if (feed.current) feed.current.scrollTop = 0; previousRoom.current = p.roomId; previousCount.current = messages.length; }, [p.roomId, messages.length]);
 if (!p.roomId || (!dm && !p.group)) return <section className="judex-v2-chat-empty"><Empty title={p.t('vpNoResults')} description={p.t('vpNoResultsSub')}><Button variant="primary" onPress={() => p.setModal({ name: 'create' })}><Plus/>{p.t('vpNewGroup')}</Button></Empty></section>;
 const parent = p.state.groups.find(g => g.id === p.group?.parentId && g.members.me);
 return <section className={'judex-v2-chat' + (p.reader ? ' judex-v2-chat-with-reader' : '')} aria-label={dm ? p.text(person?.name) : p.text(p.group?.name)}>
  <header className="judex-v2-chat-header"><div className="judex-v2-chat-heading">{dm ? <Avatar person={person} large online/> : <GroupIcon icon={p.group?.icon} tone={p.group?.tone} large/>}<div><Button className="judex-v2-chat-title" onPress={() => p.setModal({ name: dm ? 'dm' : 'group' })}>{dm ? p.text(person?.name) : p.text(p.group?.name)}{!dm && <ChevronDown/>}</Button><p>{dm ? <><span className={'judex-v2-status-dot' + (person?.online ? ' judex-v2-status-online' : '')}/>{p.t(person?.online ? 'vpOnline' : 'vpOffline')}<i/>{p.t('vpPureHuman')}</> : <>{parent && <Button onPress={() => p.openGroup(parent.id)}>{p.text(parent.name)}<ChevronRight/></Button>}<span>{p.text(p.group?.purpose)}</span></>}</p></div></div>
   <div className="judex-v2-chat-header-actions">{!dm && <Button className="judex-v2-members-button" aria-label={p.t('vpMembers')} onPress={() => p.setModal({ name: 'group' })}><div className="judex-v2-avatar-stack">{Object.keys(p.group?.members || {}).slice(0, 3).map(id => <Avatar key={id} person={p.people.find(v => v.id === id)} small/>)}</div><small>{Object.keys(p.group?.members || {}).length}</small></Button>}<IconButton label={p.t(dm ? 'vpPrivateFiles' : 'vpFiles')} onPress={() => p.setModal({ name: 'library' })}><FolderOpen/></IconButton>{!dm && <IconButton label={p.t('vpKnowledge')} onPress={() => p.setModal({ name: 'knowledge' })}><Layers3/></IconButton>}{!dm && <IconButton label={p.t('vpGroupSettings')} onPress={() => p.setModal({ name: 'group' })}><Ellipsis/></IconButton>}</div>
  </header>
  {p.group?.archived && !dm && <div className="judex-v2-archive-banner"><Archive/>{p.t('vpArchivedHint')}</div>}
  <div className="judex-v2-message-feed" ref={feed}><div className="judex-v2-chat-date"><span/>{p.t('vpToday')}<span/></div><div className="judex-v2-messages">{messages.map(message => <ChatMessage key={message.id} message={message}/>)}{p.state.discussions.filter(d => d.roomId === p.roomId).map(d => <DiscussionCard key={d.id} discussion={d}/>)}{!messages.length && <Empty title={dm ? p.t('vpChats') : p.text(p.group?.name)} description={p.text(p.group?.purpose) || p.t('vpNoResultsSub')}/>}<div ref={end}/></div></div>
  <Composer/>
 </section>;
}
