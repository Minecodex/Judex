import { useEffect, useRef, useState } from 'react';
import { Card, Input, Label, TextArea, TextField } from '@heroui/react';
import {
  ArrowLeft, ArrowUpRight, Check, CheckCheck, ChevronDown, ChevronRight,
  Circle, CircleCheck, FileText, FolderOpen, Inbox, Maximize2, MessageCircle, Minimize2,
  PanelRightClose, PanelRightOpen, Paperclip, Plus, Search, Send, Settings, Sparkles, X,
} from 'lucide-react';
import { Action, Badge, Brand, DemoDialog, EmptyState, IconAction, PersonAvatar, useDemo } from './shared';
import type { CopyKey } from './copy';
import type { Topic } from './model';

function SeedConversation({ onProposal }: { onProposal: () => void }) {
  const { t, locale } = useDemo();
  const isZh = locale === 'zh-CN';
  return <>
    <article className="judex-demo-message"><PersonAvatar /><div className="judex-demo-message-body"><header><strong>{isZh ? '林晓' : 'Xiaolin'}</strong><time>10:32</time></header><p>{isZh ? '目前登录和项目列表太空了，按钮和排版也缺少层次。想先看一版整体优化方案，再一起确定方向。' : 'Sign-in and the project list feel too empty. Actions and typography need a clearer hierarchy. Let’s review a cohesive proposal before choosing a direction.'}</p></div></article>
    <article className="judex-demo-message"><PersonAvatar initials="CA" color="violet" /><div className="judex-demo-message-body"><header><strong>{isZh ? '陈安' : 'Chen'}</strong><time>10:35</time></header><p>{isZh ? '赞同。项目内保留现在的三栏协作方式，把常用操作的位置和间距统一，聊天还是主要的阅读空间。' : 'Agreed. Keep the three-column workspace, align common actions and spacing, and give the conversation room to breathe.'}</p></div></article>
    <article className="judex-demo-message judex-demo-ai-message"><span className="judex-demo-ai-avatar"><Sparkles /></span><div className="judex-demo-message-body"><header><strong>{t('aiRole')}</strong><Badge tone="green">{t('aiTag')}</Badge><time>10:38</time></header><div className="judex-demo-ai-content"><p>{isZh ? '可以从三个层面整理这次优化：' : 'We can organize this improvement in three parts:'}</p><ol><li><strong>{isZh ? '入口更明确' : 'A clearer starting point'}</strong><span>{isZh ? '登录聚焦表单；项目卡片突出名称、角色和进入操作。' : 'Keep sign-in focused. Give project names, roles and entry actions a clear hierarchy.'}</span></li><li><strong>{isZh ? '讨论更好读' : 'More readable discussions'}</strong><span>{isZh ? '统一消息宽度、段落间距和 AI 建议的视觉区分。' : 'Align message widths and spacing, and distinguish AI suggestions.'}</span></li><li><strong>{isZh ? '工作更顺手' : 'Work within reach'}</strong><span>{isZh ? '主操作突出，辅助操作收敛；任务与资料随时在右侧打开。' : 'Emphasize the next action. Keep tasks and files close at hand.'}</span></li></ol><p className="judex-demo-ai-footnote">{t('aiBoundary')}</p></div></div></article>
    <Card className="judex-demo-conversation-proposal"><Card.Header><div className="judex-demo-row"><span className="judex-demo-proposal-label"><FolderOpen />{t('proposal')}</span><Badge tone="amber">{t('waitingConfirmation')}</Badge></div><Card.Title>{t('proposalTitle')}</Card.Title><Card.Description>{t('proposalBody')}</Card.Description></Card.Header><Card.Footer><div className="judex-demo-avatar-stack"><PersonAvatar small /><PersonAvatar small initials="CA" color="violet" /><PersonAvatar small initials="ZN" color="sand" /></div><Action variant="outline" size="sm" onPress={onProposal}>{t('reviewProposal')}<ArrowUpRight /></Action></Card.Footer></Card>
  </>;
}

function WorkPanel({ tab, setTab, expanded, onExpand, onClose, onProposal, confirmed, handoffAccepted, onHandoff }: {
  tab: 'tasks' | 'materials' | 'decisions'; setTab: (tab: 'tasks' | 'materials' | 'decisions') => void;
  expanded: boolean; onExpand: () => void; onClose: () => void; onProposal: () => void;
  confirmed: boolean; handoffAccepted: boolean; onHandoff: () => void;
}) {
  const { t, project } = useDemo();
  const [task, setTask] = useState<CopyKey | null>(null);
  const [file, setFile] = useState<CopyKey | null>(null);
  const taskNames: CopyKey[] = ['taskOne', 'taskTwo', 'taskThree', 'taskFour'];
  const [taskFilter, setTaskFilter] = useState('all');
  const populated = project.id === 'judex';

  return <aside className="judex-demo-work-panel" aria-label={t('workspace')}>
    <div className="judex-demo-pane-bar"><nav className="judex-demo-panel-tabs" aria-label={t('workspace')}>{(['tasks', 'materials', 'decisions'] as const).map((id) => <Action key={id} size="sm" aria-pressed={tab === id} className={tab === id ? 'judex-demo-panel-tab-active' : ''} onPress={() => setTab(id)}>{t(id)}{id === 'decisions' && populated && (!confirmed || !handoffAccepted) && <span className="judex-demo-unread-dot" />}</Action>)}</nav><div className="judex-demo-actions"><IconAction size="sm" label={t(expanded ? 'restorePanel' : 'expandPanel')} onPress={onExpand}>{expanded ? <Minimize2 /> : <Maximize2 />}</IconAction><IconAction size="sm" label={t('hidePanel')} onPress={onClose}><X /></IconAction></div></div>
    <div className="judex-demo-panel-content">
      <div className="judex-demo-panel-heading"><p className="judex-demo-eyebrow">{tab === 'tasks' ? 'KEEP MOVING' : tab === 'materials' ? 'SHARED CONTEXT' : 'YOUR NEXT STEP'}</p><h2>{t(tab === 'decisions' ? 'yourDecisions' : tab)}</h2><p>{t(tab === 'tasks' ? 'taskHint' : tab === 'materials' ? 'filesHint' : 'decisionHint')}</p></div>
      {!populated ? <EmptyState icon={<FolderOpen />} title={t('newProjectWelcome')} description={t('newProjectHint')} /> : tab === 'tasks' ? <>
        <Card className="judex-demo-plan-summary"><div className="judex-demo-row"><strong>{t('planTitle')}</strong><ArrowUpRight /></div><div className="judex-demo-progress-label"><span>{t('progress')}</span><span>1 / 4 {t('completed')}</span></div><div className="judex-demo-progress-track" role="progressbar" aria-label={t('progress')} aria-valuenow={25} aria-valuemin={0} aria-valuemax={100}><span /></div></Card>
        <div className="judex-demo-task-filters">{['all', 'active', 'done'].map((id) => <Action key={id} size="sm" className={taskFilter === id ? 'judex-demo-filter-active' : ''} aria-pressed={taskFilter === id} onPress={() => setTaskFilter(id)}>{t(id === 'all' ? 'all' : id === 'active' ? 'inProgress' : 'done')}</Action>)}</div>
        <div className="judex-demo-task-list">{taskNames.map((key, index) => ({ key, index })).filter(({ index }) => taskFilter === 'all' || (taskFilter === 'done' ? index === 0 : index === 1)).map(({ key, index }) => <Action className={`judex-demo-task-row ${index === 0 ? 'judex-demo-task-complete' : ''}`} key={key} onPress={() => setTask(key)}><span className="judex-demo-task-status">{index === 0 ? <CircleCheck /> : index === 1 ? <span className="judex-demo-progress-circle" /> : <Circle />}</span><span><strong>{t(key)}</strong><span className="judex-demo-task-subtitle">{index < 2 ? (t('you') + ' · ') : ''}{t(index === 0 ? 'done' : index === 1 ? 'inProgress' : 'toDo')}</span></span><ChevronRight /></Action>)}</div>
        <div className="judex-demo-panel-note"><Sparkles /><p>{t('aiBoundary')}</p></div>
      </> : tab === 'materials' ? <div className="judex-demo-file-list">{(['fileOne', 'fileTwo'] as CopyKey[]).map((key) => <Action className="judex-demo-file-row" key={key} onPress={() => setFile(key)}><span className="judex-demo-file-icon"><FileText /></span><span><strong>{t(key)}</strong><span>{t('updatedBy')}</span></span><ArrowUpRight /></Action>)}</div> : <div className="judex-demo-decisions-list">
        {!confirmed && <Card className="judex-demo-decision-card"><Badge tone="amber">{t('proposal')}</Badge><Card.Title>{t('proposalTitle')}</Card.Title><Card.Description>{t('proposalHint')}</Card.Description><Action variant="primary" size="sm" onPress={onProposal}>{t('reviewProposal')}<ArrowRightIcon /></Action></Card>}
        {!handoffAccepted && <Card className="judex-demo-decision-card"><Badge>{t('handoffs')}</Badge><Card.Title>{t('receivedHandoff')}</Card.Title><Card.Description>{t('handoffHint')}</Card.Description><Action variant="outline" size="sm" onPress={onHandoff}>{t('openFile')}<ArrowUpRight /></Action></Card>}
        {confirmed && handoffAccepted && <EmptyState icon={<CheckCheck />} title={t('allDone')} description={t('allDoneHint')} />}
      </div>}
    </div>
    <DemoDialog open={!!task} onClose={() => setTask(null)} title={task ? t(task) : t('taskDetail')} description={t('taskDetailHint')} footer={<Action onPress={() => setTask(null)} variant="primary">{t('close')}</Action>}>
      <div className="judex-demo-detail-fields"><div><span>{t('assignee')}</span><strong>{t('you')}</strong></div><div><span>{t('reviewer')}</span><strong>{t('proposalScopeValue').split(' · ')[1]}</strong></div><div><span>{t('due')}</span><strong>{t('dueValue')}</strong></div></div><div className="judex-demo-detail-section"><h3>{t('acceptance')}</h3><p>{t('taskCriteria')}</p></div><div className="judex-demo-detail-section"><h3>{t('relatedDiscussion')}</h3><p>{t('planTitle')}</p></div>
    </DemoDialog>
    <DemoDialog open={!!file} onClose={() => setFile(null)} title={file ? t(file) : t('openFile')} description={t('updatedBy')} footer={<Action onPress={() => setFile(null)}>{t('close')}</Action>}><div className="judex-demo-detail-section"><h3>{t('proposalTitle')}</h3><p>{t('fileContent')}</p></div></DemoDialog>
  </aside>;
}
function ArrowRightIcon() { return <ChevronRight />; }

export function WorkspacePage() {
  const { t, text, locale, project, topics: allTopics, messages, navigate, createTopic, addMessage, notify, openSettings } = useDemo();
  const topics = allTopics.filter((topic) => topic.projectId === project.id);
  const [activeId, setActiveId] = useState(topics[0]?.id ?? '');
  const [opened, setOpened] = useState(topics.slice(0, 2).map((topic) => topic.id));
  const [search, setSearch] = useState('');
  const [newTopic, setNewTopic] = useState(false);
  const [topicName, setTopicName] = useState('');
  const [panel, setPanel] = useState(true);
  const [expanded, setExpanded] = useState(false);
  const [panelTab, setPanelTab] = useState<'tasks' | 'materials' | 'decisions'>('tasks');
  const [proposal, setProposal] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const [handoff, setHandoff] = useState(false);
  const [handoffAccepted, setHandoffAccepted] = useState(false);
  const [drafts, setDrafts] = useState<Record<string, { text: string; files: string[] }>>({});
  const fileInput = useRef<HTMLInputElement>(null);
  const scroll = useRef<HTMLDivElement>(null);
  const topic = topics.find((value) => value.id === activeId);
  const draft = drafts[activeId] ?? { text: '', files: [] };
  const currentMessages = messages.filter((message) => message.topicId === activeId);
  const openTopic = (value: Topic) => { setActiveId(value.id); setOpened((ids) => ids.includes(value.id) ? ids : [...ids, value.id]); setExpanded(false); };
  const send = () => {
    if (!activeId || (!draft.text.trim() && !draft.files.length)) return;
    addMessage(activeId, draft.text.trim(), draft.files);
    setDrafts((values) => ({ ...values, [activeId]: { text: '', files: [] } }));
  };
  useEffect(() => { scroll.current?.scrollTo({ top: currentMessages.length ? scroll.current.scrollHeight : 0, behavior: 'auto' }); }, [currentMessages.length, activeId]);

  return <main className={`judex-demo-workspace ${!panel ? 'judex-demo-focused' : ''} ${expanded ? 'judex-demo-expanded' : ''}`}>
    <aside className="judex-demo-sidebar">
      <div className="judex-demo-sidebar-brand"><Brand small /><IconAction size="sm" label={t('returnProjects')} onPress={() => navigate('projects')}><ArrowLeft /></IconAction></div>
      <Action className="judex-demo-project-switch" variant="outline" onPress={() => navigate('projects')} aria-label={t('returnProjects')}><span className={`judex-demo-project-mark judex-demo-tone-${project.color}`}><FolderOpen /></span><span><strong>{text(project.title)}</strong><span>{project.members} {t('members')}</span></span><ChevronDown /></Action>
      <Action variant="primary" fullWidth onPress={() => setNewTopic(true)}><Plus />{t('newDiscussion')}</Action>
      <TextField value={search} onChange={setSearch} aria-label={t('searchDiscussions')} className="judex-demo-search"><Search /><Input placeholder={t('searchDiscussions')} /></TextField>
      <div className="judex-demo-sidebar-section"><span>{t('conversations')}</span><span>{topics.length}</span></div>
      <nav className="judex-demo-topic-list" aria-label={t('conversations')}>
        {topics.filter((item) => text(item.title).toLocaleLowerCase().includes(search.toLocaleLowerCase())).map((item) => <Action key={item.id} className={`judex-demo-topic ${activeId === item.id ? 'judex-demo-topic-active' : ''}`} aria-current={activeId === item.id ? 'page' : undefined} onPress={() => openTopic(item)}><MessageCircle /><span>{text(item.title)}</span>{item.seeded && <span className="judex-demo-unread-dot" />}</Action>)}
        {search && !topics.some((item) => text(item.title).toLocaleLowerCase().includes(search.toLocaleLowerCase())) && <p className="judex-demo-sidebar-empty">{t('noDiscussions')}</p>}
      </nav>
      <div className="judex-demo-sidebar-section"><span>{t('handoffs')}</span></div>
      <Action className="judex-demo-sidebar-link" onPress={() => { setPanel(true); setPanelTab('decisions'); }}><Inbox />{t('inbox')}{project.id === 'judex' && !handoffAccepted && <span className="judex-demo-count">1</span>}</Action>
      <div className="judex-demo-sidebar-spacer" />
      <Action className="judex-demo-sidebar-decision" onPress={() => { setPanel(true); setPanelTab('decisions'); }}><CircleCheck /><span>{t('yourDecisions')}</span>{project.id === 'judex' && (!confirmed || !handoffAccepted) && <span className="judex-demo-count">{Number(!confirmed) + Number(!handoffAccepted)}</span>}</Action>
      <Action className="judex-demo-account-row" onPress={openSettings}><PersonAvatar /><span><strong>{locale === 'zh-CN' ? '林晓' : 'Xiaolin'}</strong><span>{t('account')}</span></span><Settings /></Action>
    </aside>
    <section className="judex-demo-conversation">
      <div className="judex-demo-pane-bar"><div className="judex-demo-chat-tabs">{opened.map((id) => {
        const value = topics.find((item) => item.id === id);
        if (!value) return null;
        return <div className={`judex-demo-chat-tab ${activeId === id ? 'judex-demo-chat-tab-active' : ''}`} key={id}><Action size="sm" onPress={() => openTopic(value)} aria-current={activeId === id ? 'page' : undefined} aria-label={text(value.title)}><MessageCircle /><span title={text(value.title)}>{text(value.title)}</span></Action>{opened.length > 1 && <IconAction size="sm" label={`${t('close')} · ${text(value.title)}`} onPress={() => { const remaining = opened.filter((value) => value !== id); setOpened(remaining); if (activeId === id) setActiveId(remaining[0]); }}><X /></IconAction>}</div>;
      })}</div><IconAction size="sm" label={t(panel ? 'hidePanel' : 'showPanel')} onPress={() => { setPanel(!panel); setExpanded(false); }}>{panel ? <PanelRightClose /> : <PanelRightOpen />}</IconAction></div>
      <div className="judex-demo-conversation-scroll" ref={scroll}>
        <div className="judex-demo-conversation-content">
          {!topic ? <EmptyState icon={<MessageCircle />} title={t('newProjectWelcome')} description={t('newProjectHint')}><Action variant="primary" onPress={() => setNewTopic(true)}><Plus />{t('newDiscussion')}</Action></EmptyState> : <>
            <div className="judex-demo-date-divider"><span />{t('today')}<span /></div>
            {topic.seeded ? <SeedConversation onProposal={() => setProposal(true)} /> : currentMessages.length === 0 && <EmptyState icon={<MessageCircle />} title={t('emptyDiscussion')} description={t('emptyDiscussionHint')} />}
            {currentMessages.map((message) => <article key={message.id} className="judex-demo-message judex-demo-message-self"><PersonAvatar /><div className="judex-demo-message-body"><header><strong>{t('you')}</strong><time>{message.time}</time><CheckCheck /></header>{message.text && <p>{message.text}</p>}{message.attachments.map((name) => <span className="judex-demo-attachment" key={name}><Paperclip />{name}</span>)}</div></article>)}
          </>}
        </div>
      </div>
      <div className="judex-demo-composer-area">
        <div className="judex-demo-composer">
          <TextField aria-label={t('messagePlaceholder')} value={draft.text} onChange={(value) => setDrafts((values) => ({ ...values, [activeId]: { ...draft, text: value } }))} isDisabled={!topic}>
            <TextArea placeholder={t('messagePlaceholder')} rows={2} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); send(); } }} />
          </TextField>
          {draft.files.length > 0 && <div className="judex-demo-draft-files">{draft.files.map((name) => <span className="judex-demo-attachment" key={name}><Paperclip />{name}<IconAction size="sm" label={`${t('close')} · ${name}`} onPress={() => setDrafts((values) => ({ ...values, [activeId]: { ...draft, files: draft.files.filter((file) => file !== name) } }))}><X /></IconAction></span>)}</div>}
          <div className="judex-demo-composer-tools"><div className="judex-demo-actions"><IconAction label={t('attach')} isDisabled={!topic} onPress={() => fileInput.current?.click()}><Paperclip /></IconAction><span>{t('composerHint')}</span></div><Action variant="primary" size="sm" isDisabled={!topic || (!draft.text.trim() && !draft.files.length)} onPress={send}>{t('send')}<Send /></Action></div>
        </div>
        <p className="judex-demo-composer-note"><span />{t('aiBoundary')}</p>
        <input ref={fileInput} type="file" multiple hidden onChange={(event) => { const files = Array.from(event.target.files ?? []).map((file) => file.name); setDrafts((values) => ({ ...values, [activeId]: { ...draft, files: [...new Set([...draft.files, ...files])] } })); event.target.value = ''; if (files.length) notify(t('attachmentReady')); }} />
      </div>
    </section>
    {panel && <WorkPanel tab={panelTab} setTab={setPanelTab} expanded={expanded} onExpand={() => setExpanded(!expanded)} onClose={() => { setPanel(false); setExpanded(false); }} onProposal={() => setProposal(true)} confirmed={confirmed} handoffAccepted={handoffAccepted} onHandoff={() => setHandoff(true)} />}
    <DemoDialog open={newTopic} onClose={() => setNewTopic(false)} title={t('newDiscussion')} description={t('discussionHint')}>
      <form className="judex-demo-form" onSubmit={(event) => { event.preventDefault(); if (!topicName.trim()) return; const id = createTopic(topicName.trim()); setActiveId(id); setOpened((ids) => [...ids, id]); setNewTopic(false); setTopicName(''); setExpanded(false); }}><TextField isRequired value={topicName} onChange={setTopicName} name="topic"><Label>{t('discussTitle')}</Label><Input autoFocus placeholder={t('discussPlaceholder')} maxLength={100} /></TextField><div className="judex-demo-form-actions"><Action onPress={() => setNewTopic(false)}>{t('cancel')}</Action><Action type="submit" variant="primary" isDisabled={!topicName.trim()}>{t('newDiscussion')}</Action></div></form>
    </DemoDialog>
    <DemoDialog open={proposal} onClose={() => setProposal(false)} title={t('proposalTitle')} description={t('proposalBody')} wide footer={<><Action onPress={() => setProposal(false)}>{t('close')}</Action><Action variant="primary" isDisabled={confirmed} onPress={() => { setConfirmed(true); notify(t('approvedToast')); }}><Check />{t(confirmed ? 'approved' : 'agree')}</Action></>}>
      <div className="judex-demo-detail-fields"><div><span>{t('proposalOwner')}</span><strong>{locale === 'zh-CN' ? '林晓' : 'Xiaolin'}</strong></div><div><span>{t('proposalScope')}</span><strong>{t('proposalScopeValue')}</strong></div></div><div className="judex-demo-detail-section"><h3>{t('proposalOutcome')}</h3><p>{t('proposalCriteria')}</p></div><p className="judex-demo-detail-notice">{t('proposalNotice')}</p>
    </DemoDialog>
    <DemoDialog open={handoff} onClose={() => setHandoff(false)} title={t('receivedHandoff')} description={t('handoffHint')} footer={<><Action onPress={() => setHandoff(false)}>{t('cancel')}</Action><Action variant="primary" onPress={() => { setHandoffAccepted(true); setHandoff(false); notify(t('handoffAccepted')); }}><Check />{t('receive')}</Action></>}><div className="judex-demo-detail-section"><FileText /><h3>{t('fileTwo')}</h3><p>{t('proposalCriteria')}</p></div></DemoDialog>
  </main>;
}
