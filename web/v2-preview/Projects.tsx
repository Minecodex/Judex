import {ProjectArt} from '../src/components/ProjectArt';
import { Card, Input } from '@heroui/react';
import { ArrowUpRight, MoreHorizontal, Plus, Search, Sparkles } from 'lucide-react';
import { useState } from 'react';
import { usePreview } from './context';
import { Avatar, Brand, Button, GroupIcon, IconButton, Tabs } from './ui';

export function Projects() {
 const p = usePreview(); const [search, setSearch] = useState(''); const [tab, setTab] = useState('all');
 const projects = p.state.projects.filter(project => (p.text(project.name) + p.text(project.description)).toLowerCase().includes(search.toLowerCase()));
 return <main className="judex-v2-projects">
  <header className="judex-v2-projects-top"><Brand/><span className="judex-v2-demo-label">V2 <span>· {p.t('vpDemo')}</span></span><div className="judex-v2-projects-account"><span>{p.t('vpWorkspace')}</span><Avatar person={p.people[0]} small/></div></header>
  <div className="judex-v2-projects-content"><section className="judex-v2-welcome"><div><p className="judex-v2-eyebrow"><span className="judex-v2-live-dot"/>{p.t('vpMorning')}</p><h1>{p.t('vpWelcome')}</h1><p>{p.t('vpWelcomeSub')}</p></div><Button variant="primary" size="md" onPress={() => p.setModal({ name: 'new-project' })}><Plus/>{p.t('vpNewProject')}</Button></section>
   <div className="judex-v2-projects-toolbar"><Tabs label={p.t('vpProjects')} value={tab} onChange={setTab} items={[{ id: 'all', label: p.t('vpAllProjects'), count: p.state.projects.length }, { id: 'recent', label: p.t('vpRecent') }]}/><div className="judex-v2-search"><Search/><Input aria-label={p.t('vpProjectSearch')} placeholder={p.t('vpProjectSearch')} value={search} onChange={e => setSearch(e.target.value)}/></div></div>
   <div className="judex-v2-project-grid">{(tab === 'recent' ? projects.slice(0, 2) : projects).map((project, i) => <Card className="judex-v2-project-card" key={project.id}>
    <Button className="judex-v2-project-hit" aria-label={p.t('vpOpenSpace') + ' · ' + p.text(project.name)} data-testid={'project-' + project.id} onPress={() => p.openProject(project.id)}/>
    <ProjectArt tone={project.tone} variant={i}/><Card.Header><div className="judex-v2-project-title"><GroupIcon icon={project.icon} tone={project.tone}/><Card.Title>{p.text(project.name)}</Card.Title><span className="judex-v2-project-menu"><IconButton label={p.t('vpProjectSettings') + ' · ' + p.text(project.name)} onPress={() => { p.openProject(project.id); p.setModal({ name: 'project' }); }}><MoreHorizontal/></IconButton></span></div><Card.Description>{p.text(project.description)}</Card.Description></Card.Header>
    <Card.Content><span className="judex-v2-project-meta"><span className="judex-v2-live-dot"/>{p.t('vpUpdated')}</span></Card.Content><Card.Footer><div className="judex-v2-avatar-stack">{project.members.slice(0, 4).map(id => <Avatar key={id} person={p.people.find(person => person.id === id)} small/>)}</div><span>{project.members.length} {p.t('vpPeople')}<i/> {p.state.groups.filter(g => g.projectId === project.id && g.members.me && !g.archived).length} {p.t('vpGroupCount')}</span><ArrowUpRight/></Card.Footer>
   </Card>)}</div>
   {!projects.length && <p className="judex-v2-no-results">{p.t('vpNoResults')}</p>}
   <div className="judex-v2-projects-bottom"><span><Sparkles/>{p.t('vpReference')}</span><span>{p.t('vpInfo')}</span></div>
  </div><footer className="judex-v2-projects-footer"><span>Judex · A space to work together.</span><Button onPress={() => p.setModal({ name: 'help' })}>{p.t('vpHelp')}<ArrowUpRight/></Button></footer>
 </main>;
}
