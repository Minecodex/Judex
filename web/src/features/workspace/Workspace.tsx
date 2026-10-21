import {Bell,Check,BookOpen,LayoutGrid,LogOut,MessageCircle,MessagesSquare,Moon,Sun,User,Languages} from 'lucide-react';
import {WorkspaceProvider,useWorkspace} from './context';
import {Button,Brand,IconButton,Avatar,ActionMenu} from './ui';
import {Projects} from './Projects';
import {PaneLayout} from './PaneLayout';
import {Sidebar} from './Sidebar';
import {Chat} from './Chat';
import {ReaderPane} from './Files';
import {Dialogs} from './dialogs';
import {logout} from '../auth/api';
import {useDrafts} from './store';
import '../../styles/workspace.css';
import '../../styles/workspace-live.css';
import '../../styles/workspace-navigation.css';
import '../../styles/workspace-settings.css';
import '../../styles/workspace-flow.css';

function Rail(){const w=useWorkspace();const unread=w.notices.data?.filter(n=>!n.readAt).length||0;return <nav className="judex-v2-rail" aria-label={w.t('vpWorkspace')}>
 <Button className="judex-v2-rail-brand" aria-label="Judex" onPress={()=>w.navigate('/')}><Brand compact/></Button><div className="judex-v2-rail-main">
 <IconButton label={w.t('vpProjects')} className={w.screen==='projects'?'judex-v2-rail-active':''} onPress={()=>w.navigate('/')}><LayoutGrid/></IconButton><span className="judex-v2-rail-divider"/>
 <IconButton label={w.t('vpGroups')} className={w.screen==='groups'?'judex-v2-rail-active':''} onPress={()=>w.projectId?w.navigate('/?project='+w.projectId):w.navigate('/')}><MessagesSquare/></IconButton>
 <IconButton label={w.t('vpChats')} className={w.screen==='chats'?'judex-v2-rail-active':''} onPress={()=>w.navigate('/?view=chats'+(w.projectId?'&project='+w.projectId:''))}><MessageCircle/></IconButton>
 <span className="judex-v2-rail-divider"/><span className="judex-v2-notification-wrap"><IconButton label={w.t('vpNotifications')} onPress={()=>w.setModal({name:'notifications'})}><Bell/></IconButton>{unread>0&&<b>{unread}</b>}</span>
 </div><div className="judex-v2-rail-bottom"><IconButton label={w.t('guideTitle')} onPress={()=>w.setModal({name:'cli'})}><BookOpen/></IconButton><ActionMenu label={w.t('v2Account')} className="judex-v2-profile-button" items={[
 {id:'account',label:w.t('v2Account'),icon:<User/>,action:()=>w.setModal({name:'account'})},
 {id:'appearance',label:w.t('vpAppearance'),icon:w.theme==='dark'?<Sun/>:<Moon/>,action:()=>w.setTheme(w.theme==='dark'?'light':'dark')},
 {id:'language',label:w.t('vpLanguage'),icon:<Languages/>,action:()=>w.setLocale(w.locale==='en'?'zh-CN':'en')},
 {id:'logout',label:w.t('v2Logout'),icon:<LogOut/>,action:()=>{void logout().finally(()=>{useDrafts.getState().clearUser(w.user.id);void w.auth.signOut();});}}
 ]}><Avatar userId={w.user.id} avatarVersion={w.user.avatarVersion} name={w.user.displayName}/></ActionMenu></div></nav>;}
function Workspace(){const w=useWorkspace();return <div className="judex-v2-app"><div className={'judex-v2-frame'+(w.expanded&&w.reader?' judex-v2-frame-reading':'')}><Rail/>{w.screen==='projects'?<Projects/>:<PaneLayout list={<Sidebar/>} chat={<Chat/>} reader={<ReaderPane key={w.roomId}/>}/>}</div><Dialogs/>{w.toast&&<div className="judex-v2-toast" role="status"><span><Check/></span>{w.toast}</div>}{w.uploading&&<div className="judex-v2-upload-status" role="status">{w.uploading}</div>}</div>;}
export default function WorkspaceApp(){return <WorkspaceProvider><Workspace/></WorkspaceProvider>;}
