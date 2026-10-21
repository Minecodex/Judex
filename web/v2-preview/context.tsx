import { createContext, useContext, useEffect, useState, type ReactNode, type Dispatch, type SetStateAction } from 'react';
import { v2PreviewZh, v2PreviewEn, type V2Key } from '../src/i18n/v2Preview';
import { c, uid, now, descendants, type DemoState, type Copy, type FileRef, type ModalState, type Person, type Project, type Group, type Message, type SharedFile } from './model';
import { seed, people as initialPeople } from './seed';

const STORAGE = 'judex.v2.preview.20261019';
type Preview = {
 state: DemoState; setState: Dispatch<SetStateAction<DemoState>>; people: Person[];
 locale: string; setLocale: (locale: string) => void; theme: string; setTheme: (theme: string) => void;
 screen: 'projects' | 'groups' | 'chats'; project: Project; group?: Group; roomId: string;
 groupId: string; setGroupId: (id: string) => void; dmId: string; setDmId: (id: string) => void;
 setScreen: (screen: 'projects' | 'groups' | 'chats') => void; openProject: (id: string) => void; openGroup: (id: string) => void;
 modal: ModalState; setModal: (modal: ModalState) => void; reader: FileRef | null; setReader: (ref: FileRef | null) => void;
 expanded: boolean; setExpanded: (expanded: boolean) => void;
 t: (key: V2Key, values?: Record<string, number | string>) => string; text: (copy: Copy | undefined) => string;
 toast: (text: string) => void; toastText: string; roleLabel: (personId: string, roleId: string) => string;
 send: (text: string, files?: FileRef[]) => void; push: (text: string, recordOnly: boolean, includeFile: boolean) => void;
 archive: (id: string) => void; resolveDiscussion: (id: string, action: 'continued' | 'stopped', feedback: string) => void;
 upload: (files: FileList | null, updateId?: string) => Promise<void>; reset: () => void; filesInRoom: SharedFile[];
};
const Context = createContext<Preview | null>(null);
const loadState = () => { try { const saved = JSON.parse(localStorage.getItem(STORAGE) || 'null'); return saved?.guests && saved?.hiddenFiles ? saved as DemoState : seed(); } catch { return seed(); } };
export function PreviewProvider({ children }: { children: ReactNode }) {
 const params = new URLSearchParams(location.search);
 const [state, setState] = useState(loadState);
 const [locale, setLocale] = useState(() => localStorage.getItem('argus.locale') === 'en' ? 'en' : 'zh-CN');
 const [theme, setTheme] = useState(() => localStorage.getItem('judex.v2.theme') || 'light');
 const [screen, setScreen] = useState<Preview['screen']>(() => params.get('view') === 'chats' ? 'chats' : params.has('project') ? 'groups' : 'projects');
 const [projectId, setProjectId] = useState(params.get('project') || 'judex');
 const [groupId, setGroupId] = useState(params.get('group') || 'workspace');
 const [dmId, setDmId] = useState(params.get('dm') || 'dm-chen');
 const [modal, setModal] = useState<ModalState>({ name: null });
 const [reader, setReader] = useState<FileRef | null>(null);
 const [expanded, setExpanded] = useState(false);
 const [toastText, setToastText] = useState('');
 const people = [...initialPeople, ...state.guests];
 const project = state.projects.find(p => p.id === projectId) || state.projects[0];
 const group = state.groups.find(g => g.id === groupId && g.projectId === project.id && g.members.me);
 const roomId = screen === 'chats' ? dmId : group?.id || '';
 const text = (copy: Copy | undefined) => copy ? locale === 'en' ? copy.en : copy.zh : '';
 const t: Preview['t'] = (key, values = {}) => Object.entries(values).reduce((s, [k, v]) => s.replaceAll(`{${k}}`, String(v)), (locale === 'en' ? v2PreviewEn : v2PreviewZh)[key]);
 const toast = (value: string) => setToastText(value);
 useEffect(() => { try { localStorage.setItem(STORAGE, JSON.stringify(state)); } catch { setToastText(t('vpStorageError')); } }, [state]);
 useEffect(() => { document.documentElement.dataset.theme = theme; document.documentElement.classList.toggle('dark', theme === 'dark'); localStorage.setItem('judex.v2.theme', theme); }, [theme]);
 useEffect(() => { localStorage.setItem('argus.locale', locale); document.documentElement.lang = locale; document.title = locale === 'en' ? 'Judex — A space to work together' : 'Judex — 把想法，聊成可能'; }, [locale]);
 useEffect(() => { const timer = setTimeout(() => setToastText(''), 4000); return () => clearTimeout(timer); }, [toastText]);
 useEffect(() => { setReader(null); setExpanded(false); }, [roomId, screen]);
 useEffect(() => { const query = screen === 'projects' ? '' : screen === 'chats' ? `?view=chats&project=${project.id}&dm=${dmId}` : `?project=${project.id}&group=${groupId}`; history.replaceState(null, '', location.pathname + query); }, [screen, project.id, groupId, dmId]);
 const roleLabel = (personId: string, roleId: string) => `${text(people.find(p => p.id === personId)?.name)}·${text(project.roles.find(r => r.id === roleId)?.name)} AI`;
 const openGroup = (id: string) => { const next = state.groups.find(g => g.id === id && g.members.me); if (!next) return; setProjectId(next.projectId); setGroupId(id); setScreen('groups'); setState(s => ({ ...s, groups: s.groups.map(g => g.id === id ? { ...g, unread: 0 } : g) })); };
 const openProject = (id: string) => { setProjectId(id); const next = state.groups.find(g => g.projectId === id && g.members.me && !g.archived); setGroupId(next?.id || ''); setScreen('groups'); if (next) setState(s => ({ ...s, groups: s.groups.map(g => g.id === next.id ? { ...g, unread: 0 } : g) })); };
 const systemMessage = (room: string, message: Copy): Message => ({ id: uid(), roomId: room, authorId: 'me', kind: 'system', text: message, time: now() });
 const mockReply = (personId: string, roleId: string, content: string): Message => ({
   id: uid(), roomId, authorId: personId, roleId, time: now(),
   text: roleId === 'engineer' ? c('收到，我会从实现边界来看。建议把聊天和文件阅读分开维护状态，让关闭阅读区后继续讨论更自然。这个判断已经参考了本群的文件与最新讨论。', 'I’ll look at the implementation boundaries. Keep chat and reader state separate so closing a document returns naturally to the discussion. This considers the group’s files and latest conversation.') : roleId === 'product' ? c('从产品目标看，这属于需要大家一起权衡的体验问题。我会优先关注使用场景与取舍，建议从“找到讨论 → 参与讨论 → 阅读成果”这条路径来判断。', 'From a product perspective, this is an experience trade-off for the team. I’d evaluate the journey from finding a conversation, to joining it, to reading an outcome.') : c('我会从阅读和操作体验来看。建议保持清楚的信息层级，把最常用的操作留在对话附近，让次要设置在需要时出现。最终采用哪种方式，由大家决定。', 'I’ll focus on the reading and interaction experience. Keep hierarchy clear, common actions near the conversation, and secondary settings available on demand. The team can decide which approach to use.'),
   tool: c(`read · 本群最新聊天与文件；回应「${content.slice(0, 42)}」的公开依据已进入公共知识`, `read · Latest group conversation and files; public evidence for “${content.slice(0, 42)}” added to shared knowledge`),
 });
 const send: Preview['send'] = (content, files = []) => {
  if (!roomId || (!content.trim() && !files.length)) return;
  const message: Message = { id: uid(), roomId, authorId: 'me', text: c(content.trim(), content.trim()), time: now(), files };
  const replies: Message[] = []; const notices: DemoState['notices'] = [];
  if (screen === 'groups' && group) {
   for (const [personId, roles] of Object.entries(group.members)) {
    for (const roleId of roles) if (content.includes('@' + roleLabel(personId, roleId))) replies.push(mockReply(personId, roleId, content));
    const name = text(people.find(p => p.id === personId)?.name);
    if (personId !== 'me' && content.includes('@' + name) && !content.includes('@' + name + '·')) notices.push({ id: uid(), roomId, text: c(`已通知${name}查看你的提及，未唤醒其岗位 AI。`, `${name} was notified of your mention. Their AI was not woken.`), read: false });
   }
  }
  setState(s => ({ ...s, messages: [...s.messages, message, ...replies], notices: [...notices, ...s.notices], groups: s.groups.map(g => g.id === roomId ? { ...g, preview: message.text, time: now() } : g), dms: s.dms.map(d => d.id === roomId ? { ...d, preview: message.text, time: now(), unread: 0 } : d) }));
 };
 const push: Preview['push'] = (content, recordOnly, includeFile) => {
  if (!group) return;
  const file: SharedFile | null = includeFile ? { id: uid(), roomId, name: c('本地体验记录.md', 'Local experience notes.md'), kind: 'text', versions: [{ number: 1, authorId: 'me', date: now(), note: c('由本地工作区推送', 'Pushed from local workspace'), content }] } : null;
  const message: Message = { id: uid(), roomId, authorId: 'me', kind: 'local', text: c(content, content), time: now(), files: file ? [{ fileId: file.id, version: 1 }] : [] };
  const replies = recordOnly ? [] : (group.members.me || []).map(r => mockReply('me', r, content));
  setState(s => ({ ...s, files: file ? [...s.files, file] : s.files, messages: [...s.messages, message, ...replies], groups: s.groups.map(g => g.id === roomId ? { ...g, preview: message.text, time: now() } : g) }));
  setModal({ name: null }); toast(t('vpPushed'));
 };
 const archive = (id: string) => {
  const target = state.groups.find(g => g.id === id); if (!target?.members.me) return;
  const ids = descendants(id, state.groups); const archived = !target.archived;
  setState(s => ({ ...s, groups: s.groups.map(g => ids.includes(g.id) ? { ...g, archived } : g) }));
  setModal({ name: null }); toast(t(archived ? 'vpArchivedToast' : 'vpUnarchivedToast'));
 };
 const resolveDiscussion: Preview['resolveDiscussion'] = (id, action, feedback) => {
  const record = state.discussions.find(d => d.id === id); if (!record) return;
  const message = c(`${action === 'continued' ? '让 AI 继续讨论。' : '终止了本次 AI 讨论。'}${feedback ? `\n意见：${feedback}` : ''}`, `${action === 'continued' ? 'Let the AI discussion continue.' : 'Stopped this AI discussion.'}${feedback ? `\nDirection: ${feedback}` : ''}`);
  const replies = action === 'continued' ? [mockReply('chen', 'engineer', feedback || '继续讨论'), mockReply('me', 'design', feedback || '继续讨论')].map(m => ({ ...m, roomId: record.roomId })) : [];
  setState(s => ({ ...s, discussions: s.discussions.map(d => d.id === id ? { ...d, status: action, feedback } : d), messages: [...s.messages, systemMessage(record.roomId, message), ...replies], notices: s.notices.map(n => n.roomId === record.roomId ? { ...n, read: true } : n) }));
 };
 const upload: Preview['upload'] = async (list, updateId) => {
  if (!list?.length || !roomId) return;
  for (const input of Array.from(list)) {
   if (input.size > 1024 * 1024) { toast(t('vpFileSize')); continue; }
   const isText = /\.(md|txt|json|css|js|ts|tsx|csv|html|yaml|yml|log)$/i.test(input.name) || input.type.startsWith('text/');
   const isImage = /^image\/(png|jpeg|webp|gif)$/.test(input.type);
   const content = isText ? await input.text() : await new Promise<string>((resolve, reject) => { const r = new FileReader(); r.onload = () => resolve(String(r.result)); r.onerror = reject; r.readAsDataURL(input); });
   const original = state.files.find(f => f.id === updateId && f.roomId === roomId);
   const number = original ? original.versions.at(-1)!.number + 1 : 1;
   const kind: SharedFile['kind'] = isText ? 'text' : isImage ? 'image' : 'binary';
   const version = { number, authorId: 'me', date: now(), note: c('通过网页上传', 'Uploaded from web'), content, kind };
   const file: SharedFile = original ? { ...original, versions: [...original.versions, version] } : { id: uid(), roomId, name: c(input.name, input.name), kind: isText ? 'text' : isImage ? 'image' : 'binary', mime: input.type, versions: [version] };
   const msg: Message = { id: uid(), roomId, authorId: 'me', text: original ? c('更新了一份文件。', 'Updated a file.') : c('分享一份文件。', 'Sharing a file.'), time: now(), files: [{ fileId: file.id, version: number }] };
   setState(s => ({ ...s, files: original ? s.files.map(f => f.id === original.id ? file : f) : [...s.files, file], messages: [...s.messages, msg], hiddenFiles: { ...s.hiddenFiles, [roomId]: (s.hiddenFiles[roomId] || []).filter(id => id !== file.id) } }));
   toast(t(original ? 'vpVersionUpdated' : 'vpUploaded'));
  }
 };
 const reset = () => { setState(seed()); setModal({ name: null }); setScreen('projects'); setProjectId('judex'); setGroupId('workspace'); setDmId('dm-chen'); setReader(null); setExpanded(false); toast(t('vpResetToast')); };
 const sharedIds = new Set(state.messages.filter(m => m.roomId === roomId).flatMap(m => m.files?.map(f => f.fileId) || []));
 const filesInRoom = state.files.filter(f => ((f.roomId === roomId && !f.removed) || sharedIds.has(f.id)) && !(state.hiddenFiles[roomId] || []).includes(f.id));
 return <Context.Provider value={{ state, setState, people, locale, setLocale, theme, setTheme, screen, setScreen, project, group, roomId, groupId, setGroupId, dmId, setDmId, openProject, openGroup, modal, setModal, reader, setReader, expanded, setExpanded, t, text, toast, toastText, roleLabel, send, push, archive, resolveDiscussion, upload, reset, filesInRoom }}>{children}</Context.Provider>;
}
export const usePreview = () => { const value = useContext(Context); if (!value) throw new Error('V2 preview context missing'); return value; };
