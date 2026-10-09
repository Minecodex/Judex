import {useState,type ReactNode} from "react";
import {Popover} from "@heroui/react";
import {ChevronDown,FolderOpen,LogOut,Shield,Globe,Sun,Moon} from "lucide-react";
import {Button} from "../../components/ui/Button";
import {Brand,PersonAvatar} from "../../components/ui/Presentation";
import {UIWarning} from "../../components/ui/FormControls";
import {usePreferences} from "../../stores/preferences";
import {translate} from "../../i18n";
export function PortalHeader({name,onLogout,onProjects,onSecurity,crumbs=[],demoControls}:{name:string;onLogout:()=>Promise<void>;onProjects?:()=>void;onSecurity?:()=>void;crumbs?:{title:string;onPress?:()=>void}[];demoControls?:ReactNode}){
 const {locale,theme,setLocale,setTheme}=usePreferences(),t=(key:Parameters<typeof translate>[1])=>translate(locale,key);
 const [open,setOpen]=useState(false),[pending,setPending]=useState(false),[error,setError]=useState(false);
 return <header className="judex-co-header"><div className="judex-co-header-left"><Button variant="ghost" className="judex-co-brand" onPress={onProjects} aria-label={t("portalProjects")}><Brand small/></Button>{crumbs.length>0&&<span className="judex-co-divider"/>}{crumbs.map((c,i)=><Button key={i} variant="ghost" size="sm" className="judex-co-crumb" onPress={c.onPress}>{c.title}</Button>)}</div><Popover isOpen={open} onOpenChange={setOpen}><Button className="judex-co-account" variant="ghost" data-testid="account-menu-button" aria-label={t("accountMenu")}><PersonAvatar name={name}/><strong>{name}</strong><ChevronDown/></Button><Popover.Content placement="bottom end" className="judex-account-popover"><Popover.Dialog aria-label={t("accountMenu")} className="judex-account-menu">
  {onProjects&&<Button variant="ghost" data-testid="account-projects" onPress={()=>{setOpen(false);onProjects();}}><FolderOpen/>{t("portalProjects")}</Button>}
  <Button variant="ghost" data-testid="next-language" onPress={()=>setLocale(locale==="en"?"zh-CN":"en")}><Globe/>{t("accountLanguage")} · {locale==="en"?"English":"简体中文"}</Button>
  <Button variant="ghost" data-testid="next-theme" onPress={()=>setTheme(theme==="dark"?"light":"dark")}>{theme==="dark"?<Moon/>:<Sun/>}{t("accountTheme")} · {t(theme==="dark"?"accountDark":"accountLight")}</Button>
  {onSecurity&&<Button variant="ghost" onPress={()=>{setOpen(false);onSecurity();}}><Shield/>{t("portalAccount")}</Button>}
  {demoControls}<Button variant="ghost" data-testid="account-logout" isPending={pending} onPress={async()=>{setPending(true);try{await onLogout();}catch{setPending(false);setError(true);}}}><LogOut/>{t("accountLogout")}</Button>{error&&<UIWarning>{t("accountLogoutFailed")}</UIWarning>}
 </Popover.Dialog></Popover.Content></Popover></header>;
}
