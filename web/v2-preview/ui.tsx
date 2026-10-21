import { Avatar as HeroAvatar, Button as HeroButton, Checkbox, Input, Label, Modal, Select, ListBox, TextArea, TextField, type ButtonProps } from '@heroui/react';
import { Bot, Check, ChevronDown, Coffee, FileText, Hash, Layers3, LayoutTemplate, Leaf, MessageCircle, Orbit, Sparkles, X } from 'lucide-react';
import type { ReactNode } from 'react';
import type { Person } from './model';
import { usePreview } from './context';

export function Button({ className = '', variant = 'ghost', size = 'sm', title, ...props }: ButtonProps & { title?: string }) {
 return <HeroButton {...props} aria-label={props['aria-label'] || title} variant={variant} size={size} className={'judex-v2-button ' + className} />;
}
export function IconButton({ label, children, className = '', ...props }: Omit<ButtonProps, 'children'> & { label: string; children: ReactNode }) {
 return <span className="judex-v2-tooltip-anchor" title={label}><Button {...props} isIconOnly aria-label={label} className={'judex-v2-icon-button ' + className}>{children}</Button></span>;
}
export function Brand({ compact = false }: { compact?: boolean }) {
 return <span className="judex-v2-brand"><span className="judex-v2-brand-mark"><svg viewBox="0 0 32 32" fill="none" aria-hidden="true"><path d="M11 8h12v12a7 7 0 0 1-14 0" stroke="currentColor" strokeWidth="3" strokeLinecap="round"/><path d="M17 8v11a2 2 0 0 1-4 0" stroke="currentColor" strokeWidth="3" strokeLinecap="round"/></svg></span>{!compact && <span>Judex<span className="judex-v2-brand-dot">.</span></span>}</span>;
}
export function Avatar({ person, ai = false, small = false, large = false, online = false }: { person?: Person; ai?: boolean; small?: boolean; large?: boolean; online?: boolean }) {
 const p = usePreview();
 return <span className={`judex-v2-avatar-wrap${small ? ' judex-v2-avatar-small' : ''}${large ? ' judex-v2-avatar-large' : ''}`}>
  <HeroAvatar aria-label={p.text(person?.name)} className={'judex-v2-avatar judex-v2-tone-' + (person?.tone || 'olive')}>
   <HeroAvatar.Fallback><svg viewBox="0 0 64 64" fill="none" aria-hidden="true"><rect width="64" height="64" fill="var(--avatar-bg)"/><path d="M8 64c1-16 10-23 24-23s23 7 24 23" fill="var(--avatar-shirt)"/><path d="M26 36h12v12c-4 5-8 5-12 0" fill="var(--avatar-skin)"/><ellipse cx="32" cy="26" rx="15" ry="18" fill="var(--avatar-hair)"/><ellipse cx="32" cy="29" rx="12" ry="16" fill="var(--avatar-skin)"/><path d={['tang', 'xu', 'he'].includes(person?.id || '') ? 'M17 32C12 9 27 5 38 11c11 5 11 17 9 29l-5 7-2-24c-6 1-13-1-17-6l-3 23z' : 'M18 24C14 9 28 4 39 10c7 4 9 10 6 18l-5-10c-8 5-14 4-20 3l-2 8z'} fill="var(--avatar-hair)"/><path d="M26 29h1m10 0h1" stroke="var(--avatar-hair)" strokeWidth="2.4" strokeLinecap="round"/><path d="M29 37c2 2 4 2 6 0" stroke="var(--avatar-lip)" strokeWidth="1.5" strokeLinecap="round"/><path d="M32 29l-1 5h2" stroke="var(--avatar-lip)" strokeWidth="1" opacity=".45"/></svg></HeroAvatar.Fallback>
  </HeroAvatar>{ai ? <span className="judex-v2-avatar-badge"><Bot/></span> : online && person?.online ? <span className="judex-v2-online-dot"/> : null}
 </span>;
}
const icons = { flower: Leaf, sparkle: Sparkles, orbit: Orbit, layout: LayoutTemplate, hash: Hash, message: MessageCircle, file: FileText, coffee: Coffee, leaf: Leaf, layers: Layers3 };
export function GroupIcon({ icon = 'hash', tone = 'olive', large = false }: { icon?: string; tone?: string; large?: boolean }) {
 const Icon = icons[icon as keyof typeof icons] || Hash;
 return <span className={`judex-v2-group-icon judex-v2-tone-${tone}${large ? ' judex-v2-group-icon-large' : ''}`}><Icon/></span>;
}
export function Dialog({ title, description, children, footer, wide = false }: { title: string; description?: string; children: ReactNode; footer?: ReactNode; wide?: boolean }) {
 const p = usePreview();
 return <Modal.Backdrop isOpen isDismissable onOpenChange={open => { if (!open) p.setModal({ name: null }); }} className="judex-v2-backdrop">
  <Modal.Container placement="center" size={wide ? 'lg' : 'md'} className={'judex-v2-modal-container' + (wide ? ' judex-v2-modal-wide' : '')}>
   <Modal.Dialog aria-label={title} className="judex-v2-modal"><Modal.CloseTrigger aria-label={p.t('vpClose')} className="judex-v2-modal-close"><X/></Modal.CloseTrigger>
    <Modal.Header><Modal.Heading>{title}</Modal.Heading>{description && <p>{description}</p>}</Modal.Header><Modal.Body>{children}</Modal.Body>{footer && <Modal.Footer>{footer}</Modal.Footer>}
   </Modal.Dialog>
  </Modal.Container>
 </Modal.Backdrop>;
}
export function Field({ label, value, onChange, placeholder, multiline = false, type = 'text', hint, testId }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string; multiline?: boolean; type?: string; hint?: string; testId?: string }) {
 return <TextField className="judex-v2-field" type={type}><Label>{label}</Label>{multiline ? <TextArea aria-label={label} data-testid={testId} value={value} onChange={e => onChange(e.target.value)} placeholder={placeholder} rows={3}/> : <Input aria-label={label} data-testid={testId} value={value} onChange={e => onChange(e.target.value)} placeholder={placeholder} type={type}/>} {hint && <small>{hint}</small>}</TextField>;
}
export function CheckField({ selected, onChange, children }: { selected: boolean; onChange: (value: boolean) => void; children: ReactNode }) {
 return <Checkbox isSelected={selected} onChange={onChange} className="judex-v2-check"><Checkbox.Content><Checkbox.Control><Checkbox.Indicator><Check/></Checkbox.Indicator></Checkbox.Control><Label>{children}</Label></Checkbox.Content></Checkbox>;
}
export function Choice({ label, value, onChange, items }: { label: string; value: string; onChange: (value: string) => void; items: { id: string; name: string }[] }) {
 return <Select aria-label={label} value={value} onChange={key => { if (key != null) onChange(String(key)); }} className="judex-v2-choice"><Select.Trigger><Select.Value/><Select.Indicator><ChevronDown/></Select.Indicator></Select.Trigger><Select.Popover className="judex-v2-choice-popover"><ListBox>{items.map(item => <ListBox.Item key={item.id} id={item.id} textValue={item.name}><Label>{item.name}</Label><ListBox.ItemIndicator/></ListBox.Item>)}</ListBox></Select.Popover></Select>;
}
export function Empty({ title, description, children }: { title: string; description?: string; children?: ReactNode }) {
 return <div className="judex-v2-empty"><span><MessageCircle/></span><h3>{title}</h3>{description && <p>{description}</p>}{children}</div>;
}
export function Tabs({ items, value, onChange, label }: { items: { id: string; label: string; count?: number }[]; value: string; onChange: (key: string) => void; label: string }) {
 return <div className="judex-v2-tabs" role="group" aria-label={label}>{items.map(item => <Button key={item.id} aria-pressed={value === item.id} className={value === item.id ? 'judex-v2-tab-active' : ''} onPress={() => onChange(item.id)}>{item.label}{!!item.count && <small>{item.count}</small>}</Button>)}</div>;
}
