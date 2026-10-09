import { useState } from 'react';
import { Card, Input, Label, TextArea, TextField } from '@heroui/react';
import { ArrowRight, ArrowUpRight, Command, Folder, Layers, MessageCircle, Plus, Search, Settings, Sparkles } from 'lucide-react';
import { Action, Badge, Brand, DemoDialog, EmptyState, PersonAvatar, useDemo } from './shared';
import type { Project } from './model';
import { people } from './model';

function ProjectCard({ project }: { project: Project }) {
  const { t, text, navigate } = useDemo();
  return <Card className="judex-demo-project-card">
    <Card.Header>
      <div className="judex-demo-row"><span className={`judex-demo-project-mark judex-demo-tone-${project.color}`}>{project.color === 'green' ? <Command /> : project.color === 'violet' ? <Layers /> : <Folder />}</span><Badge>{t(project.role)}</Badge></div>
      <Card.Title>{text(project.title)}</Card.Title>
      <Card.Description>{text(project.description)}</Card.Description>
    </Card.Header>
    <Card.Content><div className="judex-demo-project-meta"><span><MessageCircle />{project.discussionCount} {t('discussions')}</span><span className="judex-demo-meta-dot" /><span>{project.members} {t('members')}</span></div></Card.Content>
    <Card.Footer>
      <div className="judex-demo-avatar-stack" aria-label={`${project.members} ${t('members')}`}>{people.slice(0, Math.min(project.members, 3)).map((person) => <PersonAvatar key={person.initials} initials={person.initials} color={person.color} small />)}{project.members > 3 && <span>+{project.members - 3}</span>}</div>
      <Action className="judex-demo-project-enter" variant="ghost" size="sm" onPress={() => navigate('workspace', project.id)} aria-label={`${t('enterProject')} · ${text(project.title)}`}>{t('enterProject')}<ArrowUpRight /></Action>
    </Card.Footer>
  </Card>;
}

export function ProjectsPage() {
  const { t, text, projects, topics, navigate, createProject, openSettings } = useDemo();
  const [filter, setFilter] = useState('all');
  const [search, setSearch] = useState('');
  const [creating, setCreating] = useState(false);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const filtered = projects.filter((project) => (filter === 'all' || (filter === 'owned' ? project.role === 'owner' : project.role !== 'owner')) && `${text(project.title)} ${text(project.description)}`.toLocaleLowerCase().includes(search.toLocaleLowerCase()));
  const recent = topics.filter((topic) => ['experience', 'tokens', 'reliability'].includes(topic.id));

  return <div className="judex-demo-projects-page">
    <header className="judex-demo-projects-header"><div><Brand small /><span className="judex-demo-header-divider" /><span>{t('internal')}</span></div><div className="judex-demo-actions"><Action size="sm" onPress={openSettings}><Settings />{t('account')}</Action><PersonAvatar /></div></header>
    <main className="judex-demo-projects-content">
      <div className="judex-demo-page-heading"><div><p className="judex-demo-eyebrow">YOUR WORKSPACE</p><h1>{t('myProjects')}</h1><p>{t('projectHint')}</p></div><Action variant="primary" onPress={() => setCreating(true)}><Plus />{t('newProject')}</Action></div>
      <div className="judex-demo-project-toolbar">
        <nav className="judex-demo-filter" aria-label={t('projects')}>{(['all', 'owned', 'joined'] as const).map((id) => <Action key={id} size="sm" aria-pressed={filter === id} onPress={() => setFilter(id)} className={filter === id ? 'judex-demo-filter-active' : ''}>{t(id === 'all' ? 'allProjects' : id === 'owned' ? 'owned' : 'participating')}{id === 'all' && <span>{projects.length}</span>}</Action>)}</nav>
        <TextField aria-label={t('searchProjects')} value={search} onChange={setSearch} className="judex-demo-search"><Search /><Input placeholder={t('searchProjects')} /></TextField>
      </div>
      <div className="judex-demo-project-grid">
        {filtered.map((project) => <ProjectCard project={project} key={project.id} />)}
        {!search && filter === 'all' && <Action className="judex-demo-new-project-card" onPress={() => setCreating(true)}><span className="judex-demo-new-project-icon"><Plus /></span><span className="judex-demo-new-project-copy"><strong>{t('newCardTitle')}</strong><span>{t('newCardHint')}</span></span><span className="judex-demo-new-project-link">{t('newProject')}<ArrowRight /></span></Action>}
      </div>
      {filtered.length === 0 && <EmptyState icon={<Search />} title={t('emptyProjects')} description={t('emptyProjectsHint')} />}
      <section className="judex-demo-recent"><div className="judex-demo-section-heading"><h2>{t('recent')}</h2><span>{t('recentlyUpdated')}</span></div><div className="judex-demo-recent-list">{recent.map((topic, index) => {
        const project = projects.find((value) => value.id === topic.projectId)!;
        return <Action className="judex-demo-recent-row" key={topic.id} onPress={() => navigate('workspace', project.id)}><span className={`judex-demo-recent-icon judex-demo-tone-${project.color}`}><MessageCircle /></span><span className="judex-demo-recent-title"><strong>{text(topic.title)}</strong><span>{text(project.title)}</span></span><span className="judex-demo-recent-date">{t(index === 0 ? 'today' : 'yesterday')} · {index === 0 ? '10:42' : '16:20'}</span><ArrowUpRight /></Action>;
      })}</div></section>
      <footer className="judex-demo-projects-footer"><span>{t('footer')}</span><span><Sparkles />{t('aiBoundary')}</span></footer>
    </main>
    <DemoDialog open={creating} onClose={() => setCreating(false)} title={t('newProject')} description={t('newCardHint')}>
      <form className="judex-demo-form" onSubmit={(event) => { event.preventDefault(); if (!title.trim()) return; createProject(title.trim(), description.trim()); setCreating(false); setTitle(''); setDescription(''); }}>
        <TextField isRequired value={title} onChange={setTitle} name="title"><Label>{t('title')}</Label><Input autoFocus placeholder={t('projectNamePlaceholder')} maxLength={100} /></TextField>
        <TextField value={description} onChange={setDescription} name="description"><Label>{t('description')}</Label><TextArea placeholder={t('projectDescriptionPlaceholder')} maxLength={300} rows={3} /></TextField>
        <div className="judex-demo-form-actions"><Action onPress={() => setCreating(false)}>{t('cancel')}</Action><Action type="submit" variant="primary" isDisabled={!title.trim()}><Plus />{t('create')}</Action></div>
      </form>
    </DemoDialog>
  </div>;
}
