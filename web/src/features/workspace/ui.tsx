import { Avatar as HeroAvatar, Button as HeroButton, Checkbox, Dropdown, Input, Label, Modal, Select, ListBox, TextArea, TextField, type ButtonProps } from '@heroui/react';
import { Bot, ChevronDown, Coffee, FileText, Hash, Layers3, LayoutTemplate, Leaf, MessageCircle, Orbit, Sparkles, X } from 'lucide-react';
import type { ReactNode } from 'react';
import type {Person} from './types';
import {useWorkspace} from './context';
import {apiUrl,APIError} from '../../lib/api/client';

export function Button({ className = '', variant = 'ghost', size = 'sm', title, ...props }: ButtonProps & { title?: string }) {
 return <HeroButton {...props} aria-label={props['aria-label'] || title} variant={variant} size={size} className={'judex-v2-button ' + className} />;
}
export function IconButton({ label, children, className = '', ...props }: Omit<ButtonProps, 'children'> & { label: string; children: ReactNode }) {
 return <span className="judex-v2-tooltip-anchor" title={label}><Button {...props} isIconOnly aria-label={label} className={'judex-v2-icon-button ' + className}>{children}</Button></span>;
}
export function ActionMenu({label,children,items,className=''}:{label:string;children:ReactNode;items:{id:string;label:string;icon:ReactNode;action:()=>void;danger?:boolean}[];className?:string}){
 return <Dropdown><Button isIconOnly aria-label={label} className={'judex-v2-icon-button '+className}>{children}</Button><Dropdown.Popover className="judex-v2-action-popover" placement="bottom end"><Dropdown.Menu aria-label={label} onAction={key=>items.find(item=>item.id===key)?.action()}>{items.map(item=><Dropdown.Item key={item.id} id={item.id} textValue={item.label} variant={item.danger?'danger':undefined}>{item.icon}<Label>{item.label}</Label></Dropdown.Item>)}</Dropdown.Menu></Dropdown.Popover></Dropdown>;
}
export function Brand({ compact = false }: { compact?: boolean }) {
 return <span className="judex-v2-brand"><span className="judex-v2-brand-mark"><svg viewBox="0 0 32 32" fill="none" aria-hidden="true"><path d="M11 8h12v12a7 7 0 0 1-14 0" stroke="currentColor" strokeWidth="3" strokeLinecap="round"/><path d="M17 8v11a2 2 0 0 1-4 0" stroke="currentColor" strokeWidth="3" strokeLinecap="round"/></svg></span>{!compact && <span>Judex<span className="judex-v2-brand-dot">.</span></span>}</span>;
}
export function Avatar({name='',userId,avatarVersion=0,src,ai=false,small=false,large=false}:{name?:string;userId?:string;avatarVersion?:number;src?:string;ai?:boolean;small?:boolean;large?:boolean}){
 return <span className={`judex-v2-avatar-wrap${small?' judex-v2-avatar-small':''}${large?' judex-v2-avatar-large':''}`}><HeroAvatar aria-label={name} className="judex-v2-avatar judex-v2-tone-olive">{(src||userId&&avatarVersion>0)&&<HeroAvatar.Image alt={name} src={src||apiUrl(`/users/${userId}/avatar?v=${avatarVersion}`)}/>}<HeroAvatar.Fallback>{name.slice(0,2).toUpperCase()}</HeroAvatar.Fallback></HeroAvatar>{ai&&<span className="judex-v2-avatar-badge"><Bot/></span>}</span>;
}
export function GroupAvatar({members=[],name="",large=false}:{members?:Person[];name?:string;large?:boolean}){
 const people=members.slice(0,4);
 if(!people.length)return <GroupIcon large={large}/>;
 return <span role="img" aria-label={name} className={'judex-v2-group-avatar'+(large?' judex-v2-group-avatar-large':'')} data-count={people.length}>
  {people.map(person=><span key={person.userId} aria-hidden="true"><Avatar userId={person.userId} avatarVersion={person.avatarVersion} name={person.displayName}/></span>)}
 </span>;
}
const icons = { flower: Leaf, sparkle: Sparkles, orbit: Orbit, layout: LayoutTemplate, hash: Hash, message: MessageCircle, file: FileText, coffee: Coffee, leaf: Leaf, layers: Layers3 };
export function GroupIcon({ icon = 'hash', tone = 'olive', large = false }: { icon?: string; tone?: string; large?: boolean }) {
 const Icon = icons[icon as keyof typeof icons] || Hash;
 return <span className={`judex-v2-group-icon judex-v2-tone-${tone}${large ? ' judex-v2-group-icon-large' : ''}`}><Icon/></span>;
}
export function Dialog({ title, description, children, footer, wide = false, className='' }: { title: string; description?: string; children: ReactNode; footer?: ReactNode; wide?: boolean;className?:string }) {
 const p = useWorkspace();
 return <Modal.Backdrop isOpen isDismissable onOpenChange={open => { if (!open) p.setModal(null); }} className="judex-v2-backdrop">
  <Modal.Container placement="center" size={wide ? 'lg' : 'md'} className={'judex-v2-modal-container' + (wide ? ' judex-v2-modal-wide' : '')+' '+className}>
   <Modal.Dialog aria-label={title} className="judex-v2-modal"><Modal.CloseTrigger aria-label={p.t('vpClose')} className="judex-v2-modal-close"><X/></Modal.CloseTrigger>
    <Modal.Header><Modal.Heading>{title}</Modal.Heading>{description && <p>{description}</p>}</Modal.Header><Modal.Body>{children}</Modal.Body>{footer && <Modal.Footer>{footer}</Modal.Footer>}
   </Modal.Dialog>
  </Modal.Container>
 </Modal.Backdrop>;
}
export function Field({ label, value, onChange, placeholder, multiline = false, type = 'text', hint, testId, disabled=false, rows=3 }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string; multiline?: boolean; type?: string; hint?: string; testId?: string; disabled?:boolean; rows?:number }) {
 return <TextField isDisabled={disabled} className="judex-v2-field" type={type}><Label>{label}</Label>{multiline ? <TextArea aria-label={label} data-testid={testId} value={value} onChange={e => onChange(e.target.value)} placeholder={placeholder} rows={rows}/> : <Input aria-label={label} data-testid={testId} value={value} onChange={e => onChange(e.target.value)} placeholder={placeholder} type={type}/>} {hint && <small>{hint}</small>}</TextField>;
}
export function CheckField({ selected, onChange, children,disabled=false }: { selected: boolean; onChange: (value: boolean) => void; children: ReactNode;disabled?:boolean }) {
 return <Checkbox isDisabled={disabled} isSelected={selected} onChange={onChange} className="judex-v2-check"><Checkbox.Content><Checkbox.Control><Checkbox.Indicator/></Checkbox.Control><Label>{children}</Label></Checkbox.Content></Checkbox>;
}
export function Choice({ label, value, onChange, items, disabled=false }: { disabled?:boolean; label: string; value: string; onChange: (value: string) => void; items: { id: string; name: string }[] }) {
 return <Select isDisabled={disabled} placeholder={label} aria-label={label} value={value} onChange={key => { if (key != null) onChange(String(key)); }} className="judex-v2-choice"><Select.Trigger><Select.Value/><Select.Indicator><ChevronDown/></Select.Indicator></Select.Trigger><Select.Popover className="judex-v2-choice-popover"><ListBox>{items.map(item => <ListBox.Item key={item.id} id={item.id} textValue={item.name}><Label>{item.name}</Label><ListBox.ItemIndicator/></ListBox.Item>)}</ListBox></Select.Popover></Select>;
}
export function Empty({ title, description, children }: { title: string; description?: string; children?: ReactNode }) {
 return <div className="judex-v2-empty"><span><MessageCircle/></span><h3>{title}</h3>{description && <p>{description}</p>}{children}</div>;
}
type QueryStatus={isPending:boolean;isError:boolean;error:Error|null;refetch:()=>unknown};
export function QueryState({query,empty=false,emptyTitle,children}:{query:QueryStatus;empty?:boolean;emptyTitle?:string;children:ReactNode}){
 const w=useWorkspace();
 if(query.isPending)return <div role="status"><Empty title={w.t('v2Loading')}/></div>;
 if(query.isError){const denied=query.error instanceof APIError&&[401,403,404].includes(query.error.status);return <div role="alert"><Empty title={w.t(denied?'v2AccessUnavailable':'v2Failed')}><Button onPress={()=>{if(denied){w.setModal(null);w.navigate('/');}else void query.refetch();}}>{w.t(denied?'vpBack':'v2Retry')}</Button></Empty></div>;}
 if(empty)return <Empty title={emptyTitle||w.t('vpNoResults')}/>;
 return <>{children}</>;
}
export function ConfigChange({stale,onReload}:{stale:boolean;onReload:()=>void}){
 const w=useWorkspace();return stale?<div className="judex-v2-config-change" role="status"><p>{w.t('v2RemoteChange')}</p><Button onPress={onReload}>{w.t('v2ReloadLatest')}</Button></div>:null;
}
export function Tabs({ items, value, onChange, label }: { items: { id: string; label: string; count?: number }[]; value: string; onChange: (key: string) => void; label: string }) {
 return <div className="judex-v2-tabs" role="group" aria-label={label}>{items.map(item => <Button key={item.id} aria-pressed={value === item.id} className={value === item.id ? 'judex-v2-tab-active' : ''} onPress={() => onChange(item.id)}>{item.label}{!!item.count && <small>{item.count}</small>}</Button>)}</div>;
}
