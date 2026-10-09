import {useEffect,useRef,useState} from "react";
import {createRoot} from "react-dom/client";
import {Check,Files,MessageCircle,RotateCcw,X} from "lucide-react";
import {Button} from "../src/components/ui/Button";
import {Brand,PersonAvatar,PreferenceControls} from "../src/components/ui/Presentation";
import {usePreferences} from "../src/stores/preferences";
import {translate} from "../src/i18n";
import {PreviewContext,type Dialog,type Mode,type Preview} from "./context";
import {l,seedMaterials,seedMessages} from "./data";
import {Library,Discussion} from "./views";
import {Dialogs} from "./dialogs";
import "../src/styles/material-preview.css";
const readMode=():Mode=>location.hash==="#task"?"task":location.hash==="#plan"?"plan":"library";
function App(){
 const [files,setFiles]=useState(seedMaterials),[messages,setMessages]=useState(seedMessages),[mode,updateMode]=useState(readMode),[dialog,open]=useState<Dialog|null>(null),[notice,notify]=useState(""),[draft,setDraft]=useState(""),[pending,setPending]=useState<string[]>([]);
 const urls=useRef<string[]>([]),{locale,theme}=usePreferences();
 const t:Preview["t"]=(key,values)=>translate(locale,key,values),text:Preview["text"]=value=>value[locale==="en"?"en":"zh"];
 const setMode=(next:Mode)=>{updateMode(next);location.hash=next;};
 const reset=()=>{urls.current.forEach(url=>URL.revokeObjectURL(url));urls.current=[];setFiles(seedMaterials());setMessages(seedMessages());setPending([]);setDraft("");open(null);setMode("library");};
 const send=()=>{
  const valid=pending.filter(id=>files.some(f=>f.id===id&&!f.deleted));if(!draft.trim()&&!valid.length)return;
  const scope=mode==="task"?"task":"plan",time=new Intl.DateTimeFormat("en-GB",{timeZone:"Asia/Shanghai",hour:"2-digit",minute:"2-digit"}).format(new Date());
  setMessages(previous=>[...previous,{id:crypto.randomUUID(),actor:l("林晓","Lin Xiao"),time,text:l(draft.trim(),draft.trim()),fileIds:valid,scope}]);
  setFiles(previous=>previous.map(file=>valid.includes(file.id)?{...file,task:file.task||scope==="task",usages:[...file.usages,{scope,actor:l("林晓","Lin Xiao"),time:"2026-10-08 "+time,note:l("在讨论中分享，供当前工作参考。","Shared in the discussion as a reference for this work.")}]}:file));
  setDraft("");setPending([]);notify(t("mpSentNotice"));
 };
 useEffect(()=>{document.documentElement.lang=locale;document.documentElement.dataset.theme=theme;document.documentElement.classList.toggle("dark",theme==="dark");document.title="Judex · "+t("mpLibrary");},[locale,theme]);
 useEffect(()=>{const change=()=>{updateMode(readMode());open(null);};window.addEventListener("hashchange",change);return()=>window.removeEventListener("hashchange",change);},[]);
 useEffect(()=>{if(!notice)return;const timer=setTimeout(()=>notify(""),4500);return()=>clearTimeout(timer);},[notice]);
 useEffect(()=>{if(messages.length>4)document.querySelector(".judex-mp-messages")?.scrollTo({top:999999,behavior:"smooth"});},[messages.length]);
 const value:Preview={locale,mode,files,messages,pending,setPending,t,text,setMode,open,notify,
  remove:id=>{setFiles(previous=>previous.map(f=>f.id===id?{...f,deleted:true}:f));setPending(previous=>previous.filter(v=>v!==id));},
  upload:file=>{if(file.url)urls.current.push(file.url);setFiles(previous=>[file,...previous]);},
 };
 return <PreviewContext value={value}><div className="judex-mp-app"><header className="judex-mp-header"><div className="judex-mp-header-brand"><Brand small/><span className="judex-mp-header-divider"/><span>{t("mpProject")}</span><small>{t("mpPrototype")}</small></div><nav aria-label={t("mpPrototype")}>{(["library","plan","task"] as const).map(v=><Button size="sm" key={v} aria-pressed={mode===v} onPress={()=>setMode(v)}>{v==="library"?<Files/>:<MessageCircle/>}{t(v==="library"?"mpLibrary":v==="plan"?"mpPlanChat":"mpTaskChat")}</Button>)}</nav><div className="judex-mp-header-actions"><PreferenceControls/><Button size="sm" isIconOnly aria-label={t("mpReset")} onPress={reset}><RotateCcw/></Button><span className="judex-mp-header-divider"/><PersonAvatar name={locale==="en"?"Lin Xiao":"林晓"}/></div></header>{mode==="library"?<Library/>:<Discussion draft={draft} setDraft={setDraft} onSend={send}/>}<footer className="judex-mp-prototype-note">{t("mpDemoHint")}</footer>{dialog&&<Dialogs dialog={dialog}/>} {notice&&<div className="judex-mp-toast" role="status"><Check/><span>{notice}</span><Button size="sm" isIconOnly aria-label={t("mpClose")} onPress={()=>notify("")}><X/></Button></div>}</div></PreviewContext>;
}
createRoot(document.getElementById("root")!).render(<App/>);
