import { createRoot } from 'react-dom/client';
import { Bell, Check, CircleHelp, LayoutGrid, MessageCircle, MessagesSquare, Moon, Sun } from 'lucide-react';
import { PreviewProvider, usePreview } from './context';
import { Avatar, Brand, Button, IconButton } from './ui';
import { Projects } from './Projects';
import { Sidebar } from './Sidebar';
import { Chat } from './Chat';
import { Reader } from './Files';
import { Dialogs } from './dialogs';
import '../src/styles/workspace.css';

function Rail() {
 const p = usePreview(); const unread = p.state.notices.filter(n => !n.read).length;
 return <nav className="judex-v2-rail" aria-label={p.t('vpWorkspace')}><Button className="judex-v2-rail-brand" aria-label="Judex" onPress={() => p.setScreen('projects')}><Brand compact/></Button>
  <div className="judex-v2-rail-main"><IconButton label={p.t('vpProjects')} className={p.screen === 'projects' ? 'judex-v2-rail-active' : ''} onPress={() => p.setScreen('projects')}><LayoutGrid/></IconButton><span className="judex-v2-rail-divider"/><IconButton label={p.t('vpGroups')} className={p.screen === 'groups' ? 'judex-v2-rail-active' : ''} onPress={() => p.setScreen('groups')}><MessagesSquare/></IconButton><IconButton label={p.t('vpChats')} className={p.screen === 'chats' ? 'judex-v2-rail-active' : ''} onPress={() => p.setScreen('chats')}><MessageCircle/></IconButton><span className="judex-v2-rail-divider"/><span className="judex-v2-notification-wrap"><IconButton label={p.t('vpNotifications')} onPress={() => p.setModal({ name: 'notifications' })}><Bell/></IconButton>{unread > 0 && <b>{unread}</b>}</span></div>
  <div className="judex-v2-rail-bottom"><IconButton label={p.t('vpAppearance')} onPress={() => p.setTheme(p.theme === 'dark' ? 'light' : 'dark')}>{p.theme === 'dark' ? <Sun/> : <Moon/>}</IconButton><Button className="judex-v2-language" aria-label={p.t('vpLanguage')} onPress={() => p.setLocale(p.locale === 'en' ? 'zh-CN' : 'en')}>{p.locale === 'en' ? '中' : 'EN'}</Button><IconButton label={p.t('vpHelp')} onPress={() => p.setModal({ name: 'help' })}><CircleHelp/></IconButton><span className="judex-v2-rail-divider"/><Button className="judex-v2-profile-button" aria-label={p.t('vpProjectSettings')} onPress={() => p.setModal({ name: 'project' })}><Avatar person={p.people[0]} online/></Button></div>
 </nav>;
}
function App() {
 const p = usePreview();
 return <div className="judex-v2-app"><div className={'judex-v2-frame' + (p.expanded && p.reader ? ' judex-v2-frame-reading' : '')}><Rail/>{p.screen === 'projects' ? <Projects/> : <><Sidebar/><Chat/><Reader/></>}</div><Dialogs/>{p.toastText && <div className="judex-v2-toast" role="status"><span><Check/></span>{p.toastText}</div>}</div>;
}
createRoot(document.getElementById('root')!).render(<PreviewProvider><App/></PreviewProvider>);
