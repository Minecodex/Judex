import { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Check, LogOut, Moon, Sun } from 'lucide-react';
import { en, zh, type CopyKey } from './copy';
import { seedProjects, seedTopics, type Message, type Project, type Text, type Topic } from './model';
import { Action, DemoContext, DemoDialog, ThemeControls, type Demo, type Page } from './shared';
import { LoginPage } from './Login';
import { ProjectsPage } from './Projects';
import { WorkspacePage } from './Workspace';
import '../src/styles/design-preview.css';

function stored(key: string, fallback: string) {
  try { return localStorage.getItem(key) ?? fallback; } catch { return fallback; }
}
function save(key: string, value: string) {
  try { localStorage.setItem(key, value); } catch { /* file:// can deny browser storage */ }
}
function route() {
  const [page, projectId] = location.hash.slice(1).split('/');
  return { page: (['login', 'workspace'].includes(page) ? page : 'projects') as Page, projectId: projectId || 'judex' };
}

function App() {
  const [locale, setLocale] = useState<'zh-CN' | 'en'>(stored('argus.locale', 'zh-CN') === 'en' ? 'en' : 'zh-CN');
  const [theme, setTheme] = useState<'light' | 'dark'>(stored('judex.design-preview.theme', 'light') === 'dark' ? 'dark' : 'light');
  const [view, setView] = useState(route);
  const [projects, setProjects] = useState<Project[]>(seedProjects);
  const [topics, setTopics] = useState<Topic[]>(seedTopics);
  const [messages, setMessages] = useState<Message[]>([]);
  const [settings, setSettings] = useState(false);
  const [toast, setToast] = useState<{ id: number; message: string } | null>(null);
  const ids = useRef(0);
  const copy = locale === 'en' ? en : zh;
  const t = (key: CopyKey) => copy[key];
  const text = (value: Text) => locale === 'en' ? value.en : value.zh;
  const project = projects.find((value) => value.id === view.projectId) ?? projects[0];
  const notify = (message: string) => setToast({ id: ++ids.current, message });
  const navigate = (page: Page, projectId = project.id) => { location.hash = page === 'workspace' ? `workspace/${projectId}` : page; };

  useEffect(() => { const change = () => setView(route()); window.addEventListener('hashchange', change); return () => window.removeEventListener('hashchange', change); }, []);
  useEffect(() => {
    document.documentElement.lang = locale;
    document.documentElement.dataset.theme = theme;
    document.documentElement.classList.toggle('dark', theme === 'dark');
    document.title = `Judex · ${copy.preview} · ${copy[view.page]}`;
    save('argus.locale', locale); save('judex.design-preview.theme', theme);
  }, [locale, theme, view.page, copy]);
  useEffect(() => { if (!toast) return; const timer = window.setTimeout(() => setToast(null), 3200); return () => window.clearTimeout(timer); }, [toast]);

  const context: Demo = {
    locale, theme, page: view.page, t, text, setLocale, setTheme, navigate,
    projects, project, topics, messages, notify, openSettings: () => setSettings(true),
    createProject: (title, description) => {
      const id = `preview-project-${++ids.current}`;
      setProjects((values) => [...values, { id, title: { zh: title, en: title }, description: { zh: description || zh.newProjectHint, en: description || en.newProjectHint }, role: 'owner', color: 'green', members: 1, discussionCount: 0, pending: 0, fresh: true }]);
      notify(t('projectCreated')); navigate('workspace', id);
    },
    createTopic: (title) => {
      const id = `preview-topic-${++ids.current}`;
      setTopics((values) => [...values, { id, projectId: project.id, title: { zh: title, en: title }, subtitle: { zh: '', en: '' } }]);
      setProjects((values) => values.map((value) => value.id === project.id ? { ...value, discussionCount: value.discussionCount + 1 } : value));
      notify(t('discussCreated')); return id;
    },
    addMessage: (topicId, message, attachments) => {
      setMessages((values) => [...values, { id: `preview-message-${++ids.current}`, topicId, text: message, attachments, time: new Date().toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Shanghai', hour12: false }) }]);
    },
  };

  return <DemoContext.Provider value={context}>
    <div className="judex-demo-root">
      <header className="judex-demo-preview-bar"><div className="judex-demo-preview-label"><span className="judex-demo-preview-symbol">J</span><strong>{t('preview')}</strong><span className="judex-demo-sample-label">{t('samples')}</span></div><nav aria-label={t('preview')} className="judex-demo-preview-nav">{(['login', 'projects', 'workspace'] as const).map((page, index) => <Action key={page} size="sm" onPress={() => navigate(page)} aria-current={view.page === page ? 'page' : undefined} className={view.page === page ? 'judex-demo-preview-active' : ''}><span>0{index + 1}</span>{t(page)}</Action>)}</nav><ThemeControls /></header>
      <div className={`judex-demo-stage judex-demo-stage-${view.page}`}>{view.page === 'login' ? <LoginPage /> : view.page === 'projects' ? <ProjectsPage /> : <WorkspacePage key={project.id} />}</div>
    </div>
    <DemoDialog open={settings} onClose={() => setSettings(false)} title={t('preferences')} description={t('settingsHint')} footer={<><Action onPress={() => { setSettings(false); navigate('login'); }}><LogOut />{t('logout')}</Action><Action variant="primary" onPress={() => setSettings(false)}>{t('close')}</Action></>}>
      <div className="judex-demo-settings-field"><h3>{t('appearance')}</h3><div>{(['light', 'dark'] as const).map((value) => <Action key={value} variant={theme === value ? 'secondary' : 'outline'} aria-pressed={theme === value} onPress={() => setTheme(value)}>{value === 'light' ? <Sun /> : <Moon />}{t(value)}{theme === value && <Check />}</Action>)}</div></div>
      <div className="judex-demo-settings-field"><h3>{t('displayLanguage')}</h3><div>{(['zh-CN', 'en'] as const).map((value) => <Action key={value} variant={locale === value ? 'secondary' : 'outline'} aria-pressed={locale === value} onPress={() => setLocale(value)}>{value === 'zh-CN' ? '简体中文' : 'English'}{locale === value && <Check />}</Action>)}</div></div>
    </DemoDialog>
    {toast && <div className="judex-demo-toast" role="status" aria-live="polite"><Check /><span>{toast.message}</span></div>}
  </DemoContext.Provider>;
}

createRoot(document.getElementById('root')!).render(<App />);
