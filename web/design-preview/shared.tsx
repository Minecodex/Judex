import { createContext, useContext, type ComponentProps, type ReactNode } from 'react';
import { Avatar, Button, Chip, Modal } from '@heroui/react';
import { ArrowUpRight, Moon, Sun } from 'lucide-react';
import type { CopyKey } from './copy';
import type { Message, Project, Text, Topic } from './model';

export type Page = 'login' | 'projects' | 'workspace';
export type Demo = {
  locale: 'zh-CN' | 'en'; theme: 'light' | 'dark'; page: Page;
  t: (key: CopyKey) => string; text: (value: Text) => string;
  setLocale: (locale: 'zh-CN' | 'en') => void; setTheme: (theme: 'light' | 'dark') => void;
  navigate: (page: Page, projectId?: string) => void;
  projects: Project[]; project: Project; topics: Topic[]; messages: Message[];
  createProject: (title: string, description: string) => void;
  createTopic: (title: string) => string;
  addMessage: (topicId: string, text: string, attachments: string[]) => void;
  notify: (message: string) => void; openSettings: () => void;
};
export const DemoContext = createContext<Demo | null>(null);
export function useDemo() {
  const value = useContext(DemoContext);
  if (!value) throw new Error('Design preview context is missing');
  return value;
}

export function Action({ className = '', variant = 'ghost', ...props }: ComponentProps<typeof Button>) {
  return <Button {...props} variant={variant} className={`judex-demo-button ${className}`} />;
}
export function IconAction({ label, children, ...props }: Omit<ComponentProps<typeof Button>, 'children'> & { label: string; children: ReactNode }) {
  return <Action {...props} isIconOnly aria-label={label}>{children}</Action>;
}
export function Brand({ small = false }: { small?: boolean }) {
  return <span className={`judex-demo-brand ${small ? 'judex-demo-brand-small' : ''}`}>
    <span className="judex-demo-logomark" aria-hidden="true">
      <svg viewBox="0 0 32 32" fill="none"><path d="M12 9h9v11a6 6 0 0 1-12 0" stroke="currentColor" strokeWidth="2.7" strokeLinecap="round" /><path d="M17 9v10a2 2 0 0 1-4 0" stroke="currentColor" strokeWidth="2.7" strokeLinecap="round" /></svg>
    </span><span>Judex<span className="judex-demo-brand-dot">.</span></span>
  </span>;
}
export function PersonAvatar({ initials = 'LX', color = 'green', small = false }: { initials?: string; color?: string; small?: boolean }) {
  return <Avatar size="sm" className={`judex-demo-avatar judex-demo-tone-${color} ${small ? 'judex-demo-avatar-small' : ''}`}>
    <Avatar.Fallback>{initials}</Avatar.Fallback>
  </Avatar>;
}
export function Badge({ children, tone = 'neutral' }: { children: ReactNode; tone?: 'green' | 'amber' | 'neutral' }) {
  return <Chip size="sm" variant="soft" className={`judex-demo-badge judex-demo-badge-${tone}`}><Chip.Label>{children}</Chip.Label></Chip>;
}
export function ThemeControls() {
  const { locale, theme, t, setLocale, setTheme } = useDemo();
  return <div className="judex-demo-actions">
    <Action size="sm" aria-label={t('language')} onPress={() => setLocale(locale === 'zh-CN' ? 'en' : 'zh-CN')}>{locale === 'zh-CN' ? 'EN' : '中文'}</Action>
    <IconAction size="sm" label={t('theme')} onPress={() => setTheme(theme === 'light' ? 'dark' : 'light')}>
      {theme === 'light' ? <Moon /> : <Sun />}
    </IconAction>
  </div>;
}
export function DemoDialog({ open, onClose, title, description, children, footer, wide = false }: {
  open: boolean; onClose: () => void; title: string; description?: string;
  children: ReactNode; footer?: ReactNode; wide?: boolean;
}) {
  const { t } = useDemo();
  return <Modal.Backdrop isOpen={open} onOpenChange={(value) => !value && onClose()}>
    <Modal.Container placement="center" size={wide ? 'lg' : 'md'}>
      <Modal.Dialog className="judex-demo-dialog">
        <Modal.CloseTrigger aria-label={t('close')} />
        <Modal.Header><Modal.Heading>{title}</Modal.Heading>{description && <p>{description}</p>}</Modal.Header>
        <Modal.Body>{children}</Modal.Body>
        {footer && <Modal.Footer>{footer}</Modal.Footer>}
      </Modal.Dialog>
    </Modal.Container>
  </Modal.Backdrop>;
}
export function EmptyState({ title, description, children, icon }: { title: string; description: string; children?: ReactNode; icon: ReactNode }) {
  return <div className="judex-demo-empty"><div className="judex-demo-empty-icon">{icon}</div><h2>{title}</h2><p>{description}</p>{children}</div>;
}
export function TextLink({ children, onPress }: { children: ReactNode; onPress: () => void }) {
  return <Action className="judex-demo-text-link" size="sm" onPress={onPress}>{children}<ArrowUpRight /></Action>;
}
