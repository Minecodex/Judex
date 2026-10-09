import { Avatar, Card, Modal } from '@heroui/react';
import { ArrowUpRight, Check, MessageCircle, Moon, Sparkles, Sun } from 'lucide-react';
import type { ReactNode } from 'react';
import { Button } from './Button';
import { ActionGroup } from './ActionGroup';
import { usePreferences } from '../../stores/preferences';
import { translate } from '../../i18n';

export function Brand({ small = false }: { small?: boolean }) {
  return <span className={'judex-brand' + (small ? ' judex-brand-small' : '')}>
    <span className="judex-brand-mark" aria-hidden="true"><svg viewBox="0 0 32 32" fill="none"><path d="M12 9h9v11a6 6 0 0 1-12 0" stroke="currentColor" strokeWidth="2.7" strokeLinecap="round"/><path d="M17 9v10a2 2 0 0 1-4 0" stroke="currentColor" strokeWidth="2.7" strokeLinecap="round"/></svg></span>
    <span>Judex<span className="judex-brand-dot">.</span></span>
  </span>;
}
export function PersonAvatar({ name, tone = 'green', small = false }: { name: string; tone?: 'green' | 'violet' | 'sand'; small?: boolean }) {
  const initials = name.trim().split(/\s+/).slice(0,2).map((part) => Array.from(part)[0] ?? '').join('').toUpperCase();
  return <Avatar size="sm" aria-label={name} className={`judex-person-avatar judex-color-${tone}${small ? ' judex-person-avatar-small' : ''}`}><Avatar.Fallback>{initials || 'J'}</Avatar.Fallback></Avatar>;
}
export function PageHeading({ title, description, eyebrow, children, className = '' }: { title: string; description?: string; eyebrow?: string; children?: ReactNode; className?: string }) {
  return <div className={`judex-page-heading ${className}`}><div>{eyebrow && <p className="judex-eyebrow">{eyebrow}</p>}<h1>{title}</h1>{description && <p>{description}</p>}</div>{children && <ActionGroup className="judex-page-actions">{children}</ActionGroup>}</div>;
}
export function EmptyState({ title, description, icon = <MessageCircle />, children }: { title: string; description?: string; icon?: ReactNode; children?: ReactNode }) {
  return <div className="judex-empty-state"><span className="judex-empty-icon">{icon}</span>{title && <h2>{title}</h2>}{description && <p>{description}</p>}{children}</div>;
}
export function UIDialog({ title, description, onClose, children, footer, wide = false, fullscreen = false, dismissable = true }: { title: string; description?: string; onClose: () => void; children: ReactNode; footer?: ReactNode; wide?: boolean; fullscreen?: boolean; dismissable?: boolean }) {
  const { locale } = usePreferences();
  return <Modal.Backdrop isOpen isDismissable={dismissable} isKeyboardDismissDisabled={!dismissable} onOpenChange={(open) => { if (!open && dismissable) onClose(); }} className="judex-overlay">
    <Modal.Container placement="center" size={fullscreen ? 'full' : wide ? 'lg' : 'md'} className={'judex-dialog-container' + (wide ? ' judex-dialog-container-wide' : '') + (fullscreen ? ' judex-dialog-container-full' : '')}>
      <Modal.Dialog aria-label={title} className="judex-dialog-content"><Modal.CloseTrigger isDisabled={!dismissable} aria-label={translate(locale,'close')} />
        <Modal.Header className="judex-dialog-header"><Modal.Heading>{title}</Modal.Heading>{description && <p>{description}</p>}</Modal.Header>
        <Modal.Body className="judex-dialog-body">{children}</Modal.Body>{footer && <Modal.Footer className="judex-dialog-footer">{footer}</Modal.Footer>}
      </Modal.Dialog>
    </Modal.Container>
  </Modal.Backdrop>;
}
export function PreferenceControls() {
  const { locale, theme, setLocale, setTheme } = usePreferences();
  return <div className="judex-preference-controls"><Button size="sm" aria-label={translate(locale,'language')} onPress={() => setLocale(locale === 'en' ? 'zh-CN' : 'en')}>{locale === 'en' ? '中文' : 'EN'}</Button><Button size="sm" isIconOnly aria-label={translate(locale,'theme')} onPress={() => setTheme(theme === 'dark' ? 'light' : 'dark')}>{theme === 'dark' ? <Sun /> : <Moon />}</Button></div>;
}
export function AuthStory() {
  const { locale } = usePreferences();
  const t = (key: Parameters<typeof translate>[1]) => translate(locale,key);
  return <section className="judex-entry-story"><Brand /><div className="judex-entry-story-content"><p className="judex-eyebrow"><span className="judex-live-dot" />{t('portalSpace')}</p><h1>{t('portalBrandFirst')}<br/><span>{t('portalBrandSecond')}</span></h1><p className="judex-entry-story-description">{t('portalBrandDescription')}</p>
    <div className="judex-entry-art" aria-hidden="true"><div className="judex-entry-art-orbit" /><Card className="judex-entry-art-conversation"><div className="judex-entry-art-top"><span><MessageCircle />{t('portalExampleTopic')}</span><ArrowUpRight /></div><div className="judex-entry-art-message"><PersonAvatar name="Judex"/><div><strong>{t('portalExampleProposal')}</strong><span className="judex-entry-art-line"/><span className="judex-entry-art-line judex-entry-art-line-short"/></div></div><div className="judex-entry-art-ai"><Sparkles />{t('portalExampleHint')}</div></Card><Card className="judex-entry-art-decision"><span><Check /></span><div><strong>{t('portalBrandLine')}</strong><p>{t('portalBrandCaption')}</p></div></Card></div>
    </div><p className="judex-entry-story-footer">{t('portalBrandCaption')}</p></section>;
}
