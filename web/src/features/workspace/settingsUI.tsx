import type {ReactNode} from 'react';
import {ArrowLeft} from 'lucide-react';
import {Button} from './ui';

export function SettingsHeading({title,description,actions}:{title:string;description?:string;actions?:ReactNode}){
 return <header className="judex-settings-heading"><div><h2>{title}</h2>{description&&<p>{description}</p>}</div>{actions&&<div className="judex-settings-actions">{actions}</div>}</header>;
}
export function SettingsSection({title,description,children}:{title:string;description?:string;children:ReactNode}){
 return <section className="judex-settings-section"><div className="judex-settings-section-heading"><h3>{title}</h3>{description&&<p>{description}</p>}</div><div className="judex-v2-form-stack">{children}</div></section>;
}
export function BackButton({label,onPress}:{label:string;onPress:()=>void}){return <Button className="judex-settings-back" onPress={onPress}><ArrowLeft/>{label}</Button>;}
export function Badge({children,accent=false}:{children:ReactNode;accent?:boolean}){return <span className={'judex-settings-badge'+(accent?' judex-settings-badge-accent':'')}>{children}</span>;}
