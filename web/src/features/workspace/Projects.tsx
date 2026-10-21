import {Card,Input} from '@heroui/react';
import {ArrowUpRight,MoreHorizontal,Plus,Search,Sparkles} from 'lucide-react';
import {useState} from 'react';
import {ProjectArt} from '../../components/ProjectArt';
import {useWorkspace} from './context';
import {Brand,Button,Empty,GroupIcon,IconButton} from './ui';

export function Projects(){const w=useWorkspace(),[search,setSearch]=useState('');const projects=w.projects.data?.filter(p=>(p.name+p.description).toLowerCase().includes(search.toLowerCase()))||[];return <main className="judex-v2-projects">
 <header className="judex-v2-projects-top"><Brand/><span className="judex-v2-demo-label">V2</span></header>
 <div className="judex-v2-projects-content"><section className="judex-v2-welcome"><div><p className="judex-v2-eyebrow">{w.t('vpWorkspace')}</p><h1>{w.t('vpWelcome')}</h1><p>{w.t('vpWelcomeSub')}</p></div><Button variant="primary" size="md" onPress={()=>w.setModal({name:'new-project'})}><Plus/>{w.t('vpNewProject')}</Button></section>
 <div className="judex-v2-projects-toolbar"><strong>{w.t('vpAllProjects')} · {w.projects.data?.length||0}</strong><div className="judex-v2-search"><Search/><Input aria-label={w.t('vpProjectSearch')} placeholder={w.t('vpProjectSearch')} value={search} onChange={e=>setSearch(e.target.value)}/></div></div>
 {w.projects.isPending?<p role="status">{w.t('v2Loading')}</p>:w.projects.isError?<Empty title={w.t('v2Failed')}><Button onPress={()=>void w.projects.refetch()}>{w.t('v2Retry')}</Button></Empty>:<div className="judex-v2-project-grid">{projects.map((project,i)=><Card className="judex-v2-project-card" key={project.id}>
 <Button className="judex-v2-project-hit" aria-label={w.t('vpOpenSpace')+' · '+project.name} onPress={()=>w.openProject(project.id)}/><ProjectArt tone={['olive','peach','lavender'][i%3]} variant={i}/>
 <Card.Header><div className="judex-v2-project-title"><GroupIcon icon="flower"/><Card.Title>{project.name}</Card.Title><span className="judex-v2-project-menu"><IconButton label={w.t('vpProjectSettings')+' · '+project.name} onPress={()=>{w.openProject(project.id);w.setModal({name:'project'});}}><MoreHorizontal/></IconButton></span></div><Card.Description>{project.description}</Card.Description></Card.Header>
 <Card.Content><span className="judex-v2-project-meta">{new Date(project.createdAt).toLocaleDateString(w.locale)}</span></Card.Content><Card.Footer><span>{project.memberCount} {w.t('vpPeople')}</span><ArrowUpRight/></Card.Footer></Card>)}</div>}
 {!w.projects.isPending&&!w.projects.isError&&!projects.length&&<Empty title={search?w.t('vpNoResults'):w.t('v2NoProjects')} description={w.t('vpNewProjectHint')}/>}
 <div className="judex-v2-projects-bottom"><span><Sparkles/>{w.t('vpInfo')}</span></div></div><footer className="judex-v2-projects-footer"><span>Judex · A space to work together.</span><span title={w.t(w.connection==='connected'?'v2Connected':'v2Reconnect')}>{w.t(w.connection==='connected'?'v2Connected':'v2Reconnect')}</span></footer>
 </main>;}
