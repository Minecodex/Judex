import {Card} from "@heroui/react";
import {ArrowRight,Settings,UserPlus,Folder,MessageCircle,Users} from "lucide-react";
import {Button} from "../../components/ui/Button";
import {UIStatus} from "../../components/ui/FormControls";
import {PersonAvatar} from "../../components/ui/Presentation";
import {usePreferences} from "../../stores/preferences";
import {translate,type Key} from "../../i18n";
export type ProjectCardData={id:string;title:string;description:string;role:string;memberCount:number;topicCount:number;members:{id:string;name:string}[]};
export function ProjectCards({items,onEnter,onSettings,onInvite}:{items:ProjectCardData[];onEnter:(id:string)=>void;onSettings:(id:string)=>void;onInvite:(id:string)=>void}){
 const {locale}=usePreferences(),t=(key:Key)=>translate(locale,key);
 return <div className="judex-co-project-grid">{items.map(p=><Card key={p.id} className="judex-co-project-card"><Button variant="ghost" className="judex-co-card-hit" aria-label={t("portalEnter")+" · "+p.title} onPress={()=>onEnter(p.id)}/><Card.Header><div className="judex-co-card-top"><span className="judex-co-card-mark"><Folder/></span><UIStatus>{t(p.role==="owner"?"portalRoleOwner":p.role==="manager"?"portalRoleManager":"portalRoleMember")}</UIStatus></div><Card.Title>{p.title}</Card.Title><Card.Description>{p.description||t("portalNoDescription")}</Card.Description></Card.Header><Card.Content><div className="judex-co-meta"><Users/>{p.memberCount}<span>·</span><MessageCircle/>{p.topicCount}</div><div className="judex-avatar-stack">{p.members.slice(0,3).map(m=><PersonAvatar key={m.id} name={m.name} small/>)}</div></Card.Content><Card.Footer><div className="judex-co-card-footer-actions"><Button size="sm" variant="outline" disabled={!["owner","manager"].includes(p.role)} onPress={()=>onInvite(p.id)}><UserPlus/>{t("accountInvite")}</Button><Button size="sm" variant="ghost" onPress={()=>onSettings(p.id)}><Settings/>{t("accountSettings")}</Button></div><Button size="sm" variant="primary" data-testid={"project-enter-"+p.id} onPress={()=>onEnter(p.id)}>{t("coopHub")}<ArrowRight/></Button></Card.Footer></Card>)}</div>;
}
