import {useEffect,useRef,useState} from 'react';
import {createRoot} from 'react-dom/client';
import {Check,X} from 'lucide-react';
import {usePreferences} from '../src/stores/preferences';
import {taskRoutePreviewEn,taskRoutePreviewZh,type TaskRoutePreviewKey} from '../src/i18n/taskRoutePreview';
import {apply,type Action} from './actions';
import {seed,STORAGE,type ModalState,type Page,type State} from './model';
import {Provider,type GraphView,type Preview,type PreviewOps} from './context';
import {Header,Button} from './ui';
import {ProjectList,PlanList,PlanPage,FullScreenViewer} from './pages';
import {ChatView} from './chat';
import {Dialogs} from './dialogs';
import './style.css';
function read(){try{const s=JSON.parse(localStorage.getItem(STORAGE)??'null');if(s?.schema===4&&Array.isArray(s.tasks)&&Array.isArray(s.changes))return s as State;}catch{}return seed();}
function hashPage():Page{const [kind,id,taskId]=location.hash.slice(1).split('/');return kind==='projects'?{kind}:kind==='plans'?{kind,projectId:id||'product'}:kind==='chat'?{kind,planId:id||'launch',taskId}:kind==='route'?{kind,planId:id||'launch'}:{kind:'route',planId:'launch'};}
function App(){
 const [state,setStored]=useState(read),stateRef=useRef(state),[page,setPage]=useState<Page>(hashPage),[modal,setModal]=useState<ModalState|null>(null),[drawer,setDrawer]=useState<Preview['drawer']>(null),[viewer,setViewer]=useState<Preview['viewer']>(null),[notice,notify]=useState(''),[views,setViews]=useState<Record<string,GraphView>>({}),native=useRef(false);
 const prefs=usePreferences(),locale=prefs.locale;
 const returnView=useRef<{drawer:Preview['drawer'];planId:string;view:GraphView}|null>(null);
 const text:Preview['text']=v=>typeof v==='string'?v:v[locale==='en'?'en':'zh'];
 const t:Preview['t']=(key,values={})=>{let value:string=(locale==='en'?taskRoutePreviewEn:taskRoutePreviewZh)[key];for(const [k,v] of Object.entries(values))value=value.replaceAll('{'+k+'}',String(v));return value;};
 const setState=(update:(s:State)=>State)=>{const next=update(stateRef.current);stateRef.current=next;setStored(next);};
 const perform=(action:Action)=>{const result=apply(stateRef.current,action);if(result.state){stateRef.current=result.state;setStored(result.state);}if(result.error)notify(t(result.error));else if(result.notice)notify(t(result.notice));return result;};
 const restoreView=()=>{const previous=returnView.current;if(previous){setDrawer(previous.drawer);setViews(old=>({...old,[previous.planId]:previous.view}));returnView.current=null;}};
 const closeViewer=()=>{setViewer(null);restoreView();if(document.fullscreenElement)void document.exitFullscreen().catch(()=>{});};
 const go=(p:Page)=>{closeViewer();setModal(null);setDrawer(null);setPage(p);location.hash=p.kind==='projects'?'projects':p.kind+'/'+(p.kind==='plans'?p.projectId:p.planId)+(p.taskId?'/'+p.taskId:'');};
 const openViewer=(planId:string,preview=false,focus?:string)=>{if(!viewer)returnView.current={drawer,planId,view:{...(views[planId]??{zoom:1,left:0,top:0})}};setViewer({planId,preview,focus});setDrawer(focus?{id:focus,section:'overview'}:drawer);if(!document.fullscreenElement&&document.fullscreenEnabled)void document.documentElement.requestFullscreen().catch(()=>{});};
 useEffect(()=>{try{localStorage.setItem(STORAGE,JSON.stringify(state));}catch{}},[state]);
 useEffect(()=>{document.documentElement.lang=locale;document.documentElement.dataset.theme=prefs.theme;document.documentElement.classList.toggle('dark',prefs.theme==='dark');document.title='Judex · Demo 4 · '+t('d4Route');},[locale,prefs.theme]);
 useEffect(()=>{const change=()=>{setPage(hashPage());setModal(null);setDrawer(null);};window.addEventListener('hashchange',change);return()=>window.removeEventListener('hashchange',change);},[]);
 useEffect(()=>{const change=()=>{if(document.fullscreenElement)native.current=true;else if(native.current){native.current=false;setViewer(null);restoreView();}};document.addEventListener('fullscreenchange',change);return()=>document.removeEventListener('fullscreenchange',change);},[]);
 useEffect(()=>{const key=(e:KeyboardEvent)=>{if(e.key!=='Escape'||modal)return;if(viewer){e.preventDefault();closeViewer();}else if(drawer)setDrawer(null);};document.addEventListener('keydown',key);return()=>document.removeEventListener('keydown',key);},[viewer,drawer,modal]);
 useEffect(()=>{if(!notice)return;const timer=setTimeout(()=>notify(''),4500);return()=>clearTimeout(timer);},[notice]);
 const reset=()=>{const fresh=seed();stateRef.current=fresh;setStored(fresh);setViews({});go({kind:'route',planId:'launch'});};
 const value:Preview&PreviewOps={state,setState,page,go,t,text,locale,modal,setModal,notify,drawer,setDrawer,viewer,openViewer,closeViewer,views,setView:(id,v)=>setViews(old=>({...old,[id]:{...(old[id]??{zoom:1,left:0,top:0}),...v}})),discuss:task=>go({kind:'chat',planId:task.planId,taskId:task.id}),perform};
 return <Provider value={value}><div className="judex-d4-app">{viewer?<FullScreenViewer/>:<><Header reset={reset}/>{page.kind==='projects'?<ProjectList/>:page.kind==='plans'?<PlanList/>:page.kind==='chat'?<ChatView key={(page.planId??'')+':'+(page.taskId??'')}/>:<PlanPage/>}</>}{modal&&<Dialogs key={modal.kind+modal.id} modal={modal}/>} {notice&&<div className="judex-d4-notice" role="status"><Check/><span>{notice}</span><Button size="sm" isIconOnly aria-label={t('d4Close')} onPress={()=>notify('')}><X/></Button></div>}</div></Provider>;
}
createRoot(document.getElementById('root')!).render(<App/>);
