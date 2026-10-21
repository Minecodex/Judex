import { ArrowDownToLine, ArrowRight, ArrowUpRight, Check, FileText, History, Maximize2, Minimize2, Plus, Share2, Sparkles, Upload, X } from 'lucide-react';
import { useRef, useState } from 'react';
import { usePreview } from './context';
import { Avatar, Button, Dialog, Empty, IconButton } from './ui';
import type { FileRef, SharedFile } from './model';

export function availableVersions(file: SharedFile, roomId: string, messages: ReturnType<typeof usePreview>['state']['messages']) {
 return file.roomId === roomId ? file.versions : file.versions.filter(v => messages.some(m => m.roomId === roomId && m.files?.some(ref => ref.fileId === file.id && ref.version === v.number)));
}
export function FileCard({ reference }: { reference: FileRef }) {
 const p = usePreview(); const file = p.state.files.find(f => f.id === reference.fileId); if (!file) return null;
 const kind = file.versions.find(v => v.number === reference.version)?.kind || file.kind;
 return <Button className={'judex-v2-file-card judex-v2-file-card-' + kind} onPress={() => p.setReader(reference)} aria-label={p.t('vpOpenFile') + ' · ' + p.text(file.name) + ' v' + reference.version}>
  <span className="judex-v2-file-card-top"><span><FileText/><strong>{p.text(file.name)}</strong><small>v{reference.version}</small></span><ArrowUpRight/></span>
  {kind === 'brief' && <span className="judex-v2-document-thumb"><span className="judex-v2-thumb-sidebar"><i/><i/><i/><i/></span><span className="judex-v2-thumb-chat"><b>Better, together.</b><span><i/><em/></span><span><i/><em/></span><span className="judex-v2-thumb-composer"/></span><span className="judex-v2-thumb-reader"><b/><i/><i/><span/><i/><i/></span></span>}
  {kind === 'table' && <span className="judex-v2-table-thumb"><span><b>{p.locale === 'en' ? 'OUR IDEAS' : '我们的想法'}</b><b>{p.locale === 'en' ? 'DIRECTION' : '讨论方向'}</b></span>{[p.locale === 'en' ? 'A quieter workspace' : '更安静的工作空间', p.locale === 'en' ? 'Files beside the chat' : '文件就在聊天旁边', p.locale === 'en' ? 'AI when you need it' : 'AI 在需要时参与'].map(label => <span key={label}><i>{label}</i><em><Check/></em></span>)}</span>}
  {!['brief', 'table'].includes(kind) && <span className="judex-v2-file-simple"><span className="judex-v2-file-extension">{p.text(file.name).split('.').at(-1)?.slice(0, 4).toUpperCase()}</span><span>{p.text(file.versions.find(v => v.number === reference.version)?.note)}<small>{file.versions.find(v => v.number === reference.version)?.date}</small></span></span>}
  <span className="judex-v2-file-card-bottom"><span>{kind === 'table' ? 'TABLE' : p.t('vpDocument')}<i/> {p.t('vpPreview')}</span><span>{p.t('vpOpenFile')}<ArrowRight/></span></span>
 </Button>;
}
function Brief({ version }: { version: number }) {
 const p = usePreview(); const en = p.locale === 'en';
 return <article className="judex-v2-document"><div className="judex-v2-document-eyebrow"><span/>JUDEX / EXPERIENCE NOTES</div><h1>{en ? <>A little less friction.<br/>A little more together.</> : <>让协作，<br/>自然发生。</>}</h1><p className="judex-v2-document-intro">{en ? 'A space for people, conversations and the work that happens between them.' : '把人、讨论和成果放在一起。工作不必在一张张表单之间流转，从一段对话开始就好。'}</p>
  <div className="judex-v2-document-byline"><Avatar person={p.people[0]} small/><span>{en ? 'Lin Zhou · Product design' : '林舟 · 产品设计'}</span><span>Oct 19, 2026</span></div>
  <div className="judex-v2-doc-illustration"><div className="judex-v2-doc-art-lines"><i/><i/><i/></div><span className="judex-v2-doc-bubble-a"><Sparkles/>{en ? 'An idea starts here.' : '一个想法，从这里开始。'}</span><span className="judex-v2-doc-bubble-b"><Check/>{en ? 'Let’s make it together.' : '那就一起把它做好。'}</span><div className="judex-v2-doc-art-mark">j.</div></div>
  <h2><small>01</small>{en ? 'Start with the conversation' : '从对话出发'}</h2><p>{en ? 'Create a group for a shared purpose. Bring in the right people and let the conversation find its shape. Subgroups give a focused idea its own space, up to three levels.' : '为一个共同的目的建群，邀请合适的人进来。需要更聚焦的时候，就给这个想法建一个子群；从群到子群，最多三层。'}</p>
  <div className="judex-v2-document-principles"><div><MessageSymbol/><strong>{en ? 'Find your people' : '找到伙伴'}</strong><p>{en ? 'Groups built around a purpose.' : '围绕目的，自由组群。'}</p></div><div><Sparkles/><strong>{en ? 'Think together' : '一起思考'}</strong><p>{en ? 'AI joins when mentioned.' : '需要的时候，@ 岗位 AI。'}</p></div><div><FileText/><strong>{en ? 'Keep the work close' : '看见成果'}</strong><p>{en ? 'Files live in the conversation.' : '成果就在对话旁边。'}</p></div></div>
  <h2><small>02</small>{en ? 'Give the work room to breathe' : '给阅读一点空间'}</h2><p>{en ? 'Keep the reader closed until it is needed. Open a file beside the conversation and keep its exact version, so everyone can point to the same thing.' : '文件阅读区按需展开。看文件时，聊天依然在旁边；关闭文件后，把空间还给对话。每一条消息里的附件，都保留当时引用的那个版本。'}</p>
  <blockquote>{en ? 'A quiet interface lets the conversation be heard.' : '界面安静一点，让对话被听见。'}</blockquote>
  {version > 1 && <><h2><small>03</small>{en ? 'AI has a role, people have a voice' : 'AI 有岗位，人来定方向'}</h2><p>{en ? 'Web messages only wake an AI when explicitly mentioned. A local push wakes your own roles in this group. They share the same group knowledge, with different role and personal prompts.' : '网页消息只有明确 @ 岗位 AI 才会唤醒它。本地推送唤醒本人在当前群的岗位，分别判断职责。各个岗位看见相同的群知识，用不同的职位和个人提示词思考。'}</p><div className="judex-v2-doc-note"><Sparkles/><div><strong>{en ? 'Let people decide when it matters' : '需要取舍时，把方向交给人'}</strong><p>{en ? 'After 10 discussion loops, pause for a little human direction. Continue with a note, continue without one, or stop this discussion.' : '10 个讨论闭环后，卡片会邀请人给出方向。可以写意见继续，也可以直接继续，或终止本次讨论。'}</p></div></div></>}
  <footer>{en ? 'An open proposal. Add your perspective in the conversation.' : '这是一份开放的提案，欢迎回到群里补充你的想法。'}</footer>
 </article>;
}
function MessageSymbol() { return <svg viewBox="0 0 24 24" fill="none"><path d="M20 11a8 8 0 0 1-8 8H5l-3 3V11a9 9 0 1 1 18 0Z" stroke="currentColor" strokeWidth="1.5"/></svg>; }
function TableDocument() {
 const p = usePreview(); const en = p.locale === 'en';
 const rows = en ? [['Conversation layout', 'Quiet surfaces, more breathing room', 'Lin · Ke'], ['File reading', 'Open beside the chat, on demand', 'Mo · Lin'], ['Mentioning AI', 'People and role AIs are distinct', 'Zia · Mo'], ['Group hierarchy', 'Multiple children, three levels max', 'Lin · Zia'], ['Shared knowledge', 'One group context, role-specific prompts', 'Mo · Yao'], ['Discussion loops', 'Pause after 10 loops for human direction', 'Everyone']] : [['聊天布局', '安静的界面，更舒展的阅读节奏', '林舟 · 唐可'], ['文件阅读', '点击后在右侧展开，边看边聊', '陈墨 · 林舟'], ['提及 AI', '明确区分人和岗位 AI', '许知夏 · 陈墨'], ['群聊层级', '多个同级子群，最多三层', '林舟 · 许知夏'], ['公共知识', '相同的群知识，不同岗位提示词', '陈墨 · 陆遥'], ['讨论循环', '10 个闭环后暂停，听人的意见', '所有人']];
 return <article className="judex-v2-document judex-v2-table-document"><div className="judex-v2-document-eyebrow">JUDEX / CO-DESIGN</div><h1>{en ? 'Our ideas, together.' : '把想法，放在一起。'}</h1><p className="judex-v2-document-intro">{en ? 'A living record of this group’s conversations. Perspectives, not assignments.' : '记录大家在群里形成的想法与不同视角，让后来的讨论有据可依。'}</p><div className="judex-v2-record-table"><table><thead><tr><th>{en ? 'TOPIC' : '讨论主题'}</th><th>{en ? 'CURRENT DIRECTION' : '当前讨论方向'}</th><th>{en ? 'PEOPLE' : '参与者'}</th></tr></thead><tbody>{rows.map(([topic, direction, names]) => <tr key={topic}><td>{topic}</td><td>{direction}</td><td>{names}</td></tr>)}</tbody></table></div><div className="judex-v2-doc-note"><LeafSymbol/><p>{en ? 'A shared record, always open to another perspective.' : '共识可以继续生长。想法有变化，就回到对话里接着聊。'}</p></div></article>;
}
function LeafSymbol() { return <Sparkles/>; }
export function Reader() {
 const p = usePreview(); const [history, setHistory] = useState(false); const reference = p.reader;
 const file = p.state.files.find(f => f.id === reference?.fileId); const version = file?.versions.find(v => v.number === reference?.version);
 if (!file || !version || !reference) return null;
 const versions = availableVersions(file, p.roomId, p.state.messages);
 const kind = version.kind || file.kind;
 const download = () => {
  const content = version.content || document.querySelector('.judex-v2-reader-paper')?.textContent || p.text(file.name);
  const dataFile = (kind === 'image' || kind === 'binary') && content.startsWith('data:');
  const url = dataFile ? content : URL.createObjectURL(new Blob([content], { type: 'text/plain;charset=utf-8' }));
  const anchor = document.createElement('a'); anchor.href = url; anchor.download = p.text(file.name) + (p.text(file.name).includes('.') ? '' : '.txt'); anchor.click();
  if (!dataFile) setTimeout(() => URL.revokeObjectURL(url), 1000);
 };
 return <aside className={'judex-v2-reader' + (p.expanded ? ' judex-v2-reader-expanded' : '')} aria-label={p.t('vpPreview')}>
  <header className="judex-v2-reader-header"><span><FileText/><strong>{p.text(file.name)}</strong><small>v{version.number}</small></span><div><IconButton label={p.t('vpVersionHistory')} onPress={() => setHistory(!history)}><History/></IconButton>{p.screen !== 'chats' && <IconButton label={p.t('vpShare')} onPress={() => p.setModal({ name: 'share', file: reference })}><Share2/></IconButton>}<IconButton label={p.t('vpDownload')} onPress={download}><ArrowDownToLine/></IconButton><IconButton label={p.expanded ? p.t('vpCollapse') : p.t('vpExpand')} onPress={() => p.setExpanded(!p.expanded)}>{p.expanded ? <Minimize2/> : <Maximize2/>}</IconButton><IconButton label={p.t('vpClosePreview')} onPress={() => { p.setReader(null); p.setExpanded(false); }}><X/></IconButton></div></header>
  {history && <div className="judex-v2-version-history"><strong>{p.t('vpVersionHistory')}</strong>{[...versions].reverse().map(v => <Button key={v.number} aria-pressed={v.number === version.number} onPress={() => { p.setReader({ fileId: file.id, version: v.number }); setHistory(false); }}><span>v{v.number}<small>{p.text(v.note)}</small></span><span>{v.date}{v.number === version.number && <Check/>}</span></Button>)}</div>}
  <div className="judex-v2-reader-paper">{kind === 'brief' ? <Brief version={version.number}/> : kind === 'table' ? <TableDocument/> : <article className="judex-v2-document judex-v2-plain-document"><div className="judex-v2-document-eyebrow">{p.t('vpDocument')} / V{version.number}</div><h1>{p.text(file.name)}</h1><p className="judex-v2-document-intro">{p.text(version.note)} · {version.date}</p>{kind === 'image' ? <img className="judex-v2-uploaded-image" src={version.content} alt={p.text(file.name)}/> : kind === 'binary' ? <div className="judex-v2-doc-note"><FileText/><div><p>{p.t('vpBinary')}</p><Button variant="outline" onPress={download}>{p.t('vpDownload')}</Button></div></div> : <pre className={kind === 'code' ? 'judex-v2-code' : 'judex-v2-text-content'}>{version.content}</pre>}</article>}</div>
 </aside>;
}
export function FileLibrary() {
 const p = usePreview(); const input = useRef<HTMLInputElement>(null); const [updateId, setUpdateId] = useState<string>();
 return <Dialog wide title={p.t(p.screen === 'chats' ? 'vpPrivateFiles' : 'vpFiles')} description={p.t('vpFileHint')} footer={<><Button onPress={() => p.setModal({ name: null })}>{p.t('vpClose')}</Button><Button variant="primary" onPress={() => { setUpdateId(undefined); input.current?.click(); }}><Plus/>{p.t('vpAttach')}</Button></>}>
  <input ref={input} type="file" hidden onChange={e => { void p.upload(e.target.files, updateId); e.target.value = ''; }}/>
  {p.screen === 'chats' && <p className="judex-v2-inline-hint">{p.t('vpPrivate')}</p>}
  <div className="judex-v2-file-list">{p.filesInRoom.map(file => { const versions = availableVersions(file, p.roomId, p.state.messages); const latest = versions.at(-1); if (!latest) return null; return <div className="judex-v2-file-list-row" key={file.id}><Button className="judex-v2-file-list-open" onPress={() => { p.setReader({ fileId: file.id, version: latest.number }); p.setModal({ name: null }); }}><span className="judex-v2-file-list-symbol"><FileText/></span><span><strong>{p.text(file.name)}</strong><small>{p.text(latest.note)} · v{latest.number}</small></span><ArrowUpRight/></Button><div>{file.roomId === p.roomId && <IconButton label={p.t('vpNewVersion') + ' · ' + p.text(file.name)} onPress={() => { setUpdateId(file.id); input.current?.click(); }}><Upload/></IconButton>}<IconButton label={p.t('vpRemoveFile') + ' · ' + p.text(file.name)} onPress={() => { p.setState(s => ({ ...s, hiddenFiles: { ...s.hiddenFiles, [p.roomId]: [...(s.hiddenFiles[p.roomId] || []), file.id] } })); p.toast(p.t('vpRemoved')); }}><X/></IconButton></div></div>; })}</div>
  {!p.filesInRoom.length && <Empty title={p.t('vpEmptyFiles')} description={p.t('vpEmptyFilesSub')}/>}
 </Dialog>;
}
