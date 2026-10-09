import {Dropdown,Label} from "@heroui/react";
import {ArrowLeft,Check,ChevronDown,LogOut,Moon,Sun} from "lucide-react";
import type {ReactNode} from "react";
import {Button} from "../src/components/ui/Button";
import {Brand,PersonAvatar,UIDialog} from "../src/components/ui/Presentation";
import {UIStatus} from "../src/components/ui/FormControls";
import {usePreview} from "./context";
import type {CooperationKey} from "../src/i18n/cooperationPreview";
export {Button,PersonAvatar};
export function Status({status}:{status:string}){
 const {t}=usePreview();
 const key:CooperationKey=status==="draft"?"cpDraft":status==="ready"?"cpReady":status==="blocked"?"cpBlocked":status==="delivered"?"cpDelivered":status==="accepted"?"cpAccepted":status==="rework"?"cpRework":status==="cancelled"?"cpCancelled":"cpWorking";
 return <UIStatus className={status==="accepted"?"judex-co-status-good":["draft","blocked","delivered","rework"].includes(status)?"judex-co-status-warn":""}>{t(key)}</UIStatus>;
}
export function Header(){
 const p=usePreview(),project=p.state.projects.find(v=>v.id===p.route.projectId),user=p.state.people.find(v=>v.id===p.state.userId)!;
 const select=(key:string)=>{if(key==="theme")p.setTheme(p.theme==="dark"?"light":"dark");else if(key==="language")p.setLocale(p.locale==="en"?"zh-CN":"en");else if(key==="logout")p.logout();else if(key==="reset")p.reset();else if(key.startsWith("person:"))p.perform({kind:"user",userId:key.slice(7)});};
 return <header className="judex-co-header">
  <div className="judex-co-header-left"><Button className="judex-co-brand" onPress={()=>p.go({page:"projects",projectId:"launch"})}><Brand small/></Button><span className="judex-co-divider"/>{p.route.page==="projects"?<span>{p.t("cpProjects")}</span>:<Button size="sm" onPress={()=>p.go({page:"hub",projectId:project?.id??"launch",tab:"plans"})}>{p.text(project?.title??"")}</Button>}<span className="judex-co-prototype" title={p.t("cpPreviewHint")}>{p.t("cpPreview")}</span></div>
  <Dropdown><Button data-testid="co-account" className="judex-co-account"><PersonAvatar name={p.text(user.name)}/><span>{p.text(user.name)}</span><ChevronDown size={14}/></Button><Dropdown.Popover><Dropdown.Menu aria-label={p.t("cpUser")} onAction={key=>select(String(key))}>
   <Dropdown.Item id="language" textValue={p.t("cpLanguage")}><Label>{p.locale==="en"?"中文":"English"}</Label></Dropdown.Item>
   <Dropdown.Item id="theme" textValue={p.t("cpTheme")}>{p.theme==="dark"?<Sun/>:<Moon/>}<Label>{p.t("cpTheme")}</Label></Dropdown.Item>
   <Dropdown.Section aria-label={p.t("cpExperience")}>{p.state.people.slice(0,4).map(person=><Dropdown.Item id={"person:"+person.id} key={person.id} textValue={p.text(person.name)}><Label>{p.t("cpExperience")} · {p.text(person.name)}</Label>{p.state.userId===person.id&&<Check/>}</Dropdown.Item>)}</Dropdown.Section>
   <Dropdown.Item id="reset" textValue={p.t("cpReset")}><Label>{p.t("cpReset")}</Label></Dropdown.Item>
   <Dropdown.Item id="logout" textValue={p.t("cpLogout")}><LogOut/><Label>{p.t("cpLogout")}</Label></Dropdown.Item>
  </Dropdown.Menu></Dropdown.Popover></Dropdown>
 </header>;
}
export function PageTitle({title,hint,actions,back}:{title:string;hint?:string;actions?:ReactNode;back?:{label:string;run:()=>void}}){
 return <div className="judex-co-page-heading">{back&&<Button size="sm" className="judex-co-back" onPress={back.run}><ArrowLeft size={14}/>{back.label}</Button>}<div><div><h1>{title}</h1>{hint&&<p>{hint}</p>}</div>{actions&&<div className="judex-co-actions">{actions}</div>}</div></div>;
}
export function Modal({title,children,onClose,wide=true,description}:{title:string;children:ReactNode;onClose:()=>void;wide?:boolean;description?:string}){
 return <UIDialog title={title} description={description} onClose={onClose} wide={wide}>{children}</UIDialog>;
}
export function Empty({children}:{children:ReactNode}){return <div className="judex-co-empty">{children}</div>;}
