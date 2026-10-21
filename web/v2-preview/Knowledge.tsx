import { Bot, Check, FileText, Layers3, MessageCircle, Sparkles, Terminal } from 'lucide-react';
import { useState } from 'react';
import { usePreview } from './context';
import { Avatar, Button, Choice, Dialog } from './ui';

export function Knowledge() {
 const p = usePreview(); const group = p.group; if (!group) return null;
 const identities = Object.entries(group.members).flatMap(([personId, roles]) => roles.map(roleId => ({ id: `${personId}:${roleId}`, personId, roleId, name: p.roleLabel(personId, roleId) })));
 const [identity, setIdentity] = useState(identities[0]?.id || '');
 const current = identities.find(i => i.id === identity); const role = p.project.roles.find(r => r.id === current?.roleId);
 const messages = p.state.messages.filter(m => m.roomId === group.id); const tools = messages.filter(m => m.tool);
 const snapshot = messages.length + p.filesInRoom.reduce((total, f) => total + f.versions.length, 0);
 return <Dialog wide title={p.t('vpKnowledge')} description={p.t('vpSharedContext')} footer={<Button variant="primary" onPress={() => p.setModal({ name: null })}>{p.t('vpClose')}</Button>}>
  <div className="judex-v2-context-overview"><div className="judex-v2-context-avatars">{Object.keys(group.members).slice(0, 3).map(id => <Avatar key={id} person={p.people.find(person => person.id === id)} ai/>)}</div><div><strong>{p.t('vpSnapshot')} v{snapshot}</strong><p>{p.t('vpSharedContextHint')}</p></div><span><Layers3/></span></div>
  <div className="judex-v2-knowledge-counts"><div><MessageCircle/><strong>{messages.length}</strong><span>{p.t('vpChatHistory')}</span></div><div><FileText/><strong>{p.filesInRoom.length}</strong><span>{p.t('vpFileKnowledge')}</span></div><div><Terminal/><strong>{tools.length}</strong><span>{p.t('vpToolKnowledge')}</span></div></div>
  <section className="judex-v2-form-section"><h3><Sparkles/>{p.t('vpSummary')}</h3><div className="judex-v2-summary-card"><p>{p.text(group.purpose)}</p><p>{p.t('vpSummaryText')}</p><small>{p.locale === 'en' ? 'Illustrative summary · no model or compression service is running in this demo.' : '示例摘要 · Demo 未运行真实模型或压缩服务。'}</small></div></section>
  {tools.length > 0 && <details className="judex-v2-knowledge-tools"><summary>{p.t('vpToolKnowledge')}<span>{tools.length}</span></summary>{tools.map(message => <div key={message.id}><Terminal/><span><strong>{p.text(p.people.find(person => person.id === message.authorId)?.name)} · {p.text(p.project.roles.find(r => r.id === message.roleId)?.name)}</strong><p>{p.text(message.tool)}</p><small>{message.time} · {p.locale === 'en' ? 'Source: group-visible tool result' : '来源：本群公开工具结果'}</small></span></div>)}</details>}
  <section className="judex-v2-form-section"><h3><Bot/>{p.t('vpPromptLayers')}</h3>{identities.length > 0 && <Choice label={p.t('vpPromptLayers')} value={identity} onChange={setIdentity} items={identities}/>}
   <div className="judex-v2-prompt-layers">{[
    [p.t('vpPromptProject'), p.text(p.project.description)], [p.t('vpPromptGroup'), p.text(group.purpose)], [p.t('vpPromptWorkflow'), group.workflows.map(id => p.project.workflows.find(w => w.id === id)).filter(Boolean).map(w => `${p.text(w?.name)}：${p.text(w?.description)}`).join('\n') || p.t('vpNoWorkflow')], [p.t('vpPromptRole'), p.text(role?.prompt)], [p.t('vpPromptPersonal'), current?.personId === 'me' ? p.state.personalPrompts[`${p.project.id}:me:${current.roleId}`] || p.t('vpNoPersonal') : p.t('vpPersonalPrivate')],
   ].map(([name, value], i) => <div key={name}><span>{String(i + 1).padStart(2, '0')}</span><div><strong>{name}</strong><p>{value}</p></div>{i < 3 && <small><Check/>{p.locale === 'en' ? 'Shared' : '共同背景'}</small>}</div>)}</div>
  </section>
 </Dialog>;
}
