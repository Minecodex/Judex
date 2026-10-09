import {useEffect,useRef,useState} from "react";
import {createRoot} from "react-dom/client";
import {Check,X} from "lucide-react";
import {cooperationPreviewZh,cooperationPreviewEn} from "../src/i18n/cooperationPreview";
import {usePreferences} from "../src/stores/preferences";
import {apply,type Action} from "./actions";
import {seed} from "./seed";
import {routeFromHash,routeHash,type Route,type State} from "./model";
import {PreviewProvider,type ModalState,type Preview} from "./context";
import {Button,Header} from "./ui";
import {ProjectList,ProjectHub} from "./boards";
import {PlanRoute} from "./routes";
import {ChatView} from "./chat";
import {Dialogs} from "./dialogs";
import "./style.css";
const key="judex.cooperation-preview.v1";
function read(){try{const s=JSON.parse(localStorage.getItem(key)||"null");if(s?.schema===1)return s as State;}catch{}return seed();}
function App(){
 const [state,setState]=useState(read),ref=useRef(state),[route,setRoute]=useState(routeFromHash),[modal,setModal]=useState<ModalState|null>(null),[notice,notify]=useState(""),[signedOut,setSignedOut]=useState(false);
 const {locale,theme,setLocale,setTheme}=usePreferences();
 const text:Preview["text"]=value=>typeof value==="string"?value:value[locale==="en"?"en":"zh"];
 const t:Preview["t"]=k=>(locale==="en"?cooperationPreviewEn:cooperationPreviewZh)[k];
 const go=(r:Route)=>{setModal(null);location.hash=routeHash(r);setRoute(r);};
 const perform=(a:Action)=>{const result=apply(ref.current,a);if(result.state){ref.current=result.state;setState(result.state);}if(result.error)notify(t(result.error));else if(result.notice)notify(t(result.notice));return result;};
 useEffect(()=>{try{localStorage.setItem(key,JSON.stringify(state));}catch{}},[state]);
 useEffect(()=>{const pop=()=>{setRoute(routeFromHash());setModal(null);};window.addEventListener("hashchange",pop);return()=>window.removeEventListener("hashchange",pop);},[]);
 useEffect(()=>{document.documentElement.lang=locale;document.documentElement.dataset.theme=theme;document.documentElement.classList.toggle("dark",theme==="dark");document.title="Judex · "+t("cpPreview");},[locale,theme]);
 useEffect(()=>{if(!notice)return;const timer=setTimeout(()=>notify(""),4500);return()=>clearTimeout(timer);},[notice]);
 const value:Preview={
  state,route,locale,theme,text,t,perform,go,setModal,setLocale,setTheme,notify,
  person:who=>who==="ai"?t("cpAI"):text(state.people.find(p=>p.id===who)?.name??who),
  planChat:p=>go({page:"chat",projectId:p.projectId,planId:p.id,topicId:p.mainTopicId}),
  taskChat:task=>{const result=perform({kind:"ensure-task",taskId:task.id});if(result.id)go({page:"chat",projectId:task.projectId,taskId:task.id,topicId:result.id});},
  topicChat:(topic,scope)=>{const task=scope?.taskId??(scope?.planId?undefined:route.taskId);const plan=scope?.planId??route.planId??topic.planIds[0]??state.tasks.find(t=>topic.taskIds.includes(t.id))?.planId;go({page:"chat",projectId:topic.projectId,taskId:task,planId:task?undefined:plan,topicId:topic.id,messageSeq:scope?.messageSeq});},
  reset:()=>{const fresh=seed();ref.current=fresh;setState(fresh);try{localStorage.setItem(key,JSON.stringify(fresh));for(const item of Object.keys(sessionStorage))if(item.startsWith("judex.cooperation."))sessionStorage.removeItem(item);}catch{}go({page:"projects",projectId:"launch"});},
  logout:()=>{setSignedOut(true);setModal(null);},
 };
 return <PreviewProvider value={value}><div className="judex-co-app"><Header/>{signedOut?<main className="judex-co-signed-out"><h1>{t("cpSignedOut")}</h1><Button variant="primary" onPress={()=>setSignedOut(false)}>{t("cpResume")}</Button></main>:route.page==="projects"?<ProjectList/>:route.page==="hub"?<ProjectHub/>:route.page==="route"?<PlanRoute/>:<ChatView/>}{!signedOut&&modal&&<Dialogs modal={modal} onClose={()=>setModal(null)}/>}<div className="judex-co-preview-note">{t("cpPreviewHint")}</div>{notice&&<div className="judex-co-toast" role="status"><Check size={16}/><span>{notice}</span><Button size="sm" isIconOnly aria-label={t("cpClose")} onPress={()=>notify("")}><X size={14}/></Button></div>}</div></PreviewProvider>;
}
createRoot(document.getElementById("root")!).render(<App/>);
