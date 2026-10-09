import { useState } from 'react';
import { Card, Input, Label, TextField } from '@heroui/react';
import { ArrowRight, ArrowUpRight, Check, Eye, EyeOff, MessageCircle, Sparkles } from 'lucide-react';
import { Action, Badge, Brand, IconAction, PersonAvatar, useDemo } from './shared';

export function LoginPage() {
  const { t, navigate, notify } = useDemo();
  const [mode, setMode] = useState<'login' | 'register' | 'recover'>('login');
  const [reveal, setReveal] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const title = mode === 'login' ? 'welcome' : mode === 'register' ? 'registerTitle' : 'recoverTitle';

  return <main className="judex-demo-login">
    <section className="judex-demo-login-story">
      <Brand />
      <div className="judex-demo-story-content">
        <p className="judex-demo-eyebrow"><span />{t('internal')}</p>
        <h1>{t('brandFirst')}<br /><span>{t('brandSecond')}</span></h1>
        <p className="judex-demo-story-description">{t('brandDescription')}</p>
        <div className="judex-demo-story-art" aria-hidden="true">
          <div className="judex-demo-art-orbit" />
          <Card className="judex-demo-art-conversation">
            <div className="judex-demo-row"><span className="judex-demo-art-topic"><MessageCircle />{t('planTitle')}</span><ArrowUpRight /></div>
            <div className="judex-demo-art-message"><PersonAvatar /><div><strong>{t('proposalTitle')}</strong><span className="judex-demo-skeleton" /><span className="judex-demo-skeleton judex-demo-skeleton-short" /></div></div>
            <div className="judex-demo-art-ai"><Sparkles /><span>{t('proposalHint')}</span></div>
          </Card>
          <Card className="judex-demo-art-decision"><span className="judex-demo-art-check"><Check /></span><div><strong>{t('brandLine')}</strong><p>{t('brandCaption')}</p></div></Card>
        </div>
      </div>
      <p className="judex-demo-story-footer">{t('brandCaption')}</p>
    </section>
    <section className="judex-demo-login-form-area">
      <div className="judex-demo-login-form-wrap">
        <Badge tone="green">{t('internal')}</Badge>
        <h2>{t(title)}</h2>
        <p className="judex-demo-login-subtitle">{t('loginHint')}</p>
        <form className="judex-demo-form" onSubmit={(event) => {
          event.preventDefault();
          if (mode === 'recover') { notify(t('resetSent')); return; }
          notify(t(mode === 'register' ? 'registered' : 'loginToast'));
          navigate('projects');
        }}>
          {mode === 'register' && <TextField isRequired value={name} onChange={setName} name="name"><Label>{t('name')}</Label><Input placeholder={t('namePlaceholder')} autoComplete="off" /></TextField>}
          <TextField isRequired type="email" name="email" value={email} onChange={setEmail}><Label>{t('email')}</Label><Input placeholder={t('emailPlaceholder')} autoComplete="off" /></TextField>
          {mode !== 'recover' && <TextField isRequired name="password" type={reveal ? 'text' : 'password'} value={password} onChange={setPassword}>
            <div className="judex-demo-field-heading"><Label>{t('password')}</Label>{mode === 'login' && <Action size="sm" className="judex-demo-text-link" onPress={() => setMode('recover')}>{t('forgot')}</Action>}</div>
            <div className="judex-demo-password"><Input placeholder={t('passwordPlaceholder')} autoComplete="off" /><IconAction label={t(reveal ? 'hidePassword' : 'showPassword')} onPress={() => setReveal(!reveal)}>{reveal ? <EyeOff /> : <Eye />}</IconAction></div>
          </TextField>}
          <Action type="submit" variant="primary" size="lg" fullWidth>{t(mode === 'login' ? 'login' : mode === 'register' ? 'register' : 'sendReset')}<ArrowRight /></Action>
        </form>
        <div className="judex-demo-login-registration">{mode === 'login' ? <>{t('noAccount')}<Action className="judex-demo-text-link" onPress={() => setMode('register')}>{t('register')}</Action></> : <Action className="judex-demo-text-link" onPress={() => setMode('login')}>{t('backLogin')}</Action>}</div>
        <div className="judex-demo-login-divider"><span />Judex<span /></div>
        <Action variant="outline" fullWidth onPress={() => navigate('projects')}>{t('loginDemo')}<ArrowUpRight /></Action>
        <p className="judex-demo-login-note">{t('loginNote')}</p>
      </div>
      <p className="judex-demo-login-footer">{t('footer')}</p>
    </section>
  </main>;
}
